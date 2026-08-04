// Seeds the default quick searches into _searches on the launcher.
// Quick searches used to live in a frontend-created "search" collection inside
// each project db; they are global settings, so they now belong here.
package migrations

import (
	"log"

	"github.com/glitchedgitz/grroxy/internal/schemas"
	"github.com/glitchedgitz/pocketbase/daos"
	m "github.com/glitchedgitz/pocketbase/migrations"
	"github.com/glitchedgitz/pocketbase/models"
	"github.com/pocketbase/dbx"
)

func init() {
	m.Register(func(db dbx.Builder) error {
		dao := daos.New(db)

		collection, err := dao.FindCollectionByNameOrId("_searches")
		if err != nil {
			log.Printf("[migration][searches] Error finding _searches collection: %v\n", err)
			return err
		}

		// _searches has a unique index on name, so look up before inserting to
		// keep this idempotent across restarts
		for _, search := range schemas.DefaultSearches {
			existing, _ := dao.FindFirstRecordByData("_searches", "name", search.Name)
			if existing != nil {
				continue
			}

			record := models.NewRecord(collection)
			record.Set("name", search.Name)
			record.Set("data", search.Pattern)

			if err := dao.SaveRecord(record); err != nil {
				log.Printf("[migration][searches] Error seeding %s: %v", search.Name, err)
				continue
			}

			log.Printf("[migration][searches] Seeded default search: %s", search.Name)
		}

		log.Println("[migration][searches] Successfully seeded default searches")
		return nil
	}, func(db dbx.Builder) error {
		// revert changes...

		return nil
	})
}
