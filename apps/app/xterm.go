package app

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/glitchedgitz/pocketbase/apis"
	"github.com/glitchedgitz/pocketbase/core"
	"github.com/labstack/echo/v5"
	"github.com/rs/xid"
	"golang.org/x/net/websocket"
)

// XtermManager manages all terminal sessions
type XtermManager struct {
	sessions   map[string]*XtermSession
	sessionsMu sync.RWMutex
}

// scrollbackBufferSize is the max bytes of PTY output kept for replay on reconnect.
const scrollbackBufferSize = 256 * 1024 // 256 KB

// XtermSession represents a single terminal session
type XtermSession struct {
	ID        string
	Cmd       *exec.Cmd
	Pty       *os.File
	CreatedAt time.Time
	Shell     string
	WorkDir   string
	Env       []string
	mu        sync.Mutex
	closed    bool

	// Scrollback ring buffer for replay on reconnect
	scrollback   []byte
	scrollbackMu sync.Mutex

	// Connected WebSocket clients
	clients   map[*websocket.Conn]struct{}
	clientsMu sync.Mutex
}

// XtermMessage represents WebSocket messages between client and server
type XtermMessage struct {
	Type string      `json:"type"` // "input", "resize", "ping"
	Data interface{} `json:"data"`
}

// XtermResizeData represents terminal resize data
type XtermResizeData struct {
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

// XtermStartRequest represents the request to start a new terminal
type XtermStartRequest struct {
	Shell   string            `json:"shell"`   // e.g., "bash", "zsh", "sh", "powershell"
	WorkDir string            `json:"workdir"` // working directory
	Env     map[string]string `json:"env"`     // additional environment variables
}

// XtermStartResponse represents the response after starting a terminal
type XtermStartResponse struct {
	SessionID string `json:"session_id"`
	Shell     string `json:"shell"`
	WorkDir   string `json:"workdir"`
}

// NewXtermManager creates a new terminal manager
func NewXtermManager() *XtermManager {
	return &XtermManager{
		sessions: make(map[string]*XtermSession),
	}
}

// getDefaultShell returns the default shell for the current OS
func getDefaultShell() string {
	switch runtime.GOOS {
	case "windows":
		// Try PowerShell first, then cmd
		if _, err := exec.LookPath("powershell.exe"); err == nil {
			return "powershell.exe"
		}
		return "cmd.exe"
	default:
		// Try to get user's shell from SHELL env var
		if shell := os.Getenv("SHELL"); shell != "" {
			return shell
		}
		// Default to bash, fallback to sh
		if _, err := exec.LookPath("bash"); err == nil {
			return "bash"
		}
		return "sh"
	}
}

// getDefaultWorkDir returns the default working directory
func getDefaultWorkDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "/"
}

// CreateSession creates a new terminal session
func (m *XtermManager) CreateSession(shell, workDir string, envVars map[string]string) (*XtermSession, error) {
	// Use defaults if not specified
	if shell == "" {
		shell = getDefaultShell()
	}
	if workDir == "" {
		workDir = getDefaultWorkDir()
	}

	// Validate working directory
	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		workDir = getDefaultWorkDir()
		log.Printf("[Xterm] Working directory does not exist, using default: %s", workDir)
	}

	// Generate session ID
	sessionID := xid.New().String()

	// Prepare command based on OS
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		if shell == "powershell.exe" || shell == "powershell" {
			cmd = exec.Command("powershell.exe", "-NoLogo", "-NoExit")
		} else {
			cmd = exec.Command(shell)
		}
	default:
		// For Unix-like systems, use login shell
		cmd = exec.Command(shell)
	}

	// Set working directory
	cmd.Dir = workDir

	// Set environment variables
	cmd.Env = os.Environ()
	for key, value := range envVars {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	// Add TERM environment variable for better terminal compatibility
	cmd.Env = append(cmd.Env, "TERM=xterm-256color")

	// Start the command with a PTY
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to start PTY: %w", err)
	}

	// Set initial terminal size (default)
	if err := pty.Setsize(ptmx, &pty.Winsize{
		Rows: 24,
		Cols: 80,
	}); err != nil {
		log.Printf("[Xterm] Failed to set initial terminal size: %v", err)
	}

	session := &XtermSession{
		ID:         sessionID,
		Cmd:        cmd,
		Pty:        ptmx,
		CreatedAt:  time.Now(),
		Shell:      shell,
		WorkDir:    workDir,
		Env:        cmd.Env,
		closed:     false,
		scrollback: make([]byte, 0, scrollbackBufferSize),
		clients:    make(map[*websocket.Conn]struct{}),
	}

	// Store session
	m.sessionsMu.Lock()
	m.sessions[sessionID] = session
	m.sessionsMu.Unlock()

	log.Printf("[Xterm] Created session %s with shell %s in %s", sessionID, shell, workDir)

	// Start persistent PTY reader that buffers output and broadcasts to clients
	go session.readPtyLoop()

	// Start goroutine to clean up when process exits
	go func() {
		_ = cmd.Wait()
		log.Printf("[Xterm] Session %s process exited", sessionID)
		m.CloseSession(sessionID)
	}()

	return session, nil
}

// GetSession retrieves a session by ID
func (m *XtermManager) GetSession(sessionID string) (*XtermSession, error) {
	m.sessionsMu.RLock()
	defer m.sessionsMu.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return session, nil
}

// CloseSession closes a terminal session
func (m *XtermManager) CloseSession(sessionID string) error {
	m.sessionsMu.Lock()
	session, exists := m.sessions[sessionID]
	if !exists {
		m.sessionsMu.Unlock()
		return fmt.Errorf("session not found: %s", sessionID)
	}
	delete(m.sessions, sessionID)
	m.sessionsMu.Unlock()

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.closed {
		return nil
	}

	session.closed = true

	// Close PTY
	if session.Pty != nil {
		if err := session.Pty.Close(); err != nil {
			log.Printf("[Xterm] Error closing PTY for session %s: %v", sessionID, err)
		}
	}

	// Kill process if still running
	if session.Cmd != nil && session.Cmd.Process != nil {
		if err := session.Cmd.Process.Kill(); err != nil {
			log.Printf("[Xterm] Error killing process for session %s: %v", sessionID, err)
		}
	}

	log.Printf("[Xterm] Closed session %s", sessionID)
	return nil
}

// ListSessions returns all active sessions
func (m *XtermManager) ListSessions() []map[string]interface{} {
	m.sessionsMu.RLock()
	defer m.sessionsMu.RUnlock()

	sessions := make([]map[string]interface{}, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, map[string]interface{}{
			"id":         session.ID,
			"shell":      session.Shell,
			"workdir":    session.WorkDir,
			"created_at": session.CreatedAt,
			"running":    session.Cmd.ProcessState == nil || !session.Cmd.ProcessState.Exited(),
		})
	}

	return sessions
}

// CleanupAllSessions closes all terminal sessions
func (m *XtermManager) CleanupAllSessions() {
	m.sessionsMu.Lock()
	sessionIDs := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		sessionIDs = append(sessionIDs, id)
	}
	m.sessionsMu.Unlock()

	for _, id := range sessionIDs {
		if err := m.CloseSession(id); err != nil {
			log.Printf("[Xterm] Error closing session %s: %v", id, err)
		}
	}
}

// readPtyLoop is a persistent goroutine that reads from the PTY,
// appends to the scrollback buffer, and broadcasts to all connected WebSocket clients.
func (s *XtermSession) readPtyLoop() {
	buf := make([]byte, 8192)
	for {
		n, err := s.Pty.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.Printf("[Xterm] Error reading from PTY (session %s): %v", s.ID, err)
			}
			return
		}

		data := buf[:n]

		// Append to scrollback buffer
		s.scrollbackMu.Lock()
		s.scrollback = append(s.scrollback, data...)
		// Trim to max size, keeping the most recent output
		if len(s.scrollback) > scrollbackBufferSize {
			s.scrollback = s.scrollback[len(s.scrollback)-scrollbackBufferSize:]
		}
		s.scrollbackMu.Unlock()

		// Broadcast to all connected clients
		msg := map[string]interface{}{
			"type": "output",
			"data": string(data),
		}

		s.clientsMu.Lock()
		for ws := range s.clients {
			if err := websocket.JSON.Send(ws, msg); err != nil {
				log.Printf("[Xterm] Error broadcasting to client (session %s): %v", s.ID, err)
				delete(s.clients, ws)
			}
		}
		s.clientsMu.Unlock()
	}
}

// addClient registers a WebSocket connection and replays the scrollback buffer.
func (s *XtermSession) addClient(ws *websocket.Conn) {
	// Take a snapshot of the scrollback while holding the lock,
	// then register the client before releasing, so no output is lost
	// between replay and going live.
	s.scrollbackMu.Lock()
	snapshot := make([]byte, len(s.scrollback))
	copy(snapshot, s.scrollback)
	// Register client while still holding scrollbackMu so the PTY reader
	// will start broadcasting to this client before we release.
	s.clientsMu.Lock()
	s.clients[ws] = struct{}{}
	s.clientsMu.Unlock()
	s.scrollbackMu.Unlock()

	// Replay buffered output
	if len(snapshot) > 0 {
		msg := map[string]interface{}{
			"type": "output",
			"data": string(snapshot),
		}
		if err := websocket.JSON.Send(ws, msg); err != nil {
			log.Printf("[Xterm] Error replaying scrollback (session %s): %v", s.ID, err)
		}
	}
}

// removeClient unregisters a WebSocket connection.
func (s *XtermSession) removeClient(ws *websocket.Conn) {
	s.clientsMu.Lock()
	delete(s.clients, ws)
	s.clientsMu.Unlock()
}

// HandleWebSocket handles WebSocket connections for terminal I/O
func (m *XtermManager) HandleWebSocket(ws *websocket.Conn, sessionID string) {
	defer ws.Close()

	session, err := m.GetSession(sessionID)
	if err != nil {
		log.Printf("[Xterm] WebSocket connection failed: %v", err)
		errorMsg := fmt.Sprintf(`{"type":"error","data":"Session not found: %s"}`, sessionID)
		websocket.Message.Send(ws, errorMsg)
		return
	}

	log.Printf("[Xterm] WebSocket connected for session %s", sessionID)

	// Register client and replay scrollback buffer
	session.addClient(ws)
	defer session.removeClient(ws)

	// Read from WebSocket and write to PTY
	for {
		var msg XtermMessage
		err := websocket.JSON.Receive(ws, &msg)
		if err != nil {
			if err != io.EOF {
				log.Printf("[Xterm] Error receiving from WebSocket: %v", err)
			}
			break
		}

		switch msg.Type {
		case "input":
			// Handle terminal input
			if data, ok := msg.Data.(string); ok {
				if _, err := session.Pty.Write([]byte(data)); err != nil {
					log.Printf("[Xterm] Error writing to PTY: %v", err)
					return
				}
			}

		case "resize":
			// Handle terminal resize
			resizeData := XtermResizeData{}
			dataBytes, err := json.Marshal(msg.Data)
			if err != nil {
				log.Printf("[Xterm] Error marshaling resize data: %v", err)
				continue
			}
			if err := json.Unmarshal(dataBytes, &resizeData); err != nil {
				log.Printf("[Xterm] Error unmarshaling resize data: %v", err)
				continue
			}

			if err := pty.Setsize(session.Pty, &pty.Winsize{
				Rows: resizeData.Rows,
				Cols: resizeData.Cols,
			}); err != nil {
				log.Printf("[Xterm] Error resizing terminal: %v", err)
			} else {
				log.Printf("[Xterm] Terminal resized to %dx%d", resizeData.Cols, resizeData.Rows)
			}

		case "ping":
			// Respond to ping with pong
			pongMsg := map[string]interface{}{
				"type": "pong",
				"data": msg.Data,
			}
			if err := websocket.JSON.Send(ws, pongMsg); err != nil {
				log.Printf("[Xterm] Error sending pong: %v", err)
				return
			}
		}
	}

	log.Printf("[Xterm] WebSocket disconnected for session %s", sessionID)
}

// RegisterXtermRoutes registers all xterm-related routes
func (backend *Backend) RegisterXtermRoutes() {
	// Initialize xterm manager if not exists
	if backend.XtermManager == nil {
		backend.XtermManager = NewXtermManager()
	}

	// POST /api/xterm/start - Start a new terminal session
	backend.App.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		e.Router.POST("/api/xterm/start", func(c echo.Context) error {
			var req XtermStartRequest
			if err := c.Bind(&req); err != nil {
				return apis.NewBadRequestError("Invalid request body", err)
			}

			session, err := backend.XtermManager.CreateSession(req.Shell, req.WorkDir, req.Env)
			if err != nil {
				return apis.NewBadRequestError("Failed to create terminal session", err)
			}

			return c.JSON(http.StatusOK, XtermStartResponse{
				SessionID: session.ID,
				Shell:     session.Shell,
				WorkDir:   session.WorkDir,
			})
		})

		return nil
	})

	// GET /api/xterm/sessions - List all terminal sessions
	backend.App.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		e.Router.GET("/api/xterm/sessions", func(c echo.Context) error {
			sessions := backend.XtermManager.ListSessions()
			return c.JSON(http.StatusOK, map[string]interface{}{
				"sessions": sessions,
			})
		})

		return nil
	})

	// DELETE /api/xterm/sessions/:id - Close a terminal session
	backend.App.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		e.Router.DELETE("/api/xterm/sessions/:id", func(c echo.Context) error {
			sessionID := c.PathParam("id")
			if sessionID == "" {
				return apis.NewBadRequestError("Session ID required", nil)
			}

			if err := backend.XtermManager.CloseSession(sessionID); err != nil {
				return apis.NewNotFoundError("Session not found", err)
			}

			return c.JSON(http.StatusOK, map[string]interface{}{
				"message": "Session closed successfully",
			})
		})

		return nil
	})

	// GET /api/xterm/ws/:id - WebSocket endpoint for terminal I/O
	backend.App.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		e.Router.GET("/api/xterm/ws/:id", func(c echo.Context) error {
			sessionID := c.PathParam("id")
			if sessionID == "" {
				return apis.NewBadRequestError("Session ID required", nil)
			}

			// Verify session exists before upgrading to WebSocket
			if _, err := backend.XtermManager.GetSession(sessionID); err != nil {
				return apis.NewNotFoundError("Session not found", err)
			}

			// Upgrade to WebSocket
			websocket.Handler(func(ws *websocket.Conn) {
				backend.XtermManager.HandleWebSocket(ws, sessionID)
			}).ServeHTTP(c.Response(), c.Request())

			return nil
		})

		return nil
	})

	log.Println("[Xterm] Routes registered successfully")
}
