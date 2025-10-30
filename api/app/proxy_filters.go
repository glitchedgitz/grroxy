package api

import (
	"encoding/json"
	"log"

	"github.com/glitchedgitz/dadql/dadql"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

// SetupFiltersHook sets up the event hook for filter management
// Monitors the _proxies collection for changes to intercept filters
func (backend *Backend) SetupFiltersHook() error {
	log.Println("[FiltersManager] Setting up filters hook...")

	// Note: Initial filters will be loaded when proxy starts, not here
	// because the DAO might not be fully initialized yet during app setup

	// Hook: Monitor filter changes in _proxies collection (per-proxy filters)
	backend.App.OnRecordAfterUpdateRequest("_proxies").Add(func(e *core.RecordUpdateEvent) error {
		proxyDBID := e.Record.Id
		log.Printf("[FiltersManager][Hook] _proxies update - proxy_id: %s", proxyDBID)

		// Extract filterstring from data JSON
		data := e.Record.Get("data")
		intercept := e.Record.GetBool("intercept")

		if data == nil {
			log.Println("[FiltersManager][WARN] No data field in _proxies record")
			return nil
		}

		log.Printf("[InterceptManager] Proxy %s intercept changed to: %v", proxyDBID, intercept)

		// Find the proxy instance with this ID (map key is now the formatted ID)
		ProxyMgr.mu.RLock()
		inst := ProxyMgr.instances[proxyDBID]
		ProxyMgr.mu.RUnlock()

		if inst == nil || inst.Proxy == nil {
			log.Printf("[InterceptManager] Proxy with ID %s not found in running instances", proxyDBID)
			return nil
		}

		if !intercept {
			// Intercept turned OFF for this proxy - forward all pending intercepts from this proxy
			log.Printf("[InterceptManager] Intercept disabled for proxy %s - forwarding pending requests", proxyDBID)
			inst.Proxy.Intercept = false

			// Forward all pending intercepts for this proxy
			go backend.forwardProxyIntercepts(proxyDBID)
		} else {
			// Intercept turned ON for this proxy
			log.Printf("[InterceptManager] Intercept enabled for proxy %s", proxyDBID)
			inst.Proxy.Intercept = true
		}

		log.Printf("[FiltersManager][DEBUG] data type: %T", data)

		filterstring := ""

		// Handle types.JsonRaw (PocketBase's JSON type)
		if jsonRaw, ok := data.(types.JsonRaw); ok {
			log.Printf("[FiltersManager][DEBUG] Unmarshaling JsonRaw: %s", string(jsonRaw))

			var dataMap map[string]any
			if err := json.Unmarshal(jsonRaw, &dataMap); err != nil {
				log.Printf("[FiltersManager][ERROR] Failed to unmarshal JSON: %v", err)
				return nil
			}

			if fs, ok := dataMap["filterstring"].(string); ok {
				filterstring = fs
			} else {
				log.Printf("[FiltersManager][WARN] No filterstring in data. Keys: %v", getMapKeys(dataMap))
			}
		} else if dataMap, ok := data.(map[string]any); ok {
			// Fallback: already a map
			if fs, ok := dataMap["filterstring"].(string); ok {
				filterstring = fs
			} else {
				log.Printf("[FiltersManager][WARN] No filterstring in data. Keys: %v", getMapKeys(dataMap))
			}
		} else {
			log.Printf("[FiltersManager][ERROR] Unexpected data type: %T", data)
			return nil
		}

		if inst == nil || inst.Proxy == nil {
			log.Printf("[FiltersManager] Proxy with ID %s not found in running instances", proxyDBID)
			return nil
		}

		inst.Proxy.Filters = filterstring
		log.Printf("[FiltersManager] Updated filters for proxy %s: %s", proxyDBID, filterstring)

		return nil
	})

	// Keep backward compatibility: Monitor filter changes in _ui collection (global filters)
	// backend.App.OnRecordAfterUpdateRequest("_ui").Add(func(e *core.RecordUpdateEvent) error {
	// 	// Check if this is the INTERCEPT filters record
	// 	uniqueID := e.Record.GetString("unique_id")
	// 	log.Printf("[FiltersManager][Hook] _ui update - unique_id: %s", uniqueID)

	// 	if uniqueID != "___INTERCEPT___" {
	// 		return nil
	// 	}

	// 	// Extract filterstring from data JSON
	// 	// PocketBase JSON fields are stored as types.JsonRaw ([]byte)
	// 	data := e.Record.Get("data")
	// 	if data == nil {
	// 		log.Println("[FiltersManager][WARN] No data field in _ui record")
	// 		return nil
	// 	}

	// 	log.Printf("[FiltersManager][DEBUG] data type: %T", data)

	// 	filterstring := ""

	// 	// Handle types.JsonRaw (PocketBase's JSON type)
	// 	if jsonRaw, ok := data.(types.JsonRaw); ok {
	// 		log.Printf("[FiltersManager][DEBUG] Unmarshaling JsonRaw: %s", string(jsonRaw))

	// 		var dataMap map[string]any
	// 		if err := json.Unmarshal(jsonRaw, &dataMap); err != nil {
	// 			log.Printf("[FiltersManager][ERROR] Failed to unmarshal JSON: %v", err)
	// 			return nil
	// 		}

	// 		if fs, ok := dataMap["filterstring"].(string); ok {
	// 			filterstring = fs
	// 		} else {
	// 			log.Printf("[FiltersManager][WARN] No filterstring in data. Keys: %v", getMapKeys(dataMap))
	// 		}
	// 	} else if dataMap, ok := data.(map[string]any); ok {
	// 		// Fallback: already a map
	// 		if fs, ok := dataMap["filterstring"].(string); ok {
	// 			filterstring = fs
	// 		} else {
	// 			log.Printf("[FiltersManager][WARN] No filterstring in data. Keys: %v", getMapKeys(dataMap))
	// 		}
	// 	} else {
	// 		log.Printf("[FiltersManager][ERROR] Unexpected data type: %T", data)
	// 		return nil
	// 	}

	// 	// Update filters for all proxies (global filter)
	// 	ProxyMgr.ApplyToAllProxies(func(proxy *RawProxyWrapper, proxyID string) {
	// 		proxy.Filters = filterstring
	// 	})
	// 	log.Printf("[FiltersManager] Updated global filters: %s", filterstring)

	// 	return nil
	// })

	// // Also handle create events (for initial setup)
	// backend.App.OnRecordAfterCreateRequest("_ui").Add(func(e *core.RecordCreateEvent) error {
	// 	uniqueID := e.Record.GetString("unique_id")
	// 	log.Printf("[FiltersManager][Hook] _ui create - unique_id: %s", uniqueID)

	// 	if uniqueID != "___INTERCEPT___" {
	// 		return nil
	// 	}

	// 	data := e.Record.Get("data")
	// 	if data == nil {
	// 		log.Println("[FiltersManager][WARN] No data in created _ui record")
	// 		return nil
	// 	}

	// 	filterstring := ""

	// 	// Handle types.JsonRaw (PocketBase's JSON type)
	// 	if jsonRaw, ok := data.(types.JsonRaw); ok {
	// 		var dataMap map[string]any
	// 		if err := json.Unmarshal(jsonRaw, &dataMap); err != nil {
	// 			log.Printf("[FiltersManager][ERROR] Failed to unmarshal JSON on create: %v", err)
	// 			return nil
	// 		}

	// 		if fs, ok := dataMap["filterstring"].(string); ok {
	// 			filterstring = fs
	// 		}
	// 	} else if dataMap, ok := data.(map[string]any); ok {
	// 		// Fallback: already a map
	// 		if fs, ok := dataMap["filterstring"].(string); ok {
	// 			filterstring = fs
	// 		}
	// 	}

	// 	// Update filters for all proxies
	// 	ProxyMgr.ApplyToAllProxies(func(proxy *RawProxyWrapper, proxyID string) {
	// 		proxy.Filters = filterstring
	// 	})
	// 	log.Printf("[FiltersManager] Initialized filters on create: %s", filterstring)

	// 	return nil
	// })

	log.Println("[FiltersManager] Filters hook registered successfully")
	return nil
}

// loadInterceptFilters loads the current intercept filters from the database
func (backend *Backend) loadInterceptFilters() error {
	log.Println("[FiltersManager] Loading intercept filters from database...")

	dao := backend.App.Dao()

	// Find the INTERCEPT filters record using FindFirstRecordByFilter
	record, err := dao.FindFirstRecordByFilter("_ui", "unique_id = '___INTERCEPT___'")

	if err != nil {
		log.Printf("[FiltersManager] No INTERCEPT filters record found, using empty filters: %v", err)
		// Update filters for all proxies
		ProxyMgr.ApplyToAllProxies(func(proxy *RawProxyWrapper, proxyID string) {
			proxy.Filters = ""
		})
		return nil
	}
	log.Printf("[FiltersManager] Found _ui record with ID: %s", record.Id)

	data := record.Get("data")
	if data == nil {
		log.Println("[FiltersManager] No data field, using empty filters")
		// Update filters for all proxies
		ProxyMgr.ApplyToAllProxies(func(proxy *RawProxyWrapper, proxyID string) {
			proxy.Filters = ""
		})
		return nil
	}

	log.Printf("[FiltersManager][DEBUG] data type: %T", data)

	filterstring := ""

	// Handle types.JsonRaw (PocketBase's JSON type)
	if jsonRaw, ok := data.(types.JsonRaw); ok {
		var dataMap map[string]any
		if err := json.Unmarshal(jsonRaw, &dataMap); err != nil {
			log.Printf("[FiltersManager][ERROR] Failed to unmarshal JSON: %v", err)
			// Update filters for all proxies
			ProxyMgr.ApplyToAllProxies(func(proxy *RawProxyWrapper, proxyID string) {
				proxy.Filters = ""
			})
			return err
		}

		if fs, ok := dataMap["filterstring"].(string); ok {
			filterstring = fs
		} else {
			log.Printf("[FiltersManager] No filterstring in data (keys: %v), using empty filters", getMapKeys(dataMap))
		}
	} else if dataMap, ok := data.(map[string]any); ok {
		// Fallback: already a map
		if fs, ok := dataMap["filterstring"].(string); ok {
			filterstring = fs
		} else {
			log.Printf("[FiltersManager] No filterstring in data (keys: %v), using empty filters", getMapKeys(dataMap))
		}
	} else {
		log.Printf("[FiltersManager][ERROR] Unexpected data type: %T, using empty filters", data)
		// Update filters for all proxies
		ProxyMgr.ApplyToAllProxies(func(proxy *RawProxyWrapper, proxyID string) {
			proxy.Filters = ""
		})
		return nil
	}

	// Update filters for all proxies
	updatedCount := 0
	ProxyMgr.ApplyToAllProxies(func(proxy *RawProxyWrapper, proxyID string) {
		proxy.Filters = filterstring
		updatedCount++
	})
	log.Printf("[FiltersManager] ✓ Loaded initial filters: %s (updated %d proxies)", filterstring, updatedCount)

	return nil
}

// Helper function to get map keys for debugging
func getMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func (rp *RawProxyWrapper) checkFilters(data map[string]any) bool {
	if rp.Filters == "" {
		return true
	}

	check, err := dadql.Filter(data, rp.Filters)
	if err != nil {
		log.Println("[Proxy.checkFilters] Filter parsing: ", rp.Filters, "Error: ", err)
		return false
	}

	log.Println("[Proxy.checkFilters] Filter parsing: ", rp.Filters, "\nResults: ", check)

	return check
}
