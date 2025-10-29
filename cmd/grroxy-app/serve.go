package main

import (
	"log"

	// "github.com/pocketbase/dbx"

	"github.com/glitchedgitz/cook/v2/pkg/cook"
	api "github.com/glitchedgitz/grroxy-db/api/app"
	"github.com/glitchedgitz/grroxy-db/process"
	wappalyzer "github.com/glitchedgitz/wappalyzergo"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"

	_ "github.com/glitchedgitz/grroxy-db/cmd/grroxy-app/migrations"
)

func serve(projectPath string) {

	wappalyzerClient, err := wappalyzer.New()
	if err != nil {
		log.Println("Wappylyzer Error: ", err)
	}

	// Create an instance of the app structure
	API = api.Backend{
		App: pocketbase.NewWithConfig(
			pocketbase.Config{
				ProjectDir:      projectPath,
				DefaultDataDir:  "grroxy",
				HideStartBanner: true,
				// DefaultDev: true,
				// DefaultEncryptionEnv: "hJH#GRJ#HG$JH$54h5kjhHJG#JHG#*&Y&EG#F&GIG@JKGH$JHRGJ##JKJH#JHG",
			},
		),
		Cook:       cook.NewGenerator(),
		Wappalyzer: wappalyzerClient,
		Config:     &conf,
		CmdChannel: make(chan process.RunCommandData),
	}

	// if !noProxy {

	migratecmd.MustRegister(API.App, API.App.RootCmd, migratecmd.Config{})

	API.App.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		record, err := API.App.Dao().FindRecordById("_settings", "PROXY__________")
		if err != nil {
			log.Println("Error finding record: ", err)
			return nil
		}

		record.Set("value", "")
		if err := API.App.Dao().SaveRecord(record); err != nil {
			log.Println("Error saving record: ", err)
		}
		return nil
	})

	// Adding custom endpoints
	API.App.OnBeforeServe().Add(API.LabelAttach)
	API.App.OnBeforeServe().Add(API.LabelDelete)
	API.App.OnBeforeServe().Add(API.LabelNew)
	API.App.OnBeforeServe().Add(API.BindFrontend)
	API.App.OnBeforeServe().Add(API.SitemapNew)
	API.App.OnBeforeServe().Add(API.SitemapFetch)
	API.App.OnBeforeServe().Add(API.SendRawRequest)
	API.App.OnBeforeServe().Add(API.TextSQL)
	API.App.OnBeforeServe().Add(API.SaveFile)
	API.App.OnBeforeServe().Add(API.ReadFile)
	API.App.OnBeforeServe().Add(API.DownloadCert)
	API.App.OnBeforeServe().Add(API.SearchRegex)
	API.App.OnBeforeServe().Add(API.FileWatcher)
	API.App.OnBeforeServe().Add(API.TemplatesList)
	API.App.OnBeforeServe().Add(API.TemplatesNew)
	API.App.OnBeforeServe().Add(API.TemplatesDelete)
	API.App.OnBeforeServe().Add(API.RunCommand)
	API.App.OnBeforeServe().Add(API.Tools)
	API.App.OnBeforeServe().Add(API.CookSearch)
	API.App.OnBeforeServe().Add(API.CookApplyMethods)
	API.App.OnBeforeServe().Add(API.CookGenerate)
	API.App.OnBeforeServe().Add(API.PlaygroundNew)
	API.App.OnBeforeServe().Add(API.PlaygroundDelete)
	API.App.OnBeforeServe().Add(API.PlaygroundAddChild)
	API.App.OnBeforeServe().Add(API.StartProxy)
	API.App.OnBeforeServe().Add(API.StopProxy)
	API.App.OnBeforeServe().Add(API.ListProxies)
	API.App.OnBeforeServe().Add(API.AddRequest)
	API.App.OnBeforeServe().Add(API.InterceptEndpoints)

	// Setup intercept hooks
	API.SetupInterceptHooks()

	// Setup filters hook
	API.SetupFiltersHook()

	API.App.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		API.App.Dao().DB().NewQuery(`
			DELETE FROM _intercept;
			DELETE FROM tmp_intercept;
		`).Execute()
		return nil
	})

	API.Serve()
}
