// Initial setup to start the db file with pre build tables and user
package migrations

import (
	"log"

	"github.com/glitchedgitz/grroxy-db/types"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/daos"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/models/schema"
	pbTypes "github.com/pocketbase/pocketbase/tools/types"
)

// collections map
var collections = map[string]schema.Schema{
	"data":      Rows,
	"intercept": Intercept,
	"store":     Store,
	"sites":     Sites,
	"settings":  Settings,
}

func init() {
	m.Register(func(db dbx.Builder) error {

		// you can also access the Dao helpers
		dao := daos.New(db)

		collection, err := dao.FindCollectionByNameOrId("_pb_users_auth_")
		if err != nil {
			log.Println(err)
		}

		//Delete users
		if err := dao.DeleteCollection(collection); err != nil {
			log.Println(err)
		}

		// create admin
		admin := &models.Admin{
			Email:        "new@example.com",
			PasswordHash: "$2a$13$1EIwr9jv9bJJxfIUd.EtrOGfXCWAm.NuaFt6ZG6OlWmHSUE1Wwdi.",
		}

		if err := dao.SaveAdmin(admin); err != nil {
			log.Println(err)
		}

		// create collections
		for name, schema := range collections {
			collection := &models.Collection{
				Name:       name,
				Type:       models.CollectionTypeBase,
				ListRule:   pbTypes.Pointer(""),
				ViewRule:   pbTypes.Pointer(""),
				CreateRule: pbTypes.Pointer(""),
				UpdateRule: pbTypes.Pointer(""),
				DeleteRule: nil,
				Schema:     schema,
			}

			if err := dao.SaveCollection(collection); err != nil {
				log.Println("[migration][init] Error: ", err)
			}

			log.Println("[migration][init] Creating collection: ", name)
		}

		collection, err = dao.FindCollectionByNameOrId("settings")
		if err != nil {
			return err
		}

		record := models.NewRecord(collection)
		record.Set("id", types.Settings.Intercept)
		record.Set("option", "Intercept")
		record.Set("value", true)

		if err := dao.SaveRecord(record); err != nil {
			return err
		}

		return nil
	}, func(db dbx.Builder) error {
		// revert changes...

		return nil
	})
}
