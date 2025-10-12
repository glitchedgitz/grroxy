package migrations

import (
	"encoding/json"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/daos"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/models/schema"
)

func init() {
	m.Register(func(db dbx.Builder) error {
		dao := daos.New(db)

		collection, err := dao.FindCollectionByNameOrId("_data")
		if err != nil {
			return err
		}

		// add
		new_index := &schema.SchemaField{}
		json.Unmarshal([]byte(`{
			"system": false,
			"id": "tmsnkhkc",
			"name": "index_minor",
			"type": "number",
			"required": false,
			"presentable": false,
			"unique": false,
			"options": {
				"min": null,
				"max": null,
				"noDecimal": false
			}
		}`), new_index)
		collection.Schema.AddField(new_index)

		return dao.SaveCollection(collection)
	}, func(db dbx.Builder) error {
		dao := daos.New(db)

		collection, err := dao.FindCollectionByNameOrId("_data")
		if err != nil {
			return err
		}

		// remove
		collection.Schema.RemoveField("tmsnkhkc")

		return dao.SaveCollection(collection)
	})
}
