package app

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/glitchedgitz/dadql/dadql"
	"github.com/glitchedgitz/pocketbase/core"
	"github.com/glitchedgitz/pocketbase/tools/types"
)

// SetupFiltersHook sets up the event hook for filter management
// Monitors the _ui collection for changes to proxy filters
func (backend *Backend) SetupFiltersHook() error {
	log.Println("[FiltersManager] Setting up filters hook...")

	// Note: Initial filters will be loaded when proxy starts, not here
	// because the DAO might not be fully initialized yet during app setup

	// Hook: Monitor filter changes in _ui collection (per-proxy filters)
	// Format: unique_id = "proxy/{proxyDBID}"
	backend.App.OnRecordAfterUpdateRequest("_ui").Add(func(e *core.RecordUpdateEvent) error {
		uniqueID := e.Record.GetString("unique_id")
		log.Printf("[FiltersManager][Hook] _ui update - unique_id: %s", uniqueID)

		// Check if this is a proxy filter record (starts with "proxy/")
		if len(uniqueID) < 7 || uniqueID[:6] != "proxy/" {
			return nil
		}

		// Extract proxy ID from unique_id (format: "proxy/{proxyDBID}")
		proxyDBID := uniqueID[6:] // Skip "proxy/" prefix
		log.Printf("[FiltersManager] Processing filter update for proxy: %s", proxyDBID)

		// Find the proxy instance
		ProxyMgr.mu.RLock()
		inst := ProxyMgr.instances[proxyDBID]
		ProxyMgr.mu.RUnlock()

		if inst == nil || inst.Proxy == nil {
			log.Printf("[FiltersManager] Proxy with ID %s not found in running instances", proxyDBID)
			return nil
		}

		// Extract filterstring from data JSON
		data := e.Record.Get("data")
		if data == nil {
			log.Println("[FiltersManager][WARN] No data field in _ui record")
			inst.Proxy.Filters = ""
			return nil
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

		inst.Proxy.Filters = filterstring
		log.Printf("[FiltersManager] Updated filters for proxy %s: %s", proxyDBID, filterstring)

		return nil
	})

	// Hook: Monitor filter creation in _ui collection (for initial filter setup)
	backend.App.OnRecordAfterCreateRequest("_ui").Add(func(e *core.RecordCreateEvent) error {
		uniqueID := e.Record.GetString("unique_id")
		log.Printf("[FiltersManager][Hook] _ui create - unique_id: %s", uniqueID)

		// Check if this is a proxy filter record (starts with "proxy/")
		if len(uniqueID) < 7 || uniqueID[:6] != "proxy/" {
			return nil
		}

		// Extract proxy ID from unique_id (format: "proxy/{proxyDBID}")
		proxyDBID := uniqueID[6:] // Skip "proxy/" prefix
		log.Printf("[FiltersManager] Processing filter creation for proxy: %s", proxyDBID)

		// Find the proxy instance
		ProxyMgr.mu.RLock()
		inst := ProxyMgr.instances[proxyDBID]
		ProxyMgr.mu.RUnlock()

		if inst == nil || inst.Proxy == nil {
			log.Printf("[FiltersManager] Proxy with ID %s not found in running instances", proxyDBID)
			return nil
		}

		// Extract filterstring from data JSON
		data := e.Record.Get("data")
		if data == nil {
			log.Println("[FiltersManager][WARN] No data in created _ui record")
			inst.Proxy.Filters = ""
			return nil
		}

		filterstring := ""

		// Handle types.JsonRaw (PocketBase's JSON type)
		if jsonRaw, ok := data.(types.JsonRaw); ok {
			var dataMap map[string]any
			if err := json.Unmarshal(jsonRaw, &dataMap); err != nil {
				log.Printf("[FiltersManager][ERROR] Failed to unmarshal JSON on create: %v", err)
				return nil
			}

			if fs, ok := dataMap["filterstring"].(string); ok {
				filterstring = fs
			}
		} else if dataMap, ok := data.(map[string]any); ok {
			// Fallback: already a map
			if fs, ok := dataMap["filterstring"].(string); ok {
				filterstring = fs
			}
		}

		inst.Proxy.Filters = filterstring
		log.Printf("[FiltersManager] Initialized filters on create for proxy %s: %s", proxyDBID, filterstring)

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

// loadProxyFilters loads the filters for a specific proxy from the database
// Format: unique_id = "proxy/{proxyDBID}" in _ui collection
func (backend *Backend) loadProxyFilters(proxyDBID string) (string, error) {
	log.Printf("[FiltersManager] Loading filters for proxy %s from database...", proxyDBID)

	dao := backend.App.Dao()

	// Find the proxy filter record using unique_id = "proxy/{proxyDBID}"
	uniqueID := fmt.Sprintf("proxy/%s", proxyDBID)
	record, err := dao.FindFirstRecordByFilter("_ui", fmt.Sprintf("unique_id = '%s'", uniqueID))

	if err != nil {
		log.Printf("[FiltersManager] No filter record found for proxy %s, using empty filters: %v", proxyDBID, err)
		return "", nil
	}
	log.Printf("[FiltersManager] Found _ui filter record for proxy %s", proxyDBID)

	data := record.Get("data")
	if data == nil {
		log.Printf("[FiltersManager] No data field for proxy %s, using empty filters", proxyDBID)
		return "", nil
	}

	log.Printf("[FiltersManager][DEBUG] data type: %T", data)

	filterstring := ""

	// Handle types.JsonRaw (PocketBase's JSON type)
	if jsonRaw, ok := data.(types.JsonRaw); ok {
		var dataMap map[string]any
		if err := json.Unmarshal(jsonRaw, &dataMap); err != nil {
			log.Printf("[FiltersManager][ERROR] Failed to unmarshal JSON: %v", err)
			return "", err
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
		return "", nil
	}

	log.Printf("[FiltersManager] ✓ Loaded filters for proxy %s: %s", proxyDBID, filterstring)
	return filterstring, nil
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

	filter := rp.Filters
	filter = strings.ReplaceAll(filter, "req.", "req_json.")
	filter = strings.ReplaceAll(filter, "req_edited.", "req_edited_json.")
	filter = strings.ReplaceAll(filter, "resp.", "resp_json.")
	filter = strings.ReplaceAll(filter, "resp_edited.", "resp_edited_json.")

	log.Println("[Proxy.checkFilters] data: ", data)

	check, err := dadql.Filter(data, filter)
	if err != nil {
		log.Println("[Proxy.checkFilters] Filter parsing: ", filter, "Error: ", err)
		return false
	}

	log.Println("[Proxy.checkFilters] Filter parsing: ", filter, "\nResults: ", check)

	return check
}
