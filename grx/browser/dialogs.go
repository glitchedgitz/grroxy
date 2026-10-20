package browser

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

// JavaScript dialogs (alert, confirm, prompt, "Leave site?") stop the page's
// main thread until someone closes them: every eval, click or wait on that tab
// hangs until it times out. Chrome reports a dialog over CDP, and lets it be
// closed with Page.handleJavaScriptDialog, only when the Page domain was
// enabled on the tab before the dialog opened. So grroxy attaches to tabs as
// soon as they exist, records the dialogs they open, and fails calls on a
// blocked tab straight away with an error that says how to unblock it.

// attachWait is how long a call waits for grroxy to attach to a tab. Attaching
// never finishes on a tab whose dialog opened before grroxy attached.
const attachWait = 10 * time.Second

// DialogInfo describes a JavaScript dialog that is open in a tab.
type DialogInfo struct {
	Type          string    `json:"type"` // alert, confirm, prompt or beforeunload
	Message       string    `json:"message"`
	URL           string    `json:"url,omitempty"`
	DefaultPrompt string    `json:"defaultPrompt,omitempty"`
	OpenedAt      time.Time `json:"openedAt"`
}

func (d DialogInfo) describe() string {
	if d.Type == "beforeunload" {
		return `a "Leave site?" dialog`
	}
	return fmt.Sprintf("a JavaScript %s(%q)", d.Type, d.Message)
}

// DialogError is returned when a tab is blocked by a dialog grroxy can close.
type DialogError struct {
	TabID  string
	Dialog DialogInfo
	// Opened is true when the call itself made the page open the dialog.
	Opened bool
}

func (e *DialogError) Error() string {
	var how string
	switch e.Dialog.Type {
	case "beforeunload":
		how = "Call proxyHandleDialog with accept: true to leave the page, which completes the navigation, or accept: false to stay on it."
	case "prompt":
		how = "Call proxyHandleDialog to close it: accept: true for OK (promptText sets the answer, otherwise the prompt's default is kept), false for Cancel."
	default:
		how = "Call proxyHandleDialog to close it (accept: true for OK, false for Cancel)."
	}
	if e.Opened {
		return fmt.Sprintf("the page opened %s in tab %s, and it blocks the tab until it is closed. %s",
			e.Dialog.describe(), e.TabID, how)
	}
	return fmt.Sprintf("tab %s is blocked by %s; nothing runs in the page until it is closed. %s",
		e.TabID, e.Dialog.describe(), how)
}

// TabBlockedError is returned when grroxy cannot attach to a tab in time,
// which in practice means a dialog opened before grroxy attached to it.
type TabBlockedError struct {
	TabID string
}

func (e *TabBlockedError) Error() string {
	return fmt.Sprintf("tab %s is not responding. Most likely it shows a dialog (alert, confirm or prompt) "+
		"that opened before grroxy attached to the tab, which Chrome does not let grroxy close. "+
		"Ask the user to close the dialog in the browser window, or close the tab with proxyCloseTab.", e.TabID)
}

// attachState tracks a tab grroxy is attaching to.
type attachState struct {
	done    chan struct{}
	err     error
	started time.Time
}

// dialogGuard lets a dialog that opens mid-call cut the call short instead
// of leaving it to hang until its own timeout.
type dialogGuard struct {
	cancel  context.CancelCauseFunc
	tripped *DialogInfo
}

var errDialogOpened = errors.New("a JavaScript dialog opened")

// watchDialogs records the dialogs a tab opens. It must be registered before
// the tab context's first Run so that no event is missed.
func (cr *ChromeRemote) watchDialogs(ctx context.Context, targetID string) {
	chromedp.ListenTarget(ctx, func(ev any) {
		switch e := ev.(type) {
		case *page.EventJavascriptDialogOpening:
			cr.dialogOpened(targetID, DialogInfo{
				Type:          string(e.Type),
				Message:       e.Message,
				URL:           e.URL,
				DefaultPrompt: e.DefaultPrompt,
				OpenedAt:      time.Now(),
			})
		case *page.EventJavascriptDialogClosed:
			cr.dialogClosed(targetID)
		}
	})
}

func (cr *ChromeRemote) dialogOpened(targetID string, info DialogInfo) {
	log.Printf("[ChromeRemote] Tab %s opened %s", targetID, info.describe())
	cr.dialogMu.Lock()
	defer cr.dialogMu.Unlock()
	cr.dialogs[targetID] = &info
	for g := range cr.guards[targetID] {
		if g.tripped == nil {
			g.tripped = &info
			g.cancel(errDialogOpened)
		}
	}
}

func (cr *ChromeRemote) dialogClosed(targetID string) {
	cr.dialogMu.Lock()
	defer cr.dialogMu.Unlock()
	delete(cr.dialogs, targetID)
}

// OpenDialog returns the dialog open in a tab, or nil.
func (cr *ChromeRemote) OpenDialog(targetID string) *DialogInfo {
	cr.dialogMu.Lock()
	defer cr.dialogMu.Unlock()
	if d := cr.dialogs[targetID]; d != nil {
		c := *d
		return &c
	}
	return nil
}

// tabContext returns a context for one call on a tab, plus done, which the
// caller defers with its named error result. It fails at once when the tab
// already shows a dialog, cancels the call when the page opens one, and in
// both cases leaves a DialogError in *err.
func (cr *ChromeRemote) tabContext(targetID string, wait time.Duration) (context.Context, func(*error), error) {
	if targetID == "" {
		var err error
		if targetID, err = cr.CurrentTarget(); err != nil {
			return nil, nil, err
		}
	}
	if d := cr.OpenDialog(targetID); d != nil {
		return nil, nil, &DialogError{TabID: targetID, Dialog: *d}
	}

	base, err := cr.attach(targetID, wait)
	if err != nil {
		return nil, nil, cr.unknownTab(targetID, err)
	}

	ctx, cancel := context.WithCancelCause(base)
	g := &dialogGuard{cancel: cancel}

	cr.dialogMu.Lock()
	// Checked again under the lock: a dialog may have opened while attaching.
	if d := cr.dialogs[targetID]; d != nil {
		cr.dialogMu.Unlock()
		cancel(nil)
		return nil, nil, &DialogError{TabID: targetID, Dialog: *d}
	}
	if cr.guards[targetID] == nil {
		cr.guards[targetID] = map[*dialogGuard]struct{}{}
	}
	cr.guards[targetID][g] = struct{}{}
	cr.dialogMu.Unlock()

	done := func(errp *error) {
		cr.dialogMu.Lock()
		delete(cr.guards[targetID], g)
		if len(cr.guards[targetID]) == 0 {
			delete(cr.guards, targetID)
		}
		tripped := g.tripped
		cr.dialogMu.Unlock()
		cancel(nil)

		// Even when the call itself went through, the tab is now blocked, and
		// the caller has to know before its next call.
		if tripped != nil {
			*errp = &DialogError{TabID: targetID, Dialog: *tripped, Opened: true}
		}
	}
	return ctx, done, nil
}

// attach returns the tab's persistent chromedp context, attaching to the tab
// when needed. The attach runs on its own goroutine — chromedp ties the tab's
// event loop to the context of its first Run, so it cannot be given a timeout —
// and attach waits for it at most wait. A tab whose dialog opened before
// grroxy attached never finishes attaching; callers get a TabBlockedError.
func (cr *ChromeRemote) attach(targetID string, wait time.Duration) (context.Context, error) {
	cr.mu.Lock()
	if ctx, ok := cr.targetCtxs[targetID]; ok {
		select {
		case <-ctx.Done():
			delete(cr.targetCtxs, targetID)
			delete(cr.targetCancel, targetID)
		default:
			cr.mu.Unlock()
			return ctx, nil
		}
	}

	st, attaching := cr.attaching[targetID]
	if !attaching {
		st = &attachState{done: make(chan struct{}), started: time.Now()}
		cr.attaching[targetID] = st

		log.Printf("[ChromeRemote] Creating new persistent context for target %s", targetID)
		ctx, cancel := chromedp.NewContext(cr.browserCtx, chromedp.WithTargetID(target.ID(targetID)))
		cr.watchDialogs(ctx, targetID)

		go func() {
			err := chromedp.Run(ctx)
			cr.mu.Lock()
			if err != nil {
				cancel()
				st.err = fmt.Errorf("failed to initialize target context: %v", err)
			} else {
				cr.targetCtxs[targetID] = ctx
				cr.targetCancel[targetID] = cancel
			}
			delete(cr.attaching, targetID)
			cr.mu.Unlock()
			close(st.done)
		}()
	}
	cr.mu.Unlock()

	if wait <= 0 {
		return nil, nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-st.done:
		if st.err != nil {
			return nil, st.err
		}
		cr.mu.Lock()
		defer cr.mu.Unlock()
		if ctx, ok := cr.targetCtxs[targetID]; ok {
			return ctx, nil
		}
		return nil, fmt.Errorf("tab %s closed while attaching", targetID)
	case <-timer.C:
		return nil, &TabBlockedError{TabID: targetID}
	}
}

// blockedTabs reports the tabs grroxy has been failing to attach to for a
// while, which is what a dialog that predates grroxy looks like.
func (cr *ChromeRemote) blockedTabs() map[string]bool {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	out := map[string]bool{}
	for id, st := range cr.attaching {
		if time.Since(st.started) > 3*time.Second {
			out[id] = true
		}
	}
	return out
}

// watchTabs attaches to every page tab as soon as it exists, so that the
// dialogs it opens can be seen and closed (see the top of this file).
func (cr *ChromeRemote) watchTabs() {
	chromedp.ListenBrowser(cr.browserCtx, func(ev any) {
		e, ok := ev.(*target.EventTargetCreated)
		if !ok || e.TargetInfo == nil || e.TargetInfo.Type != "page" {
			return
		}
		// Off the event goroutine: attach takes cr.mu.
		go cr.attach(e.TargetInfo.TargetID.String(), 0)
	})
	if err := target.SetDiscoverTargets(true).Do(cr.browserExec()); err != nil {
		log.Printf("[ChromeRemote] could not watch for new tabs: %v", err)
	}

	tabs, err := cr.ListTabs()
	if err != nil {
		return
	}
	for _, t := range tabs {
		go cr.attach(t.ID, 0)
	}
}

// HandleDialog closes the dialog open in a tab: accept presses OK, otherwise
// Cancel. promptText answers a prompt(); when it is nil, accepting keeps the
// prompt's default text, as pressing OK in the browser would.
func (cr *ChromeRemote) HandleDialog(targetID string, accept bool, promptText *string) (string, *DialogInfo, error) {
	if targetID == "" {
		var err error
		if targetID, err = cr.CurrentTarget(); err != nil {
			return "", nil, err
		}
	}
	open := cr.OpenDialog(targetID)

	ctx, err := cr.attach(targetID, 5*time.Second)
	if err != nil {
		return targetID, nil, cr.unknownTab(targetID, err)
	}
	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	h := page.HandleJavaScriptDialog(accept)
	switch {
	case promptText != nil:
		h = h.WithPromptText(*promptText)
	case accept && open != nil && open.Type == "prompt":
		// Without a prompt text Chrome answers with "", not the default.
		h = h.WithPromptText(open.DefaultPrompt)
	}
	if err := chromedp.Run(tctx, h); err != nil {
		if strings.Contains(err.Error(), "No dialog is showing") {
			return targetID, nil, fmt.Errorf("no dialog is open in tab %s", targetID)
		}
		return targetID, nil, cr.unknownTab(targetID, fmt.Errorf("failed to close dialog in tab %s: %v", targetID, err))
	}
	cr.dialogClosed(targetID)
	return targetID, open, nil
}

// unknownTab replaces err with a plain message when the tab does not exist,
// instead of Chrome's "No target with given id found". It only lists tabs
// once a call has already failed.
func (cr *ChromeRemote) unknownTab(targetID string, err error) error {
	if err == nil || targetID == "" {
		return err
	}
	var blocked *TabBlockedError
	if errors.As(err, &blocked) {
		return err
	}
	tabs, lerr := cr.ListTabs()
	if lerr != nil {
		return err
	}
	for _, t := range tabs {
		if t.ID == targetID {
			return err
		}
	}
	return fmt.Errorf("no open tab with id %s; call proxyListTabs for the ids of the open tabs", targetID)
}

// forgetTab drops what grroxy knows about a closed tab's dialogs.
func (cr *ChromeRemote) forgetTab(targetID string) {
	cr.dialogMu.Lock()
	defer cr.dialogMu.Unlock()
	delete(cr.dialogs, targetID)
}

// noteDialogs fills in the dialog and blocked state of listed tabs.
func (cr *ChromeRemote) noteDialogs(tabs []TabInfo) {
	blocked := cr.blockedTabs()
	for i := range tabs {
		tabs[i].Dialog = cr.OpenDialog(tabs[i].ID)
		tabs[i].Blocked = blocked[tabs[i].ID]
	}
}

// withinWait caps the attach wait at a call's own timeout.
func withinWait(timeoutMs int) time.Duration {
	if d := time.Duration(timeoutMs) * time.Millisecond; d > 0 && d < attachWait {
		return d
	}
	return attachWait
}
