package browser

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// launchDialogChrome starts a real Chrome on startURL and returns its debug url.
func launchDialogChrome(t *testing.T, startURL string) string {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping browser test in -short mode")
	}
	if _, err := os.Stat(tabsChromePath); err != nil {
		t.Skipf("Chrome not installed at %s", tabsChromePath)
	}
	profile := filepath.Join(t.TempDir(), "profile")
	cmd := exec.Command(tabsChromePath,
		"--user-data-dir="+profile,
		"--remote-debugging-port=0",
		"--no-first-run", "--no-default-browser-check", "--test-type",
		startURL,
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
	return debugURL
}

func dialogPage(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name+".html")
	if err := os.WriteFile(p, []byte("<!doctype html><title>"+name+"</title>"+body), 0o644); err != nil {
		t.Fatal(err)
	}
	return "file://" + p
}

func waitForDialog(t *testing.T, cr *ChromeRemote, tab string) *DialogInfo {
	t.Helper()
	for i := 0; i < 50; i++ {
		if d := cr.OpenDialog(tab); d != nil {
			return d
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("no dialog recorded for tab %s", tab)
	return nil
}

// Dialogs opened while grroxy is attached are reported, fail calls fast with
// a DialogError, and can be closed with HandleDialog.
func TestDialogsAreReportedAndClosable(t *testing.T) {
	plain := dialogPage(t, "plain", `<button id=b onclick="alert('from click')">b</button>`)
	debugURL := launchDialogChrome(t, plain)
	time.Sleep(time.Second)

	cr, err := NewChromeRemote(debugURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer cr.Close()
	tabs, _ := cr.ListTabs()
	tab := tabs[0].ID

	// An eval that opens an alert returns at once, well inside its timeout.
	start := time.Now()
	err = cr.Evaluate(tab, "alert('from eval')", nil, 15000)
	var de *DialogError
	if !errors.As(err, &de) || !de.Opened || de.Dialog.Type != "alert" || de.Dialog.Message != "from eval" {
		t.Fatalf("eval opening an alert: got %v, want a DialogError for alert('from eval')", err)
	}
	if took := time.Since(start); took > 5*time.Second {
		t.Errorf("eval opening an alert took %v; it should return as soon as the dialog opens", took)
	}
	if !strings.Contains(err.Error(), "proxyHandleDialog") {
		t.Errorf("error should say how to unblock the tab: %v", err)
	}

	// The tab lists its dialog, and further calls fail straight away.
	tabs, _ = cr.ListTabs()
	if tabs[0].Dialog == nil || tabs[0].Dialog.Message != "from eval" {
		t.Errorf("ListTabs dialog = %+v, want the open alert", tabs[0].Dialog)
	}
	start = time.Now()
	var two int
	if err := cr.Evaluate(tab, "1+1", &two, 15000); !errors.As(err, &de) || de.Opened {
		t.Errorf("eval on a blocked tab: got %v, want a DialogError for the open dialog", err)
	}
	if took := time.Since(start); took > time.Second {
		t.Errorf("eval on a blocked tab took %v; it should fail at once", took)
	}

	// Closing it unblocks the tab.
	if _, d, err := cr.HandleDialog(tab, true, nil); err != nil || d == nil || d.Message != "from eval" {
		t.Fatalf("HandleDialog: dialog=%+v err=%v", d, err)
	}
	if err := cr.Evaluate(tab, "1+1", &two, 5000); err != nil || two != 2 {
		t.Fatalf("eval after closing the dialog: %v (%d)", err, two)
	}
	if _, _, err := cr.HandleDialog(tab, true, nil); err == nil || !strings.Contains(err.Error(), "no dialog is open") {
		t.Errorf("HandleDialog with nothing open: got %v", err)
	}

	// A click that opens an alert worked: it succeeds and reports the dialog.
	if res, err := cr.ClickElement(tab, "", "#b", false, false); err != nil || res == nil || res.Dialog == nil || res.Dialog.Message != "from click" {
		t.Errorf("click opening an alert: result=%+v err=%v, want success with the dialog", res, err)
	}
	if _, _, err := cr.HandleDialog(tab, false, nil); err != nil {
		t.Fatalf("dismiss: %v", err)
	}

	// A prompt takes the text it is given.
	answerText := "grroxy"
	var answer string
	go func() { _ = cr.Evaluate(tab, "window.answer = prompt('name?')", nil, 15000) }()
	waitForDialog(t, cr, tab)
	if _, d, err := cr.HandleDialog(tab, true, &answerText); err != nil || d.Type != "prompt" {
		t.Fatalf("answer prompt: dialog=%+v err=%v", d, err)
	}
	if err := cr.Evaluate(tab, "window.answer", &answer, 5000); err != nil || answer != "grroxy" {
		t.Errorf("prompt answer = %q (%v), want grroxy", answer, err)
	}

	// Accepting a prompt with no text keeps its default, like pressing OK.
	go func() { _ = cr.Evaluate(tab, "window.answer = prompt('name?', 'def')", nil, 15000) }()
	waitForDialog(t, cr, tab)
	if _, _, err := cr.HandleDialog(tab, true, nil); err != nil {
		t.Fatalf("accept prompt with default: %v", err)
	}
	if err := cr.Evaluate(tab, "window.answer", &answer, 5000); err != nil || answer != "def" {
		t.Errorf("prompt accepted without text = %q (%v), want the default def", answer, err)
	}

	// An unknown tab gets a plain message, not Chrome's raw error.
	if _, _, err := cr.HandleDialog("NOSUCHTAB", true, nil); err == nil || !strings.Contains(err.Error(), "no open tab with id NOSUCHTAB") {
		t.Errorf("HandleDialog on an unknown tab: got %v", err)
	}
	if err := cr.Evaluate("NOSUCHTAB", "1", nil, 3000); err == nil || !strings.Contains(err.Error(), "no open tab with id NOSUCHTAB") {
		t.Errorf("Evaluate on an unknown tab: got %v", err)
	}

	// A tab opened later is attached right away, so a dialog it opens on load
	// is seen and closable too.
	early := dialogPage(t, "early", `<script>setTimeout(() => alert('on load'), 300)</script>`)
	tab2, err := cr.OpenTab(early)
	if err != nil {
		t.Fatalf("OpenTab: %v", err)
	}
	if d := waitForDialog(t, cr, tab2); d.Message != "on load" {
		t.Errorf("dialog in new tab = %+v", d)
	}
	if _, _, err := cr.HandleDialog(tab2, true, nil); err != nil {
		t.Errorf("close dialog in new tab: %v", err)
	}
}

// A dialog that opened before grroxy connected cannot be closed over CDP. The
// tab must report as blocked, fail calls with a TabBlockedError, and not hold
// up anything else.
func TestDialogBeforeGrroxyAttachedBlocksOnlyThatTab(t *testing.T) {
	blocked := dialogPage(t, "blocked", `<script>alert('before grroxy')</script>`)
	debugURL := launchDialogChrome(t, blocked)
	time.Sleep(1500 * time.Millisecond)

	cr, err := NewChromeRemote(debugURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer cr.Close()
	tabs, _ := cr.ListTabs()
	tab := tabs[0].ID

	start := time.Now()
	var two int
	err = cr.Evaluate(tab, "1+1", &two, 3000)
	var be *TabBlockedError
	if !errors.As(err, &be) {
		t.Fatalf("eval on a tab blocked before grroxy attached: got %v, want a TabBlockedError", err)
	}
	if took := time.Since(start); took > 5*time.Second {
		t.Errorf("eval took %v; it should give up within its own timeout", took)
	}

	// Other tabs keep working.
	other, err := cr.OpenTab(dialogPage(t, "other", "ok"))
	if err != nil {
		t.Fatalf("OpenTab: %v", err)
	}
	if err := cr.Evaluate(other, "1+1", &two, 5000); err != nil || two != 2 {
		t.Fatalf("eval on another tab: %v", err)
	}

	time.Sleep(3 * time.Second)
	tabs, _ = cr.ListTabs()
	for _, tb := range tabs {
		if tb.ID == tab && !tb.Blocked {
			t.Errorf("ListTabs should mark the tab blocked: %+v", tb)
		}
		if tb.ID == other && tb.Blocked {
			t.Errorf("the other tab should not be blocked: %+v", tb)
		}
	}

	// Closing the tab is the way out.
	if err := cr.CloseTab(tab); err != nil {
		t.Fatalf("CloseTab on the blocked tab: %v", err)
	}
}

// The "Leave site?" error says that accepting completes the navigation.
func TestDialogErrorLeaveSiteWording(t *testing.T) {
	err := &DialogError{TabID: "T", Dialog: DialogInfo{Type: "beforeunload"}}
	if msg := err.Error(); !strings.Contains(msg, `"Leave site?"`) || !strings.Contains(msg, "accept: true to leave the page, which completes the navigation") {
		t.Errorf("leave-site error = %q", msg)
	}
}
