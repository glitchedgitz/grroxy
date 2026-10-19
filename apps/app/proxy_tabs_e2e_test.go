package app

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// Tab tools called without a targetId act on the current tab, and their
// response has to name that tab — echoing back the empty input left the
// caller unable to tell which tab was used.
func TestTabToolsReportResolvedTab(t *testing.T) {
	proxyID, fixtureURL := startBatchFixture(t)
	backend := &Backend{}

	call := func(name string, handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error), args map[string]any) string {
		t.Helper()
		req := mcp.CallToolRequest{}
		req.Params.Arguments = args
		res, err := handler(context.Background(), req)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		text := res.Content[0].(mcp.TextContent).Text
		if res.IsError {
			t.Fatalf("%s failed: %s", name, text)
		}
		var out struct {
			TargetID string `json:"targetId"`
		}
		if err := json.Unmarshal([]byte(text), &out); err != nil {
			t.Fatalf("%s: unparseable response %q", name, text)
		}
		return out.TargetID
	}

	chrome, err := ProxyMgr.GetChromeRemote(proxyID)
	if err != nil {
		t.Fatal(err)
	}
	tabs, err := chrome.ListTabs()
	if err != nil || len(tabs) != 1 {
		t.Fatalf("expected one tab, got %v (%v)", tabs, err)
	}
	want := tabs[0].ID

	// Reload, back and forward return once Chrome accepts them, not once the
	// page has loaded; the next call can land mid-navigation. Wait it out so
	// this test is only about the reported tab.
	settle := func() {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			var state string
			if chrome.Evaluate(want, "document.readyState", &state, 1000) == nil && state == "complete" {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	got := map[string]string{}
	got["navigate"] = call("navigate", backend.proxyNavigateTabHandler, map[string]any{"proxyId": proxyID, "url": fixtureURL})
	settle()
	got["reload"] = call("reload", backend.proxyReloadTabHandler, map[string]any{"proxyId": proxyID})
	settle()
	got["goBack"] = call("goBack", backend.proxyGoBackHandler, map[string]any{"proxyId": proxyID})
	settle()
	got["goForward"] = call("goForward", backend.proxyGoForwardHandler, map[string]any{"proxyId": proxyID})
	for name, id := range got {
		if id != want {
			t.Errorf("%s with no targetId reported %q, want the tab it used, %q", name, id, want)
		}
	}
}
