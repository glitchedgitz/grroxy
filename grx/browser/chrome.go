package browser

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

func launchChrome(proxyAddress string, customCertPath string, profileDir string) (*exec.Cmd, error) {
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
		"grroxy.com",
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
	return cmd, nil
}

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

// TakeChromeScreenshot captures a screenshot using Chrome DevTools Protocol
// debugURL: WebSocket URL to connect to Chrome (from GetChromeDebugURL)
// targetURL: URL to navigate to before capturing (empty string = capture current page)
// fullPage: If true, captures the entire page; if false, captures only the viewport
func TakeChromeScreenshot(debugURL string, targetURL string, fullPage bool) ([]byte, error) {
	log.Printf("[TakeChromeScreenshot] Starting screenshot capture (fullPage=%v, targetURL=%s)", fullPage, targetURL)

	// Create allocator context to connect to existing Chrome instance
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), debugURL)
	defer allocCancel()

	// Create chrome context
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// Set timeout for the entire operation
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var buf []byte
	var tasks []chromedp.Action

	// Navigate to URL if provided
	if targetURL != "" {
		log.Printf("[TakeChromeScreenshot] Navigating to URL: %s", targetURL)
		tasks = append(tasks, chromedp.Navigate(targetURL))
		// Wait for page to be ready
		tasks = append(tasks, chromedp.WaitReady("body"))
	}

	// Capture screenshot
	if fullPage {
		log.Printf("[TakeChromeScreenshot] Capturing full page screenshot")
		tasks = append(tasks, chromedp.FullScreenshot(&buf, 90))
	} else {
		log.Printf("[TakeChromeScreenshot] Capturing viewport screenshot")
		tasks = append(tasks, chromedp.CaptureScreenshot(&buf))
	}

	// Execute all tasks
	if err := chromedp.Run(ctx, tasks...); err != nil {
		return nil, fmt.Errorf("[TakeChromeScreenshot] failed to capture screenshot: %v", err)
	}

	log.Printf("[TakeChromeScreenshot] Screenshot captured successfully (%d bytes)", len(buf))
	return buf, nil
}

// ClickChromeElement clicks an element on the page using Chrome DevTools Protocol
// debugURL: WebSocket URL to connect to Chrome (from GetChromeDebugURL)
// targetURL: URL to navigate to before clicking (empty string = use current page)
// selector: CSS selector for the element to click (e.g., "#button-id", ".class-name", "button[type='submit']")
// waitForNavigation: If true, waits for navigation after click (useful for form submissions)
func ClickChromeElement(debugURL string, targetURL string, selector string, waitForNavigation bool) error {
	log.Printf("[ClickChromeElement] Starting click operation (selector=%s, targetURL=%s, waitNav=%v)",
		selector, targetURL, waitForNavigation)

	// Create allocator context to connect to existing Chrome instance
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), debugURL)
	defer allocCancel()

	// Create chrome context
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// Set timeout for the entire operation
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var tasks []chromedp.Action

	// Navigate to URL if provided
	if targetURL != "" {
		log.Printf("[ClickChromeElement] Navigating to URL: %s", targetURL)
		tasks = append(tasks, chromedp.Navigate(targetURL))
		// Wait for page to be ready
		tasks = append(tasks, chromedp.WaitReady("body"))
	}

	// Wait for the element to be visible
	log.Printf("[ClickChromeElement] Waiting for element: %s", selector)
	tasks = append(tasks, chromedp.WaitVisible(selector))

	// Click the element
	log.Printf("[ClickChromeElement] Clicking element: %s", selector)
	if waitForNavigation {
		tasks = append(tasks, chromedp.Click(selector, chromedp.ByQuery))
		// Wait for navigation to complete
		tasks = append(tasks, chromedp.WaitReady("body"))
	} else {
		tasks = append(tasks, chromedp.Click(selector, chromedp.ByQuery))
	}

	// Execute all tasks
	if err := chromedp.Run(ctx, tasks...); err != nil {
		return fmt.Errorf("[ClickChromeElement] failed to click element: %v", err)
	}

	log.Printf("[ClickChromeElement] Element clicked successfully")
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

// GetChromeElements extracts information about clickable elements on the page
// debugURL: WebSocket URL to connect to Chrome (from GetChromeDebugURL)
// targetURL: URL to navigate to before extracting (empty string = use current page)
func GetChromeElements(debugURL string, targetURL string) ([]ElementInfo, error) {
	log.Printf("[GetChromeElements] Starting element extraction (targetURL=%s)", targetURL)

	// Create allocator context to connect to existing Chrome instance
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), debugURL)
	defer allocCancel()

	// Create chrome context
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// Set timeout for the entire operation
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var tasks []chromedp.Action

	// Navigate to URL if provided
	if targetURL != "" {
		log.Printf("[GetChromeElements] Navigating to URL: %s", targetURL)
		tasks = append(tasks, chromedp.Navigate(targetURL))
		tasks = append(tasks, chromedp.WaitReady("body"))
	}

	// JavaScript to extract clickable elements information
	jsCode := `
	(function() {
		const elements = [];
		const clickableSelectors = [
			'button', 'a', 'input[type="button"]', 'input[type="submit"]', 
			'input[type="reset"]', '[role="button"]', '[onclick]'
		];
		
		const seen = new Set();
		
		clickableSelectors.forEach(selector => {
			document.querySelectorAll(selector).forEach((el, index) => {
				if (seen.has(el)) return;
				seen.add(el);
				
				const rect = el.getBoundingClientRect();
				if (rect.width === 0 || rect.height === 0) return; // Skip hidden elements
				
				const info = {
					tagName: el.tagName.toLowerCase(),
					id: el.id || '',
					class: el.className || '',
					text: (el.textContent || el.value || '').trim().substring(0, 100),
					type: el.type || '',
					href: el.href || '',
					name: el.name || '',
					aria: el.getAttribute('aria-label') || '',
					placeholder: el.placeholder || ''
				};
				
				// Build selector (prefer ID, then class, then tag)
				let selectorStr = el.tagName.toLowerCase();
				if (info.id) {
					selectorStr = '#' + info.id;
				} else if (info.class) {
					const firstClass = info.class.split(' ')[0];
					selectorStr = selectorStr + '.' + firstClass;
				}
				
				// Make selector more specific if needed
				if (el.name) {
					selectorStr = selectorStr + '[name="' + el.name + '"]';
				} else if (el.type) {
					selectorStr = selectorStr + '[type="' + el.type + '"]';
				}
				
				info.selector = selectorStr;
				elements.push(info);
			});
		});
		
		return elements;
	})()
	`

	var elements []ElementInfo
	tasks = append(tasks, chromedp.Evaluate(jsCode, &elements))

	// Execute all tasks
	if err := chromedp.Run(ctx, tasks...); err != nil {
		return nil, fmt.Errorf("[GetChromeElements] failed to extract elements: %v", err)
	}

	log.Printf("[GetChromeElements] Found %d clickable elements", len(elements))
	return elements, nil
}
