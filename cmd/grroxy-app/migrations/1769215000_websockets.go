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

func init() {
	m.Register(func(db dbx.Builder) error {
		dao := daos.New(db)

		collection := &models.Collection{
			Name:       "_websockets",
			Type:       models.CollectionTypeBase,
			ListRule:   pbTypes.Pointer(""),
			ViewRule:   pbTypes.Pointer(""),
			CreateRule: pbTypes.Pointer(""),
			UpdateRule: pbTypes.Pointer(""),
			DeleteRule: nil,
			Schema:     schemas.Websockets,
		}

		if err := dao.SaveCollection(collection); err != nil {
			log.Println("[migration][websockets] Error creating collection: ", err)
			return err
		}

		log.Println("[migration][websockets] Successfully created _websockets collection")
		return nil
	}, func(db dbx.Builder) error {
		dao := daos.New(db)

		collection, err := dao.FindCollectionByNameOrId("_websockets")
		if err != nil {
			return err
		}

		return dao.DeleteCollection(collection)
	})
}
