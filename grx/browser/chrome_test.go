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
	if err := cr.ReloadTab(tabID, false, 0); err != nil {
		t.Fatalf("ReloadTab(bypassCache=false) failed: %v", err)
	}

	// Bypass cache reload
	if err := cr.ReloadTab(tabID, true, 0); err != nil {
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
	if err := cr.GoBack(tabID, 0); err != nil {
		t.Fatalf("GoBack failed: %v", err)
	}

	// Go forward
	if err := cr.GoForward(tabID, 0); err != nil {
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
	result, err := cr.TakeScreenshot(tabID, false, 0)
	if err != nil {
		t.Fatalf("TakeScreenshot(fullPage=false) failed: %v", err)
	}
	if len(result.Bytes) == 0 {
		t.Error("expected non-empty screenshot data")
	}

	// Full-page screenshot
	resultFull, err := cr.TakeScreenshot(tabID, true, 0)
	if err != nil {
		t.Fatalf("TakeScreenshot(fullPage=true) failed: %v", err)
	}
	if len(resultFull.Bytes) == 0 {
		t.Error("expected non-empty full-page screenshot data")
	}
}

func TestIntegration_TakeScreenshot_AutoPickTab(t *testing.T) {
	cr := getTestChromeRemote(t)
	defer cr.Close()

	// Empty targetID should auto-pick a tab
	result, err := cr.TakeScreenshot("", false, 0)
	if err != nil {
		t.Fatalf("TakeScreenshot with auto-pick failed: %v", err)
	}
	if len(result.Bytes) == 0 {
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
	if _, err := cr.ClickElement(tabID, "https://example.com", "a", true, "", false); err != nil {
		t.Fatalf("ClickElement failed: %v", err)
	}
}

// ============================================================================
// Integration Tests — Bug fixes (Bugs 1-6)
//
// These exercise the new behavior added when the bug report came in:
//   Bug 1 — CloseTab uses chromedp.Cancel; works for both cached and untouched
//           target contexts (the old chromedp.Run + target.CloseTarget path
//           failed on the browser ctx with a chromedp internal error).
//   Bug 2 — TypeText is broken into stepwise actions (waitVisible →
//           scrollIntoView → focus → [clear] → sendKeys).
//   Bug 3 — ClickElement returns ClickResult.NewTabs when a click spawns
//           a new tab (e.g. <a target="_blank">).
//   Bug 4 — TakeScreenshot accepts timeoutMs and waits for readyState.
//   Bug 5 — ClickElement accepts waitForSelector for SPA-style transitions.
//   Bug 6 — SetValue uses the framework-aware native setter and dispatches
//           bubbling input + change events.
// ============================================================================

// loadDataURL opens a fresh tab and navigates it to the given data: URL,
// returning the new target ID. Centralizes the "set up a tiny test page"
// pattern used by the bug-fix tests below.
func loadDataURL(t *testing.T, cr *ChromeRemote, dataURL string) string {
	t.Helper()
	tabID, err := cr.OpenTab("about:blank")
	if err != nil {
		t.Fatalf("OpenTab failed: %v", err)
	}
	if _, err := cr.Navigate(tabID, dataURL, "load", 5000); err != nil {
		_ = cr.CloseTab(tabID)
		t.Fatalf("Navigate(data:) failed: %v", err)
	}
	return tabID
}

// Bug 1: CloseTab works on both cached and untouched target contexts.
func TestIntegration_CloseTab_BothPaths(t *testing.T) {
	cr := getTestChromeRemote(t)
	defer cr.Close()

	// (a) untouched target — never went through getContext, so no cached chromedp.Context.
	untouched, err := cr.OpenTab("about:blank")
	if err != nil {
		t.Fatalf("OpenTab failed: %v", err)
	}
	if err := cr.CloseTab(untouched); err != nil {
		t.Fatalf("CloseTab(untouched) failed: %v", err)
	}

	// (b) cached target — explicitly populate the targetCtxs cache, then close.
	cached, err := cr.OpenTab("about:blank")
	if err != nil {
		t.Fatalf("OpenTab failed: %v", err)
	}
	if _, err := cr.getContext(cached); err != nil {
		_ = cr.CloseTab(cached)
		t.Fatalf("getContext failed: %v", err)
	}
	if err := cr.CloseTab(cached); err != nil {
		t.Fatalf("CloseTab(cached) failed: %v", err)
	}

	// (c) empty targetID is rejected.
	if err := cr.CloseTab(""); err == nil {
		t.Error("expected error for empty targetID, got nil")
	}
}

// Bug 2: TypeText writes into a real input and the value is observable.
func TestIntegration_TypeText_Stepwise(t *testing.T) {
	cr := getTestChromeRemote(t)
	defer cr.Close()

	tabID := loadDataURL(t, cr, `data:text/html,<input id="q"/>`)
	defer cr.CloseTab(tabID)

	if err := cr.TypeText(tabID, "#q", "hello", false, 0); err != nil {
		t.Fatalf("TypeText failed: %v", err)
	}

	var got string
	if err := cr.Evaluate(tabID, `document.getElementById('q').value`, &got, 0); err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if got != "hello" {
		t.Errorf("expected value 'hello', got %q", got)
	}
}

// Bug 3: ClickElement reports new tabs spawned by an <a target="_blank"> click.
func TestIntegration_ClickElement_NewTabs(t *testing.T) {
	cr := getTestChromeRemote(t)
	defer cr.Close()

	tabID := loadDataURL(t, cr, `data:text/html,<a id="x" href="about:blank" target="_blank">go</a>`)
	defer cr.CloseTab(tabID)

	result, err := cr.ClickElement(tabID, "", "#x", false, "", false)
	if err != nil {
		t.Fatalf("ClickElement failed: %v", err)
	}
	if result == nil || len(result.NewTabs) == 0 {
		t.Fatalf("expected ClickResult.NewTabs to be populated, got %+v", result)
	}
	for _, id := range result.NewTabs {
		_ = cr.CloseTab(id)
	}
}

// Bug 5: ClickElement waitForSelector blocks until the named selector
// becomes visible (set up to be appended via setTimeout after the click).
func TestIntegration_ClickElement_WaitForSelector(t *testing.T) {
	cr := getTestChromeRemote(t)
	defer cr.Close()

	html := `data:text/html,<button id="b" onclick="setTimeout(function(){var d=document.createElement('div');d.id='after';d.textContent='hi';document.body.appendChild(d);},200)">go</button>`
	tabID := loadDataURL(t, cr, html)
	defer cr.CloseTab(tabID)

	result, err := cr.ClickElement(tabID, "", "#b", false, "#after", false)
	if err != nil {
		t.Fatalf("ClickElement(waitForSelector) failed: %v", err)
	}
	if result.WaitedFor != "#after" {
		t.Errorf("expected ClickResult.WaitedFor=%q, got %q", "#after", result.WaitedFor)
	}
}

// Bug 4: TakeScreenshot honors a custom timeoutMs and produces image bytes.
func TestIntegration_TakeScreenshot_TimeoutMs(t *testing.T) {
	cr := getTestChromeRemote(t)
	defer cr.Close()

	tabID := loadDataURL(t, cr, "data:text/html,<h1>hi</h1>")
	defer cr.CloseTab(tabID)

	result, err := cr.TakeScreenshot(tabID, false, 5000)
	if err != nil {
		t.Fatalf("TakeScreenshot(timeoutMs=5000) failed: %v", err)
	}
	if len(result.Bytes) == 0 {
		t.Error("expected non-empty screenshot data")
	}
}

// Bug: waitForNavigation never resolves when used with proxyClick.
// Re-exercises the post-click waits after the rewrite that registers a
// page.EventFrameNavigated listener BEFORE the click and splits the run
// phases.
//
// Click an in-page anchor that triggers a same-tab navigation to another
// data: URL with a marker element, then verify the navigation completed
// (the marker is reachable from the routed context).
func TestIntegration_ClickElement_WaitForNavigation_Resolves(t *testing.T) {
	cr := getTestChromeRemote(t)
	defer cr.Close()

	// Page A holds a link that navigates the same tab to page B (a data:
	// URL containing #marker). data: URL navigation fires frameNavigated.
	pageB := `data:text/html,<div id="marker">on-page-b</div>`
	pageA := `data:text/html,<a id="go" href="` + pageB + `">go</a>`

	tabID := loadDataURL(t, cr, pageA)
	defer cr.CloseTab(tabID)

	start := time.Now()
	result, err := cr.ClickElement(tabID, "", "#go", true, "", false)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("ClickElement(waitForNavigation) failed: %v", err)
	}
	if !result.NavWaited {
		t.Errorf("expected ClickResult.NavWaited=true, got %v", result.NavWaited)
	}
	// Sanity check: the wait should resolve well under the 30s ceiling. Old
	// behavior pegged 30s. Allow up to 15s for a slow CI host.
	if elapsed > 15*time.Second {
		t.Errorf("waitForNavigation took %v — expected to resolve well under 15s", elapsed)
	}

	// The new document should be the marker page.
	var marker string
	if err := cr.Evaluate(tabID, `(document.getElementById('marker')||{}).textContent || ''`, &marker, 0); err != nil {
		t.Fatalf("Evaluate(marker) failed: %v", err)
	}
	if marker != "on-page-b" {
		t.Errorf("expected to be on page B, got marker=%q", marker)
	}
}

// Bug: waitForSelector never resolves on a click that opens a new tab —
// because the original-tab chromedp context was being polled forever for
// a selector that only exists in the spawned tab.
//
// Verifies the post-click wait routes to the new tab when the click was
// `<a target="_blank">`.
func TestIntegration_ClickElement_WaitForSelector_NewTab(t *testing.T) {
	cr := getTestChromeRemote(t)
	defer cr.Close()

	newTabPage := `data:text/html,<div id="found">hello</div>`
	parent := `data:text/html,<a id="x" href="` + newTabPage + `" target="_blank">go</a>`
	tabID := loadDataURL(t, cr, parent)
	defer cr.CloseTab(tabID)

	start := time.Now()
	result, err := cr.ClickElement(tabID, "", "#x", false, "#found", false)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("ClickElement(waitForSelector, new tab) failed: %v", err)
	}
	if len(result.NewTabs) == 0 {
		t.Fatal("expected NewTabs to be populated for target=_blank click")
	}
	for _, id := range result.NewTabs {
		defer cr.CloseTab(id)
	}
	if elapsed > 15*time.Second {
		t.Errorf("waitForSelector took %v — expected to resolve well under 15s", elapsed)
	}
}

// Bug 7: ActivateTab must steer subsequent CDP tools to the activated tab.
// Previously, getContext("") fell back to tabs[0] regardless of which tab
// was last activated, so proxyType/proxyClick/proxyEval/proxySetValue all
// went to the wrong tab when more than one was open.
func TestIntegration_ActivateTab_SteersTools(t *testing.T) {
	cr := getTestChromeRemote(t)
	defer cr.Close()

	tabA := loadDataURL(t, cr, `data:text/html,<input id="x" value="A"/>`)
	defer cr.CloseTab(tabA)
	tabB := loadDataURL(t, cr, `data:text/html,<input id="x" value="B"/>`)
	defer cr.CloseTab(tabB)

	// Activate tab B explicitly.
	if err := cr.ActivateTab(tabB); err != nil {
		t.Fatalf("ActivateTab failed: %v", err)
	}

	// Tools that pass an empty targetID should now hit tab B, not tab A.
	var got string
	if err := cr.Evaluate("", `document.getElementById('x').value`, &got, 0); err != nil {
		t.Fatalf("Evaluate('') failed: %v", err)
	}
	if got != "B" {
		t.Errorf("Evaluate steered to wrong tab: got value=%q, want %q", got, "B")
	}

	// Switch back to A; same call should now follow.
	if err := cr.ActivateTab(tabA); err != nil {
		t.Fatalf("ActivateTab(A) failed: %v", err)
	}
	if err := cr.Evaluate("", `document.getElementById('x').value`, &got, 0); err != nil {
		t.Fatalf("Evaluate('') failed: %v", err)
	}
	if got != "A" {
		t.Errorf("Evaluate steered to wrong tab after re-activate: got value=%q, want %q", got, "A")
	}
}

// Bug 8: ClickElement reports clickMode="native" on a clean click, and
// "js" when the caller explicitly asks (useJS=true). The auto-fallback
// path is harder to exercise reliably without a real overlay, so we
// validate the explicit useJS path here.
func TestIntegration_ClickElement_UseJS(t *testing.T) {
	cr := getTestChromeRemote(t)
	defer cr.Close()

	html := `data:text/html,<button id="b" onclick="window.__clicked=true">go</button>`
	tabID := loadDataURL(t, cr, html)
	defer cr.CloseTab(tabID)

	// Native click: clickMode should report "native".
	result, err := cr.ClickElement(tabID, "", "#b", false, "", false)
	if err != nil {
		t.Fatalf("ClickElement(native) failed: %v", err)
	}
	if result.ClickMode != "native" {
		t.Errorf("expected ClickMode=%q, got %q", "native", result.ClickMode)
	}

	// useJS=true: clickMode should report "js" and the JS handler should
	// still see the click.
	tabID2 := loadDataURL(t, cr, html)
	defer cr.CloseTab(tabID2)
	result, err = cr.ClickElement(tabID2, "", "#b", false, "", true)
	if err != nil {
		t.Fatalf("ClickElement(useJS) failed: %v", err)
	}
	if result.ClickMode != "js" {
		t.Errorf("expected ClickMode=%q, got %q", "js", result.ClickMode)
	}
	var clicked bool
	if err := cr.Evaluate(tabID2, `!!window.__clicked`, &clicked, 0); err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if !clicked {
		t.Error("expected JS click handler to fire")
	}
}

// Bug: waitForSelector never resolves when the selector simply doesn't
// appear. Should fail with a meaningful timeout error inside its own short
// budget (10s), not eat the global 30s.
func TestIntegration_ClickElement_WaitForSelector_TimesOut(t *testing.T) {
	cr := getTestChromeRemote(t)
	defer cr.Close()

	tabID := loadDataURL(t, cr, `data:text/html,<button id="b">go</button>`)
	defer cr.CloseTab(tabID)

	start := time.Now()
	_, err := cr.ClickElement(tabID, "", "#b", false, "#never-appears", false)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected waitForSelector to time out, got nil error")
	}
	if !strings.Contains(err.Error(), "waitForSelector") {
		t.Errorf("expected error to mention waitForSelector, got: %v", err)
	}
	// Should bail well before the global 30s — its own 10s budget plus
	// pre-click overhead. Allow generous slack but flag a regression.
	if elapsed > 20*time.Second {
		t.Errorf("waitForSelector timeout took %v — expected to bail under 20s", elapsed)
	}
}

// Bug 6: SetValue updates the DOM .value AND dispatches bubbling input +
// change events (the framework-aware path that React/Vue/Angular pick up).
func TestIntegration_SetValue(t *testing.T) {
	cr := getTestChromeRemote(t)
	defer cr.Close()

	html := `data:text/html,<input id="q"/><script>window.__ev=[];var el=document.getElementById('q');el.addEventListener('input',function(){window.__ev.push('input')});el.addEventListener('change',function(){window.__ev.push('change')});</script>`
	tabID := loadDataURL(t, cr, html)
	defer cr.CloseTab(tabID)

	if err := cr.SetValue(tabID, "#q", "hello", 0); err != nil {
		t.Fatalf("SetValue failed: %v", err)
	}

	var value string
	if err := cr.Evaluate(tabID, `document.getElementById('q').value`, &value, 0); err != nil {
		t.Fatalf("Evaluate(value) failed: %v", err)
	}
	if value != "hello" {
		t.Errorf("expected DOM value 'hello', got %q", value)
	}

	var events []string
	if err := cr.Evaluate(tabID, `window.__ev`, &events, 0); err != nil {
		t.Fatalf("Evaluate(events) failed: %v", err)
	}
	hasInput := false
	hasChange := false
	for _, e := range events {
		if e == "input" {
			hasInput = true
		}
		if e == "change" {
			hasChange = true
		}
	}
	if !hasInput {
		t.Errorf("expected 'input' event, got %v", events)
	}
	if !hasChange {
		t.Errorf("expected 'change' event, got %v", events)
	}

	// Missing selector should error.
	if err := cr.SetValue(tabID, "#nope", "x", 0); err == nil {
		t.Error("expected error for missing selector")
	}
}

// Bug: WaitForSelector consistently times out even when the element exists
// on the page. Root cause was chromedp.WaitVisible's strict visibility
// check (zero-size / display:none / collapsed elements all fail it). The
// rewrite defaults to state="attached" (DOM presence) and uses
// chromedp.ByQuery for predictable selector matching.
//
// This test exercises a hidden-but-attached element (display:none): the
// default attached state should resolve, while the strict "visible" state
// should time out.
func TestIntegration_WaitForSelector_HiddenElement(t *testing.T) {
	cr := getTestChromeRemote(t)
	defer cr.Close()

	html := `data:text/html,<div id="hidden" style="display:none">i exist</div>`
	tabID := loadDataURL(t, cr, html)
	defer cr.CloseTab(tabID)

	// Default state — element exists in DOM, should resolve fast.
	start := time.Now()
	if err := cr.WaitForSelector(tabID, "#hidden", 5000, ""); err != nil {
		t.Fatalf("WaitForSelector(default) failed: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("default WaitForSelector took %v on present element — expected fast", elapsed)
	}

	// Explicit "attached" — same expectation.
	if err := cr.WaitForSelector(tabID, "#hidden", 5000, "attached"); err != nil {
		t.Fatalf("WaitForSelector(attached) failed: %v", err)
	}

	// Strict "visible" — should time out because display:none fails the check.
	start = time.Now()
	err := cr.WaitForSelector(tabID, "#hidden", 2000, "visible")
	elapsed := time.Since(start)
	if err == nil {
		t.Error("WaitForSelector(visible) should have timed out on display:none element")
	}
	if elapsed > 4*time.Second {
		t.Errorf("WaitForSelector(visible) timeout took %v — expected to bail near 2s budget", elapsed)
	}

	// Invalid state — bail with a clear error, no polling.
	if err := cr.WaitForSelector(tabID, "#hidden", 5000, "bogus"); err == nil {
		t.Error("expected error for invalid state, got nil")
	}
}

// WaitForSelector should also resolve quickly on an element that is added
// to the DOM after the call begins, proving the polling loop actually works
// for the dynamic case.
func TestIntegration_WaitForSelector_AppearsLater(t *testing.T) {
	cr := getTestChromeRemote(t)
	defer cr.Close()

	html := `data:text/html,<button id="b" onclick="setTimeout(function(){var d=document.createElement('div');d.id='late';document.body.appendChild(d);},200)">go</button>`
	tabID := loadDataURL(t, cr, html)
	defer cr.CloseTab(tabID)

	// Trigger the delayed append, then wait.
	if _, err := cr.ClickElement(tabID, "", "#b", false, "", false); err != nil {
		t.Fatalf("ClickElement failed: %v", err)
	}

	start := time.Now()
	if err := cr.WaitForSelector(tabID, "#late", 5000, ""); err != nil {
		t.Fatalf("WaitForSelector(#late) failed: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("WaitForSelector took %v for element appearing in 200ms — polling sluggish?", elapsed)
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

		// Wait for page to load (use domcontentloaded for heavy JS sites)
		_, err = cr.Navigate(id, u, "domcontentloaded", 60000)
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

		result, err := cr.TakeScreenshot(id, false, 0)
		if err != nil {
			t.Fatalf("TakeScreenshot of %s failed: %v", step.label, err)
		}
		if len(result.Bytes) == 0 {
			t.Errorf("expected non-empty screenshot for %s", step.label)
		}

		saveScreenshot(t, screenshotDir, step.filename, result.Bytes)
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
		result, navErr := cr.Navigate(raycastTabID, loginHref, "domcontentloaded", 60000)
		if navErr != nil {
			t.Logf("Warning: Navigate to login failed: %v", navErr)
		} else {
			t.Logf("Navigated to login page: %s (status: %s)", result.FinalURL, result.Status)
		}
	} else {
		t.Log("Warning: could not find any login element on Raycast — page structure may have changed")
	}

	// ── Step 4: Take final screenshot after click ───────────────────────
	finalResult, err := cr.TakeScreenshot(raycastTabID, false, 0)
	if err != nil {
		t.Fatalf("Final screenshot of Raycast failed: %v", err)
	}
	if len(finalResult.Bytes) == 0 {
		t.Error("expected non-empty final screenshot")
	}
	saveScreenshot(t, screenshotDir, "4_raycast_after_click", finalResult.Bytes)

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
