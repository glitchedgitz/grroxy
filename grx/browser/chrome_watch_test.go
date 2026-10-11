package browser

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

// fakeChrome is a browser-level DevTools websocket: it waits for
// Target.setDiscoverTargets, then sends whatever events the test pushes.
// writePort points the profile's DevToolsActivePort at it, like Chrome does
// once it is up.
func fakeChrome(t *testing.T) (profileDir string, events chan<- string, writePort func()) {
	t.Helper()
	ch := make(chan string, 16)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			return
		}
		defer conn.Close()
		if _, err := wsutil.ReadClientText(conn); err != nil {
			return
		}
		for ev := range ch {
			if wsutil.WriteServerText(conn, []byte(ev)) != nil {
				return
			}
		}
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(ch) }) // runs first: lets the handler return

	dir := t.TempDir()
	u, _ := url.Parse(srv.URL)
	writePort = func() {
		if err := os.WriteFile(filepath.Join(dir, "DevToolsActivePort"), []byte(u.Port()+"\n/devtools/browser/x\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir, ch, writePort
}

func created(id, typ string) string {
	return fmt.Sprintf(`{"method":"Target.targetCreated","params":{"targetInfo":{"targetId":%q,"type":%q}}}`, id, typ)
}

func destroyed(id string) string {
	return fmt.Sprintf(`{"method":"Target.targetDestroyed","params":{"targetId":%q}}`, id)
}

func withFastDial(t *testing.T) {
	old := chromeDialRetry
	chromeDialRetry = 10 * time.Millisecond
	t.Cleanup(func() { chromeDialRetry = old })
}

func expectPending(t *testing.T, done <-chan bool, why string) {
	t.Helper()
	select {
	case <-done:
		t.Fatal(why)
	case <-time.After(100 * time.Millisecond):
	}
}

func expectResult(t *testing.T, done <-chan bool, want bool) {
	t.Helper()
	select {
	case got := <-done:
		if got != want {
			t.Fatalf("got %v, want %v", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("did not return")
	}
}

func TestWaitForChromeWindowsClosed_OnlyWhenLastWindowCloses(t *testing.T) {
	withFastDial(t)
	dir, events, writePort := fakeChrome(t)
	writePort()

	done := make(chan bool)
	go func() { done <- WaitForChromeWindowsClosed(dir, make(chan struct{})) }()

	events <- `{"id":1,"result":{}}`
	events <- created("A", "page")
	events <- created("B", "page")
	events <- created("SW", "service_worker")

	events <- destroyed("A")
	expectPending(t, done, "closing one tab must not count as closing the browser")
	events <- destroyed("SW")
	expectPending(t, done, "non-page targets must not count")

	events <- destroyed("B")
	expectResult(t, done, true)
}

func TestWaitForChromeWindowsClosed_WaitsForChromeToStart(t *testing.T) {
	withFastDial(t)
	dir, events, writePort := fakeChrome(t)

	done := make(chan bool)
	go func() { done <- WaitForChromeWindowsClosed(dir, make(chan struct{})) }()

	expectPending(t, done, "returned before Chrome was up")
	writePort()
	events <- created("A", "page")
	events <- destroyed("A")
	expectResult(t, done, true)
}

func TestWaitForChromeWindowsClosed_StopReturnsFalse(t *testing.T) {
	withFastDial(t)
	dir, events, writePort := fakeChrome(t)
	writePort()

	stop := make(chan struct{})
	done := make(chan bool)
	go func() { done <- WaitForChromeWindowsClosed(dir, stop) }()

	events <- created("A", "page")
	expectPending(t, done, "returned while a window was open")
	close(stop)
	expectResult(t, done, false)
}

func TestWaitForChromeWindowsClosed_StopBeforeChromeUp(t *testing.T) {
	withFastDial(t)
	dir := t.TempDir() // no DevToolsActivePort: Chrome never came up

	stop := make(chan struct{})
	done := make(chan bool)
	go func() { done <- WaitForChromeWindowsClosed(dir, stop) }()

	expectPending(t, done, "returned before Chrome was up")
	close(stop)
	expectResult(t, done, false)
}
