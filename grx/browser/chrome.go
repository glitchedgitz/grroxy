package browser

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

// reclaimFocusWhenChromeSteals fires off an AppleScript that captures
// whichever macOS app is currently frontmost, then waits for Chrome to
// actually grab focus, and immediately re-activates the captured app.
// Event-driven (the script polls *inside* AppleScript on the system-events
// runloop and exits the moment the condition is met), so we're not guessing
// timing in Go. Bounded to ~5s so a stuck Chrome can't leak the script.
// No-op when not on darwin or when the user was already focused on Chrome.
func reclaimFocusWhenChromeSteals() {
	if runtime.GOOS != "darwin" {
		return
	}
	const script = `
tell application "System Events"
    set prevApp to name of first application process whose frontmost is true
    if prevApp starts with "Google Chrome" then return
    if prevApp starts with "Chromium" then return
    repeat 50 times
        try
            set chromeFront to false
            repeat with p in (every application process whose name starts with "Google Chrome")
                if frontmost of p then set chromeFront to true
            end repeat
            if chromeFront then
                tell application prevApp to activate
                return
            end if
        end try
        delay 0.1
    end repeat
end tell
`
	go func() { _ = exec.Command("osascript", "-e", script).Run() }()
}

func launchChrome(proxyAddress string, customCertPath string, profileDir string, startURL string) (*exec.Cmd, error) {
	log.Println("[launchChrome] Starting Chrome launch process")

	// Use provided profile directory
	chromeDataDir := profileDir
	log.Printf("[launchChrome] Chrome data directory: %s", chromeDataDir)

	// Create profile directory if it doesn't exist (keep existing profile for persistence)
	if err := os.MkdirAll(chromeDataDir, 0755); err != nil {
		return nil, fmt.Errorf("[launchChrome] failed to create Chrome data directory: %v", err)
	}
	log.Printf("[launchChrome] Created Chrome data directory successfully")

	// Copy CA certificate to Chrome's certificate store directory (note: this does not add trust itself)
	certPath := filepath.Join(chromeDataDir, "ca.crt")
	log.Printf("[launchChrome] Copying certificate from %s to %s", customCertPath, certPath)
	if err := copyFile(customCertPath, certPath); err != nil {
		return nil, fmt.Errorf("[launchChrome] failed to copy certificate: %v", err)
	}
	log.Printf("[launchChrome] Certificate copied successfully")

	// Prefer the stable leaf SPKI if available (written by MITM init), else fall back to CA SPKI
	var fingerprint string
	leafSpkiPath := filepath.Join(filepath.Dir(customCertPath), "leaf.spki")
	if data, err := os.ReadFile(leafSpkiPath); err == nil {
		fingerprint = string(data)
		log.Printf("[launchChrome] Using leaf SPKI from %s", leafSpkiPath)
	} else {
		log.Printf("[launchChrome] leaf SPKI not found (%v), calculating CA SPKI instead", err)
		log.Printf("[launchChrome] Calculating certificate fingerprint")
		fp, ferr := GetSPKIFingerprint(certPath)
		if ferr != nil {
			log.Printf("[launchChrome] Warning: couldn't calculate certificate fingerprint: %v", ferr)
			log.Printf("[launchChrome] Certificate trust may not work correctly")
		} else {
			fingerprint = fp
			log.Printf("[launchChrome] Certificate fingerprint calculated successfully")
		}
	}

	// Determine Chrome executable path
	var chromePath string
	switch runtime.GOOS {
	case "darwin": // macOS
		chromePath = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
		if _, err := os.Stat(chromePath); err != nil {
			log.Printf("[launchChrome] Chrome not found at primary path, trying Chromium")
			chromePath = "/Applications/Chromium.app/Contents/MacOS/Chromium"
		}
	case "linux":
		chromePath = "google-chrome"
	case "windows":
		chromePath = "C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe"
		if _, err := os.Stat(chromePath); err != nil {
			log.Printf("[launchChrome] Chrome not found at primary path, trying alternative path")
			chromePath = "C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe"
		}
	default:
		return nil, fmt.Errorf("[launchChrome] unsupported operating system: %s", runtime.GOOS)
	}
	log.Printf("[launchChrome] Using Chrome path: %s", chromePath)

	// Verify Chrome executable exists
	if _, err := os.Stat(chromePath); err != nil {
		return nil, fmt.Errorf("[launchChrome] Chrome executable not found at %s: %v", chromePath, err)
	}
	log.Printf("[launchChrome] Chrome executable found and verified")

	// Construct Chrome command line arguments
	args := []string{
		"--user-data-dir=" + chromeDataDir,
		"--proxy-server=" + proxyAddress,
	}

	// Add certificate fingerprint to the ignore list if we were able to calculate it
	if fingerprint != "" {
		args = append(args, "--ignore-certificate-errors-spki-list="+fingerprint)
	} else {
		// Fallback to the older, less secure method
		args = append(args, "--ignore-certificate-errors")
	}

	// Add other standard arguments
	args = append(args,
		"--remote-debugging-port=0", // Auto-assign debug port for CDP access
		"--allow-insecure-localhost",
		"--unsafely-treat-insecure-origin-as-secure",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-restore-session-state",
		"--disable-popup-blocking",
		"--disable-translate",
		"--disable-infobars",
		"--enable-features=SuppressDifferentOriginSubframeDialogs",
		"--disable-extensions-except=",
		"--disable-component-extensions-with-background-pages",
		"--start-maximized",
		"--disable-default-apps",
		"--disable-sync",
		"--enable-fixed-layout",
		"--noerrdialogs",
		"--test-type",
		startURL,
	)

	log.Printf("[launchChrome] Chrome arguments: %v", args)

	// Launch Chrome
	log.Printf("[launchChrome] Attempting to launch Chrome with command: %s %v", chromePath, args)
	cmd := exec.Command(chromePath, args...)
	log.Println("[launchChrome] " + cmd.String())
	// macOS hands OS focus to any newly exec'd GUI app and CDP flags can't
	// prevent it. Kick off an AppleScript that watches for Chrome to actually
	// become frontmost and re-activates the previously-frontmost app the
	// moment that happens — event-driven, not timing-based.
	reclaimFocusWhenChromeSteals()
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("[launchChrome] failed to launch Chrome: %v", err)
	}

	log.Printf("[launchChrome] Chrome process started successfully")
	log.Printf("[launchChrome] Chrome profile at: %s", chromeDataDir)

	// Focus the address bar after Chrome finishes loading
	// go focusAddressBar(chromeDataDir)

	return cmd, nil
}

// // focusAddressBar waits for Chrome to be ready, then sends Cmd+L / Ctrl+L to focus the omnibox.
// func focusAddressBar(profileDir string) {
// 	// Wait for DevToolsActivePort file to appear (Chrome writes it once ready)
// 	devToolsFile := filepath.Join(profileDir, "DevToolsActivePort")
// 	var debugURL string
// 	for i := 0; i < 30; i++ {
// 		time.Sleep(500 * time.Millisecond)
// 		url, err := GetChromeDebugURL(profileDir)
// 		if err == nil {
// 			debugURL = url
// 			break
// 		}
// 		_ = devToolsFile // suppress unused hint
// 	}
// 	if debugURL == "" {
// 		log.Println("[focusAddressBar] Could not get Chrome debug URL, skipping address bar focus")
// 		return
// 	}

// 	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), debugURL)
// 	defer allocCancel()

// 	ctx, cancel := chromedp.NewContext(allocCtx)
// 	defer cancel()

// 	ctx, timeoutCancel := context.WithTimeout(ctx, 5*time.Second)
// 	defer timeoutCancel()

// 	// Determine the modifier flag based on OS
// 	var modifiers input.Modifier = 2 // Ctrl
// 	if runtime.GOOS == "darwin" {
// 		modifiers = 4 // Meta (Cmd)
// 	}

// 	// Send Ctrl+L / Cmd+L via CDP Input.dispatchKeyEvent to focus the address bar
// 	err := chromedp.Run(ctx, chromedp.ActionFunc(func(c context.Context) error {
// 		// Key down
// 		if err := input.DispatchKeyEvent(input.KeyDown).
// 			WithModifiers(modifiers).
// 			WithKey("l").
// 			WithCode("KeyL").
// 			WithWindowsVirtualKeyCode(76).
// 			WithNativeVirtualKeyCode(76).
// 			Do(c); err != nil {
// 			return err
// 		}
// 		// Key up
// 		return input.DispatchKeyEvent(input.KeyUp).
// 			WithModifiers(modifiers).
// 			WithKey("l").
// 			WithCode("KeyL").
// 			WithWindowsVirtualKeyCode(76).
// 			WithNativeVirtualKeyCode(76).
// 			Do(c)
// 	}))
// 	if err != nil {
// 		log.Printf("[focusAddressBar] Failed to send key event: %v", err)
// 		return
// 	}

// 	log.Println("[focusAddressBar] Address bar focus sent")
// }

// GetChromeDebugURL reads the DevTools WebSocket URL from Chrome's profile directory
// Chrome writes this information to DevToolsActivePort file when launched with --remote-debugging-port
func GetChromeDebugURL(profileDir string) (string, error) {
	devToolsFile := filepath.Join(profileDir, "DevToolsActivePort")

	// Read the DevToolsActivePort file
	data, err := os.ReadFile(devToolsFile)
	if err != nil {
		return "", fmt.Errorf("[GetChromeDebugURL] failed to read DevToolsActivePort file: %v", err)
	}

	// File format:
	// Line 1: port number
	// Line 2: browser DevTools WebSocket URL path
	lines := strings.Split(string(data), "\n")
	if len(lines) < 2 {
		return "", fmt.Errorf("[GetChromeDebugURL] invalid DevToolsActivePort file format")
	}

	port := strings.TrimSpace(lines[0])
	wsPath := strings.TrimSpace(lines[1])

	if port == "" || wsPath == "" {
		return "", fmt.Errorf("[GetChromeDebugURL] empty port or WebSocket path in DevToolsActivePort")
	}

	// Construct the full WebSocket URL
	debugURL := fmt.Sprintf("ws://127.0.0.1:%s%s", port, wsPath)
	log.Printf("[GetChromeDebugURL] Found Chrome debug URL: %s", debugURL)

	return debugURL, nil
}

// ChromeRemote manages a connection to a Chrome instance via DevTools Protocol
type ChromeRemote struct {
	debugURL      string
	allocCtx      context.Context
	allocCancel   context.CancelFunc
	browserCtx    context.Context
	browserCancel context.CancelFunc

	targetCtxs   map[string]context.Context
	targetCancel map[string]context.CancelFunc
	// lastActiveTargetID is updated whenever ActivateTab/OpenTab/Navigate is
	// called explicitly. TakeScreenshot("") prefers this over the
	// "first non-blank tab" heuristic so multi-tab workflows behave as the
	// caller expects.
	lastActiveTargetID string
	mu                 sync.Mutex
}

// NewChromeRemote creates a new ChromeRemote instance connected to the given debug URL
func NewChromeRemote(debugURL string) (*ChromeRemote, error) {
	log.Printf("[ChromeRemote] Connecting to %s", debugURL)

	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), debugURL)

	// Create the first context (this establishes the connection)
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)

	// Trigger the browser to start by running an empty action to ensure connection
	if err := chromedp.Run(browserCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		return nil
	})); err != nil {
		browserCancel()
		allocCancel()
		return nil, fmt.Errorf("failed to connect to Chrome: %v", err)
	}

	return &ChromeRemote{
		debugURL:      debugURL,
		allocCtx:      allocCtx,
		allocCancel:   allocCancel,
		browserCtx:    browserCtx,
		browserCancel: browserCancel,
		targetCtxs:    make(map[string]context.Context),
		targetCancel:  make(map[string]context.CancelFunc),
	}, nil
}

// Close closes the connection to Chrome and all sub-contexts
func (cr *ChromeRemote) Close() {
	log.Println("[ChromeRemote] Closing connection and all cached contexts")
	cr.mu.Lock()
	defer cr.mu.Unlock()

	for id, cancel := range cr.targetCancel {
		log.Printf("[ChromeRemote] Closing context for target %s", id)
		cancel()
	}
	cr.targetCtxs = make(map[string]context.Context)
	cr.targetCancel = make(map[string]context.CancelFunc)

	if cr.browserCancel != nil {
		cr.browserCancel()
	}
	if cr.allocCancel != nil {
		cr.allocCancel()
	}
}

// getContext returns a context for the specific target ID.
// It caches contexts to avoid frequent opening/closing of sessions, which helps prevent tab closure.
//
// When targetID is empty we prefer the most-recently activated/opened/
// navigated tab (lastActiveTargetID) so that proxyActivateTab actually
// steers downstream tools (proxyType / proxyClick / proxyEval / etc.) to
// the activated tab instead of always landing on tabs[0]. Falls back to
// the first listed tab if no last-active is recorded yet.
func (cr *ChromeRemote) getContext(targetID string) (context.Context, error) {
	if targetID == "" {
		cr.mu.Lock()
		targetID = cr.lastActiveTargetID
		cr.mu.Unlock()
	}
	if targetID == "" {
		tabs, err := cr.ListTabs()
		if err != nil || len(tabs) == 0 {
			return cr.browserCtx, nil
		}
		targetID = tabs[0].ID
	}

	cr.mu.Lock()
	defer cr.mu.Unlock()

	// Check cache
	if ctx, ok := cr.targetCtxs[targetID]; ok {
		select {
		case <-ctx.Done():
			// Context expired, remove from cache
			delete(cr.targetCtxs, targetID)
			delete(cr.targetCancel, targetID)
		default:
			return ctx, nil
		}
	}

	// Create new target context
	log.Printf("[ChromeRemote] Creating new persistent context for target %s", targetID)
	ctx, cancel := chromedp.NewContext(cr.browserCtx, chromedp.WithTargetID(target.ID(targetID)))

	// Initialize it
	if err := chromedp.Run(ctx); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize target context: %v", err)
	}

	cr.targetCtxs[targetID] = ctx
	cr.targetCancel[targetID] = cancel

	return ctx, nil
}

// CloseTargetContext manually closes and removes a context from the cache
func (cr *ChromeRemote) CloseTargetContext(targetID string) {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	if cancel, ok := cr.targetCancel[targetID]; ok {
		cancel()
		delete(cr.targetCtxs, targetID)
		delete(cr.targetCancel, targetID)
	}
}

// TakeScreenshot captures a screenshot of a specific tab (targetID).
// If targetID == "", it will try to pick a "best" tab (heuristic).
func (cr *ChromeRemote) TakeScreenshot(targetID string, fullPage bool, timeoutMs int) ([]byte, error) {
	log.Printf("[ChromeRemote] Starting screenshot (targetID=%s, fullPage=%v, timeoutMs=%d)", targetID, fullPage, timeoutMs)
	if timeoutMs <= 0 {
		timeoutMs = 30000
	}

	// Fall back to the most recently activated/opened/navigated tab when no
	// explicit targetID is given. This makes activate-then-screenshot work as
	// callers expect, instead of always grabbing the first non-blank tab.
	if targetID == "" {
		cr.mu.Lock()
		targetID = cr.lastActiveTargetID
		cr.mu.Unlock()
		if targetID != "" {
			log.Printf("[ChromeRemote] Using last-active target: %s", targetID)
		}
	}

	ctx, err := cr.getContext(targetID)
	if err != nil {
		return nil, err
	}

	if targetID == "" {
		// Pick a "best" existing page tab if no targetID provided
		var picked target.ID
		err := chromedp.Run(ctx,
			chromedp.ActionFunc(func(c context.Context) error {
				infos, err := chromedp.Targets(c)
				if err != nil {
					return err
				}

				var fallbackTab target.ID

				for _, info := range infos {
					if info.Type != "page" {
						continue
					}

					// Remember first page tab as fallback
					if fallbackTab == "" {
						fallbackTab = info.TargetID
					}

					// Try to find a "good" tab (not blank/chrome/extension)
					u := strings.TrimSpace(info.URL)
					if u == "" || u == "about:blank" ||
						strings.HasPrefix(u, "chrome://") ||
						strings.HasPrefix(u, "chrome-extension://") ||
						strings.HasPrefix(u, "devtools://") {
						continue
					}
					picked = info.TargetID
					break
				}

				if picked == "" && fallbackTab != "" {
					picked = fallbackTab
				}

				if picked == "" {
					return fmt.Errorf("no page tabs found in Chrome")
				}

				log.Printf("[ChromeRemote] Selected tab ID: %s", picked.String())
				return nil
			}),
		)
		if err != nil {
			return nil, fmt.Errorf("[ChromeRemote] failed selecting tab: %v", err)
		}

		// Create a specific context for the picked tab (this will cache it)
		ctx, err = cr.getContext(string(picked))
		if err != nil {
			return nil, err
		}
	}

	// Timeout for the screenshot operation (caller-configurable).
	ctx, timeoutCancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer timeoutCancel()

	// Note: we deliberately do NOT call target.ActivateTarget here. CDP's
	// Page.captureScreenshot (and chromedp.FullScreenshot) work on background
	// tabs, and activating the target steals OS focus on macOS — Chrome.app
	// pops over grroxy on every AI-driven screenshot.

	// Best-effort wait for the page to be done loading before we capture.
	// Pages stuck in 'loading' (e.g. after a CAPTCHA) used to make the
	// FullScreenshot action hang until the global timeout fired with a
	// generic "context deadline exceeded".
	readyCtx, readyCancel := context.WithTimeout(ctx, 3*time.Second)
	var ready bool
	_ = chromedp.Run(readyCtx, chromedp.Evaluate(`document.readyState === 'complete'`, &ready))
	readyCancel()

	capture := func() ([]byte, error) {
		var buf []byte
		var tasks []chromedp.Action
		tasks = append(tasks, chromedp.WaitReady("body", chromedp.ByQuery))
		if fullPage {
			tasks = append(tasks, chromedp.FullScreenshot(&buf, 90))
		} else {
			tasks = append(tasks, chromedp.CaptureScreenshot(&buf))
		}
		err := chromedp.Run(ctx, tasks...)
		return buf, err
	}

	buf, err := capture()
	if err != nil {
		// One short retry — the most common cause of a transient timeout
		// is the page momentarily blocked on a heavy frame/CAPTCHA load.
		log.Printf("[ChromeRemote] Screenshot retry after error: %v", err)
		buf, err = capture()
		if err != nil {
			return nil, fmt.Errorf("[ChromeRemote] failed to capture screenshot: %v", err)
		}
	}

	log.Printf("[ChromeRemote] Screenshot captured (%d bytes)", len(buf))
	return buf, nil
}

// ClickElement clicks an element on the page using Chrome DevTools Protocol.
// targetID can be empty to pick the best tab.
// ClickResult is what ClickElement returns. NewTabs lists any page-type
// targets that appeared between the start and end of the click — useful for
// callers because clicking links like Bing's "Images" can silently spawn a
// new tab instead of navigating the current one. WaitedFor reports whether
// the optional waitForSelector matched. ClickMode is "native" (CDP
// dispatchMouseEvent), "js" (caller asked for JS click directly), or
// "js-fallback" (native click timed out so we fell back to el.click()).
type ClickResult struct {
	NewTabs   []string `json:"newTabs"`
	WaitedFor string   `json:"waitedFor,omitempty"`
	NavWaited bool     `json:"navWaited,omitempty"`
	ClickMode string   `json:"clickMode,omitempty"`
}

// ClickElement clicks an element on the page using CDP and reports any new
// tabs that opened as a side effect. If waitForSelector is non-empty, after
// the click it waits until that selector becomes visible — useful for SPAs
// where waitForNavigation never fires because the route swap is client-side.
//
// useJS forces a JS .click() instead of CDP dispatchMouseEvent (skips the
// native path entirely). Useful when overlays/focus-traps intercept native
// mouse events on complex sites. When useJS is false, native CDP click is
// tried first; if it times out within its 5s budget the function auto-falls
// back to JS .click() so the AI agent doesn't have to retry. The result's
// ClickMode reports which path actually fired.
//
// Implementation notes (re: the "waitForNavigation/waitForSelector never
// resolve" bugs):
//   - Phases are split into separate chromedp.Run calls so that the click
//     completing doesn't poison the post-click waits.
//   - waitForNavigation listens for page.EventFrameNavigated registered
//     BEFORE the click. WaitReady("body") on its own can return on the
//     pre-navigation document and look like a false positive.
//   - When the click spawns a new tab, the post-click waits are routed to
//     the new tab's context — otherwise WaitVisible would poll the old tab
//     forever for a selector that only appears in the new one.
func (cr *ChromeRemote) ClickElement(targetID string, targetURL string, selector string, waitForNavigation bool, waitForSelector string, useJS bool) (*ClickResult, error) {
	log.Printf("[ChromeRemote] Starting click operation (targetID=%s, selector=%s, targetURL=%s, waitNav=%v, waitForSelector=%q, useJS=%v)",
		targetID, selector, targetURL, waitForNavigation, waitForSelector, useJS)

	ctx, err := cr.getContext(targetID)
	if err != nil {
		return nil, err
	}

	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 30*time.Second)
	defer timeoutCancel()

	// Snapshot existing target IDs so we can diff after the click and report
	// any new tabs that were spawned.
	existing := make(map[target.ID]bool)
	if err := chromedp.Run(timeoutCtx, chromedp.ActionFunc(func(c context.Context) error {
		infos, err := chromedp.Targets(c)
		if err != nil {
			return err
		}
		for _, info := range infos {
			existing[info.TargetID] = true
		}
		return nil
	})); err != nil {
		return nil, fmt.Errorf("[ChromeRemote] failed to enumerate existing targets: %v", err)
	}

	// Optional pre-navigation runs in its own phase so a failure here doesn't
	// look like a click failure.
	if targetURL != "" {
		log.Printf("[ChromeRemote] Navigating to URL: %s", targetURL)
		if err := chromedp.Run(timeoutCtx,
			chromedp.Navigate(targetURL),
			chromedp.WaitReady("body", chromedp.ByQuery),
		); err != nil {
			return nil, fmt.Errorf("[ChromeRemote] navigate before click failed: %v", err)
		}
	}

	// Register a frameNavigated listener BEFORE the click so we don't miss
	// the event. We only fire navDone for the main frame so subframe loads
	// (ads/iframes/etc.) don't trip the wait.
	navDone := make(chan struct{}, 1)
	if waitForNavigation {
		chromedp.ListenTarget(timeoutCtx, func(ev interface{}) {
			if e, ok := ev.(*page.EventFrameNavigated); ok && e.Frame != nil && e.Frame.ParentID == "" {
				select {
				case navDone <- struct{}{}:
				default:
				}
			}
		})
	}

	// Stepwise click: each phase has its own short timeout so a failure
	// surfaces as "click step X failed" instead of eating the global budget.
	runStep := func(name string, action chromedp.Action, perStepMs int) error {
		sctx, scancel := context.WithTimeout(timeoutCtx, time.Duration(perStepMs)*time.Millisecond)
		defer scancel()
		if err := chromedp.Run(sctx, action); err != nil {
			return fmt.Errorf("[ChromeRemote] click step %q failed: %v", name, err)
		}
		return nil
	}

	if err := runStep("waitVisible", chromedp.WaitVisible(selector, chromedp.ByQuery), 5000); err != nil {
		return nil, err
	}
	// Best-effort scroll into view so the click coordinates resolve to the
	// element instead of an off-screen point.
	_ = runStep("scrollIntoView", chromedp.ScrollIntoView(selector, chromedp.ByQuery), 3000)

	// JS click — runs el.click() via Runtime.evaluate. Fires synthetic
	// (untrusted) events but reliably bypasses overlays/focus-traps that
	// intercept native mouse events.
	jsClick := func() error {
		expr := fmt.Sprintf(`(function(){var e=document.querySelector(%q); if(!e) return false; e.click(); return true;})()`, selector)
		var ok bool
		jctx, jcancel := context.WithTimeout(timeoutCtx, 5*time.Second)
		defer jcancel()
		if err := chromedp.Run(jctx, chromedp.Evaluate(expr, &ok)); err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("element not found for JS click")
		}
		return nil
	}

	clickMode := ""
	if useJS {
		log.Printf("[ChromeRemote] JS click (caller-requested) on %s", selector)
		if err := jsClick(); err != nil {
			return nil, fmt.Errorf("[ChromeRemote] click step %q failed: %v", "jsClick", err)
		}
		clickMode = "js"
	} else {
		// Native CDP click first; auto-fall back to JS click on timeout so
		// callers don't have to retry on overlay-heavy pages.
		log.Printf("[ChromeRemote] Native click on %s", selector)
		nativeErr := runStep("click", chromedp.Click(selector, chromedp.ByQuery), 5000)
		if nativeErr == nil {
			clickMode = "native"
		} else {
			log.Printf("[ChromeRemote] native click failed, falling back to JS: %v", nativeErr)
			if err := jsClick(); err != nil {
				return nil, fmt.Errorf("[ChromeRemote] click failed (native: %v; JS fallback: %v)", nativeErr, err)
			}
			clickMode = "js-fallback"
		}
	}

	// Settle window for new tab detection. Some pop-ups don't appear in
	// chromedp.Targets immediately after the click returns.
	settle, settleCancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer settleCancel()
	var after []*target.Info
	_ = chromedp.Run(settle, chromedp.ActionFunc(func(c context.Context) error {
		infos, err := chromedp.Targets(c)
		if err != nil {
			return err
		}
		after = infos
		return nil
	}))

	result := &ClickResult{NavWaited: waitForNavigation, WaitedFor: waitForSelector, ClickMode: clickMode}
	for _, info := range after {
		if info.Type != "page" {
			continue
		}
		if !existing[info.TargetID] {
			result.NewTabs = append(result.NewTabs, info.TargetID.String())
		}
	}

	// Route post-click waits: if a new tab spawned, run them in the new
	// tab's chromedp context; otherwise stay in the original tab.
	waitCtx := ctx
	if len(result.NewTabs) > 0 {
		if newCtx, err := cr.getContext(result.NewTabs[0]); err == nil {
			waitCtx = newCtx
		}
	}

	// waitForNavigation: rely on the frameNavigated listener registered
	// before the click. New-tab opens count as the navigation succeeding.
	if waitForNavigation {
		if len(result.NewTabs) > 0 {
			// New tab is its own navigation; wait briefly for the body in it.
			ready, readyCancel := context.WithTimeout(waitCtx, 5*time.Second)
			_ = chromedp.Run(ready, chromedp.WaitReady("body", chromedp.ByQuery))
			readyCancel()
		} else {
			navWait, navCancel := context.WithTimeout(timeoutCtx, 10*time.Second)
			select {
			case <-navDone:
				// Frame navigated; wait for the new doc to be ready.
				_ = chromedp.Run(navWait, chromedp.WaitReady("body", chromedp.ByQuery))
			case <-navWait.Done():
				// No frameNavigated within the budget — either an SPA route
				// change (no real navigation) or a click that didn't
				// navigate. Don't error: caller can use waitForSelector.
				log.Printf("[ChromeRemote] waitForNavigation: no frameNavigated in 10s (likely SPA or no-op click)")
			}
			navCancel()
		}
	}

	// waitForSelector: poll on the routed context with its own short timeout
	// so a missing selector doesn't eat the global click budget.
	if waitForSelector != "" {
		ws, wsCancel := context.WithTimeout(waitCtx, 10*time.Second)
		err := chromedp.Run(ws, chromedp.WaitVisible(waitForSelector, chromedp.ByQuery))
		wsCancel()
		if err != nil {
			return result, fmt.Errorf("[ChromeRemote] waitForSelector %q timed out: %v", waitForSelector, err)
		}
	}

	log.Printf("[ChromeRemote] Element clicked successfully, newTabs=%v", result.NewTabs)
	return result, nil
}

// ElementInfo represents information about a clickable element on the page
type ElementInfo struct {
	Selector    string `json:"selector"`    // CSS selector to use for clicking
	TagName     string `json:"tagName"`     // HTML tag name (e.g., "button", "a", "input")
	ID          string `json:"id"`          // Element ID attribute (if present)
	Class       string `json:"class"`       // Element class attribute (if present)
	Text        string `json:"text"`        // Visible text content
	Type        string `json:"type"`        // Input type or button type (if applicable)
	Href        string `json:"href"`        // Link href (for anchor tags)
	Name        string `json:"name"`        // Name attribute (if present)
	Aria        string `json:"aria"`        // ARIA label (if present)
	Placeholder string `json:"placeholder"` // Placeholder text (for inputs)
}

// GetElements extracts information about clickable elements on the page.
// targetID can be empty to pick the best tab.
func (cr *ChromeRemote) GetElements(targetID string, targetURL string) ([]ElementInfo, error) {
	log.Printf("[ChromeRemote] Starting element extraction (targetID=%s, targetURL=%s)", targetID, targetURL)

	ctx, err := cr.getContext(targetID)
	if err != nil {
		return nil, err
	}

	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 30*time.Second)
	defer timeoutCancel()

	var tasks []chromedp.Action

	if targetURL != "" {
		log.Printf("[ChromeRemote] Navigating to URL: %s", targetURL)
		tasks = append(tasks, chromedp.Navigate(targetURL))
		tasks = append(tasks, chromedp.WaitReady("body"))
	}

	jsCode := `
	(function() {
		const elements = [];
		const clickableSelectors = [
			'button', 'a', 'input[type="button"]', 'input[type="submit"]',
			'input[type="reset"]', '[role="button"]', '[onclick]',
			'input[type="text"]', 'input[type="password"]', 'input[type="email"]',
			'input[type="search"]', 'input[type="tel"]', 'input[type="url"]',
			'input[type="number"]', 'textarea', 'select',
			'input:not([type])'
		];

		// Build a unique CSS selector path using nth-of-type from the element to the root
		function uniqueSelector(el) {
			if (el.id) return '#' + CSS.escape(el.id);
			const parts = [];
			let cur = el;
			while (cur && cur !== document.body && cur !== document.documentElement) {
				let seg = cur.tagName.toLowerCase();
				if (cur.id) {
					parts.unshift('#' + CSS.escape(cur.id));
					break;
				}
				const parent = cur.parentElement;
				if (parent) {
					const siblings = Array.from(parent.children).filter(c => c.tagName === cur.tagName);
					if (siblings.length > 1) {
						const idx = siblings.indexOf(cur) + 1;
						seg += ':nth-of-type(' + idx + ')';
					}
				}
				parts.unshift(seg);
				cur = parent;
			}
			if (parts.length === 0) return el.tagName.toLowerCase();
			return parts.join(' > ');
		}

		const seen = new Set();
		clickableSelectors.forEach(selector => {
			document.querySelectorAll(selector).forEach((el) => {
				if (seen.has(el)) return;
				seen.add(el);
				const rect = el.getBoundingClientRect();
				if (rect.width === 0 || rect.height === 0) return;
				const info = {
					tagName: el.tagName.toLowerCase(),
					id: el.id || '',
					class: (typeof el.className === 'string' ? el.className : '') || '',
					text: (el.textContent || el.value || '').trim().substring(0, 100),
					type: el.type || '',
					href: el.href || '',
					name: el.name || '',
					aria: el.getAttribute('aria-label') || '',
					placeholder: el.placeholder || ''
				};
				info.selector = uniqueSelector(el);
				elements.push(info);
			});
		});
		return elements;
	})()`

	var elements []ElementInfo
	tasks = append(tasks, chromedp.Evaluate(jsCode, &elements))

	if err := chromedp.Run(timeoutCtx, tasks...); err != nil {
		return nil, fmt.Errorf("[ChromeRemote] failed to extract elements: %v", err)
	}

	log.Printf("[ChromeRemote] Found %d clickable elements", len(elements))
	return elements, nil
}

// TabInfo represents information about a Chrome tab
type TabInfo struct {
	ID          string `json:"id"`          // Target ID
	Title       string `json:"title"`       // Page title
	URL         string `json:"url"`         // Current URL
	Type        string `json:"type"`        // Target type (usually "page")
	Description string `json:"description"` // Description
}

// ListTabs lists all open tabs in Chrome
func (cr *ChromeRemote) ListTabs() ([]TabInfo, error) {
	log.Printf("[ChromeRemote] Listing all Chrome tabs")

	var tabs []TabInfo
	if err := chromedp.Run(cr.browserCtx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			infos, err := chromedp.Targets(ctx)
			if err != nil {
				return err
			}
			for _, info := range infos {
				if info.Type == "page" {
					tabs = append(tabs, TabInfo{
						ID:          info.TargetID.String(),
						Title:       info.Title,
						URL:         info.URL,
						Type:        info.Type,
						Description: "",
					})
				}
			}
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("[ChromeRemote] failed to list tabs: %v", err)
	}

	log.Printf("[ChromeRemote] Found %d tabs", len(tabs))
	return tabs, nil
}

// OpenTab opens a new tab and returns its target ID.
// URL is optional.
func (cr *ChromeRemote) OpenTab(url string) (string, error) {
	if url == "" {
		url = "about:blank"
	}
	log.Printf("[ChromeRemote] Opening new tab with URL: %s", url)

	var targetID target.ID
	// Background:true creates the tab as a non-active tab in its window;
	// reclaimFocusWhenChromeSteals() handles the rare case where new-tab
	// creation also raises the Chrome window on macOS.
	reclaimFocusWhenChromeSteals()
	err := chromedp.Run(cr.browserCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		targetID, err = target.CreateTarget(url).WithBackground(true).Do(ctx)
		return err
	}))
	if err != nil {
		return "", fmt.Errorf("failed to create target: %v", err)
	}

	cr.mu.Lock()
	cr.lastActiveTargetID = string(targetID)
	cr.mu.Unlock()

	return string(targetID), nil
}

// NavigationResult contains the result of a navigation operation
type NavigationResult struct {
	FinalURL     string `json:"finalUrl"`
	Status       string `json:"status"` // "success", "timeout", "error"
	NavigationID string `json:"navigationId"`
}

// Navigate navigates a specific tab to a URL
func (cr *ChromeRemote) Navigate(targetID string, url string, waitUntil string, timeoutMs int) (*NavigationResult, error) {
	log.Printf("[ChromeRemote] Navigating tab %s to URL: %s (waitUntil=%s, timeout=%dms)", targetID, url, waitUntil, timeoutMs)

	if waitUntil == "" {
		waitUntil = "load"
	}
	if timeoutMs == 0 {
		timeoutMs = 30000
	}

	ctx, err := cr.getContext(targetID)
	if err != nil {
		return nil, err
	}

	timeout := time.Duration(timeoutMs) * time.Millisecond
	ctx, timeoutCancel := context.WithTimeout(ctx, timeout)
	defer timeoutCancel()

	var tasks []chromedp.Action
	tasks = append(tasks, chromedp.Navigate(url))

	switch waitUntil {
	case "domcontentloaded":
		tasks = append(tasks, chromedp.WaitReady("body"))
	case "load":
		tasks = append(tasks, chromedp.WaitReady("body"))
	case "networkidle":
		tasks = append(tasks, chromedp.WaitReady("body"))
		tasks = append(tasks, chromedp.Sleep(500*time.Millisecond))
	default:
		return nil, fmt.Errorf("[ChromeRemote] invalid waitUntil: %s", waitUntil)
	}

	startTime := time.Now()
	var finalURL string
	tasks = append(tasks, chromedp.Location(&finalURL))

	err = chromedp.Run(ctx, tasks...)

	result := &NavigationResult{
		FinalURL:     finalURL,
		NavigationID: fmt.Sprintf("nav_%d", time.Now().UnixNano()),
	}

	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			result.Status = "timeout"
			return result, fmt.Errorf("navigation timeout after %dms", timeoutMs)
		}
		result.Status = "error"
		return result, fmt.Errorf("[ChromeRemote] failed to navigate: %v", err)
	}

	result.Status = "success"
	if targetID != "" {
		cr.mu.Lock()
		cr.lastActiveTargetID = targetID
		cr.mu.Unlock()
	}
	log.Printf("[ChromeRemote] Navigation successful in %v", time.Since(startTime))
	return result, nil
}

// ActivateTab switches focus to a specific tab
func (cr *ChromeRemote) ActivateTab(targetID string) error {
	log.Printf("[ChromeRemote] Activating tab: %s", targetID)
	if err := chromedp.Run(cr.browserCtx, target.ActivateTarget(target.ID(targetID))); err != nil {
		return err
	}
	cr.mu.Lock()
	cr.lastActiveTargetID = targetID
	cr.mu.Unlock()
	return nil
}

// CloseTab closes a specific tab.
//
// Running target.CloseTarget via chromedp.Run on a chromedp-managed context
// fails with "to close the target, cancel its context or use chromedp.Cancel".
// chromedp.Cancel sends Target.closeTarget over the target's own session and
// then waits for the listener to disconnect — that is the documented way to
// close a chromedp-managed target.
func (cr *ChromeRemote) CloseTab(targetID string) error {
	log.Printf("[ChromeRemote] Closing tab: %s", targetID)
	if targetID == "" {
		return fmt.Errorf("targetID is required")
	}

	// Pop the cached context so concurrent callers can't reuse a context that's
	// about to be cancelled.
	cr.mu.Lock()
	ctx, hasCtx := cr.targetCtxs[targetID]
	cancel := cr.targetCancel[targetID]
	if hasCtx {
		delete(cr.targetCtxs, targetID)
		delete(cr.targetCancel, targetID)
	}
	cr.mu.Unlock()

	var closeCtx context.Context
	var closeCancel context.CancelFunc
	if hasCtx {
		closeCtx, closeCancel = ctx, cancel
	} else {
		// No cached chromedp context — attach a temporary one bound to this
		// target so chromedp.Cancel has a target-scoped context to operate on.
		closeCtx, closeCancel = chromedp.NewContext(cr.browserCtx, chromedp.WithTargetID(target.ID(targetID)))
		if err := chromedp.Run(closeCtx); err != nil {
			closeCancel()
			return fmt.Errorf("failed to attach to target %s: %v", targetID, err)
		}
	}

	if err := chromedp.Cancel(closeCtx); err != nil {
		closeCancel()
		return fmt.Errorf("failed to close target %s: %v", targetID, err)
	}
	closeCancel()
	return nil
}

// ReloadTab reloads a specific tab.
//
// Uses raw Page.reload so the call returns as soon as Chrome ACKs the
// command instead of blocking until Page.frameStoppedLoading fires —
// which on ad/analytics-heavy sites can take 10+ seconds. timeoutMs
// caps the ACK round-trip; the actual reload continues in the browser
// after we return. Default 5s.
func (cr *ChromeRemote) ReloadTab(targetID string, bypassCache bool, timeoutMs int) error {
	log.Printf("[ChromeRemote] Reloading tab %s (bypassCache=%v, timeoutMs=%d)", targetID, bypassCache, timeoutMs)
	ctx, err := cr.getContext(targetID)
	if err != nil {
		return err
	}
	if timeoutMs <= 0 {
		timeoutMs = 5000
	}
	tctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()
	return chromedp.Run(tctx, chromedp.ActionFunc(func(c context.Context) error {
		return page.Reload().WithIgnoreCache(bypassCache).Do(c)
	}))
}

// GoBack navigates back in browser history.
//
// Uses raw Page.getNavigationHistory + Page.navigateToHistoryEntry so the
// call returns immediately after Chrome ACKs instead of blocking on
// frameStoppedLoading. The previous chromedp.NavigateBack() helper waited
// for full load completion, which was unbearably slow on heavy pages.
// Callers that need to wait can chain proxyWaitForSelector.
func (cr *ChromeRemote) GoBack(targetID string, timeoutMs int) error {
	log.Printf("[ChromeRemote] Going back in tab: %s (timeoutMs=%d)", targetID, timeoutMs)
	ctx, err := cr.getContext(targetID)
	if err != nil {
		return err
	}
	if timeoutMs <= 0 {
		timeoutMs = 5000
	}
	tctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()
	return chromedp.Run(tctx, chromedp.ActionFunc(func(c context.Context) error {
		current, entries, err := page.GetNavigationHistory().Do(c)
		if err != nil {
			return err
		}
		idx := int(current) - 1
		if idx < 0 || idx >= len(entries) {
			return fmt.Errorf("no history entry to go back to")
		}
		return page.NavigateToHistoryEntry(entries[idx].ID).Do(c)
	}))
}

// GoForward navigates forward in browser history. Same fast-path semantics
// as GoBack — fires Page.navigateToHistoryEntry and returns without waiting
// for frameStoppedLoading.
func (cr *ChromeRemote) GoForward(targetID string, timeoutMs int) error {
	log.Printf("[ChromeRemote] Going forward in tab: %s (timeoutMs=%d)", targetID, timeoutMs)
	ctx, err := cr.getContext(targetID)
	if err != nil {
		return err
	}
	if timeoutMs <= 0 {
		timeoutMs = 5000
	}
	tctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()
	return chromedp.Run(tctx, chromedp.ActionFunc(func(c context.Context) error {
		current, entries, err := page.GetNavigationHistory().Do(c)
		if err != nil {
			return err
		}
		idx := int(current) + 1
		if idx < 0 || idx >= len(entries) {
			return fmt.Errorf("no history entry to go forward to")
		}
		return page.NavigateToHistoryEntry(entries[idx].ID).Do(c)
	}))
}

// DebugURL returns the Chrome remote debugging WebSocket URL.
func (cr *ChromeRemote) DebugURL() string {
	return cr.debugURL
}

// Evaluate runs a JavaScript expression in the context of the given tab
// and unmarshals the result into dest (same semantics as chromedp.Evaluate).
// timeoutMs controls the per-call timeout; 0 uses a 15 s default.
func (cr *ChromeRemote) Evaluate(targetID string, jsExpr string, dest interface{}, timeoutMs int) error {
	ctx, err := cr.getContext(targetID)
	if err != nil {
		return err
	}
	if timeoutMs <= 0 {
		timeoutMs = 15000
	}
	tctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()
	return chromedp.Run(tctx, chromedp.Evaluate(jsExpr, dest))
}

// TypeText types text into an element identified by a CSS selector using CDP.
//
// Bing-style overlay/focus-trap inputs used to wedge the original
// "WaitVisible → Click → SendKeys all in one Run" flow until the global
// timeout expired. This implementation breaks the operation into discrete
// steps each with its own short timeout so the caller learns *which* step
// failed (visible / scroll / focus / clear / sendKeys) instead of getting a
// generic timeout. The total wall-clock budget is still capped by timeoutMs.
func (cr *ChromeRemote) TypeText(targetID string, selector string, text string, clearFirst bool, timeoutMs int) error {
	log.Printf("[ChromeRemote] Typing text into %s (targetID=%s, clearFirst=%v)", selector, targetID, clearFirst)

	ctx, err := cr.getContext(targetID)
	if err != nil {
		return err
	}
	if timeoutMs <= 0 {
		timeoutMs = 15000
	}
	overall, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	runStep := func(name string, action chromedp.Action, perStepMs int) error {
		sctx, scancel := context.WithTimeout(overall, time.Duration(perStepMs)*time.Millisecond)
		defer scancel()
		if err := chromedp.Run(sctx, action); err != nil {
			return fmt.Errorf("[ChromeRemote] type step %q failed: %v", name, err)
		}
		return nil
	}

	if err := runStep("waitVisible", chromedp.WaitVisible(selector, chromedp.ByQuery), 5000); err != nil {
		return err
	}
	if err := runStep("scrollIntoView", chromedp.ScrollIntoView(selector, chromedp.ByQuery), 3000); err != nil {
		return err
	}
	if err := runStep("focus", chromedp.Focus(selector, chromedp.ByQuery), 3000); err != nil {
		return err
	}
	if clearFirst {
		clearJS := fmt.Sprintf(`(function(){var e=document.querySelector(%q); if(e){e.value=''; e.dispatchEvent(new Event('input',{bubbles:true}));}})()`, selector)
		if err := runStep("clear", chromedp.Evaluate(clearJS, nil), 2000); err != nil {
			return err
		}
	}
	if err := runStep("sendKeys", chromedp.SendKeys(selector, text, chromedp.ByQuery), 5000); err != nil {
		return err
	}

	log.Printf("[ChromeRemote] Text typed successfully into %s", selector)
	return nil
}

// SetValue sets an input/textarea element's value through the framework-
// friendly path: it uses the prototype's native value setter (so React's
// internal tracker sees the change) and dispatches input + change events
// with bubbles:true so Vue/Angular/React state syncs to the DOM.
//
// This is the workaround for sites where SendKeys-based proxyType wedges
// (Bug 2 on Bing) and where naive `el.value = ...` via proxyEval doesn't
// notify the framework.
func (cr *ChromeRemote) SetValue(targetID string, selector string, value string, timeoutMs int) error {
	log.Printf("[ChromeRemote] SetValue selector=%s (targetID=%s)", selector, targetID)
	if selector == "" {
		return fmt.Errorf("selector is required")
	}
	ctx, err := cr.getContext(targetID)
	if err != nil {
		return err
	}
	if timeoutMs <= 0 {
		timeoutMs = 5000
	}
	tctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	js := fmt.Sprintf(`(function(){
  var el = document.querySelector(%q);
  if (!el) return { ok: false, error: "selector not found" };
  var proto = el.tagName === 'TEXTAREA'
    ? HTMLTextAreaElement.prototype
    : (el.tagName === 'SELECT' ? HTMLSelectElement.prototype : HTMLInputElement.prototype);
  var desc = Object.getOwnPropertyDescriptor(proto, 'value');
  if (desc && desc.set) {
    desc.set.call(el, %q);
  } else {
    el.value = %q;
  }
  el.dispatchEvent(new Event('input',  { bubbles: true }));
  el.dispatchEvent(new Event('change', { bubbles: true }));
  return { ok: true, tag: el.tagName };
})()`, selector, value, value)

	var res struct {
		Ok    bool   `json:"ok"`
		Error string `json:"error"`
		Tag   string `json:"tag"`
	}
	if err := chromedp.Run(tctx, chromedp.Evaluate(js, &res)); err != nil {
		return fmt.Errorf("[ChromeRemote] failed to set value: %v", err)
	}
	if !res.Ok {
		return fmt.Errorf("[ChromeRemote] failed to set value: %s", res.Error)
	}
	return nil
}

// WaitForSelector waits for a CSS selector to be present on the page.
//
// state controls the predicate:
//   - "attached" (default, empty): wait for the element to exist in the DOM
//     (chromedp.WaitReady). This is what most callers actually want.
//   - "visible": wait for the element to also be visible — non-zero size,
//     not display:none, etc. (chromedp.WaitVisible). Strict; many "real"
//     elements (collapsed <details>, off-screen sticky bars, nodes with
//     opacity:0) fail this even when they exist on the page.
//
// The selector is always evaluated through chromedp.ByQuery (i.e.
// document.querySelector), so it accepts any standard CSS selector and
// produces consistent results — instead of chromedp's default
// DOM.performSearch path which has different matching semantics.
func (cr *ChromeRemote) WaitForSelector(targetID string, selector string, timeoutMs int, state string) error {
	log.Printf("[ChromeRemote] Waiting for selector %s (targetID=%s, timeout=%dms, state=%q)", selector, targetID, timeoutMs, state)

	ctx, err := cr.getContext(targetID)
	if err != nil {
		return err
	}
	if timeoutMs <= 0 {
		timeoutMs = 30000
	}
	tctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	var action chromedp.Action
	switch state {
	case "", "attached":
		action = chromedp.WaitReady(selector, chromedp.ByQuery)
	case "visible":
		action = chromedp.WaitVisible(selector, chromedp.ByQuery)
	default:
		return fmt.Errorf("[ChromeRemote] invalid state %q for waitForSelector (must be \"attached\" or \"visible\")", state)
	}

	if err := chromedp.Run(tctx, action); err != nil {
		effectiveState := state
		if effectiveState == "" {
			effectiveState = "attached"
		}
		return fmt.Errorf("[ChromeRemote] timeout waiting for selector %s (state=%s): %v", selector, effectiveState, err)
	}

	log.Printf("[ChromeRemote] Selector %s found", selector)
	return nil
}

// --- Legacy Wrappers (Deprecated) ---

func TakeChromeScreenshot(debugURL string, targetID string, fullPage bool) ([]byte, error) {
	cr, err := NewChromeRemote(debugURL)
	if err != nil {
		return nil, err
	}
	defer cr.Close()
	return cr.TakeScreenshot(targetID, fullPage, 0)
}

func ClickChromeElement(debugURL string, targetURL string, selector string, waitForNavigation bool) error {
	cr, err := NewChromeRemote(debugURL)
	if err != nil {
		return err
	}
	defer cr.Close()
	_, err = cr.ClickElement("", targetURL, selector, waitForNavigation, "", false)
	return err
}

func GetChromeElements(debugURL string, targetURL string) ([]ElementInfo, error) {
	cr, err := NewChromeRemote(debugURL)
	if err != nil {
		return nil, err
	}
	defer cr.Close()
	return cr.GetElements("", targetURL)
}

func ListChromeTabs(debugURL string) ([]TabInfo, error) {
	cr, err := NewChromeRemote(debugURL)
	if err != nil {
		return nil, err
	}
	defer cr.Close()
	return cr.ListTabs()
}

func OpenChromeTab(debugURL string, url string) (string, error) {
	cr, err := NewChromeRemote(debugURL)
	if err != nil {
		return "", err
	}
	defer cr.Close()
	return cr.OpenTab(url)
}

func NavigateToUrl(debugURL string, targetID string, url string, waitUntil string, timeoutMs int) (*NavigationResult, error) {
	cr, err := NewChromeRemote(debugURL)
	if err != nil {
		return nil, err
	}
	defer cr.Close()
	return cr.Navigate(targetID, url, waitUntil, timeoutMs)
}

func ActivateTab(debugURL string, targetID string) error {
	cr, err := NewChromeRemote(debugURL)
	if err != nil {
		return err
	}
	defer cr.Close()
	return cr.ActivateTab(targetID)
}

func CloseTab(debugURL string, targetID string) error {
	cr, err := NewChromeRemote(debugURL)
	if err != nil {
		return err
	}
	defer cr.Close()
	return cr.CloseTab(targetID)
}

func ReloadTab(debugURL string, targetID string, bypassCache bool) error {
	cr, err := NewChromeRemote(debugURL)
	if err != nil {
		return err
	}
	defer cr.Close()
	return cr.ReloadTab(targetID, bypassCache, 0)
}

func GoBack(debugURL string, targetID string) error {
	cr, err := NewChromeRemote(debugURL)
	if err != nil {
		return err
	}
	defer cr.Close()
	return cr.GoBack(targetID, 0)
}

func GoForward(debugURL string, targetID string) error {
	cr, err := NewChromeRemote(debugURL)
	if err != nil {
		return err
	}
	defer cr.Close()
	return cr.GoForward(targetID, 0)
}
