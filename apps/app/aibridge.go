package app

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/glitchedgitz/pocketbase/apis"
	"github.com/glitchedgitz/pocketbase/core"
	"github.com/labstack/echo/v5"
)

// ---------------------------------------------------------------------------
// AI CLI bridges
//
// Claude Code and Codex run as local CLI subprocesses, so they can't be driven
// from the browser. Each has a small Node HTTP server (the "bridge") that wraps
// the vendor SDK and speaks the AI SDK UI message stream — the same shape the
// frontend's other transports already consume.
//
// grroxy owns their lifecycle so the frontend never has to know a port: bridges
// are spawned lazily on first use, bound to loopback on an OS-assigned port,
// gated behind a per-run token, and reverse-proxied under /api/aibridge/. That
// makes bridge traffic same-origin with the rest of the API, so the frontend
// derives the URL from its own base URL exactly like it does for MCP.
// ---------------------------------------------------------------------------

// How long to wait for a freshly spawned bridge to answer /health. Codex's first
// run can be slow because the SDK unpacks its platform binary.
const bridgeStartupTimeout = 45 * time.Second

type bridgeSpec struct {
	key     string // URL segment: "claude" | "codex"
	dirName string // directory name shipped alongside the binary
	label   string // log prefix
}

var bridgeSpecs = []bridgeSpec{
	{key: "claude", dirName: "claude-code-bridge", label: "Claude Code"},
	{key: "codex", dirName: "codex-bridge", label: "Codex"},
}

type bridgeProc struct {
	spec bridgeSpec

	mu   sync.Mutex
	cmd  *exec.Cmd
	dir  string
	port int
	// Write end of the child's stdin. We never write to it — holding it open is
	// the signal that we're alive. Closing it (or dying, which closes it for us)
	// is how the child learns to exit. See bridgeExitOnParentEOF.
	stdinW  *os.File
	proxy   *httputil.ReverseProxy
	lastErr string
	// Set by an explicit stop, cleared by an explicit start. Without it, the next
	// chat request would lazily respawn what the user just shut down.
	stopped bool
}

// AIBridge supervises the per-provider Node bridges.
type AIBridge struct {
	token  string
	procs  map[string]*bridgeProc
	mcpURL func() string
}

func NewAIBridge(mcpURL func() string) *AIBridge {
	token, err := bridgeToken()
	if err != nil {
		// A bridge with no token still works; it just loses the local-process
		// guard. Better to run degraded than to fail startup outright.
		log.Printf("[AIBridge] Could not generate token, running unauthenticated: %v", err)
	}

	procs := make(map[string]*bridgeProc, len(bridgeSpecs))
	for _, spec := range bridgeSpecs {
		procs[spec.key] = &bridgeProc{spec: spec}
	}
	return &AIBridge{token: token, procs: procs, mcpURL: mcpURL}
}

func bridgeToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// ---------------------------------------------------------------------------
// Locating the bridge directories
// ---------------------------------------------------------------------------

// bridgeSearchRoots returns the directories that may contain the bridge folders,
// most specific first. Packaged builds ship them next to the binary; a dev
// checkout finds them in the sibling cybernetic-ui repo.
func bridgeSearchRoots() []string {
	var roots []string

	if custom := os.Getenv("GRROXY_BRIDGES_DIR"); custom != "" {
		roots = append(roots, custom)
	}

	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		exeDir := filepath.Dir(exe)
		// Release archive: bridges/ sits next to the binaries.
		roots = append(roots, filepath.Join(exeDir, "bridges"))
		// Electron: binaries land in <resources>/bin, extraResources in <resources>/bridges.
		roots = append(roots, filepath.Join(exeDir, "..", "bridges"))
	}

	// Dev checkout: walk up from the cwd looking for the sibling frontend repo.
	if cwd, err := os.Getwd(); err == nil {
		dir := cwd
		for i := 0; i < 6; i++ {
			roots = append(roots, filepath.Join(dir, "cybernetic-ui"))
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	return roots
}

// resolveDir finds the bridge directory, or reports where it looked.
func (b *bridgeProc) resolveDir() (string, error) {
	roots := bridgeSearchRoots()
	for _, root := range roots {
		candidate := filepath.Join(root, b.spec.dirName)
		if _, err := os.Stat(filepath.Join(candidate, "server.mjs")); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%s bridge not found; looked for %s/server.mjs under: %v",
		b.spec.label, b.spec.dirName, roots)
}

// nodeBinary returns the Node executable to run the bridge with. Electron sets
// GRROXY_NODE_BIN to its own binary (with ELECTRON_RUN_AS_NODE) so packaged
// installs don't need a system Node.
func nodeBinary() (string, error) {
	if custom := os.Getenv("GRROXY_NODE_BIN"); custom != "" {
		return custom, nil
	}
	name := "node"
	if runtime.GOOS == "windows" {
		name = "node.exe"
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("node not found on PATH; install Node 18+ or set GRROXY_NODE_BIN")
	}
	return path, nil
}

// freePort asks the OS for an unused loopback port. The port is released before
// the child binds it, so this races in principle; in practice the window is tiny
// and a failed bind surfaces as a startup error we report.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

// running reports whether the child is alive, clearing state if it exited.
func (b *bridgeProc) running() bool {
	if b.cmd == nil || b.cmd.Process == nil {
		return false
	}
	if b.cmd.ProcessState != nil && b.cmd.ProcessState.Exited() {
		return false
	}
	return true
}

// ensure starts the bridge if it isn't already up and waits for /health.
func (b *bridgeProc) ensure(token, mcpURL string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.running() {
		return nil
	}

	if b.stopped {
		return fmt.Errorf("%s bridge is stopped; start it from Settings to use it", b.spec.label)
	}

	dir, err := b.resolveDir()
	if err != nil {
		b.lastErr = err.Error()
		return err
	}

	node, err := nodeBinary()
	if err != nil {
		b.lastErr = err.Error()
		return err
	}

	port, err := freePort()
	if err != nil {
		b.lastErr = err.Error()
		return err
	}

	// The child watches this pipe to detect our death. A nil Stdin would give it
	// /dev/null, which reads as immediate EOF, so the pipe has to be explicit.
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		b.lastErr = err.Error()
		return fmt.Errorf("failed to create %s bridge stdin pipe: %w", b.spec.label, err)
	}

	cmd := exec.Command(node, "server.mjs")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PORT=%d", port),
		"HOST=127.0.0.1",
		"BRIDGE_TOKEN="+token,
		"GRROXY_MCP_URL="+mcpURL,
		// Requests arrive from grroxy over loopback with no Origin header, so the
		// bridge's own CORS handling is redundant. Keep it closed.
		"ALLOW_ORIGIN=",
		// Needed when Electron's binary stands in for node.
		"ELECTRON_RUN_AS_NODE=1",
		// Opt the child into the parent-death watchdog. Left unset for anyone
		// running the bridge by hand, where stdin means something else.
		"BRIDGE_EXIT_ON_PARENT_EOF=1",
	)
	cmd.Stdin = stdinR
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Own process group, so stopping the bridge also takes down the claude/codex
	// CLI it spawned rather than orphaning it.
	setProcessGroup(cmd)

	if err := cmd.Start(); err != nil {
		stdinR.Close()
		stdinW.Close()
		b.lastErr = fmt.Sprintf("failed to start: %v", err)
		return fmt.Errorf("failed to start %s bridge: %w", b.spec.label, err)
	}
	// The child holds its own descriptor now; ours would keep the pipe from ever
	// reaching EOF.
	stdinR.Close()

	b.cmd = cmd
	b.stdinW = stdinW
	b.dir = dir
	b.port = port

	// Reap the child so a crashed bridge doesn't linger as a zombie and the next
	// request re-spawns it instead of proxying into a dead port.
	go func() {
		err := cmd.Wait()
		b.mu.Lock()
		defer b.mu.Unlock()
		if b.cmd == cmd {
			if b.stdinW != nil {
				b.stdinW.Close()
				b.stdinW = nil
			}
			b.proxy = nil
			b.port = 0
			if err != nil {
				b.lastErr = fmt.Sprintf("exited: %v", err)
			}
		}
		log.Printf("[AIBridge] %s bridge exited (%v)", b.spec.label, err)
	}()

	if err := waitForHealth(port, bridgeStartupTimeout); err != nil {
		b.lastErr = err.Error()
		killProcessTree(cmd)
		if b.stdinW != nil {
			b.stdinW.Close()
			b.stdinW = nil
		}
		b.cmd = nil
		b.port = 0
		return fmt.Errorf("%s bridge did not become healthy: %w", b.spec.label, err)
	}

	target, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", port))
	proxy := httputil.NewSingleHostReverseProxy(target)
	// The bridges stream SSE; without immediate flushing the UI would sit blank
	// until the whole response completed.
	proxy.FlushInterval = -1
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("[AIBridge] %s proxy error: %v", b.spec.label, err)
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, `{"error":%q}`, err.Error())
	}
	b.proxy = proxy
	b.lastErr = ""

	log.Printf("[AIBridge] %s bridge ready on 127.0.0.1:%d (%s)", b.spec.label, port, dir)
	return nil
}

func waitForHealth(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}
	healthURL := fmt.Sprintf("http://127.0.0.1:%d/health", port)

	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := client.Get(healthURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			lastErr = fmt.Errorf("health returned %d", resp.StatusCode)
		} else {
			lastErr = err
		}
		time.Sleep(200 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("timed out after %s", timeout)
	}
	return lastErr
}

func (b *bridgeProc) stop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.stopped = true
	if b.cmd == nil || b.cmd.Process == nil {
		return
	}
	log.Printf("[AIBridge] Stopping %s bridge", b.spec.label)
	// Closing stdin first gives the bridge its shutdown cue; killing the group
	// then takes down both it and the CLI it spawned.
	if b.stdinW != nil {
		b.stdinW.Close()
		b.stdinW = nil
	}
	killProcessTree(b.cmd)
	b.cmd = nil
	b.proxy = nil
	b.port = 0
}

// resume clears a previous explicit stop so ensure() will spawn again.
func (b *bridgeProc) resume() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.stopped = false
}

func (b *bridgeProc) status() map[string]any {
	b.mu.Lock()
	defer b.mu.Unlock()
	return map[string]any{
		"provider": b.spec.key,
		"label":    b.spec.label,
		"running":  b.running(),
		"stopped":  b.stopped,
		"port":     b.port,
		"dir":      b.dir,
		"error":    b.lastErr,
	}
}

// StopAll terminates every bridge. Safe to call when none were ever started.
func (a *AIBridge) StopAll() {
	if a == nil {
		return
	}
	for _, proc := range a.procs {
		proc.stop()
	}
}

// ---------------------------------------------------------------------------
// HTTP endpoints
// ---------------------------------------------------------------------------

// AIBridgeEndpoint registers the reverse-proxy routes under /api/aibridge/.
func (backend *Backend) AIBridgeEndpoint(e *core.ServeEvent) error {
	if backend.AIBridge == nil {
		backend.AIBridge = NewAIBridge(func() string {
			return fmt.Sprintf("http://%s/mcp/sse", backend.Config.HostAddr)
		})
	}
	bridge := backend.AIBridge

	// Codex speaks MCP over streamable HTTP; Claude Code over SSE.
	mcpURLFor := func(provider string) string {
		if provider == "codex" {
			return fmt.Sprintf("http://%s/mcp/http", backend.Config.HostAddr)
		}
		return fmt.Sprintf("http://%s/mcp/sse", backend.Config.HostAddr)
	}

	forward := func(c echo.Context, path string) error {
		provider := c.PathParam("provider")
		proc, ok := bridge.procs[provider]
		if !ok {
			return apis.NewNotFoundError("Unknown AI bridge provider", nil)
		}

		if err := proc.ensure(bridge.token, mcpURLFor(provider)); err != nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]any{
				"error":    err.Error(),
				"provider": provider,
			})
		}

		proc.mu.Lock()
		proxy := proc.proxy
		proc.mu.Unlock()
		if proxy == nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]any{
				"error":    "bridge is not ready",
				"provider": provider,
			})
		}

		req := c.Request()
		req.URL.Path = path
		req.Host = "127.0.0.1"
		req.Header.Set("x-bridge-token", bridge.token)
		// The bridge's CORS check is keyed on Origin; stripping it keeps the
		// forwarded request in the "no origin" (same-process) path.
		req.Header.Del("Origin")

		proxy.ServeHTTP(c.Response(), req)
		return nil
	}

	for _, path := range []string{"chat", "title", "summary"} {
		target := "/" + path
		e.Router.POST("/api/aibridge/:provider/"+path, func(c echo.Context) error {
			return forward(c, target)
		})
	}

	e.Router.GET("/api/aibridge/status", func(c echo.Context) error {
		statuses := make(map[string]any, len(bridge.procs))
		for key, proc := range bridge.procs {
			statuses[key] = proc.status()
		}
		return c.JSON(http.StatusOK, map[string]any{"bridges": statuses})
	})

	// Explicit lifecycle control, so the settings UI can pre-warm a bridge (the
	// first spawn is slow) or shut one down to reclaim its memory.
	lookup := func(c echo.Context) (*bridgeProc, error) {
		proc, ok := bridge.procs[c.PathParam("provider")]
		if !ok {
			return nil, apis.NewNotFoundError("Unknown AI bridge provider", nil)
		}
		return proc, nil
	}

	e.Router.POST("/api/aibridge/:provider/start", func(c echo.Context) error {
		proc, err := lookup(c)
		if err != nil {
			return err
		}
		proc.resume()
		if err := proc.ensure(bridge.token, mcpURLFor(c.PathParam("provider"))); err != nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, proc.status())
	})

	e.Router.POST("/api/aibridge/:provider/stop", func(c echo.Context) error {
		proc, err := lookup(c)
		if err != nil {
			return err
		}
		proc.stop()
		return c.JSON(http.StatusOK, proc.status())
	})

	// Lets the UI recover from a wedged bridge without restarting grroxy.
	e.Router.POST("/api/aibridge/:provider/restart", func(c echo.Context) error {
		proc, err := lookup(c)
		if err != nil {
			return err
		}
		proc.stop()
		proc.resume()
		if err := proc.ensure(bridge.token, mcpURLFor(c.PathParam("provider"))); err != nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, proc.status())
	})

	log.Println("[AIBridge] Endpoints registered at /api/aibridge/")
	return nil
}
