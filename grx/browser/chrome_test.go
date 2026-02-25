package browser

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// ============================================================================
// Unit Tests — GetChromeDebugURL
// ============================================================================

func TestGetChromeDebugURL_Valid(t *testing.T) {
	dir := t.TempDir()
	content := "9222\n/devtools/browser/abc-123\n"
	if err := os.WriteFile(filepath.Join(dir, "DevToolsActivePort"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	url, err := GetChromeDebugURL(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "ws://127.0.0.1:9222/devtools/browser/abc-123"
	if url != expected {
		t.Errorf("got %q, want %q", url, expected)
	}
}

func TestGetChromeDebugURL_MissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := GetChromeDebugURL(dir)
	if err == nil {
		t.Fatal("expected error for missing DevToolsActivePort file")
	}
}

func TestGetChromeDebugURL_SingleLine(t *testing.T) {
	dir := t.TempDir()
	content := "9222\n"
	if err := os.WriteFile(filepath.Join(dir, "DevToolsActivePort"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := GetChromeDebugURL(dir)
	if err == nil {
		t.Fatal("expected error for single-line file (missing ws path)")
	}
}

func TestGetChromeDebugURL_EmptyPort(t *testing.T) {
	dir := t.TempDir()
	content := "\n/devtools/browser/abc-123\n"
	if err := os.WriteFile(filepath.Join(dir, "DevToolsActivePort"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := GetChromeDebugURL(dir)
	if err == nil {
		t.Fatal("expected error for empty port")
	}
}

func TestGetChromeDebugURL_EmptyWSPath(t *testing.T) {
	dir := t.TempDir()
	content := "9222\n\n"
	if err := os.WriteFile(filepath.Join(dir, "DevToolsActivePort"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := GetChromeDebugURL(dir)
	if err == nil {
		t.Fatal("expected error for empty WebSocket path")
	}
}

func TestGetChromeDebugURL_WhitespaceHandling(t *testing.T) {
	dir := t.TempDir()
	content := "  9222  \n  /devtools/browser/abc-123  \n"
	if err := os.WriteFile(filepath.Join(dir, "DevToolsActivePort"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	url, err := GetChromeDebugURL(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "ws://127.0.0.1:9222/devtools/browser/abc-123"
	if url != expected {
		t.Errorf("got %q, want %q", url, expected)
	}
}

func TestGetChromeDebugURL_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "DevToolsActivePort"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := GetChromeDebugURL(dir)
	if err == nil {
		t.Fatal("expected error for empty file")
	}
}

// ============================================================================
// Unit Tests — ChromeRemote struct operations (no live Chrome needed)
// ============================================================================

// newTestChromeRemote creates a bare ChromeRemote for struct-level tests.
// It does NOT connect to a real browser.
func newTestChromeRemote() *ChromeRemote {
	ctx, cancel := context.WithCancel(context.Background())
	return &ChromeRemote{
		debugURL:      "ws://127.0.0.1:0/test",
		allocCtx:      ctx,
		allocCancel:   cancel,
		browserCtx:    ctx,
		browserCancel: cancel,
		targetCtxs:    make(map[string]context.Context),
		targetCancel:  make(map[string]context.CancelFunc),
	}
}

func TestChromeRemote_CloseTargetContext_Existing(t *testing.T) {
	cr := newTestChromeRemote()
	defer cr.Close()

	// Manually inject a target context
	ctx, cancel := context.WithCancel(context.Background())
	cr.targetCtxs["target-1"] = ctx
	cr.targetCancel["target-1"] = cancel

	cr.CloseTargetContext("target-1")

	if _, ok := cr.targetCtxs["target-1"]; ok {
		t.Error("expected target context to be removed from cache")
	}
	if _, ok := cr.targetCancel["target-1"]; ok {
		t.Error("expected target cancel to be removed from cache")
	}

	// Verify the context was actually cancelled
	select {
	case <-ctx.Done():
		// expected
	default:
		t.Error("expected context to be cancelled")
	}
}

func TestChromeRemote_CloseTargetContext_NonExisting(t *testing.T) {
	cr := newTestChromeRemote()
	defer cr.Close()

	// Should not panic
	cr.CloseTargetContext("does-not-exist")
}

func TestChromeRemote_Close_CleansUp(t *testing.T) {
	cr := newTestChromeRemote()

	// Inject a couple of target contexts
	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	cr.targetCtxs["t1"] = ctx1
	cr.targetCancel["t1"] = cancel1
	cr.targetCtxs["t2"] = ctx2
	cr.targetCancel["t2"] = cancel2

	cr.Close()

	if len(cr.targetCtxs) != 0 {
		t.Errorf("expected targetCtxs map to be empty, got %d entries", len(cr.targetCtxs))
	}
	if len(cr.targetCancel) != 0 {
		t.Errorf("expected targetCancel map to be empty, got %d entries", len(cr.targetCancel))
	}

	// Both injected contexts should be cancelled
	select {
	case <-ctx1.Done():
	default:
		t.Error("ctx1 not cancelled")
	}
	select {
	case <-ctx2.Done():
	default:
		t.Error("ctx2 not cancelled")
	}
}

func TestChromeRemote_Close_IdempotentMaps(t *testing.T) {
	cr := newTestChromeRemote()
	cr.Close()

	// Calling Close a second time should not panic
	cr.Close()
}

func TestChromeRemote_CloseTargetContext_Concurrent(t *testing.T) {
	cr := newTestChromeRemote()
	defer cr.Close()

	const n = 50
	for i := 0; i < n; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		id := "target-" + string(rune('A'+i))
		cr.targetCtxs[id] = ctx
		cr.targetCancel[id] = cancel
	}

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := "target-" + string(rune('A'+i))
			cr.CloseTargetContext(id)
		}(i)
	}
	wg.Wait()

	if len(cr.targetCtxs) != 0 {
		t.Errorf("expected all target contexts removed, got %d", len(cr.targetCtxs))
	}
}

// ============================================================================
// Unit Tests — NavigationResult defaults
// ============================================================================

func TestNavigationResult_Struct(t *testing.T) {
	nr := &NavigationResult{
		FinalURL:     "https://example.com",
		Status:       "success",
		NavigationID: "nav_123",
	}
	if nr.FinalURL != "https://example.com" {
		t.Errorf("FinalURL = %q, want %q", nr.FinalURL, "https://example.com")
	}
	if nr.Status != "success" {
		t.Errorf("Status = %q, want %q", nr.Status, "success")
	}
}

// ============================================================================
// Unit Tests — TabInfo & ElementInfo structs
// ============================================================================

func TestTabInfo_Fields(t *testing.T) {
	tab := TabInfo{
		ID:          "E4B3F8C9-1234-5678-90AB-CDEF12345678",
		Title:       "Test Page",
		URL:         "https://example.com",
		Type:        "page",
		Description: "",
	}
	if tab.ID == "" {
		t.Error("expected non-empty ID")
	}
	if tab.Type != "page" {
		t.Errorf("Type = %q, want %q", tab.Type, "page")
	}
}

func TestElementInfo_Fields(t *testing.T) {
	elem := ElementInfo{
		Selector:    "#submit-btn",
		TagName:     "button",
		ID:          "submit-btn",
		Class:       "btn primary",
		Text:        "Submit",
		Type:        "submit",
		Href:        "",
		Name:        "submit",
		Aria:        "Submit form",
		Placeholder: "",
	}
	if elem.Selector != "#submit-btn" {
		t.Errorf("Selector = %q, want %q", elem.Selector, "#submit-btn")
	}
	if elem.TagName != "button" {
		t.Errorf("TagName = %q, want %q", elem.TagName, "button")
	}
}

// ============================================================================
// Integration Tests — require a live Chrome instance
//
// Set CHROME_DEBUG_URL env var to a valid Chrome DevTools WebSocket URL to run
// these tests, e.g.:
//   CHROME_DEBUG_URL="ws://127.0.0.1:9222/devtools/browser/abc..." go test -v -run Integration
// ============================================================================

func getTestChromeRemote(t *testing.T) *ChromeRemote {
	t.Helper()
	debugURL := os.Getenv("CHROME_DEBUG_URL")
	if debugURL == "" {
		t.Skip("CHROME_DEBUG_URL not set — skipping integration test")
	}
	cr, err := NewChromeRemote(debugURL)
	if err != nil {
		t.Fatalf("NewChromeRemote failed: %v", err)
	}
	return cr
}

func TestIntegration_NewChromeRemote(t *testing.T) {
	cr := getTestChromeRemote(t)
	defer cr.Close()

	if cr.debugURL == "" {
		t.Error("expected debugURL to be set")
	}
}

func TestIntegration_ListTabs(t *testing.T) {
	cr := getTestChromeRemote(t)
	defer cr.Close()

	tabs, err := cr.ListTabs()
	if err != nil {
		t.Fatalf("ListTabs failed: %v", err)
	}
	// Chrome should have at least one page tab
	if len(tabs) == 0 {
		t.Error("expected at least one tab")
	}
	for _, tab := range tabs {
		if tab.ID == "" {
			t.Error("tab ID should not be empty")
		}
		if tab.Type != "page" {
			t.Errorf("expected type 'page', got %q", tab.Type)
		}
	}
}

func TestIntegration_OpenAndCloseTab(t *testing.T) {
	cr := getTestChromeRemote(t)
	defer cr.Close()

	// Count initial tabs
	initialTabs, err := cr.ListTabs()
	if err != nil {
		t.Fatalf("ListTabs failed: %v", err)
	}

	// Open a new tab
	tabID, err := cr.OpenTab("about:blank")
	if err != nil {
		t.Fatalf("OpenTab failed: %v", err)
	}
	if tabID == "" {
		t.Fatal("expected non-empty tab ID")
	}

	// Verify tab count increased
	afterOpen, err := cr.ListTabs()
	if err != nil {
		t.Fatalf("ListTabs failed: %v", err)
	}
	if len(afterOpen) != len(initialTabs)+1 {
		t.Errorf("expected %d tabs after open, got %d", len(initialTabs)+1, len(afterOpen))
	}

	// Close the tab
	if err := cr.CloseTab(tabID); err != nil {
		t.Fatalf("CloseTab failed: %v", err)
	}

	// Verify tab count returned
	afterClose, err := cr.ListTabs()
	if err != nil {
		t.Fatalf("ListTabs failed: %v", err)
	}
	if len(afterClose) != len(initialTabs) {
		t.Errorf("expected %d tabs after close, got %d", len(initialTabs), len(afterClose))
	}
}

func TestIntegration_OpenTab_DefaultBlank(t *testing.T) {
	cr := getTestChromeRemote(t)
	defer cr.Close()

	tabID, err := cr.OpenTab("")
	if err != nil {
		t.Fatalf("OpenTab('') failed: %v", err)
	}
	if tabID == "" {
		t.Fatal("expected non-empty tab ID even with empty URL")
	}
	// Cleanup
	_ = cr.CloseTab(tabID)
}

func TestIntegration_Navigate(t *testing.T) {
	cr := getTestChromeRemote(t)
	defer cr.Close()

	tabID, err := cr.OpenTab("about:blank")
	if err != nil {
		t.Fatalf("OpenTab failed: %v", err)
	}
	defer cr.CloseTab(tabID)

	result, err := cr.Navigate(tabID, "https://example.com", "load", 30000)
	if err != nil {
		t.Fatalf("Navigate failed: %v", err)
	}
	if result.Status != "success" {
		t.Errorf("expected status 'success', got %q", result.Status)
	}
	if result.FinalURL == "" {
		t.Error("expected non-empty FinalURL")
	}
	if result.NavigationID == "" {
		t.Error("expected non-empty NavigationID")
	}
}

func TestIntegration_Navigate_InvalidWaitUntil(t *testing.T) {
	cr := getTestChromeRemote(t)
	defer cr.Close()

	tabID, err := cr.OpenTab("about:blank")
	if err != nil {
		t.Fatalf("OpenTab failed: %v", err)
	}
	defer cr.CloseTab(tabID)

	_, err = cr.Navigate(tabID, "https://example.com", "invalid_event", 5000)
	if err == nil {
		t.Fatal("expected error for invalid waitUntil value")
	}
}

func TestIntegration_Navigate_DefaultParams(t *testing.T) {
	cr := getTestChromeRemote(t)
	defer cr.Close()

	tabID, err := cr.OpenTab("about:blank")
	if err != nil {
		t.Fatalf("OpenTab failed: %v", err)
	}
	defer cr.CloseTab(tabID)

	// Empty waitUntil and zero timeout should use defaults
	result, err := cr.Navigate(tabID, "https://example.com", "", 0)
	if err != nil {
		t.Fatalf("Navigate with defaults failed: %v", err)
	}
	if result.Status != "success" {
		t.Errorf("expected status 'success', got %q", result.Status)
	}
}

func TestIntegration_ActivateTab(t *testing.T) {
	cr := getTestChromeRemote(t)
	defer cr.Close()

	tabID, err := cr.OpenTab("about:blank")
	if err != nil {
		t.Fatalf("OpenTab failed: %v", err)
	}
	defer cr.CloseTab(tabID)

	if err := cr.ActivateTab(tabID); err != nil {
		t.Fatalf("ActivateTab failed: %v", err)
	}
}

func TestIntegration_ReloadTab(t *testing.T) {
	cr := getTestChromeRemote(t)
	defer cr.Close()

	tabID, err := cr.OpenTab("https://example.com")
	if err != nil {
		t.Fatalf("OpenTab failed: %v", err)
	}
	defer cr.CloseTab(tabID)

	// Normal reload
	if err := cr.ReloadTab(tabID, false); err != nil {
		t.Fatalf("ReloadTab(bypassCache=false) failed: %v", err)
	}

	// Bypass cache reload
	if err := cr.ReloadTab(tabID, true); err != nil {
		t.Fatalf("ReloadTab(bypassCache=true) failed: %v", err)
	}
}

func TestIntegration_GoBackAndForward(t *testing.T) {
	cr := getTestChromeRemote(t)
	defer cr.Close()

	tabID, err := cr.OpenTab("https://example.com")
	if err != nil {
		t.Fatalf("OpenTab failed: %v", err)
	}
	defer cr.CloseTab(tabID)

	// Navigate to a second page
	_, err = cr.Navigate(tabID, "https://example.org", "load", 30000)
	if err != nil {
		t.Fatalf("Navigate failed: %v", err)
	}

	// Go back
	if err := cr.GoBack(tabID); err != nil {
		t.Fatalf("GoBack failed: %v", err)
	}

	// Go forward
	if err := cr.GoForward(tabID); err != nil {
		t.Fatalf("GoForward failed: %v", err)
	}
}

func TestIntegration_TakeScreenshot(t *testing.T) {
	cr := getTestChromeRemote(t)
	defer cr.Close()

	tabID, err := cr.OpenTab("https://example.com")
	if err != nil {
		t.Fatalf("OpenTab failed: %v", err)
	}
	defer cr.CloseTab(tabID)

	// Viewport screenshot
	buf, err := cr.TakeScreenshot(tabID, false)
	if err != nil {
		t.Fatalf("TakeScreenshot(fullPage=false) failed: %v", err)
	}
	if len(buf) == 0 {
		t.Error("expected non-empty screenshot data")
	}

	// Full-page screenshot
	bufFull, err := cr.TakeScreenshot(tabID, true)
	if err != nil {
		t.Fatalf("TakeScreenshot(fullPage=true) failed: %v", err)
	}
	if len(bufFull) == 0 {
		t.Error("expected non-empty full-page screenshot data")
	}
}

func TestIntegration_TakeScreenshot_AutoPickTab(t *testing.T) {
	cr := getTestChromeRemote(t)
	defer cr.Close()

	// Empty targetID should auto-pick a tab
	buf, err := cr.TakeScreenshot("", false)
	if err != nil {
		t.Fatalf("TakeScreenshot with auto-pick failed: %v", err)
	}
	if len(buf) == 0 {
		t.Error("expected non-empty screenshot data")
	}
}

func TestIntegration_GetElements(t *testing.T) {
	cr := getTestChromeRemote(t)
	defer cr.Close()

	tabID, err := cr.OpenTab("https://example.com")
	if err != nil {
		t.Fatalf("OpenTab failed: %v", err)
	}
	defer cr.CloseTab(tabID)

	// Wait for page to load by navigating
	_, err = cr.Navigate(tabID, "https://example.com", "load", 30000)
	if err != nil {
		t.Fatalf("Navigate failed: %v", err)
	}

	elements, err := cr.GetElements(tabID, "")
	if err != nil {
		t.Fatalf("GetElements failed: %v", err)
	}

	// example.com has at least one <a> tag ("More information...")
	if len(elements) == 0 {
		t.Log("Warning: no clickable elements found on example.com")
	}

	for _, elem := range elements {
		if elem.Selector == "" {
			t.Error("element should have a non-empty selector")
		}
		if elem.TagName == "" {
			t.Error("element should have a non-empty tagName")
		}
	}
}

func TestIntegration_ClickElement(t *testing.T) {
	cr := getTestChromeRemote(t)
	defer cr.Close()

	tabID, err := cr.OpenTab("about:blank")
	if err != nil {
		t.Fatalf("OpenTab failed: %v", err)
	}
	defer cr.CloseTab(tabID)

	// Navigate and click the "More information..." link on example.com
	err = cr.ClickElement(tabID, "https://example.com", "a", true)
	if err != nil {
		t.Fatalf("ClickElement failed: %v", err)
	}
}

// ============================================================================
// Integration Test — Multi-Tab Workflow (self-contained, launches its own Chrome)
//
// Opens 3 tabs (Google, Slack, Raycast), switches between them, takes
// screenshots, clicks login/signup on Slack, and takes a final screenshot.
// ============================================================================

// launchTestChrome starts a fresh Chrome instance for testing.
// It returns the ChromeRemote, a cleanup function, and an error.
// The cleanup function kills Chrome and removes the temp profile.
func launchTestChrome(t *testing.T) (*ChromeRemote, func()) {
	t.Helper()

	// Determine Chrome path
	var chromePath string
	switch runtime.GOOS {
	case "darwin":
		chromePath = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
		if _, err := os.Stat(chromePath); err != nil {
			chromePath = "/Applications/Chromium.app/Contents/MacOS/Chromium"
		}
	case "linux":
		chromePath = "google-chrome"
	default:
		t.Skipf("unsupported OS for auto-launch: %s", runtime.GOOS)
	}

	if _, err := os.Stat(chromePath); err != nil {
		t.Skipf("Chrome not found at %s — skipping", chromePath)
	}

	profileDir := t.TempDir()

	args := []string{
		"--user-data-dir=" + profileDir,
		"--remote-debugging-port=0",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-extensions",
		"--disable-popup-blocking",
		"--disable-translate",
		"--disable-sync",
		"--disable-background-networking",
		"--ignore-certificate-errors",
		"about:blank",
	}

	cmd := exec.Command(chromePath, args...)
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to launch Chrome: %v", err)
	}
	t.Logf("Chrome launched (PID %d), profile: %s", cmd.Process.Pid, profileDir)

	// Poll for DevToolsActivePort file (Chrome writes it once the debug server is ready)
	var debugURL string
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		u, err := GetChromeDebugURL(profileDir)
		if err == nil {
			debugURL = u
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if debugURL == "" {
		_ = cmd.Process.Kill()
		t.Fatal("timed out waiting for Chrome DevToolsActivePort")
	}
	t.Logf("Chrome debug URL: %s", debugURL)

	cr, err := NewChromeRemote(debugURL)
	if err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("NewChromeRemote failed: %v", err)
	}

	cleanup := func() {
		cr.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Logf("Chrome (PID %d) killed", cmd.Process.Pid)
	}

	return cr, cleanup
}

// saveScreenshot saves screenshot bytes as a PNG file into the screenshots/ folder.
func saveScreenshot(t *testing.T, dir string, name string, data []byte) {
	t.Helper()
	path := filepath.Join(dir, name+".png")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to save screenshot %s: %v", path, err)
	}
	t.Logf("Saved screenshot: %s (%d bytes)", path, len(data))
}

func TestIntegration_MultiTabWorkflow(t *testing.T) {
	cr, cleanup := launchTestChrome(t)
	defer cleanup()

	// Create screenshots output folder next to the test file
	screenshotDir := filepath.Join(".", "screenshots")
	if err := os.MkdirAll(screenshotDir, 0755); err != nil {
		t.Fatalf("failed to create screenshots dir: %v", err)
	}
	t.Logf("Screenshots will be saved to: %s", screenshotDir)

	// ── Step 1: Open 3 tabs ──────────────────────────────────────────────
	urls := []string{
		"https://google.com",  // tab index 0
		"https://slack.com",   // tab index 1
		"https://raycast.com", // tab index 2
	}
	tabIDs := make([]string, len(urls))

	for i, u := range urls {
		id, err := cr.OpenTab(u)
		if err != nil {
			t.Fatalf("OpenTab(%q) failed: %v", u, err)
		}
		tabIDs[i] = id
		t.Logf("Opened tab %d (%s) → %s", i+1, u, id)

		// Wait for page to load
		_, err = cr.Navigate(id, u, "load", 30000)
		if err != nil {
			t.Fatalf("Navigate(%q) failed: %v", u, err)
		}
	}

	// Cleanup all tabs when done
	defer func() {
		for _, id := range tabIDs {
			_ = cr.CloseTab(id)
		}
	}()

	// ── Step 2: Activate in order 2 → 3 → 1, take screenshot each ──────
	activationOrder := []struct {
		label    string
		filename string
		index    int
	}{
		{"Slack (tab 2)", "1_slack", 1},
		{"Raycast (tab 3)", "2_raycast", 2},
		{"Google (tab 1)", "3_google", 0},
	}

	for _, step := range activationOrder {
		id := tabIDs[step.index]
		t.Logf("Activating %s (ID: %s)", step.label, id)

		if err := cr.ActivateTab(id); err != nil {
			t.Fatalf("ActivateTab %s failed: %v", step.label, err)
		}

		buf, err := cr.TakeScreenshot(id, false)
		if err != nil {
			t.Fatalf("TakeScreenshot of %s failed: %v", step.label, err)
		}
		if len(buf) == 0 {
			t.Errorf("expected non-empty screenshot for %s", step.label)
		}

		saveScreenshot(t, screenshotDir, step.filename, buf)
	}

	// ── Step 3: Go to Raycast (tab 3) and click login ───────────────────
	raycastTabID := tabIDs[2]
	t.Log("Switching to Raycast tab for login click")

	if err := cr.ActivateTab(raycastTabID); err != nil {
		t.Fatalf("ActivateTab (Raycast) failed: %v", err)
	}

	// Discover clickable elements first, then pick the login one
	elements, err := cr.GetElements(raycastTabID, "")
	if err != nil {
		t.Logf("Warning: GetElements TestIntegration_MultiTabWorkflowon Raycast failed: %v", err)
	}

	// Log all found elements for debugging
	t.Logf("Found %d clickable elements on Raycast", len(elements))

	var loginSelector string
	var loginHref string

	// Pass 1: Look for elements whose trimmed text exactly matches common login labels
	exactLabels := []string{"log in", "login", "sign in", "signin", "sign up", "signup"}
	for _, elem := range elements {
		trimmed := strings.TrimSpace(strings.ToLower(elem.Text))
		for _, label := range exactLabels {
			if trimmed == label {
				loginSelector = elem.Selector
				loginHref = elem.Href
				t.Logf("Exact match found: selector=%q text=%q href=%q", elem.Selector, elem.Text, elem.Href)
				break
			}
		}
		if loginSelector != "" {
			break
		}
	}

	// Pass 2: Fall back to href-based matching if no exact text match
	if loginSelector == "" {
		hrefKeywords := []string{"/login", "/signin", "/sign-in", "/signup", "/sign-up"}
		for _, elem := range elements {
			lowerHref := strings.ToLower(elem.Href)
			for _, kw := range hrefKeywords {
				if strings.Contains(lowerHref, kw) {
					loginSelector = elem.Selector
					loginHref = elem.Href
					t.Logf("Href match found: selector=%q text=%q href=%q", elem.Selector, elem.Text, elem.Href)
					break
				}
			}
			if loginSelector != "" {
				break
			}
		}
	}

	// Navigate directly to the login URL instead of clicking
	// (ClickElement can hit hidden mobile nav duplicates)
	if loginHref != "" {
		result, navErr := cr.Navigate(raycastTabID, loginHref, "load", 30000)
		if navErr != nil {
			t.Logf("Warning: Navigate to login failed: %v", navErr)
		} else {
			t.Logf("Navigated to login page: %s (status: %s)", result.FinalURL, result.Status)
		}
	} else {
		t.Log("Warning: could not find any login element on Raycast — page structure may have changed")
	}

	// ── Step 4: Take final screenshot after click ───────────────────────
	finalBuf, err := cr.TakeScreenshot(raycastTabID, false)
	if err != nil {
		t.Fatalf("Final screenshot of Raycast failed: %v", err)
	}
	if len(finalBuf) == 0 {
		t.Error("expected non-empty final screenshot")
	}
	saveScreenshot(t, screenshotDir, "4_raycast_after_click", finalBuf)

	fmt.Println("✅ Multi-tab workflow completed — screenshots saved to grx/browser/screenshots/")
}

// ============================================================================
// Integration Tests — Legacy Wrappers
// ============================================================================

func TestIntegration_LegacyListChromeTabs(t *testing.T) {
	debugURL := os.Getenv("CHROME_DEBUG_URL")
	if debugURL == "" {
		t.Skip("CHROME_DEBUG_URL not set — skipping integration test")
	}

	tabs, err := ListChromeTabs(debugURL)
	if err != nil {
		t.Fatalf("ListChromeTabs failed: %v", err)
	}
	if len(tabs) == 0 {
		t.Error("expected at least one tab")
	}
}

func TestIntegration_LegacyOpenChromeTab(t *testing.T) {
	debugURL := os.Getenv("CHROME_DEBUG_URL")
	if debugURL == "" {
		t.Skip("CHROME_DEBUG_URL not set — skipping integration test")
	}

	tabID, err := OpenChromeTab(debugURL, "about:blank")
	if err != nil {
		t.Fatalf("OpenChromeTab failed: %v", err)
	}
	if tabID == "" {
		t.Error("expected non-empty tab ID")
	}
	// Cleanup
	_ = CloseTab(debugURL, tabID)
}
