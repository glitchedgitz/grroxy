package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// elementsFixtureHTML has one element per selector strategy, plus the shapes
// that used to produce brittle selectors: a React-style generated id, a
// button only its panel can tell apart, and true duplicates.
const elementsFixtureHTML = `<!doctype html>
<html><head><meta charset="utf-8"></head><body>
<button id=":r1a:">Post</button>
<input placeholder="Search sounds">
<button aria-label="Close">×</button>
<button data-e2e="publish-btn">Publish</button>
<button id="save">Save</button>
<div id="open-new-editor"><div></div><div><button>Edit</button></div></div>
<div><button>Edit</button></div>
<div><button>Use</button></div>
<div><button>Use</button></div>
</body></html>`

// proxyElements should hand back the selector a person would write — label,
// test hook or visible text — and only fall back to a position path when
// nothing else names the element uniquely.
func TestProxyElementsStableSelectors(t *testing.T) {
	proxyID, _ := startBatchFixture(t)

	fixture := filepath.Join(t.TempDir(), "elements.html")
	if err := os.WriteFile(fixture, []byte(elementsFixtureHTML), 0o644); err != nil {
		t.Fatal(err)
	}

	// Load it the way an agent does, through a batch: that also pins the tab,
	// so the listing and the clicks below address the same page.
	if resp, err := (&Backend{}).proxyBatchActionsLogic(context.Background(), ProxyBatchRequest{
		ID:      proxyID,
		Actions: []ProxyBatchAction{{Action: "navigate", URL: "file://" + fixture}},
	}); err != nil || !resp.Success {
		t.Fatalf("navigate: %v %+v", err, resp)
	}

	elements, err := ProxyMgr.GetElements(proxyID, "", "")
	if err != nil {
		t.Fatalf("GetElements: %v", err)
	}

	got := map[string]bool{}
	for _, el := range elements {
		t.Logf("%-8s %-14q -> %s", el.TagName, el.Text, el.Selector)
		got[el.Selector] = true
		if strings.Contains(el.Selector, "#:") {
			t.Errorf("selector %q is built on a generated id", el.Selector)
		}
	}

	want := []string{
		`//button[normalize-space(.)="Post"]`,
		`input[placeholder="Search sounds"]`,
		`button[aria-label="Close"]`,
		`button[data-e2e="publish-btn"]`,
		`#save`,
		`//*[@id="open-new-editor"]//button[normalize-space(.)="Edit"]`,
	}
	for _, sel := range want {
		if !got[sel] {
			t.Errorf("missing selector %s", sel)
		}
	}

	// The duplicates have nothing to tell them apart, so they keep a path.
	paths := 0
	for _, el := range elements {
		if el.Text == "Use" && strings.Contains(el.Selector, "nth-of-type") {
			paths++
		}
	}
	if paths != 2 {
		t.Errorf("expected both Use buttons to fall back to nth-of-type paths, got %d", paths)
	}

	// Every generated selector has to work in the real tools, not just in the
	// page script that produced it.
	for _, sel := range want {
		if _, err := ProxyMgr.ClickElement(proxyID, "", "", sel, false, false); err != nil {
			t.Errorf("click %s: %v", sel, err)
		}
	}
}
