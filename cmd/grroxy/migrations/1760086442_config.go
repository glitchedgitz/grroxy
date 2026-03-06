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
			Name:       "_configs",
			Type:       models.CollectionTypeBase,
			ListRule:   pbTypes.Pointer(""),
			ViewRule:   pbTypes.Pointer(""),
			CreateRule: pbTypes.Pointer(""),
			UpdateRule: pbTypes.Pointer(""),
			DeleteRule: nil,
			Schema:     schemas.ConfigSchema,
		}

		// Ensure unique `key` values
		collection.Indexes = pbTypes.JsonArray[string]{
			"CREATE UNIQUE INDEX idx_configs_key ON _configs (key)",
		}

		collection.SetId("_configs")

		if err := dao.SaveCollection(collection); err != nil {
			log.Println("[migration][configs] Error creating _configs:", err)
			return err
		}

		return nil
	}, func(db dbx.Builder) error {
		dao := daos.New(db)
		c, err := dao.FindCollectionByNameOrId("_configs")
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
