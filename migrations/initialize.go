// Initial setup to start the db file with pre build tables and user
package migrations

import (
	"log"

	"github.com/glitchedgitz/grroxy-db/base"
	"github.com/glitchedgitz/grroxy-db/schemas"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/daos"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/models/schema"
	pbTypes "github.com/pocketbase/pocketbase/tools/types"
)

type db struct {
	name   string
	schema schema.Schema
}

// collections map
var collections = []db{
	{"store", schemas.Store},
	{"data", schemas.Rows},
	{"intercept", schemas.Intercept},
	{"sites", schemas.Sites},
	{"settings", schemas.Settings},
}

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
		for _, db := range collections {
			collection := &models.Collection{
				Name:       db.name,
				Type:       models.CollectionTypeBase,
				ListRule:   pbTypes.Pointer(""),
				ViewRule:   pbTypes.Pointer(""),
				CreateRule: pbTypes.Pointer(""),
				UpdateRule: pbTypes.Pointer(""),
				DeleteRule: nil,
				Schema:     db.schema,
			}

			collection.SetId(db.name)

			if err := dao.SaveCollection(collection); err != nil {
				log.Println("[migration][init] Error: ", err)
			}

			log.Println("[migration][init] Creating collection: ", db.name)
		}

		// sites
		dao.DB().NewQuery(`
			CREATE UNIQUE INDEX idx_sites_site ON sites (site);
		`).Execute()

		settingsCollection, err := dao.FindCollectionByNameOrId("settings")
		if err != nil {
			return err
		}

		settings := []setting{
			{
				ID:    base.AddUnderscore("PROJECT_NAME"),
				Name:  "Project Name",
				Value: "Untitled Project",
			},
			{
				ID:    base.AddUnderscore("PROXY"),
				Name:  "Proxy",
				Value: "127.0.0.1:8080",
			},
			{
				ID:    base.AddUnderscore("INTERCEPT"),
				Name:  "Intercept",
				Value: "true",
			},
			{
				ID:    base.AddUnderscore("MAIN_TAB"),
				Name:  "Main Tab",
				Value: "Sitemaps",
			},
		}

		for _, val := range settings {
			record := models.NewRecord(settingsCollection)

			record.Set("id", val.ID)
			record.Set("option", val.Name)
			record.Set("value", val.Value)

			if err := dao.SaveRecord(record); err != nil {
				return err
			}
		}

		return nil
	}, func(db dbx.Builder) error {
		// revert changes...

		return nil
	})
}
