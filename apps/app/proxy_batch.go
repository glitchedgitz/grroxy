package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/glitchedgitz/pocketbase/apis"
	"github.com/glitchedgitz/pocketbase/core"
	"github.com/glitchedgitz/pocketbase/models"
	"github.com/labstack/echo/v5"
)

// inputPlaceholder is replaced with the current input in every string field of
// an action before it runs, so one action list can be reused per input.
const inputPlaceholder = "$input"

// batchDefaultTimeoutMs is the per-action budget when the caller sets none.
// Deliberately generous: a batch step is usually waiting on a page, and a
// too-tight default turns a working flow into a flaky one.
const batchDefaultTimeoutMs = 15000

// batchDefaultMaxDurationMs bounds the whole batch, not just one action.
// Without it the worst case is inputs x actions x per-action timeout, which for
// even a small batch is minutes of a blocked tool call with nothing coming back
// — the caller cannot tell a slow page from a hang. When the deadline hits we
// stop and return what we have, so the caller always learns where it got to.
// Kept well under the timeouts MCP and HTTP clients tend to impose, so the
// partial result actually arrives instead of the call being killed.
const batchDefaultMaxDurationMs = 60000

// ProxyBatchAction is one step of a batch. Action names the verb, and the rest
// of the fields are its arguments — each verb forwards to the same code the
// matching proxy tool uses, so behaviour is identical to running the tools by
// hand.
type ProxyBatchAction struct {
	Action string `json:"action"`

	Selector      string   `json:"selector,omitempty"`
	ClickSelector string   `json:"clickSelector,omitempty"`
	File          string   `json:"file,omitempty"`
	Files         []string `json:"files,omitempty"`
	Text          string   `json:"text,omitempty"`
	Value         string   `json:"value,omitempty"`
	URL           string   `json:"url,omitempty"`
	JS            string   `json:"js,omitempty"`
	State         string   `json:"state,omitempty"`

	ClearFirst        bool `json:"clearFirst,omitempty"`
	WaitForNavigation bool `json:"waitForNavigation,omitempty"`
	UseJS             bool `json:"useJS,omitempty"`
	BypassCache       bool `json:"bypassCache,omitempty"`

	Ms        int `json:"ms,omitempty"`
	TimeoutMs int `json:"timeoutMs,omitempty"`
}

type ProxyBatchRequest struct {
	ID              string             `json:"id"`
	Inputs          []string           `json:"inputs"`
	Actions         []ProxyBatchAction `json:"actions"`
	ContinueOnError *bool              `json:"continueOnError"` // nil means false: stop at the first failing input
	TimeoutMs       int                `json:"timeoutMs"`
	MaxDurationMs   int                `json:"maxDurationMs"`
}

type ProxyBatchStep struct {
	Index  int    `json:"index"`
	Action string `json:"action"`
	OK     bool   `json:"ok"`
	Error  string `json:"error,omitempty"`
	Result any    `json:"result,omitempty"`
}

type ProxyBatchInputResult struct {
	Input    string           `json:"input"`
	OK       bool             `json:"ok"`
	FailedAt int              `json:"failedAt,omitempty"`
	Steps    []ProxyBatchStep `json:"steps"`
}

type ProxyBatchResponse struct {
	Success   bool `json:"success"`
	Total     int  `json:"total"`
	Completed int  `json:"completed"`
	Failed    int  `json:"failed"`
	Aborted   bool `json:"aborted,omitempty"`
	// TimedOut means the batch deadline was reached; Remaining lists the inputs
	// that never ran, so the caller can pick up where this left off.
	TimedOut bool `json:"timedOut,omitempty"`
	// Cancelled means the caller went away (Stop pressed, client disconnected)
	// and the batch stood down instead of carrying on driving the browser.
	Cancelled bool     `json:"cancelled,omitempty"`
	Remaining []string `json:"remaining,omitempty"`
	// FailureSummary is the one-line "what went wrong", set whenever Failed > 0
	// so every surface can report the same thing without re-deriving it.
	FailureSummary string                  `json:"failureSummary,omitempty"`
	Results        []ProxyBatchInputResult `json:"results"`
}

// batchVerbs is the set of actions a batch understands, each mapping to the
// proxy tool of the same name.
var batchVerbs = map[string]bool{
	"click": true, "waitFor": true, "setFile": true, "type": true,
	"setValue": true, "eval": true, "navigate": true, "sleep": true,
	"reload": true, "goBack": true, "goForward": true, "scroll": true,
}

// validateBatchAction checks one action without running it. Every action is
// checked before the batch starts, so a typo in step 4 cannot leave the page
// half-way through a flow.
func validateBatchAction(i int, a ProxyBatchAction) error {
	if a.Action == "" {
		return fmt.Errorf("actions[%d]: action is required", i)
	}
	if !batchVerbs[a.Action] {
		return fmt.Errorf("actions[%d]: unknown action %q", i, a.Action)
	}

	needsSelector := map[string]bool{"click": true, "waitFor": true, "type": true, "setValue": true, "scroll": true}
	if needsSelector[a.Action] && a.Selector == "" {
		return fmt.Errorf("actions[%d] (%s): selector is required", i, a.Action)
	}

	switch a.Action {
	case "setFile":
		if a.Selector == "" && a.ClickSelector == "" {
			return fmt.Errorf("actions[%d] (setFile): either selector or clickSelector is required", i)
		}
		if a.Selector != "" && a.ClickSelector != "" {
			return fmt.Errorf("actions[%d] (setFile): pass either selector or clickSelector, not both", i)
		}
		if a.File == "" && len(a.Files) == 0 {
			return fmt.Errorf("actions[%d] (setFile): file or files is required", i)
		}
	case "eval":
		if a.JS == "" {
			return fmt.Errorf("actions[%d] (eval): js is required", i)
		}
	case "navigate":
		if a.URL == "" {
			return fmt.Errorf("actions[%d] (navigate): url is required", i)
		}
	case "sleep":
		if a.Ms <= 0 {
			return fmt.Errorf("actions[%d] (sleep): ms must be greater than 0", i)
		}
	}
	return nil
}

// substituteInput swaps $input into every string field of an action. Returns a
// copy so the caller's action list is reusable across inputs.
func substituteInput(a ProxyBatchAction, input string) ProxyBatchAction {
	rep := func(s string) string { return strings.ReplaceAll(s, inputPlaceholder, input) }

	a.Selector = rep(a.Selector)
	a.ClickSelector = rep(a.ClickSelector)
	a.File = rep(a.File)
	a.Text = rep(a.Text)
	a.Value = rep(a.Value)
	a.URL = rep(a.URL)
	a.JS = rep(a.JS)

	if len(a.Files) > 0 {
		files := make([]string, len(a.Files))
		for i, f := range a.Files {
			files[i] = rep(f)
		}
		a.Files = files
	}
	return a
}

// runBatchAction executes one action against a proxy, forwarding to the same
// ProxyMgr / ChromeRemote calls the individual proxy tools use.
func runBatchAction(ctx context.Context, proxyID string, a ProxyBatchAction, defaultTimeoutMs int) (any, error) {
	timeout := a.TimeoutMs
	if timeout <= 0 {
		timeout = defaultTimeoutMs
	}

	switch a.Action {
	case "click":
		res, err := ProxyMgr.ClickElement(proxyID, "", a.Selector, a.WaitForNavigation, "", a.UseJS)
		if err != nil {
			return nil, err
		}
		return map[string]any{"clickMode": res.ClickMode}, nil

	case "waitFor":
		return nil, ProxyMgr.WaitForSelector(proxyID, a.Selector, timeout, a.State)

	case "setFile":
		files := a.Files
		if len(files) == 0 {
			files = []string{a.File}
		}
		return nil, ProxyMgr.SetFileInput(proxyID, a.Selector, a.ClickSelector, files, timeout)

	case "type":
		return nil, ProxyMgr.TypeText(proxyID, a.Selector, a.Text, a.ClearFirst, timeout)

	case "setValue":
		return nil, ProxyMgr.SetValue(proxyID, a.Selector, a.Value, timeout)

	case "scroll":
		return nil, ProxyMgr.ScrollIntoView(proxyID, a.Selector, timeout)

	case "eval":
		return ProxyMgr.Evaluate(proxyID, a.JS, timeout)

	case "sleep":
		// Interruptible: a 30s sleep must not keep a cancelled batch alive.
		select {
		case <-time.After(time.Duration(a.Ms) * time.Millisecond):
			return nil, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// The remaining verbs drive the tab directly, same as the tab tools do.
	chrome, err := ProxyMgr.GetChromeRemote(proxyID)
	if err != nil {
		return nil, err
	}

	switch a.Action {
	case "navigate":
		res, err := chrome.Navigate("", a.URL, "", timeout)
		if err != nil {
			return nil, err
		}
		return map[string]any{"url": res.FinalURL, "status": res.Status}, nil
	case "reload":
		return nil, chrome.ReloadTab("", a.BypassCache, timeout)
	case "goBack":
		return nil, chrome.GoBack("", timeout)
	case "goForward":
		return nil, chrome.GoForward("", timeout)
	}

	return nil, fmt.Errorf("unknown action %q", a.Action)
}

// --- running batches --------------------------------------------------------
//
// A batch keeps driving the browser until it finishes, and the caller going
// away is not enough to stop it: an MCP tool call does not share the chat's
// HTTP request, and closing the chat panel aborts no fetch. So the only
// reliable handle is a server-side one, keyed by proxy, which the progress
// socket then uses to stand the batch down when its watcher disappears.
type batchHandle struct {
	cancel context.CancelFunc
}

var runningBatches = struct {
	mu sync.Mutex
	m  map[string]*batchHandle
}{m: map[string]*batchHandle{}}

// registerBatch makes a batch cancellable by proxy id and returns the release
// to call when it finishes. Any batch already running on that proxy is
// cancelled first — two batches driving one browser only fight each other, and
// the older one is invariably the orphan nobody is watching.
func registerBatch(proxyID string, cancel context.CancelFunc) func() {
	h := &batchHandle{cancel: cancel}

	runningBatches.mu.Lock()
	if previous, ok := runningBatches.m[proxyID]; ok {
		log.Printf("[proxyBatchActions] a batch was already running on %s — cancelling it", proxyID)
		previous.cancel()
	}
	runningBatches.m[proxyID] = h
	runningBatches.mu.Unlock()

	return func() {
		runningBatches.mu.Lock()
		// Compared by pointer: a newer batch may have replaced this entry, and
		// clearing that one would leave it uncancellable.
		if runningBatches.m[proxyID] == h {
			delete(runningBatches.m, proxyID)
		}
		runningBatches.mu.Unlock()
	}
}

// CancelBatch stops the batch running on a proxy, if any. Reports whether
// there was one, so a caller can tell "stopped it" from "nothing to stop".
func CancelBatch(proxyID string) bool {
	runningBatches.mu.Lock()
	defer runningBatches.mu.Unlock()

	h, ok := runningBatches.m[proxyID]
	if !ok {
		return false
	}
	log.Printf("[proxyBatchActions] cancelling the batch on %s", proxyID)
	h.cancel()
	delete(runningBatches.m, proxyID)
	return true
}

// BatchIsRunning reports whether a batch is in flight on that proxy.
func BatchIsRunning(proxyID string) bool {
	runningBatches.mu.Lock()
	defer runningBatches.mu.Unlock()
	_, ok := runningBatches.m[proxyID]
	return ok
}

// pinBatchTarget activates one tab so every action in the batch addresses the
// same one. It prefers a real page over a blank tab, since a browser that was
// just started has an about:blank sitting alongside whatever is being driven.
func pinBatchTarget(proxyID string) {
	chrome, err := ProxyMgr.GetChromeRemote(proxyID)
	if err != nil {
		log.Printf("[proxyBatchActions] no chrome to pin for %s: %v", proxyID, err)
		return
	}

	tabs, err := chrome.ListTabs()
	if err != nil || len(tabs) == 0 {
		log.Printf("[proxyBatchActions] could not list tabs for %s: %v", proxyID, err)
		return
	}

	chosen := tabs[0].ID
	for _, tab := range tabs {
		if tab.URL != "" && tab.URL != "about:blank" {
			chosen = tab.ID
			break
		}
	}

	if err := chrome.ActivateTab(chosen); err != nil {
		log.Printf("[proxyBatchActions] could not pin tab %s: %v", chosen, err)
		return
	}
	log.Printf("[proxyBatchActions] pinned to tab %s", chosen)
}

// proxyBatchActionsLogic runs one action list once per input, substituting
// $input each time. It is the work behind /api/proxy/batch and the
// proxyBatchActions tool.
func (backend *Backend) proxyBatchActionsLogic(ctx context.Context, req ProxyBatchRequest) (ProxyBatchResponse, error) {
	if req.ID == "" {
		return ProxyBatchResponse{}, fmt.Errorf("id is required — call proxyList for the ids that exist")
	}
	if len(req.Actions) == 0 {
		return ProxyBatchResponse{}, fmt.Errorf("actions is required")
	}

	// Validate the whole list up front.
	for i, a := range req.Actions {
		if err := validateBatchAction(i, a); err != nil {
			return ProxyBatchResponse{}, err
		}
	}

	// No inputs means "run this sequence once", with $input empty.
	inputs := req.Inputs
	if len(inputs) == 0 {
		inputs = []string{""}
	}

	// Default to stopping: a flow whose step 2 failed usually leaves the page
	// somewhere unexpected, and grinding through 99 more inputs from there
	// does damage rather than work. Opt in to continuing.
	continueOnError := false
	if req.ContinueOnError != nil {
		continueOnError = *req.ContinueOnError
	}

	defaultTimeout := req.TimeoutMs
	if defaultTimeout <= 0 {
		defaultTimeout = batchDefaultTimeoutMs
	}

	maxDuration := req.MaxDurationMs
	if maxDuration <= 0 {
		maxDuration = batchDefaultMaxDurationMs
	}
	deadline := time.Now().Add(time.Duration(maxDuration) * time.Millisecond)

	resp := ProxyBatchResponse{Total: len(inputs), Results: []ProxyBatchInputResult{}}

	log.Printf("[proxyBatchActions] proxy=%s inputs=%d actions=%d maxDuration=%dms",
		req.ID, len(inputs), len(req.Actions), maxDuration)

	// Own cancellation, so the batch can be stood down by proxy id even when
	// the caller's transport cannot carry a cancel.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	release := registerBatch(req.ID, cancel)
	defer release()

	// Pin the batch to one tab before anything runs. Actions address the tab as
	// "" (whatever is current), and that resolves through lastActiveTargetID,
	// falling back to ListTabs()[0] — whose order is not stable between calls.
	// Chained steps could therefore land on different tabs: navigate would load
	// the page in one, and the next step would read another. Activating a tab
	// once sets lastActiveTargetID, so every step of this batch sees the same
	// one. Best effort: if it fails the actions report their own errors.
	pinBatchTarget(req.ID)

	publishBatchEvent(BatchEvent{
		Type: "start", ProxyID: req.ID, Total: len(inputs), Actions: len(req.Actions),
	})

	for inputIdx, input := range inputs {
		// Out of time before this input even starts: report the rest as
		// untouched rather than running past the deadline.
		if ctx.Err() != nil {
			resp.Cancelled = true
			resp.Remaining = append([]string{}, inputs[inputIdx:]...)
			break
		}
		if time.Now().After(deadline) {
			resp.TimedOut = true
			resp.Remaining = append([]string{}, inputs[inputIdx:]...)
			break
		}

		result := ProxyBatchInputResult{Input: input, OK: true, Steps: []ProxyBatchStep{}}

		for i, raw := range req.Actions {
			if ctx.Err() != nil {
				resp.Cancelled = true
				result.OK = false
				result.FailedAt = i
				result.Steps = append(result.Steps, ProxyBatchStep{
					Index: i, Action: raw.Action, OK: false,
					Error: "batch cancelled by the caller before this step ran",
				})
				break
			}
			if time.Now().After(deadline) {
				resp.TimedOut = true
				result.OK = false
				result.FailedAt = i
				result.Steps = append(result.Steps, ProxyBatchStep{
					Index: i, Action: raw.Action, OK: false,
					Error: fmt.Sprintf("batch deadline of %dms reached before this step ran", maxDuration),
				})
				break
			}

			a := substituteInput(raw, input)

			// Logged per step: a batch can run for a while, and this is the
			// only way to watch where it is while the call is still blocked.
			log.Printf("[proxyBatchActions] input %d/%d step %d/%d: %s",
				inputIdx+1, len(inputs), i+1, len(req.Actions), a.Action)

			publishBatchEvent(BatchEvent{
				Type: "step", ProxyID: req.ID, InputIndex: inputIdx, Input: input,
				StepIndex: i, Action: a.Action, Running: true,
			})

			value, err := runBatchAction(ctx, req.ID, a, defaultTimeout)

			stepEvent := BatchEvent{
				Type: "step", ProxyID: req.ID, InputIndex: inputIdx, Input: input,
				StepIndex: i, Action: a.Action, OK: err == nil,
			}
			if err != nil {
				stepEvent.Error = err.Error()
			}
			publishBatchEvent(stepEvent)

			step := ProxyBatchStep{Index: i, Action: a.Action, OK: err == nil, Result: value}
			if err != nil {
				step.Error = err.Error()
				result.OK = false
				result.FailedAt = i
				log.Printf("[proxyBatchActions] input %d/%d step %d (%s) failed: %v",
					inputIdx+1, len(inputs), i, a.Action, err)
			}
			result.Steps = append(result.Steps, step)

			if err != nil {
				break // the rest of this input's flow depends on this step
			}
		}

		// A step that failed because the caller went away is a cancellation,
		// not a flow that broke: the distinction decides whether the summary
		// tells the reader to fix something or simply to resume.
		if ctx.Err() != nil {
			resp.Cancelled = true
		}

		if result.OK {
			resp.Completed++
		} else {
			resp.Failed++
		}
		resp.Results = append(resp.Results, result)

		publishBatchEvent(BatchEvent{
			Type: "input", ProxyID: req.ID, InputIndex: inputIdx, Input: input, OK: result.OK,
		})

		if resp.TimedOut || resp.Cancelled {
			if inputIdx+1 < len(inputs) {
				resp.Remaining = append([]string{}, inputs[inputIdx+1:]...)
			}
			break
		}

		if !result.OK && !continueOnError && !resp.Cancelled {
			resp.Aborted = true
			if inputIdx+1 < len(inputs) {
				resp.Remaining = append([]string{}, inputs[inputIdx+1:]...)
			}
			break
		}
	}

	resp.Success = resp.Failed == 0
	resp.FailureSummary = batchFailureSummary(resp)

	publishBatchEvent(BatchEvent{
		Type: "end", ProxyID: req.ID, Total: resp.Total, Completed: resp.Completed,
		Failed: resp.Failed, TimedOut: resp.TimedOut, Success: resp.Success,
		Error: resp.FailureSummary,
	})

	return resp, nil
}

// batchFailureSummary describes what went wrong in one line, for callers that
// turn a failed batch into an error. Empty when nothing failed.
func batchFailureSummary(r ProxyBatchResponse) string {
	if r.Failed == 0 {
		// A batch can stop cleanly on the deadline with nothing failed, and
		// that still has to be explained — otherwise it reads as a full run.
		if r.Cancelled {
			return fmt.Sprintf("batch cancelled after %d of %d inputs; %d not run — earlier steps already took effect, so resume rather than re-running from the start",
				len(r.Results), r.Total, len(r.Remaining))
		}
		if r.TimedOut {
			return fmt.Sprintf("batch deadline reached after %d of %d inputs; %d not run — re-run with the remaining inputs, or raise maxDurationMs",
				len(r.Results), r.Total, len(r.Remaining))
		}
		return ""
	}

	// Point at the first failure; it is the one that explains the rest.
	for i, result := range r.Results {
		if result.OK {
			continue
		}

		where := fmt.Sprintf("input %d of %d", i+1, r.Total)
		if result.Input != "" {
			where = fmt.Sprintf("%s (%q)", where, result.Input)
		}

		reason := "no step ran"
		action := ""
		if len(result.Steps) > 0 {
			last := result.Steps[len(result.Steps)-1]
			reason = last.Error
			action = last.Action
		}

		msg := fmt.Sprintf("%s failed at step %d (%s): %s", where, result.FailedAt, action, reason)
		if r.Cancelled {
			msg += fmt.Sprintf(" — batch cancelled by the caller, %d input(s) not run; earlier steps already took effect, so resume from the failed step rather than re-running this batch", len(r.Remaining))
		} else if r.TimedOut {
			msg += fmt.Sprintf(" — batch deadline reached, %d input(s) not run", len(r.Remaining))
		} else if r.Aborted {
			msg += " — batch stopped here (continueOnError is false); steps before this one already took effect, so resume from the failed step rather than re-running the whole batch"
		} else if r.Failed > 1 {
			msg = fmt.Sprintf("%d of %d inputs failed; first: %s", r.Failed, r.Total, msg)
		}
		return msg
	}
	return ""
}

// ProxyBatchActionsEndpoint runs a batch of actions over a list of inputs.
func (backend *Backend) ProxyBatchActionsEndpoint(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/proxy/batch",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			authRecord, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			if admin == nil && authRecord == nil {
				return c.String(http.StatusForbidden, "")
			}

			var body ProxyBatchRequest
			if err := c.Bind(&body); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]any{"error": "Invalid request body"})
			}

			// Request context, so pressing Stop in the UI (or any client
			// disconnect) actually stands the batch down instead of leaving it
			// clicking through the rest of the flow.
			resp, err := backend.proxyBatchActionsLogic(c.Request().Context(), body)
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
			}

			return c.JSON(http.StatusOK, resp)
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}
