package app

import (
	"context"
	"strings"
	"testing"
)

func TestValidateBatchAction(t *testing.T) {
	cases := []struct {
		name string
		a    ProxyBatchAction
		want string // "" means it must pass
	}{
		{"missing action", ProxyBatchAction{}, "action is required"},
		{"unknown verb", ProxyBatchAction{Action: "hover"}, `unknown action "hover"`},
		{"click without selector", ProxyBatchAction{Action: "click"}, "selector is required"},
		{"waitFor without selector", ProxyBatchAction{Action: "waitFor"}, "selector is required"},
		{"setFile without any selector", ProxyBatchAction{Action: "setFile", File: "/a.mp4"}, "either selector or clickSelector"},
		{"setFile with both selectors", ProxyBatchAction{Action: "setFile", Selector: "#a", ClickSelector: "#b", File: "/a.mp4"}, "not both"},
		{"setFile without a file", ProxyBatchAction{Action: "setFile", Selector: "#a"}, "file or files is required"},
		{"eval without js", ProxyBatchAction{Action: "eval"}, "js is required"},
		{"navigate without url", ProxyBatchAction{Action: "navigate"}, "url is required"},
		{"sleep without ms", ProxyBatchAction{Action: "sleep"}, "ms must be greater than 0"},

		{"valid click", ProxyBatchAction{Action: "click", Selector: "#go"}, ""},
		{"valid setFile by selector", ProxyBatchAction{Action: "setFile", Selector: "#f", File: "/a.mp4"}, ""},
		{"valid setFile by clickSelector", ProxyBatchAction{Action: "setFile", ClickSelector: "#btn", Files: []string{"/a.mp4"}}, ""},
		{"valid sleep", ProxyBatchAction{Action: "sleep", Ms: 100}, ""},
		{"valid goBack needs nothing", ProxyBatchAction{Action: "goBack"}, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateBatchAction(0, tc.a)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("expected it to pass, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected an error mentioning %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not mention %q", err.Error(), tc.want)
			}
		})
	}
}

func TestSubstituteInput(t *testing.T) {
	original := ProxyBatchAction{
		Action:   "setFile",
		Selector: "input[data-name=$input]",
		File:     "$input",
		Files:    []string{"$input", "/fixed.mp4"},
		Text:     "caption for $input",
		Value:    "$input",
		URL:      "https://example.com/?q=$input",
		JS:       "document.title = '$input'",
	}

	got := substituteInput(original, "/videos/clip1.mp4")

	for field, value := range map[string]string{
		"Selector": got.Selector, "File": got.File, "Text": got.Text,
		"Value": got.Value, "URL": got.URL, "JS": got.JS, "Files[0]": got.Files[0],
	} {
		if strings.Contains(value, inputPlaceholder) {
			t.Errorf("%s still contains %s: %q", field, inputPlaceholder, value)
		}
		if !strings.Contains(value, "clip1.mp4") {
			t.Errorf("%s was not substituted: %q", field, value)
		}
	}

	if got.Files[1] != "/fixed.mp4" {
		t.Errorf("a file without the placeholder was altered: %q", got.Files[1])
	}

	// The caller's action list is reused for every input, so it must not be
	// mutated by a pass.
	if original.File != "$input" || original.Files[0] != "$input" {
		t.Fatalf("substituteInput mutated the caller's action: %+v", original)
	}
}

func TestBatchValidatesEveryActionBeforeRunning(t *testing.T) {
	// A bad verb in the last step must be caught before step 1 runs, otherwise
	// a typo leaves the page half-way through a flow.
	_, err := (&Backend{}).proxyBatchActionsLogic(context.Background(), ProxyBatchRequest{
		ID:     "no-such-proxy",
		Inputs: []string{"a", "b"},
		Actions: []ProxyBatchAction{
			{Action: "click", Selector: "#ok"},
			{Action: "nope"},
		},
	})
	if err == nil {
		t.Fatal("expected a validation error")
	}
	if !strings.Contains(err.Error(), `actions[1]`) || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("error should name the offending step: %v", err)
	}
}

func TestBatchRequiresIDAndActions(t *testing.T) {
	b := &Backend{}
	if _, err := b.proxyBatchActionsLogic(context.Background(), ProxyBatchRequest{Actions: []ProxyBatchAction{{Action: "goBack"}}}); err == nil || !strings.Contains(err.Error(), "id is required") {
		t.Fatalf("expected an id error, got %v", err)
	}
	if _, err := b.proxyBatchActionsLogic(context.Background(), ProxyBatchRequest{ID: "1"}); err == nil || !strings.Contains(err.Error(), "actions is required") {
		t.Fatalf("expected an actions error, got %v", err)
	}
}

// With a proxy that does not exist every action fails, which is a clean way to
// pin down the loop's bookkeeping without a browser.
func TestBatchFailureBookkeeping(t *testing.T) {
	actions := []ProxyBatchAction{
		{Action: "click", Selector: "#a"},
		{Action: "click", Selector: "#b"},
	}

	t.Run("stops at the first failing input by default", func(t *testing.T) {
		resp, err := (&Backend{}).proxyBatchActionsLogic(context.Background(), ProxyBatchRequest{
			ID: "no-such-proxy", Inputs: []string{"one", "two", "three"}, Actions: actions,
		})
		if err != nil {
			t.Fatalf("the batch itself should not error: %v", err)
		}
		if !resp.Aborted {
			t.Fatal("expected the default to abort")
		}
		if resp.Total != 3 || resp.Failed != 1 || resp.Completed != 0 {
			t.Fatalf("expected to stop after the first input of 3, got %+v", resp)
		}
		if resp.Success {
			t.Fatalf("expected success=false, got %+v", resp)
		}
		if len(resp.Results) != 1 {
			t.Fatalf("expected to stop after one input, got %d results", len(resp.Results))
		}
		if resp.FailureSummary == "" {
			t.Fatal("a failed batch must carry a failureSummary")
		}
		if !strings.Contains(resp.FailureSummary, "one") || !strings.Contains(resp.FailureSummary, "continueOnError") {
			t.Errorf("summary should name the input and why it stopped: %q", resp.FailureSummary)
		}
	})

	t.Run("runs every input when asked to continue", func(t *testing.T) {
		keepGoing := true
		resp, err := (&Backend{}).proxyBatchActionsLogic(context.Background(), ProxyBatchRequest{
			ID: "no-such-proxy", Inputs: []string{"one", "two", "three"},
			Actions: actions, ContinueOnError: &keepGoing,
		})
		if err != nil {
			t.Fatalf("the batch itself should not error: %v", err)
		}
		if resp.Total != 3 || resp.Failed != 3 || resp.Completed != 0 {
			t.Fatalf("expected 3 failed of 3, got %+v", resp)
		}
		if resp.Success || resp.Aborted {
			t.Fatalf("expected success=false aborted=false, got %+v", resp)
		}
		if len(resp.Results) != 3 {
			t.Fatalf("expected a result per input, got %d", len(resp.Results))
		}
		if !strings.Contains(resp.FailureSummary, "3 of 3 inputs failed") {
			t.Errorf("summary should count the failures: %q", resp.FailureSummary)
		}
		// Each input stops at its first failing step rather than running on.
		for _, r := range resp.Results {
			if len(r.Steps) != 1 {
				t.Errorf("input %q: expected to stop after the failing step, ran %d", r.Input, len(r.Steps))
			}
			if r.FailedAt != 0 || r.Steps[0].OK || r.Steps[0].Error == "" {
				t.Errorf("input %q: expected step 0 recorded as failed with a reason, got %+v", r.Input, r.Steps[0])
			}
		}
		if resp.Results[0].Input != "one" || resp.Results[2].Input != "three" {
			t.Errorf("inputs should be reported in order, got %q..%q", resp.Results[0].Input, resp.Results[2].Input)
		}
	})

	t.Run("no inputs runs the sequence once", func(t *testing.T) {
		resp, err := (&Backend{}).proxyBatchActionsLogic(context.Background(), ProxyBatchRequest{
			ID: "no-such-proxy", Actions: actions,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Total != 1 || len(resp.Results) != 1 {
			t.Fatalf("expected a single pass, got %+v", resp)
		}
		if resp.Results[0].Input != "" {
			t.Errorf("expected an empty input for the single pass, got %q", resp.Results[0].Input)
		}
	})
}
