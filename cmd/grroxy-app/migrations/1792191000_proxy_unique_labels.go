package migrations

import (
	"log"
	"sort"
	"strings"

	"github.com/glitchedgitz/grroxy/apps/app"
	"github.com/glitchedgitz/pocketbase/daos"
	m "github.com/glitchedgitz/pocketbase/migrations"
	"github.com/pocketbase/dbx"
)

// Proxy names are unique from now on, but proxies created before that rule
// existed can share one — starting a Chrome proxy used to name it after the
// count of *running* instances, so every proxy started on a clean slate was
// called "chrome". Number the later duplicates so the list is readable.
func init() {
	m.Register(func(db dbx.Builder) error {
		dao := daos.New(db)

		records, err := dao.FindRecordsByExpr("_proxies")
		if err != nil {
			log.Printf("[migration][proxy_unique_labels] No proxies found: %v", err)
			return nil
		}

		// Oldest first, so the proxy that has held a name longest keeps it and
		// the ones after it number upwards.
		sort.SliceStable(records, func(i, j int) bool {
			return records[i].GetCreated().String() < records[j].GetCreated().String()
		})

		taken := make(map[string]bool, len(records))
		for _, record := range records {
			label := strings.TrimSpace(record.GetString("label"))
			if label == "" {
				label = record.GetId()
			}

			// Same numbering rule new proxies get.
			unique := app.NextProxyLabel(taken, label)
			taken[app.ProxyLabelKey(unique)] = true

			if unique == record.GetString("label") {
				continue
			}

			record.Set("label", unique)
			if err := dao.SaveRecord(record); err != nil {
				log.Printf("[migration][proxy_unique_labels] Error renaming proxy %s: %v", record.GetId(), err)
			} else {
				log.Printf("[migration][proxy_unique_labels] Renamed proxy %s to %q", record.GetId(), unique)
			}
		}

		return nil
	}, func(db dbx.Builder) error {
		// revert: the previous names are not recoverable, and duplicates are
		// exactly what this migration removed.
		return nil
	})
}
