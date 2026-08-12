// Seeds the default global UI settings into the `settings` row of _configs on
// the launcher. Before this, the row only existed once the frontend had saved
// it, so a fresh install started on the frontend's hardcoded defaults.
package migrations

import (
	"encoding/json"
	"log"
	"maps"

	"github.com/glitchedgitz/grroxy/internal/schemas"
	"github.com/glitchedgitz/pocketbase/daos"
	m "github.com/glitchedgitz/pocketbase/migrations"
	"github.com/glitchedgitz/pocketbase/models"
	"github.com/pocketbase/dbx"
)

func init() {
	m.Register(func(db dbx.Builder) error {
		dao := daos.New(db)

		collection, err := dao.FindCollectionByNameOrId("_configs")
		if err != nil {
			log.Printf("[migration][settings] Error finding _configs collection: %v\n", err)
			return err
		}

		// the settings row is uniquely indexed on key, so reuse it when present
		record, _ := dao.FindFirstRecordByFilter("_configs", "key = {:key}", dbx.Params{
			"key": schemas.SettingsConfigKey,
		})
		if record == nil {
			record = models.NewRecord(collection)
			record.Set("key", schemas.SettingsConfigKey)
		}

		// a fresh record has no data, and unmarshalling that `null` would leave
		// the map nil — so read into stored and build the result separately
		stored := map[string]any{}
		if raw, err := json.Marshal(record.Get("data")); err == nil {
			json.Unmarshal(raw, &stored)
		}

		// any key we ship a default for is forced to that default, overwriting
		// whatever was stored; keys we don't know about are left alone
		data := make(map[string]any, len(stored)+len(schemas.DefaultSettings))
		maps.Copy(data, stored)
		maps.Copy(data, schemas.DefaultSettings)

		record.Set("data", data)
		if err := dao.SaveRecord(record); err != nil {
			log.Printf("[migration][settings] Error seeding default settings: %v\n", err)
			return err
		}

		log.Println("[migration][settings] Successfully seeded default settings")
		return nil
	}, func(db dbx.Builder) error {
		dao := daos.New(db)

		record, _ := dao.FindFirstRecordByFilter("_configs", "key = {:key}", dbx.Params{
			"key": schemas.SettingsConfigKey,
		})
		if record == nil {
			return nil
		}

		return dao.DeleteRecord(record)
	})
}
