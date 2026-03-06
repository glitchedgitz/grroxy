package migrations

import (
	"log"

	"github.com/glitchedgitz/grroxy/internal/schemas"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/daos"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/models"
	pbTypes "github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(func(db dbx.Builder) error {
		dao := daos.New(db)

		collection := &models.Collection{
			Name:       "_counters",
			Type:       models.CollectionTypeBase,
			ListRule:   pbTypes.Pointer(""),
			ViewRule:   pbTypes.Pointer(""),
			CreateRule: pbTypes.Pointer(""),
			UpdateRule: pbTypes.Pointer(""),
			DeleteRule: nil,
			Schema:     schemas.Counter,
		}

		// Ensure unique counter_key - this is the primary identifier
		collection.Indexes = pbTypes.JsonArray[string]{
			"CREATE UNIQUE INDEX idx_counters_key ON _counters (counter_key)",
		}

		collection.SetId("_counters")

		if err := dao.SaveCollection(collection); err != nil {
			log.Println("[migration][counters] Error creating _counters:", err)
			return err
		}

		return nil
	}, func(db dbx.Builder) error {
		dao := daos.New(db)
		c, err := dao.FindCollectionByNameOrId("_counters")
		if err != nil {
			// nothing to do if not found
			return nil
		}
		if err := dao.DeleteCollection(c); err != nil {
			return err
		}
		return nil
	})
}
