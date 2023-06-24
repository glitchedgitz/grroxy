package main

import (
	"log"
	"os"
	"path"

	// "github.com/pocketbase/dbx"

	"github.com/glitchedgitz/grroxy-db/api/endpoints"
	"github.com/glitchedgitz/grroxy-db/config"
	"github.com/glitchedgitz/grroxy-db/schemas"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
	pbTypes "github.com/pocketbase/pocketbase/tools/types"

	// "github.com/pocketbase/pocketbase/tools/list"
	_ "github.com/glitchedgitz/grroxy-db/migrations"
)

func main() {

	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Almost never here but panic
		panic(err)
	}

	os.MkdirAll(path.Join(homeDir, ".cache", "grroxy"), 0644)

	// Create an instance of the app structure
	pb := endpoints.DatabaseAPI{
		App: pocketbase.NewWithConfig(
			&pocketbase.Config{
				DefaultDataDir: "grroxy",
			},
		),

		Config: &config.Config{
			CacheDirectory:    path.Join(homeDir, ".cache", "grroxy"),
			ProjectDirectory:  "grroxy_test",
			DatabaseDirectory: "grroxy",
		},
	}

	go endpoints.CommandManager()

	// Adding custom endpoints
	pb.App.OnBeforeServe().Add(pb.GetData)
	pb.App.OnBeforeServe().Add(pb.SitemapNew)
	pb.App.OnBeforeServe().Add(pb.SitemapFetch)
	pb.App.OnBeforeServe().Add(pb.SitemapRows)
	pb.App.OnBeforeServe().Add(pb.RunCommand)
	pb.App.OnBeforeServe().Add(pb.SendRawRequest)
	pb.App.OnBeforeServe().Add(pb.TextSQL)
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
			Schema:     schemas.Intercept,
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
