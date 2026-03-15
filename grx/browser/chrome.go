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

	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

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
	mu           sync.Mutex
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
func (cr *ChromeRemote) getContext(targetID string) (context.Context, error) {
	if targetID == "" {
		// If no target ID, try to pick one
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
func (cr *ChromeRemote) TakeScreenshot(targetID string, fullPage bool) ([]byte, error) {
	log.Printf("[ChromeRemote] Starting screenshot (targetID=%s, fullPage=%v)", targetID, fullPage)

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

	// Timeout for the screenshot operation
	ctx, timeoutCancel := context.WithTimeout(ctx, 30*time.Second)
	defer timeoutCancel()

	// Bring it to front
	_ = chromedp.Run(ctx, chromedp.ActionFunc(func(c context.Context) error {
		tid := chromedp.FromContext(c).Target.TargetID
		return target.ActivateTarget(tid).Do(c)
	}))

	var buf []byte
	var tasks []chromedp.Action

	// Wait for body to be ready
	tasks = append(tasks, chromedp.WaitReady("body", chromedp.ByQuery))

	if fullPage {
		tasks = append(tasks, chromedp.FullScreenshot(&buf, 90))
	} else {
		tasks = append(tasks, chromedp.CaptureScreenshot(&buf))
	}

	if err := chromedp.Run(ctx, tasks...); err != nil {
		return nil, fmt.Errorf("[ChromeRemote] failed to capture screenshot: %v", err)
	}

	log.Printf("[ChromeRemote] Screenshot captured (%d bytes)", len(buf))
	return buf, nil
}

// ClickElement clicks an element on the page using Chrome DevTools Protocol.
// targetID can be empty to pick the best tab.
func (cr *ChromeRemote) ClickElement(targetID string, targetURL string, selector string, waitForNavigation bool) error {
	log.Printf("[ChromeRemote] Starting click operation (targetID=%s, selector=%s, targetURL=%s, waitNav=%v)",
		targetID, selector, targetURL, waitForNavigation)

	ctx, err := cr.getContext(targetID)
	if err != nil {
		return err
	}

	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 30*time.Second)
	defer timeoutCancel()

	var tasks []chromedp.Action

	// Navigate to URL if provided
	if targetURL != "" {
		log.Printf("[ChromeRemote] Navigating to URL: %s", targetURL)
		tasks = append(tasks, chromedp.Navigate(targetURL))
		tasks = append(tasks, chromedp.WaitReady("body"))
	}

	// Wait for the element to be visible
	log.Printf("[ChromeRemote] Waiting for element: %s", selector)
	tasks = append(tasks, chromedp.WaitVisible(selector))

	// Click the element
	log.Printf("[ChromeRemote] Clicking element: %s", selector)
	tasks = append(tasks, chromedp.Click(selector, chromedp.ByQuery))
	if waitForNavigation {
		tasks = append(tasks, chromedp.WaitReady("body"))
	}

	if err := chromedp.Run(timeoutCtx, tasks...); err != nil {
		return fmt.Errorf("[ChromeRemote] failed to click element: %v", err)
	}

	log.Printf("[ChromeRemote] Element clicked successfully")
	return nil
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
	err := chromedp.Run(cr.browserCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		targetID, err = target.CreateTarget(url).Do(ctx)
		return err
	}))
	if err != nil {
		return "", fmt.Errorf("failed to create target: %v", err)
	}

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
	log.Printf("[ChromeRemote] Navigation successful in %v", time.Since(startTime))
	return result, nil
}

// ActivateTab switches focus to a specific tab
func (cr *ChromeRemote) ActivateTab(targetID string) error {
	log.Printf("[ChromeRemote] Activating tab: %s", targetID)
	return chromedp.Run(cr.browserCtx, target.ActivateTarget(target.ID(targetID)))
}

// CloseTab closes a specific tab
func (cr *ChromeRemote) CloseTab(targetID string) error {
	log.Printf("[ChromeRemote] Closing tab: %s", targetID)

	// Clean up cache first
	cr.CloseTargetContext(targetID)

	err := chromedp.Run(cr.browserCtx, target.CloseTarget(target.ID(targetID)))
	if err != nil {
		return fmt.Errorf("failed to close target %s: %v", targetID, err)
	}
	return nil
}

// ReloadTab reloads a specific tab
func (cr *ChromeRemote) ReloadTab(targetID string, bypassCache bool) error {
	log.Printf("[ChromeRemote] Reloading tab %s (bypassCache=%v)", targetID, bypassCache)
	ctx, err := cr.getContext(targetID)
	if err != nil {
		return err
	}

	return chromedp.Run(ctx, chromedp.Reload())
}

// GoBack navigates back in browser history
func (cr *ChromeRemote) GoBack(targetID string) error {
	log.Printf("[ChromeRemote] Going back in tab: %s", targetID)
	ctx, err := cr.getContext(targetID)
	if err != nil {
		return err
	}
	return chromedp.Run(ctx, chromedp.NavigateBack())
}

// GoForward navigates forward in browser history
func (cr *ChromeRemote) GoForward(targetID string) error {
	log.Printf("[ChromeRemote] Going forward in tab: %s", targetID)
	ctx, err := cr.getContext(targetID)
	if err != nil {
		return err
	}
	return chromedp.Run(ctx, chromedp.NavigateForward())
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
// It clicks the element first to focus it, clears any existing value, then types the text.
func (cr *ChromeRemote) TypeText(targetID string, selector string, text string, clearFirst bool, timeoutMs int) error {
	log.Printf("[ChromeRemote] Typing text into %s (targetID=%s, clearFirst=%v)", selector, targetID, clearFirst)

	ctx, err := cr.getContext(targetID)
	if err != nil {
		return err
	}
	if timeoutMs <= 0 {
		timeoutMs = 15000
	}
	tctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	var tasks []chromedp.Action

	// Wait for element and click to focus
	tasks = append(tasks, chromedp.WaitVisible(selector))
	tasks = append(tasks, chromedp.Click(selector, chromedp.ByQuery))

	if clearFirst {
		// Clear existing value via JS
		clearJS := fmt.Sprintf(`document.querySelector(%q).value = ''`, selector)
		tasks = append(tasks, chromedp.Evaluate(clearJS, nil))
	}

	// Type the text using SendKeys (dispatches real key events)
	tasks = append(tasks, chromedp.SendKeys(selector, text, chromedp.ByQuery))

	if err := chromedp.Run(tctx, tasks...); err != nil {
		return fmt.Errorf("[ChromeRemote] failed to type text: %v", err)
	}

	log.Printf("[ChromeRemote] Text typed successfully into %s", selector)
	return nil
}

// WaitForSelector waits for a CSS selector to become visible on the page.
func (cr *ChromeRemote) WaitForSelector(targetID string, selector string, timeoutMs int) error {
	log.Printf("[ChromeRemote] Waiting for selector %s (targetID=%s, timeout=%dms)", selector, targetID, timeoutMs)

	ctx, err := cr.getContext(targetID)
	if err != nil {
		return err
	}
	if timeoutMs <= 0 {
		timeoutMs = 30000
	}
	tctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	if err := chromedp.Run(tctx, chromedp.WaitVisible(selector)); err != nil {
		return fmt.Errorf("[ChromeRemote] timeout waiting for selector %s: %v", selector, err)
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
	return cr.TakeScreenshot(targetID, fullPage)
}

func ClickChromeElement(debugURL string, targetURL string, selector string, waitForNavigation bool) error {
	cr, err := NewChromeRemote(debugURL)
	if err != nil {
		return err
	}
	defer cr.Close()
	return cr.ClickElement("", targetURL, selector, waitForNavigation)
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
	return cr.ReloadTab(targetID, bypassCache)
}

func GoBack(debugURL string, targetID string) error {
	cr, err := NewChromeRemote(debugURL)
	if err != nil {
		return err
	}
	defer cr.Close()
	return cr.GoBack(targetID)
}

func GoForward(debugURL string, targetID string) error {
	cr, err := NewChromeRemote(debugURL)
	if err != nil {
		return err
	}
	defer cr.Close()
	return cr.GoForward(targetID)
}
