// Initial setup to start the db file with pre build tables and user
package migrations

import (
	"log"

	"github.com/glitchedgitz/grroxy/internal/schemas"
	"github.com/glitchedgitz/pocketbase/daos"
	m "github.com/glitchedgitz/pocketbase/migrations"
	"github.com/glitchedgitz/pocketbase/models"
	pbTypes "github.com/glitchedgitz/pocketbase/tools/types"
	"github.com/pocketbase/dbx"
)

type setting struct {
	ID    string
	Name  string
	Value string
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
		for _, db := range schemas.Tools {
			collection := &models.Collection{
				Name:       db.Name,
				Type:       models.CollectionTypeBase,
				ListRule:   pbTypes.Pointer(""),
				ViewRule:   pbTypes.Pointer(""),
				CreateRule: pbTypes.Pointer(""),
				UpdateRule: pbTypes.Pointer(""),
				DeleteRule: nil,
				Schema:     db.Schema,
			}

			collection.SetId(db.Name)

			// if db.HasIndex {
			// 	collection.Indexes = pbTypes.JsonArray[string]{db.Index}
			// }

			if err := dao.SaveCollection(collection); err != nil {
				log.Println("[migration][init] Error: ", err)
			}

			// sites

			log.Println("[migration][init] Creating collectionasdf: ", db.Name)
		}

		// Create indexes with proper error checking
		for _, db := range schemas.Tools {
			if db.Index != "" && db.HasIndex {
				log.Println("[migration][init] Creating index for: ", db.Name)
				if _, err := dao.DB().NewQuery(db.Index).Execute(); err != nil {
					log.Printf("[migration][init] Error creating index for %s: %v\n", db.Name, err)
					return err
				}
			}
		}

		return nil
	}, func(db dbx.Builder) error {
		// revert changes...

		return nil
	})
}
