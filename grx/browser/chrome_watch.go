package browser

import (
	"context"
	"encoding/json"
	"time"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

// chromeDialRetry is how often WaitForChromeWindowsClosed retries connecting
// while Chrome is still starting up.
var chromeDialRetry = 200 * time.Millisecond

// WaitForChromeWindowsClosed blocks until the Chrome instance using profileDir
// has no windows left (returns true), or until stop is closed or Chrome goes
// away (returns false). Closing a tab or one of several windows doesn't count.
// On macOS closing the last window leaves Chrome running in the Dock, so process
// exit alone doesn't say the user closed the browser. Chrome pushes target
// events over CDP, so this fires the moment the last window closes.
func WaitForChromeWindowsClosed(profileDir string, stop <-chan struct{}) bool {
	conn := dialChromeBrowser(profileDir, stop)
	if conn == nil {
		return false
	}

	// conn.Read ignores its context; closing the conn is what unblocks it.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-stop:
		case <-done:
		}
		conn.Close()
	}()

	ctx := context.Background()
	// Discovery replays targetCreated for every existing target, then streams changes.
	discover := &cdproto.Message{ID: 1, Method: target.CommandSetDiscoverTargets, Params: []byte(`{"discover":true}`)}
	if err := conn.Write(ctx, discover); err != nil {
		return false
	}

	// Every window holds at least one page target, so no pages means no windows.
	pages := map[string]bool{}
	for {
		var msg cdproto.Message
		if err := conn.Read(ctx, &msg); err != nil {
			return false // stopped, or Chrome exited (the process watcher handles that)
		}
		switch msg.Method {
		case cdproto.EventTargetTargetCreated:
			var ev struct {
				TargetInfo struct {
					TargetID string `json:"targetId"`
					Type     string `json:"type"`
				} `json:"targetInfo"`
			}
			if json.Unmarshal(msg.Params, &ev) == nil && ev.TargetInfo.Type == "page" {
				pages[ev.TargetInfo.TargetID] = true
			}
		case cdproto.EventTargetTargetDestroyed:
			var ev struct {
				TargetID string `json:"targetId"`
			}
			if json.Unmarshal(msg.Params, &ev) == nil && pages[ev.TargetID] {
				delete(pages, ev.TargetID)
				if len(pages) == 0 {
					return true
				}
			}
		}
	}
}

// dialChromeBrowser connects to the browser-level DevTools websocket of the
// Chrome using profileDir, retrying until Chrome is up or stop is closed.
func dialChromeBrowser(profileDir string, stop <-chan struct{}) *chromedp.Conn {
	for {
		// A stale DevToolsActivePort from a previous run fails to dial (closed
		// port or unknown browser GUID), so we simply retry until Chrome rewrites it.
		if debugURL, err := readChromeDebugURL(profileDir); err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			conn, err := chromedp.DialContext(ctx, debugURL)
			cancel()
			if err == nil {
				return conn
			}
		}
		select {
		case <-stop:
			return nil
		case <-time.After(chromeDialRetry):
		}
	}
}
