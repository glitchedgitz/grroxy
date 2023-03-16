package main

import (
	"log"

	// "github.com/pocketbase/dbx"

	"github.com/glitchedgitz/grroxy-db/endpoints"
	"github.com/pocketbase/pocketbase"

	// "github.com/pocketbase/pocketbase/tools/list"
	_ "github.com/glitchedgitz/grroxy-db/migrations"
)

func main() {
	// Create an instance of the app structure
	pocketbaseDB := endpoints.DatabaseAPI{
		App: pocketbase.NewWithConfig(
			&pocketbase.Config{
				DefaultDataDir: "grroxy",
			},
		),
	}

	// Adding custom endpoints
	pocketbaseDB.App.OnBeforeServe().Add(pocketbaseDB.GetData)
	pocketbaseDB.App.OnBeforeServe().Add(pocketbaseDB.SitemapNew)
	pocketbaseDB.App.OnBeforeServe().Add(pocketbaseDB.SitemapFetch)

	if err := pocketbaseDB.App.Start(); err != nil {
		log.Fatal(err)
	}
}
