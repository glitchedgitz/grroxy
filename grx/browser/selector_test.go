package browser

import "testing"

// CSS must keep taking the ByQuery path untouched; anything that cannot be
// expressed as CSS has to end up as a search.
func TestResolveSelector(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		want     string
		isSearch bool
	}{
		{"plain css", "#post", "#post", false},
		{"attribute css", `input[type=file][accept*="video"]`, `input[type=file][accept*="video"]`, false},
		{"css with :not stays css", "button:not([disabled])", "button:not([disabled])", false},
		{"xpath passthrough", `//button[contains(., "Post")]`, `//button[contains(., "Post")]`, true},
		{"xpath grouped", `(//button)[1]`, `(//button)[1]`, true},
		{"xpath prefixed", `xpath=//div[@id="x"]`, `//div[@id="x"]`, true},
		{"text prefixed", "text=Post now", "Post now", true},
		{"has-text double quotes", `button:has-text("Sounds")`, `//button[contains(., "Sounds")]`, true},
		{"has-text single quotes", `button:has-text('Post')`, `//button[contains(., "Post")]`, true},
		{"has-text no tag", `:has-text("Save")`, `//*[contains(., "Save")]`, true},
		{"has-text with :not", `button:has-text('Post'):not([disabled])`, `//button[contains(., "Post") and not(@disabled)]`, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveSelector(tc.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Value != tc.want {
				t.Errorf("value = %q, want %q", got.Value, tc.want)
			}
			if got.Search != tc.isSearch {
				t.Errorf("search = %v, want %v", got.Search, tc.isSearch)
			}
		})
	}
}

// A :has-text() shape we cannot translate must say so rather than quietly
// matching the wrong element.
func TestResolveSelectorRejectsUntranslatable(t *testing.T) {
	_, err := resolveSelector(`button:has-text("Post"):nth-child(2)`)
	if err == nil {
		t.Fatal("expected an error for an untranslatable :has-text selector")
	}
	if got := err.Error(); got == "" {
		t.Fatal("error should explain what to do instead")
	}
}

// The JS form has to match the same element as the node query would.
func TestResolvedSelectorJS(t *testing.T) {
	css, _ := resolveSelector("#post")
	if js := css.JS(); js != `document.querySelector("#post")` {
		t.Errorf("css JS = %s", js)
	}

	xp, _ := resolveSelector(`button:has-text("Post")`)
	js := xp.JS()
	if want := "document.evaluate("; len(js) < len(want) || js[:len(want)] != want {
		t.Errorf("xpath JS should use document.evaluate, got %s", js)
	}
}
