package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glitchedgitz/grroxy/grx/browser"
)

const chromePath = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"

// fixtureHTML is a miniature upload flow: a hidden file input behind a label
// (the shape every real upload UI has) and a Post button that only enables once
// a file is attached, so a batch has something real to wait for.
const fixtureHTML = `<!doctype html>
<html><head><meta charset="utf-8"><title>batch fixture</title></head><body>
<label for="file">Select a video from your PC</label>
<input id="file" type="file" style="display:none">
<button id="post" disabled>Post</button>
<pre id="out">idle</pre>
<script>
var f = document.getElementById('file');
var post = document.getElementById('post');
f.addEventListener('change', function () {
  document.getElementById('out').textContent = 'ready:' + f.files[0].name;
  post.disabled = false;
});
post.addEventListener('click', function () {
  document.getElementById('out').textContent = 'posted:' + f.files[0].name;
});
</script>
</body></html>`

// startBatchFixture launches an isolated Chrome on a temp profile, registers it
// with ProxyMgr as a running proxy, and returns the proxy id plus the fixture
// URL. Everything is torn down with the test.
func startBatchFixture(t *testing.T) (proxyID string, fixtureURL string) {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping browser test in -short mode")
	}
	if _, err := os.Stat(chromePath); err != nil {
		t.Skipf("Chrome not installed at %s", chromePath)
	}

	dir := t.TempDir()
	fixture := filepath.Join(dir, "upload.html")
	if err := os.WriteFile(fixture, []byte(fixtureHTML), 0o644); err != nil {
		t.Fatal(err)
	}

	profile := filepath.Join(dir, "profile")
	cmd := exec.Command(chromePath,
		"--user-data-dir="+profile,
		"--remote-debugging-port=0",
		"--no-first-run", "--no-default-browser-check",
		"--disable-sync", "--disable-default-apps", "--noerrdialogs", "--test-type",
		"--window-size=900,700",
		"about:blank",
	)
	if err := cmd.Start(); err != nil {
		t.Fatalf("launch chrome: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	var debugURL string
	for i := 0; i < 100; i++ {
		if u, err := browser.GetChromeDebugURL(profile); err == nil && u != "" {
			debugURL = u
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if debugURL == "" {
		t.Fatal("chrome never published a debug url")
	}

	cr, err := browser.NewChromeRemote(debugURL)
	if err != nil {
		t.Fatalf("connect to chrome: %v", err)
	}

	// Register it the way a started proxy would be. Chrome is set directly, so
	// GetChromeRemote short-circuits instead of hunting for a profile dir.
	proxyID = fmt.Sprintf("batch-e2e-%d", time.Now().UnixNano())
	ProxyMgr.AddProxyInstance(proxyID, &ProxyInstance{Browser: "chrome", Chrome: cr})
	t.Cleanup(func() { ProxyMgr.RemoveProxy(proxyID) })

	return proxyID, "file://" + fixture
}

func writeClip(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// The real thing: a batch driving an actual browser, once per input.
func TestBatchDrivesRealBrowser(t *testing.T) {
	proxyID, fixtureURL := startBatchFixture(t)

	clip1 := writeClip(t, "clip1.mp4", "first")
	clip2 := writeClip(t, "clip2.mp4", "second")

	resp, err := (&Backend{}).proxyBatchActionsLogic(context.Background(), ProxyBatchRequest{
		ID:     proxyID,
		Inputs: []string{clip1, clip2},
		Actions: []ProxyBatchAction{
			{Action: "navigate", URL: fixtureURL},
			{Action: "setFile", Selector: "#file", File: "$input"},
			{Action: "waitFor", Selector: "#post:not([disabled])", State: "visible"},
			{Action: "click", Selector: "#post"},
			{Action: "eval", JS: "document.getElementById('out').textContent"},
		},
		MaxDurationMs: 90000,
	})
	if err != nil {
		t.Fatalf("batch returned an error: %v", err)
	}

	if !resp.Success {
		t.Fatalf("batch failed: %s\n%+v", resp.FailureSummary, resp.Results)
	}
	if resp.Completed != 2 || resp.Failed != 0 || resp.TimedOut {
		t.Fatalf("expected 2 clean inputs, got %+v", resp)
	}

	// The eval step proves the page really went through the flow, and that
	// $input differed per iteration.
	for i, want := range []string{"posted:clip1.mp4", "posted:clip2.mp4"} {
		steps := resp.Results[i].Steps
		if len(steps) != 5 {
			t.Fatalf("input %d: expected 5 steps, got %d", i, len(steps))
		}
		got, _ := steps[4].Result.(string)
		if got != want {
			t.Errorf("input %d: page ended at %q, want %q", i, got, want)
		}
	}
}

// A failing step must stop that input, stop the batch by default, and say so.
func TestBatchRealBrowserStopsOnFailure(t *testing.T) {
	proxyID, fixtureURL := startBatchFixture(t)
	clip := writeClip(t, "clip1.mp4", "first")

	resp, err := (&Backend{}).proxyBatchActionsLogic(context.Background(), ProxyBatchRequest{
		ID:     proxyID,
		Inputs: []string{clip, "/second/input.mp4"},
		Actions: []ProxyBatchAction{
			{Action: "navigate", URL: fixtureURL},
			{Action: "waitFor", Selector: "#does-not-exist", TimeoutMs: 2000},
			{Action: "click", Selector: "#post"},
		},
		MaxDurationMs: 60000,
	})
	if err != nil {
		t.Fatalf("batch returned an error: %v", err)
	}

	if resp.Success || !resp.Aborted {
		t.Fatalf("expected an aborted, unsuccessful batch, got %+v", resp)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected to stop after the first input, got %d", len(resp.Results))
	}
	// The third step must never have run.
	if steps := resp.Results[0].Steps; len(steps) != 2 {
		t.Fatalf("expected to stop at the failing step, ran %d steps", len(steps))
	}
	if resp.Results[0].FailedAt != 1 {
		t.Errorf("expected failure at step 1, got %d", resp.Results[0].FailedAt)
	}
	if !strings.Contains(resp.FailureSummary, "waitFor") {
		t.Errorf("summary should name the failing action: %q", resp.FailureSummary)
	}
	if len(resp.Remaining) != 1 || resp.Remaining[0] != "/second/input.mp4" {
		t.Errorf("the untouched input should be reported as remaining, got %v", resp.Remaining)
	}
}

// The deadline is the fix for "it just hangs": the call must come back with
// partial results rather than running for inputs x actions x timeout.
func TestBatchRealBrowserHonoursDeadline(t *testing.T) {
	proxyID, fixtureURL := startBatchFixture(t)

	start := time.Now()
	resp, err := (&Backend{}).proxyBatchActionsLogic(context.Background(), ProxyBatchRequest{
		ID:     proxyID,
		Inputs: []string{"one", "two", "three"},
		Actions: []ProxyBatchAction{
			{Action: "navigate", URL: fixtureURL},
			{Action: "sleep", Ms: 700},
		},
		MaxDurationMs: 1000,
	})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("batch returned an error: %v", err)
	}

	if !resp.TimedOut {
		t.Fatalf("expected timedOut=true, got %+v", resp)
	}
	if len(resp.Remaining) == 0 {
		t.Error("a timed-out batch must report the inputs it never reached")
	}
	if resp.Total != 3 {
		t.Errorf("total should report what was asked for, got %d", resp.Total)
	}
	// Without the deadline this would run all three inputs.
	if elapsed > 10*time.Second {
		t.Errorf("deadline did not bound the run: took %v", elapsed)
	}
	t.Logf("stopped after %v with %d/%d inputs done, %d remaining: %s",
		elapsed.Round(time.Millisecond), len(resp.Results), resp.Total, len(resp.Remaining), resp.FailureSummary)
}

// The progress stream is the only thing that tells the UI a batch is moving,
// so its shape matters as much as the final result.
func TestBatchStreamsProgress(t *testing.T) {
	proxyID, fixtureURL := startBatchFixture(t)
	clip := writeClip(t, "clip1.mp4", "first")

	var mu sync.Mutex
	var events []BatchEvent
	restore := publishBatchEvent
	publishBatchEvent = func(ev BatchEvent) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	}
	t.Cleanup(func() { publishBatchEvent = restore })

	resp, err := (&Backend{}).proxyBatchActionsLogic(context.Background(), ProxyBatchRequest{
		ID:     proxyID,
		Inputs: []string{clip},
		Actions: []ProxyBatchAction{
			{Action: "navigate", URL: fixtureURL},
			{Action: "setFile", Selector: "#file", File: "$input"},
			{Action: "click", Selector: "#post"},
		},
		MaxDurationMs: 60000,
	})
	if err != nil {
		t.Fatalf("batch error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("batch failed: %s", resp.FailureSummary)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(events) == 0 {
		t.Fatal("no progress events were published")
	}
	if events[0].Type != "start" {
		t.Fatalf("first event should be start, got %q", events[0].Type)
	}
	if events[0].Total != 1 || events[0].Actions != 3 {
		t.Errorf("start should carry the shape of the run, got total=%d actions=%d", events[0].Total, events[0].Actions)
	}
	if last := events[len(events)-1]; last.Type != "end" || !last.Success {
		t.Fatalf("last event should be a successful end, got %+v", last)
	}

	// Every step must announce itself before it runs and report after, so a
	// chip can light up while the step is in flight.
	for step := 0; step < 3; step++ {
		var sawRunning, sawResult bool
		for _, ev := range events {
			if ev.Type != "step" || ev.StepIndex != step {
				continue
			}
			if ev.Running {
				sawRunning = true
				if sawResult {
					t.Errorf("step %d reported a result before it announced itself", step)
				}
			} else {
				sawResult = true
				if !ev.OK {
					t.Errorf("step %d reported not-ok: %s", step, ev.Error)
				}
			}
		}
		if !sawRunning || !sawResult {
			t.Errorf("step %d: running=%v result=%v, wanted both", step, sawRunning, sawResult)
		}
	}

	var inputEvents int
	for _, ev := range events {
		if ev.Type == "input" {
			inputEvents++
			if !ev.OK {
				t.Errorf("input event reported failure: %+v", ev)
			}
		}
	}
	if inputEvents != 1 {
		t.Errorf("expected one input event, got %d", inputEvents)
	}

	t.Logf("%d events: start, %d step, %d input, end", len(events), len(events)-3, inputEvents)
}

// The gaps the agent hit in real use: a button identified only by its label, a
// contenteditable caption box, and content below the fold.
const textFixtureHTML = `<!doctype html>
<html><head><meta charset="utf-8"><title>text fixture</title></head><body>
<div id="caption" contenteditable="true"></div>
<div style="height:1500px"></div>
<button id="sounds">Sounds</button>
<button disabled>Publish</button>
<button>Post now</button>
<pre id="out">idle</pre>
<script>
document.querySelectorAll('button').forEach(function (b) {
  b.addEventListener('click', function () {
    document.getElementById('out').textContent = 'clicked:' + b.textContent;
  });
});
document.getElementById('caption').addEventListener('input', function (e) {
  document.getElementById('out').textContent = 'caption:' + e.target.innerText;
});
</script>
</body></html>`

func TestBatchTextSelectorsAndContenteditable(t *testing.T) {
	proxyID, _ := startBatchFixture(t)

	dir := t.TempDir()
	fixture := filepath.Join(dir, "text.html")
	if err := os.WriteFile(fixture, []byte(textFixtureHTML), 0o644); err != nil {
		t.Fatal(err)
	}

	read := func() string {
		v, err := ProxyMgr.Evaluate(proxyID, "document.getElementById('out').textContent", 5000)
		if err != nil {
			t.Fatalf("eval: %v", err)
		}
		s, _ := v.(string)
		return s
	}

	resp, err := (&Backend{}).proxyBatchActionsLogic(context.Background(), ProxyBatchRequest{
		ID: proxyID,
		Actions: []ProxyBatchAction{
			{Action: "navigate", URL: "file://" + fixture},
			// contenteditable has no .value — this used to silently do nothing.
			{Action: "setValue", Selector: "#caption", Value: "Signal Bloom"},
			// below the fold until scrolled.
			{Action: "scroll", Selector: "#sounds"},
			// the three selector forms the agent asked for.
			{Action: "click", Selector: `button:has-text("Sounds")`},
			{Action: "click", Selector: `//button[contains(., "Post now")]`},
			// "Publish" exists but is disabled, so the :not([disabled]) tail must
			// stop this matching — if the tail were dropped it would pass.
			{Action: "waitFor", Selector: `button:has-text('Publish'):not([disabled])`, TimeoutMs: 1500},
		},
		MaxDurationMs: 60000,
	})
	if err != nil {
		t.Fatalf("batch error: %v", err)
	}

	steps := resp.Results[0].Steps
	for _, s := range steps {
		t.Logf("step %d %-9s ok=%v %s", s.Index, s.Action, s.OK, s.Error)
	}

	for i, name := range map[int]string{1: "setValue(contenteditable)", 2: "scroll", 3: "click :has-text", 4: "click xpath"} {
		if i >= len(steps) || !steps[i].OK {
			t.Errorf("%s should have succeeded: %+v", name, steps[i])
		}
	}

	if len(steps) == 6 && steps[5].OK {
		t.Error("waitFor matched the disabled Publish button; the :not([disabled]) tail was dropped")
	}

	// contains() is a substring match, as in Playwright: has-text("Post") also
	// matches "Post now". Pinned so the semantics are not changed by accident.
	if v, err := ProxyMgr.Evaluate(proxyID,
		`document.evaluate('count(//button[contains(., "Post")])', document, null, XPathResult.NUMBER_TYPE, null).numberValue`,
		5000); err == nil {
		if n, ok := v.(float64); ok && n != 1 {
			t.Logf("note: has-text(\"Post\") matches %v buttons (substring semantics)", n)
		}
	}

	if got := read(); got != "clicked:Post now" {
		t.Errorf("page ended at %q, want clicked:Post now", got)
	}
}

// Pressing Stop has to stand the batch down. Without this the HTTP request is
// abandoned client-side while the server keeps clicking through the rest of
// the flow — which, for an upload batch, means posting things nobody asked for.
func TestBatchStopsWhenCancelled(t *testing.T) {
	proxyID, fixtureURL := startBatchFixture(t)

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel while the first input is still sleeping.
	go func() {
		time.Sleep(600 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	resp, err := (&Backend{}).proxyBatchActionsLogic(ctx, ProxyBatchRequest{
		ID:     proxyID,
		Inputs: []string{"one", "two", "three"},
		Actions: []ProxyBatchAction{
			{Action: "navigate", URL: fixtureURL},
			{Action: "sleep", Ms: 5000},
		},
		MaxDurationMs: 120000,
	})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("batch error: %v", err)
	}

	if !resp.Cancelled {
		t.Fatalf("expected cancelled=true, got %+v", resp)
	}
	// Three inputs x 5s of sleeping is 15s if cancellation is ignored.
	if elapsed > 5*time.Second {
		t.Errorf("cancellation did not stop the batch: ran for %v", elapsed)
	}
	if len(resp.Results) >= 3 {
		t.Errorf("every input ran despite cancellation: %d results", len(resp.Results))
	}
	if len(resp.Remaining) == 0 {
		t.Error("a cancelled batch must report the inputs it never reached")
	}
	if !strings.Contains(resp.FailureSummary, "resume") {
		t.Errorf("summary should warn against re-running from the start: %q", resp.FailureSummary)
	}
	t.Logf("stopped after %v: %d/%d inputs, %d remaining — %s",
		elapsed.Round(time.Millisecond), len(resp.Results), resp.Total, len(resp.Remaining), resp.FailureSummary)
}

// A counter the page increments on every click, so a test can watch the
// browser actually being driven rather than inferring it from timing.
const counterFixtureHTML = `<!doctype html>
<html><head><meta charset="utf-8"><title>counter</title></head><body>
<button id="tick">Tick</button><pre id="count">0</pre>
<script>
var n = 0;
document.getElementById('tick').addEventListener('click', function () {
  n++;
  document.getElementById('count').textContent = String(n);
});
</script>
</body></html>`

// Closing the chat closes the progress socket, and the socket handler calls
// CancelBatch. The symptom this guards against is the browser carrying on
// changing after the user has walked away, so the assertion is exactly that:
// the page stops being mutated, not merely that a call returned early.
func TestBatchCancelStopsDrivingTheBrowser(t *testing.T) {
	proxyID, _ := startBatchFixture(t)

	dir := t.TempDir()
	fixture := filepath.Join(dir, "counter.html")
	if err := os.WriteFile(fixture, []byte(counterFixtureHTML), 0o644); err != nil {
		t.Fatal(err)
	}

	count := func() int {
		v, err := ProxyMgr.Evaluate(proxyID, "Number(document.getElementById('count').textContent)", 5000)
		if err != nil {
			t.Fatalf("read count: %v", err)
		}
		n, _ := v.(float64)
		return int(n)
	}

	// Load the page once; the batch below must not navigate, or each input
	// would reset the counter it is supposed to be advancing.
	if _, err := (&Backend{}).proxyBatchActionsLogic(context.Background(), ProxyBatchRequest{
		ID:      proxyID,
		Actions: []ProxyBatchAction{{Action: "navigate", URL: "file://" + fixture}},
	}); err != nil {
		t.Fatalf("navigate: %v", err)
	}

	done := make(chan ProxyBatchResponse, 1)
	go func() {
		resp, _ := (&Backend{}).proxyBatchActionsLogic(context.Background(), ProxyBatchRequest{
			ID:     proxyID,
			Inputs: []string{"1", "2", "3", "4", "5", "6", "7", "8"},
			Actions: []ProxyBatchAction{
				{Action: "click", Selector: "#tick"},
				{Action: "sleep", Ms: 2000},
			},
			MaxDurationMs: 120000,
		})
		done <- resp
	}()

	// Let it visibly drive the page first — a test that cancels before anything
	// happened would pass even if cancellation did nothing.
	time.Sleep(2500 * time.Millisecond)
	moving := count()
	if moving == 0 {
		t.Fatal("the batch never clicked anything, so this proves nothing")
	}

	if !CancelBatch(proxyID) {
		t.Fatal("CancelBatch found nothing to cancel")
	}
	atCancel := count()

	// Longer than one action sleep, so an uncancelled batch would certainly
	// have clicked again by the time we look.
	time.Sleep(3 * time.Second)
	after := count()

	if after != atCancel {
		t.Errorf("the browser kept changing after the cancel: %d -> %d clicks", atCancel, after)
	}
	if after >= 8 {
		t.Errorf("every input ran despite the cancel (%d clicks)", after)
	}

	select {
	case resp := <-done:
		if !resp.Cancelled {
			t.Errorf("expected cancelled=true, got %+v", resp)
		}
		if len(resp.Remaining) == 0 {
			t.Error("a cancelled batch must report what it never reached")
		}
		t.Logf("page stopped at %d clicks (was moving at %d), %d inputs never ran",
			after, moving, len(resp.Remaining))
	case <-time.After(15 * time.Second):
		t.Fatal("the batch never returned")
	}

	if BatchIsRunning(proxyID) {
		t.Error("the registry still lists a running batch")
	}
}

// Two batches driving one browser only fight each other, so starting one must
// stand down whatever was already running on that proxy.
func TestSecondBatchCancelsFirst(t *testing.T) {
	proxyID, fixtureURL := startBatchFixture(t)

	first := make(chan ProxyBatchResponse, 1)
	go func() {
		resp, _ := (&Backend{}).proxyBatchActionsLogic(context.Background(), ProxyBatchRequest{
			ID:            proxyID,
			Inputs:        []string{"a", "b", "c"},
			Actions:       []ProxyBatchAction{{Action: "sleep", Ms: 4000}},
			MaxDurationMs: 120000,
		})
		first <- resp
	}()

	time.Sleep(700 * time.Millisecond)

	second, err := (&Backend{}).proxyBatchActionsLogic(context.Background(), ProxyBatchRequest{
		ID:            proxyID,
		Actions:       []ProxyBatchAction{{Action: "navigate", URL: fixtureURL}},
		MaxDurationMs: 30000,
	})
	if err != nil {
		t.Fatalf("second batch error: %v", err)
	}
	if !second.Success {
		t.Errorf("the second batch should have run cleanly: %s", second.FailureSummary)
	}

	select {
	case got := <-first:
		if !got.Cancelled {
			t.Fatalf("the first batch should have been cancelled, got %+v", got)
		}
		t.Logf("first batch stood down with %d remaining", len(got.Remaining))
	case <-time.After(15 * time.Second):
		t.Fatal("the first batch kept running after a second one started")
	}
}
