// Initial setup to start the db file with pre build tables and user
package migrations

import (
	"log"

	"github.com/glitchedgitz/grroxy/internal/schemas"
	"github.com/glitchedgitz/grroxy/internal/types"
	"github.com/glitchedgitz/grroxy/internal/utils"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/daos"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/models"
	pbTypes "github.com/pocketbase/pocketbase/tools/types"
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
		for _, db := range schemas.Collections {
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
		for _, db := range schemas.Collections {
			if db.Index != "" && db.HasIndex {
				log.Println("[migration][init] Creating index for: ", db.Name)
				if _, err := dao.DB().NewQuery(db.Index).Execute(); err != nil {
					log.Printf("[migration][init] Error creating index for %s: %v\n", db.Name, err)
					return err
				}
			}
		}

		// Setting Up Default Settings
		settingsCollection, err := dao.FindCollectionByNameOrId("_settings")
		if err != nil {
			return err
		}

		settings := []setting{
			{
				ID:    utils.AddUnderscore("PROJECT_NAME"),
				Name:  "Project Name",
				Value: "Untitled Project",
			},
			{
				ID:    utils.AddUnderscore("PROXY"),
				Name:  "Proxy",
				Value: "127.0.0.1:8080",
			},
			{
				ID:    utils.AddUnderscore("INTERCEPT"),
				Name:  "Intercept",
				Value: "false",
			},
			{
				ID:    utils.AddUnderscore("MAIN_TAB"),
				Name:  "Main Tab",
				Value: "Sitemaps",
			},
		}

		dao.RunInTransaction(func(txDao *daos.Dao) error {
			if err != nil {
				return err
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
		})
		// =================================

		// Setting Up Default Labels
		labelsCollection, err := dao.FindCollectionByNameOrId("_labels")
		if err != nil {
			return err
		}

		defaultLabels := []types.Label{
			{Name: "!high", Color: "red", Type: "mark"},
			{Name: "!medium", Color: "orange", Type: "mark"},
			{Name: "!low", Color: "yellow", Type: "mark"},
			{Name: "!info", Color: "ignore", Type: "mark"},
			{Name: "!leak", Color: "violet", Type: "mark"},
			{Name: "interesting", Color: "yellow", Type: "custom"},
			{Name: "weird", Color: "purple", Type: "custom"},
			{Name: "^dummy/folder", Color: "blue", Type: "folder"},
			{Name: "^target/reset", Color: "blue", Type: "folder"},
		}

		dao.RunInTransaction(func(txDao *daos.Dao) error {
			if err != nil {
				return err
			}

			for _, val := range defaultLabels {
				record := models.NewRecord(labelsCollection)
				id := utils.RandomString(15)
				record.Set("id", id)
				record.Set("name", val.Name)
				record.Set("color", val.Color)
				record.Set("type", val.Type)

				if err := dao.SaveRecord(record); err != nil {
					return err
				}

				collection := &models.Collection{
					Name:       "label_" + id,
					Type:       models.CollectionTypeBase,
					ListRule:   pbTypes.Pointer(""),
					ViewRule:   pbTypes.Pointer(""),
					CreateRule: pbTypes.Pointer(""),
					UpdateRule: pbTypes.Pointer(""),
					DeleteRule: nil,
					Schema:     schemas.LabelCollection,
				}

				if err := dao.SaveCollection(collection); err != nil {
					log.Println("[migration][creating label collection] Error: ", err)
				}
			}
			return nil
		})

		return nil
	}, func(db dbx.Builder) error {
		// revert changes...

		return nil
	})
}
