package browser

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

const tabsChromePath = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"

// browserTabs reads the page tabs from Chrome's own HTTP endpoint, so the test
// sees what the user sees rather than what ChromeRemote reports.
func browserTabs(t *testing.T, debugURL string) []TabInfo {
	t.Helper()
	u, err := url.Parse(debugURL)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get("http://" + u.Host + "/json/list")
	if err != nil {
		t.Fatalf("list tabs over http: %v", err)
	}
	defer resp.Body.Close()

	var all []struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		URL  string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&all); err != nil {
		t.Fatal(err)
	}
	var pages []TabInfo
	for _, x := range all {
		if x.Type == "page" {
			pages = append(pages, TabInfo{ID: x.ID, URL: x.URL, Type: x.Type})
		}
	}
	return pages
}

// expectTabs fails when the browser holds anything other than exactly want
// (by id), or when a stray about:blank has appeared.
func expectTabs(t *testing.T, debugURL, step string, want ...string) {
	t.Helper()
	// Let any tab a step created or closed show up in the list.
	time.Sleep(300 * time.Millisecond)

	got := browserTabs(t, debugURL)
	ids := map[string]bool{}
	for _, tab := range got {
		ids[tab.ID] = true
		if tab.URL == "about:blank" {
			t.Errorf("%s: a stray about:blank tab appeared (%v)", step, got)
		}
	}
	if len(got) != len(want) {
		t.Errorf("%s: browser has %d tabs, want %d: %v", step, len(got), len(want), got)
	}
	for _, id := range want {
		if !ids[id] {
			t.Errorf("%s: tab %s is gone: %v", step, id, got)
		}
	}
}

// Tab management must leave the user's browser alone: connecting must not add
// a tab, operating on tabs must not add blank ones or close others, and "no tab
// given" must keep resolving to the same tab.
func TestTabsLeaveBrowserAlone(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser test in -short mode")
	}
	if _, err := os.Stat(tabsChromePath); err != nil {
		t.Skipf("Chrome not installed at %s", tabsChromePath)
	}

	dir := t.TempDir()
	page := func(name string) string {
		p := filepath.Join(dir, name+".html")
		if err := os.WriteFile(p, []byte("<!doctype html><title>"+name+"</title><h1>"+name+"</h1>"), 0o644); err != nil {
			t.Fatal(err)
		}
		return "file://" + p
	}
	pageA, pageB, pageC := page("a"), page("b"), page("c")

	profile := filepath.Join(dir, "profile")
	cmd := exec.Command(tabsChromePath,
		"--user-data-dir="+profile,
		"--remote-debugging-port=0",
		"--no-first-run", "--no-default-browser-check", "--test-type",
		pageA,
	)
	if err := cmd.Start(); err != nil {
		t.Fatalf("launch chrome: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	var debugURL string
	for i := 0; i < 100 && debugURL == ""; i++ {
		debugURL, _ = GetChromeDebugURL(profile)
		time.Sleep(200 * time.Millisecond)
	}
	if debugURL == "" {
		t.Fatal("chrome never published a debug url")
	}

	var start []TabInfo
	for i := 0; i < 50 && len(start) == 0; i++ {
		start = browserTabs(t, debugURL)
		time.Sleep(100 * time.Millisecond)
	}
	if len(start) != 1 {
		t.Fatalf("expected chrome to start with one tab, got %v", start)
	}
	tabA := start[0].ID

	cr, err := NewChromeRemote(debugURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	expectTabs(t, debugURL, "connect", tabA)

	for i := 0; i < 3; i++ {
		cur, err := cr.CurrentTarget()
		if err != nil {
			t.Fatalf("CurrentTarget: %v", err)
		}
		if cur != tabA {
			t.Fatalf("CurrentTarget call %d = %s, want the only real tab %s", i, cur, tabA)
		}
	}
	var two int
	if err := cr.Evaluate("", "1+1", &two, 5000); err != nil {
		t.Fatalf("eval on the current tab: %v", err)
	}
	expectTabs(t, debugURL, "eval with no tab given", tabA)

	tabB, err := cr.OpenTab(pageB)
	if err != nil {
		t.Fatalf("OpenTab: %v", err)
	}
	expectTabs(t, debugURL, "OpenTab", tabA, tabB)
	if cur, _ := cr.CurrentTarget(); cur != tabB {
		t.Errorf("after OpenTab the current tab is %s, want the new tab %s", cur, tabB)
	}

	// OpenTab leaves the new tab in the background, so the current tab is one
	// Chrome is not showing. The tab list has to say so.
	listed, err := cr.ListTabs()
	if err != nil {
		t.Fatalf("ListTabs: %v", err)
	}
	cr.ProbeTabVisibility(listed)
	for _, tab := range listed {
		if tab.Visible == nil {
			t.Errorf("tab %s was not probed for visibility", tab.ID)
			continue
		}
		switch tab.ID {
		case tabB:
			if *tab.Visible {
				t.Errorf("background tab %s reported visible", tabB)
			}
		case tabA:
			// Only a soft check: macOS reports an occluded window's tab as hidden.
			if !*tab.Visible {
				t.Logf("front tab %s reported hidden (window occluded?)", tabA)
			}
		}
	}

	// An explicit tab is acted on without becoming the current tab.
	var title string
	if err := cr.Evaluate(tabA, "document.title", &title, 5000); err != nil || title != "a" {
		t.Errorf("eval on tab %s = %q (%v), want its title %q", tabA, title, err, "a")
	}
	if cur, _ := cr.CurrentTarget(); cur != tabB {
		t.Errorf("eval with an explicit tab moved the current tab to %s, want %s", cur, tabB)
	}
	expectTabs(t, debugURL, "visibility probe and explicit-tab eval", tabA, tabB)

	if err := cr.ActivateTab(tabA); err != nil {
		t.Fatalf("ActivateTab: %v", err)
	}
	expectTabs(t, debugURL, "ActivateTab", tabA, tabB)
	if cur, _ := cr.CurrentTarget(); cur != tabA {
		t.Errorf("after ActivateTab the current tab is %s, want %s", cur, tabA)
	}

	if _, err := cr.Navigate(tabB, pageC, "load", 10000); err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if err := cr.GoBack(tabB, 5000); err != nil {
		t.Fatalf("GoBack: %v", err)
	}
	if err := cr.GoForward(tabB, 5000); err != nil {
		t.Fatalf("GoForward: %v", err)
	}
	if err := cr.ReloadTab(tabB, false, 5000); err != nil {
		t.Fatalf("ReloadTab: %v", err)
	}
	expectTabs(t, debugURL, "navigate/back/forward/reload", tabA, tabB)

	if err := cr.CloseTab(tabB); err != nil {
		t.Fatalf("CloseTab: %v", err)
	}
	expectTabs(t, debugURL, "CloseTab", tabA)
	if cur, err := cr.CurrentTarget(); err != nil || cur != tabA {
		t.Errorf("after closing a tab the current tab is %s (%v), want the remaining tab %s", cur, err, tabA)
	}

	// Closing the current tab hands over to the tab Chrome now shows, not to
	// whichever tab comes first in the target list.
	tabB, err = cr.OpenTab(pageB)
	if err != nil {
		t.Fatalf("OpenTab B: %v", err)
	}
	tabC, err := cr.OpenTab(pageC)
	if err != nil {
		t.Fatalf("OpenTab C: %v", err)
	}
	if err := cr.ActivateTab(tabC); err != nil {
		t.Fatalf("ActivateTab C: %v", err)
	}
	if err := cr.CloseTab(tabC); err != nil {
		t.Fatalf("CloseTab C: %v", err)
	}
	expectTabs(t, debugURL, "close the current tab", tabA, tabB)

	cur, err := cr.CurrentTarget()
	if err != nil {
		t.Fatalf("CurrentTarget after closing the current tab: %v", err)
	}
	listed, err = cr.ListTabs()
	if err != nil {
		t.Fatalf("ListTabs: %v", err)
	}
	cr.ProbeTabVisibility(listed)
	shown := ""
	for _, tab := range listed {
		if tab.Visible != nil && *tab.Visible {
			shown = tab.ID
		}
	}
	if shown == "" {
		t.Skip("no tab reported visible (window occluded?), can't check the hand-over")
	}
	if cur != shown {
		t.Errorf("after closing the current tab, current is %s but Chrome shows %s", cur, shown)
	}
}

// Back and forward return before their navigation finishes, so a call right
// after one lands while Chrome swaps the page. It must ride out the swap
// instead of failing with "Not attached to an active page". (Each round then
// waits for the page to settle: a reload or back issued before forward has
// committed would cancel it, which is expected, not this bug.)
func TestHistoryRightAfterNavigation(t *testing.T) {
	pageA := dialogPage(t, "a", "a")
	pageB := dialogPage(t, "b", "b")
	debugURL := launchDialogChrome(t, pageA)
	time.Sleep(time.Second)

	cr, err := NewChromeRemote(debugURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer cr.Close()
	tabs, _ := cr.ListTabs()
	tab := tabs[0].ID

	if _, err := cr.Navigate(tab, pageB, "load", 10000); err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	for i := 0; i < 20; i++ {
		if err := cr.GoBack(tab, 5000); err != nil {
			t.Fatalf("round %d GoBack: %v", i, err)
		}
		if err := cr.GoForward(tab, 5000); err != nil {
			t.Fatalf("round %d GoForward: %v", i, err)
		}
		settled := false
		for j := 0; j < 50 && !settled; j++ {
			var title string
			if err := cr.Evaluate(tab, "document.readyState === 'complete' ? document.title : ''", &title, 1000); err == nil && title == "b" {
				settled = true
			} else {
				time.Sleep(100 * time.Millisecond)
			}
		}
		if !settled {
			t.Fatalf("round %d: tab never settled back on page b", i)
		}
	}
	if err := cr.ReloadTab(tab, false, 5000); err != nil {
		t.Fatalf("ReloadTab: %v", err)
	}
	if err := cr.GoBack(tab, 5000); err != nil {
		t.Fatalf("GoBack right after ReloadTab: %v", err)
	}
}
