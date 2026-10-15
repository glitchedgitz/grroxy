package app

import (
	"log"
	"sync"

	"github.com/glitchedgitz/pocketbase/apis"
	"github.com/glitchedgitz/pocketbase/core"
	"github.com/labstack/echo/v5"
	"golang.org/x/net/websocket"
)

// BatchEvent is one progress message. A batch is a single blocking call, so
// without these the UI cannot tell a slow step from a hang — it only ever sees
// the final result.
type BatchEvent struct {
	Type    string `json:"type"` // start | step | input | end
	ProxyID string `json:"proxyId"`

	// start
	Total   int `json:"total,omitempty"`
	Actions int `json:"actions,omitempty"`

	// step / input
	InputIndex int    `json:"inputIndex"`
	Input      string `json:"input,omitempty"`
	StepIndex  int    `json:"stepIndex"`
	Action     string `json:"action,omitempty"`
	Running    bool   `json:"running,omitempty"`
	OK         bool   `json:"ok,omitempty"`
	Error      string `json:"error,omitempty"`

	// end
	Completed int  `json:"completed,omitempty"`
	Failed    int  `json:"failed,omitempty"`
	TimedOut  bool `json:"timedOut,omitempty"`
	Success   bool `json:"success,omitempty"`
}

// batchBroadcaster fans batch progress out to whoever is watching a proxy.
// Subscriptions are keyed by proxy id because that is the only identifier the
// UI holds before the call returns.
type batchBroadcaster struct {
	mu   sync.Mutex
	subs map[string]map[*websocket.Conn]struct{}
}

var BatchProgress = &batchBroadcaster{subs: map[string]map[*websocket.Conn]struct{}{}}

// publishBatchEvent is the seam the batch publishes through, so the event
// stream the UI consumes can be asserted without standing up a socket.
var publishBatchEvent = func(ev BatchEvent) { BatchProgress.publish(ev) }

func (b *batchBroadcaster) add(proxyID string, ws *websocket.Conn) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.subs[proxyID] == nil {
		b.subs[proxyID] = map[*websocket.Conn]struct{}{}
	}
	b.subs[proxyID][ws] = struct{}{}
}

func (b *batchBroadcaster) remove(proxyID string, ws *websocket.Conn) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if conns, ok := b.subs[proxyID]; ok {
		delete(conns, ws)
		if len(conns) == 0 {
			delete(b.subs, proxyID)
		}
	}
}

// publish sends an event to every watcher of that proxy. Writes are serialised
// under the same lock as the subscriber map: the batch runs on its own
// goroutine while each connection's read loop runs on another, and x/net's
// websocket does not promise concurrent writes are safe.
func (b *batchBroadcaster) publish(ev BatchEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	conns := b.subs[ev.ProxyID]
	if len(conns) == 0 {
		return
	}

	for ws := range conns {
		if err := websocket.JSON.Send(ws, ev); err != nil {
			// A watcher that has gone away must not stall the batch.
			delete(conns, ws)
		}
	}
	if len(conns) == 0 {
		delete(b.subs, ev.ProxyID)
	}
}

// ProxyBatchProgressRoutes registers the progress socket.
//
// GET /api/proxy/batch/ws/:id
func (backend *Backend) ProxyBatchProgressRoutes(e *core.ServeEvent) error {
	e.Router.GET("/api/proxy/batch/ws/:id", func(c echo.Context) error {
		proxyID := c.PathParam("id")
		if proxyID == "" {
			return apis.NewBadRequestError("Proxy ID required", nil)
		}

		websocket.Handler(func(ws *websocket.Conn) {
			BatchProgress.add(proxyID, ws)
			log.Printf("[proxyBatchActions] progress watcher attached to %s", proxyID)

			// Losing the watcher stands the batch down. This socket is the only
			// thing that reliably tracks "someone is still watching": an MCP
			// tool call does not share the chat's HTTP request, and closing the
			// chat panel aborts no fetch, so without this a closed chat leaves
			// the batch clicking through the rest of the flow unattended.
			defer func() {
				BatchProgress.remove(proxyID, ws)
				if CancelBatch(proxyID) {
					log.Printf("[proxyBatchActions] watcher for %s went away — batch cancelled", proxyID)
				}
			}()

			// The client can also ask for it explicitly, so a Stop button does
			// not have to tear down the socket to stop the run.
			for {
				var msg struct {
					Action string `json:"action"`
				}
				if err := websocket.JSON.Receive(ws, &msg); err != nil {
					return
				}
				if msg.Action == "cancel" {
					if CancelBatch(proxyID) {
						log.Printf("[proxyBatchActions] cancel requested for %s", proxyID)
					}
				}
			}
		}).ServeHTTP(c.Response(), c.Request())

		return nil
	})
	return nil
}
