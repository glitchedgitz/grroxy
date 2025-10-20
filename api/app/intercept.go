package api

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
)

// SetupInterceptHooks sets up all the event hooks for intercept management
// This replaces SDK-based realtime subscriptions with native PocketBase hooks
func (backend *Backend) SetupInterceptHooks() error {
	log.Println("[InterceptManager] Setting up intercept hooks...")

	// Hook 1: Monitor intercept state changes in _settings
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

	// Hook 2: Handle new intercept requests being created
	backend.App.OnRecordAfterCreateRequest("_intercept").Add(func(e *core.RecordCreateEvent) error {
		log.Printf("[InterceptManager] New intercept request created: ID=%s", e.Record.Id)
		// Frontend will display this via realtime subscription
		// No action needed on backend side
		return nil
	})

	// Hook 3: Handle intercept updates (forward/drop actions)
	backend.App.OnRecordAfterUpdateRequest("_intercept").Add(func(e *core.RecordUpdateEvent) error {
		action := e.Record.GetString("action")
		log.Printf("[InterceptManager] Intercept updated: ID=%s Action=%s", e.Record.Id, action)

		if action == "forward" || action == "drop" {
			log.Printf("[InterceptManager] Intercept action received: %s for ID=%s", action, e.Record.Id)
			// The interceptWait goroutine will pick this up via polling
		}

		return nil
	})

	log.Println("[InterceptManager] Intercept hooks registered successfully")
	return nil
}

// forwardAllIntercepts forwards all pending intercept requests when intercept is disabled
func (backend *Backend) forwardAllIntercepts() {
	dao := backend.App.Dao()

	// Query all pending intercept records using FindRecordsByExpr (gets all records)
	records, err := dao.FindRecordsByExpr("_intercept")

	if err != nil {
		log.Printf("[InterceptManager][ERROR] Failed to list intercepts: %v", err)
		return
	}

	if len(records) == 0 {
		log.Println("[InterceptManager] No pending intercepts to forward")
		return
	}

	log.Printf("[InterceptManager] Forwarding %d pending intercepts", len(records))

	// Update each record action to forward
	for _, record := range records {
		record.Set("action", "forward")
		if err := dao.SaveRecord(record); err != nil {
			log.Printf("[InterceptManager][ERROR] Failed to forward intercept %s: %v", record.Id, err)
		} else {
			log.Printf("[InterceptManager] Forwarded intercept %s", record.Id)
		}
	}

	log.Println("[InterceptManager] All pending intercepts forwarded")
}

// UpdateInterceptFilters updates the intercept filters for the proxy
func (backend *Backend) UpdateInterceptFilters(filters string) {
	if PROXY != nil {
		PROXY.Filters = filters
		log.Printf("[InterceptManager] Updated intercept filters: %s", filters)
	}
}
