package main

import (
	"log"

	// "github.com/pocketbase/dbx"

	"github.com/glitchedgitz/grroxy-db/api/endpoints"
	"github.com/glitchedgitz/grroxy-db/migrations"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
	pbTypes "github.com/pocketbase/pocketbase/tools/types"

	// "github.com/pocketbase/pocketbase/tools/list"
	_ "github.com/glitchedgitz/grroxy-db/migrations"
)

func main() {
	// Create an instance of the app structure
	pb := endpoints.DatabaseAPI{
		App: pocketbase.NewWithConfig(
			&pocketbase.Config{
				DefaultDataDir: "grroxy",
			},
		),
	}

	// Adding custom endpoints
	pb.App.OnBeforeServe().Add(pb.GetData)
	pb.App.OnBeforeServe().Add(pb.SitemapNew)
	pb.App.OnBeforeServe().Add(pb.SitemapFetch)
	pb.App.OnBeforeServe().Add(pb.SitemapRows)
	pb.App.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		collection, err := pb.App.Dao().FindCollectionByNameOrId("intercept")
		if err != nil {
			return err
		}

		if err := pb.App.Dao().DeleteCollection(collection); err != nil {
			return err
		}

		// create collection intercept
		collection = &models.Collection{
			Name:       "intercept",
			Type:       models.CollectionTypeBase,
			ListRule:   pbTypes.Pointer(""),
			ViewRule:   pbTypes.Pointer(""),
			CreateRule: pbTypes.Pointer(""),
			UpdateRule: pbTypes.Pointer(""),
			DeleteRule: nil,
			Schema:     migrations.Intercept,
		}

		if err := pb.App.Dao().SaveCollection(collection); err != nil {
			log.Println("[migration][init] Error: ", err)
		}

		return nil
	})

	if err := pb.App.Start(); err != nil {
		log.Fatal(err)
	}

}
