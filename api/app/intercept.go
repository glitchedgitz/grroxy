package api

import (
	"log"
	"net/http"
	"sync"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
)

// InterceptUpdateChannels stores channels for each intercept waiting goroutine
var (
	interceptChannels   = make(map[string]chan InterceptUpdate)
	interceptChannelsMu sync.RWMutex
)

// RegisterInterceptChannel registers a channel for a specific intercept ID
func RegisterInterceptChannel(id string, ch chan InterceptUpdate) {
	interceptChannelsMu.Lock()
	defer interceptChannelsMu.Unlock()
	interceptChannels[id] = ch
}

// UnregisterInterceptChannel removes the channel for a specific intercept ID
func UnregisterInterceptChannel(id string) {
	interceptChannelsMu.Lock()
	defer interceptChannelsMu.Unlock()
	if ch, exists := interceptChannels[id]; exists {
		close(ch)
		delete(interceptChannels, id)
	}
}

// NotifyInterceptUpdate sends an update to the waiting goroutine
func NotifyInterceptUpdate(id string, update InterceptUpdate) {
	interceptChannelsMu.RLock()
	defer interceptChannelsMu.RUnlock()
	if ch, exists := interceptChannels[id]; exists {
		select {
		case ch <- update:
			log.Printf("[InterceptManager] Notified waiting goroutine for ID=%s", id)
		default:
			log.Printf("[InterceptManager][WARN] Channel for ID=%s is not ready", id)
		}
	}
}

// SetupInterceptHooks sets up the event hook for monitoring intercept state changes
func (backend *Backend) SetupInterceptHooks() error {
	log.Println("[InterceptManager] Setting up intercept hooks...")

	// Monitor intercept state changes in _settings to enable/disable intercept globally
	backend.App.OnRecordAfterUpdateRequest("_settings").Add(func(e *core.RecordUpdateEvent) error {
		// Check if this is the INTERCEPT setting (ID has 6 underscores, not 8)
		if e.Record.Id != "INTERCEPT______" {
			return nil
		}

		value := e.Record.GetString("value")
		log.Printf("[InterceptManager] Intercept setting changed to: %s", value)

		if value == "false" {
			// Intercept turned OFF - forward all pending intercepts
			log.Println("[InterceptManager] Intercept disabled - forwarding all pending requests")

			if PROXY != nil {
				PROXY.Intercept = false
			}

			// Forward all pending intercepts
			go backend.forwardAllIntercepts()
		} else {
			// Intercept turned ON
			log.Println("[InterceptManager] Intercept enabled")

			if PROXY != nil {
				PROXY.Intercept = true
			}
		}

		return nil
	})

	log.Println("[InterceptManager] Intercept hooks registered successfully")
	return nil
}

// forwardAllIntercepts forwards all pending intercept requests when intercept is disabled
func (backend *Backend) forwardAllIntercepts() {
	interceptChannelsMu.RLock()
	defer interceptChannelsMu.RUnlock()

	if len(interceptChannels) == 0 {
		log.Println("[InterceptManager] No pending intercepts to forward")
		return
	}

	log.Printf("[InterceptManager] Forwarding %d pending intercepts via channels", len(interceptChannels))

	// Directly notify all waiting goroutines via their channels
	// Each goroutine will handle deleting its own record
	forwardUpdate := InterceptUpdate{
		Action:        "forward",
		IsReqEdited:   false,
		IsRespEdited:  false,
		ReqEditedRaw:  "",
		RespEditedRaw: "",
	}

	for id, ch := range interceptChannels {
		select {
		case ch <- forwardUpdate:
			log.Printf("[InterceptManager] Forwarded intercept %s via channel", id)
		default:
			log.Printf("[InterceptManager][WARN] Channel for ID=%s is not ready", id)
		}
	}

	log.Println("[InterceptManager] All pending intercepts forwarded via channels")
}

// UpdateInterceptFilters updates the intercept filters for the proxy
func (backend *Backend) UpdateInterceptFilters(filters string) {
	if PROXY != nil {
		PROXY.Filters = filters
		log.Printf("[InterceptManager] Updated intercept filters: %s", filters)
	}
}

// InterceptActionRequest represents the request body for intercept actions
type InterceptActionRequest struct {
	ID           string `json:"id"`
	Action       string `json:"action"` // "forward" or "drop"
	IsReqEdited  bool   `json:"is_req_edited,omitempty"`
	IsRespEdited bool   `json:"is_resp_edited,omitempty"`
	ReqEdited    string `json:"req_edited,omitempty"`  // Raw HTTP request string
	RespEdited   string `json:"resp_edited,omitempty"` // Raw HTTP response string
}

// InterceptEndpoints registers the HTTP endpoints for intercept management
func (backend *Backend) InterceptEndpoints(e *core.ServeEvent) error {
	// POST /api/intercept/action - Handle intercept actions (forward/drop)
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/intercept/action",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil
			if isGuest {
				return c.JSON(http.StatusForbidden, map[string]interface{}{
					"error": "Unauthorized",
				})
			}

			var req InterceptActionRequest
			if err := c.Bind(&req); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{
					"error": "Invalid request body",
				})
			}

			// Validate action
			if req.Action != "forward" && req.Action != "drop" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{
					"error": "Invalid action. Must be 'forward' or 'drop'",
				})
			}

			// Validate ID
			if req.ID == "" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{
					"error": "Intercept ID is required",
				})
			}

			log.Printf("[InterceptAPI] Received action request: ID=%s, Action=%s", req.ID, req.Action)

			// dao := backend.App.Dao()

			// // Find the intercept record
			// interceptRecord, err := dao.FindRecordById("_intercept", req.ID)
			// if err != nil {
			// 	return c.JSON(http.StatusNotFound, map[string]interface{}{
			// 		"error": "Intercept not found",
			// 	})
			// }

			// // Update the intercept record with the action and edited data
			// interceptRecord.Set("action", req.Action)
			// interceptRecord.Set("is_req_edited", req.IsReqEdited)
			// interceptRecord.Set("is_resp_edited", req.IsRespEdited)

			// // Store raw edited strings in JSON fields for frontend/debugging
			// // (The actual data is passed directly to the goroutine via channel)
			// if req.IsReqEdited && req.ReqEdited != "" {
			// 	interceptRecord.Set("req_edited_json", map[string]interface{}{
			// 		"raw": req.ReqEdited,
			// 	})
			// 	log.Printf("[InterceptAPI] Stored edited request raw data for ID=%s", req.ID)
			// }

			// if req.IsRespEdited && req.RespEdited != "" {
			// 	interceptRecord.Set("resp_edited_json", map[string]interface{}{
			// 		"raw": req.RespEdited,
			// 	})
			// 	log.Printf("[InterceptAPI] Stored edited response raw data for ID=%s", req.ID)
			// }

			// // Save the record
			// if err := dao.SaveRecord(interceptRecord); err != nil {
			// 	log.Printf("[InterceptAPI][ERROR] Failed to update intercept record: %v", err)
			// 	return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			// 		"error": "Failed to update intercept",
			// 	})
			// }

			log.Printf("[InterceptAPI] Successfully updated intercept: ID=%s, Action=%s", req.ID, req.Action)

			// Directly notify the waiting goroutine via channel with the raw edited strings
			update := InterceptUpdate{
				Action:        req.Action,
				IsReqEdited:   req.IsReqEdited,
				IsRespEdited:  req.IsRespEdited,
				ReqEditedRaw:  req.ReqEdited,
				RespEditedRaw: req.RespEdited,
			}
			NotifyInterceptUpdate(req.ID, update)
			log.Printf("[InterceptAPI] Notified waiting goroutine for ID=%s (req_edited=%v, resp_edited=%v)",
				req.ID, req.IsReqEdited, req.IsRespEdited)

			return c.JSON(http.StatusOK, map[string]interface{}{
				"success": true,
				"message": "Intercept action processed successfully",
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})

	log.Println("[InterceptAPI] Intercept endpoints registered successfully")
	return nil
}
