// Initial setup to start the db file with pre build tables and user
package migrations

import (
	"log"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/daos"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/models/schema"
	pbTypes "github.com/pocketbase/pocketbase/tools/types"
)

// collections map
var collections = map[string]schema.Schema{
	"data": schema.NewSchema(
		&schema.SchemaField{
			Name: "host",
			Type: schema.FieldTypeText,
		},
		&schema.SchemaField{
			Name: "ip",
			Type: schema.FieldTypeText,
		},
		&schema.SchemaField{
			Name: "port",
			Type: schema.FieldTypeText,
		},
		&schema.SchemaField{
			Name: "url_data",
			Type: schema.FieldTypeJson,
		},
		&schema.SchemaField{
			Name: "original_request",
			Type: schema.FieldTypeJson,
		},
		&schema.SchemaField{
			Name: "original_response",
			Type: schema.FieldTypeJson,
		},
		&schema.SchemaField{
			Name: "has_response",
			Type: schema.FieldTypeBool,
		},
		&schema.SchemaField{
			Name: "is_request_edited",
			Type: schema.FieldTypeBool,
		},
		&schema.SchemaField{
			Name: "is_response_edited",
			Type: schema.FieldTypeBool,
		},
		&schema.SchemaField{
			Name: "edited_request",
			Type: schema.FieldTypeJson,
		},
		&schema.SchemaField{
			Name: "edited_response",
			Type: schema.FieldTypeJson,
		},
		&schema.SchemaField{
			Name: "labels",
			Type: schema.FieldTypeJson,
		},
	),
	"store": schema.NewSchema(
		&schema.SchemaField{
			Name: "request",
			Type: schema.FieldTypeText,
		},
		&schema.SchemaField{
			Name: "response",
			Type: schema.FieldTypeText,
		},
		&schema.SchemaField{
			Name: "request_edited",
			Type: schema.FieldTypeText,
		},
		&schema.SchemaField{
			Name: "response_edited",
			Type: schema.FieldTypeText,
		},
	),
	"sites": schema.NewSchema(
		&schema.SchemaField{
			Name:     "site",
			Type:     schema.FieldTypeText,
			Unique:   true,
			Required: true,
		},
	),
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

		// Delete row from table data where id is _pb_users_auth_
		if err := dao.DeleteCollection(&models.Collection{
			Name: "users",
			Type: "auth",
			Schema: schema.NewSchema(
				&schema.SchemaField{
					Name:     "username",
					Type:     schema.FieldTypeText,
					Required: true,
				},
				&schema.SchemaField{
					Name:     "email",
					Type:     schema.FieldTypeText,
					Required: true,
				},
				&schema.SchemaField{
					Name:     "name",
					Type:     schema.FieldTypeText,
					Required: true,
				},
				&schema.SchemaField{
					Name:     "avatar",
					Type:     schema.FieldTypeText,
					Required: true,
				},
			),
		}); err != nil {
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
				log.Println(err)
			}
		}

		return nil
	}, func(db dbx.Builder) error {
		// revert changes...

		return nil
	})
}
