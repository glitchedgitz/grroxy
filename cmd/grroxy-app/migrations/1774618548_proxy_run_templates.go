package migrations

import (
	"encoding/json"
	"log"

	"github.com/glitchedgitz/grroxy/apps/app"
	"github.com/glitchedgitz/pocketbase/daos"
	m "github.com/glitchedgitz/pocketbase/migrations"
	"github.com/pocketbase/dbx"
)

func init() {
	m.Register(func(db dbx.Builder) error {
		dao := daos.New(db)

		records, err := dao.FindRecordsByExpr("_proxies")
		if err != nil {
			log.Printf("[migration][proxy_run_templates] No proxies found: %v", err)
			return nil
		}

		for i, record := range records {
			dataRaw := record.Get("data")

			var dataMap map[string]any
			switch v := dataRaw.(type) {
			case map[string]any:
				dataMap = v
			default:
				if bytes, err := json.Marshal(v); err == nil {
					json.Unmarshal(bytes, &dataMap)
				}
			}

			if dataMap == nil {
				dataMap = make(map[string]any)
			}

			updated := false

			if _, exists := dataMap["run_templates"]; !exists {
				dataMap["run_templates"] = true
				updated = true
			}

			// Assign color if empty
			color := record.GetString("color")
			if color == "" {
				record.Set("color", app.NextProxyColor(i))
				updated = true
			}

			if updated {
				record.Set("data", dataMap)
				if err := dao.SaveRecord(record); err != nil {
					log.Printf("[migration][proxy_run_templates] Error updating proxy %s: %v", record.Id, err)
				} else {
					log.Printf("[migration][proxy_run_templates] Updated proxy %s", record.Id)
				}
			}
		}

		return nil
	}, func(db dbx.Builder) error {
		// revert: remove run_templates from data
		dao := daos.New(db)

		records, err := dao.FindRecordsByExpr("_proxies")
		if err != nil {
			return nil
		}

		for _, record := range records {
			dataRaw := record.Get("data")

			var dataMap map[string]any
			switch v := dataRaw.(type) {
			case map[string]any:
				dataMap = v
			default:
				if bytes, err := json.Marshal(v); err == nil {
					json.Unmarshal(bytes, &dataMap)
				}
			}

			if dataMap != nil {
				delete(dataMap, "run_templates")
				record.Set("data", dataMap)
				dao.SaveRecord(record)
			}
		}

		return nil
	})
}
