package main

import (
	"log"
	"path/filepath"

	// "github.com/pocketbase/dbx"

	"github.com/glitchedgitz/cook/v2/pkg/cook"
	"github.com/glitchedgitz/grroxy-db/apps/app"
	"github.com/glitchedgitz/grroxy-db/internal/process"
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

	// Extract project ID from project path (the directory name)
	projectID := filepath.Base(projectPath)
	conf.ProjectID = projectID
	log.Printf("Project ID: %s", projectID)

	// Create an instance of the app structure
	API = app.Backend{
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

	// Labels
	API.App.OnBeforeServe().Add(API.LabelAttach)
	API.App.OnBeforeServe().Add(API.LabelDelete)
	API.App.OnBeforeServe().Add(API.LabelNew)

	// Load the frontend
	API.App.OnBeforeServe().Add(API.BindFrontend)

	// Sitemap
	API.App.OnBeforeServe().Add(API.SitemapNew)
	API.App.OnBeforeServe().Add(API.SitemapFetch)

	// Send Raw Request
	API.App.OnBeforeServe().Add(API.SendRawRequest)
	API.App.OnBeforeServe().Add(API.SendHttpRaw)

	// Testing
	API.App.OnBeforeServe().Add(API.TextSQL)

	// File Operations
	API.App.OnBeforeServe().Add(API.SaveFile)
	API.App.OnBeforeServe().Add(API.ReadFile)

	// System
	API.App.OnBeforeServe().Add(API.DownloadCert)
	API.App.OnBeforeServe().Add(API.SearchRegex)
	API.App.OnBeforeServe().Add(API.FileWatcher)

	// Template
	API.App.OnBeforeServe().Add(API.TemplatesList)
	API.App.OnBeforeServe().Add(API.TemplatesNew)
	API.App.OnBeforeServe().Add(API.TemplatesDelete)

	// Commands
	API.App.OnBeforeServe().Add(API.RunCommand)
	API.App.OnBeforeServe().Add(API.Tools)

	// Cook
	API.App.OnBeforeServe().Add(API.CookSearch)
	API.App.OnBeforeServe().Add(API.CookApplyMethods)
	API.App.OnBeforeServe().Add(API.CookGenerate)

	// Playground
	API.App.OnBeforeServe().Add(API.PlaygroundNew)
	API.App.OnBeforeServe().Add(API.PlaygroundDelete)
	API.App.OnBeforeServe().Add(API.PlaygroundAddChild)

	// Proxies
	API.App.OnBeforeServe().Add(API.StartProxy)
	API.App.OnBeforeServe().Add(API.StopProxy)
	API.App.OnBeforeServe().Add(API.RestartProxy)
	API.App.OnBeforeServe().Add(API.ListProxies)

	// Other
	API.App.OnBeforeServe().Add(API.AddRequest)
	API.App.OnBeforeServe().Add(API.InterceptEndpoints)

	// Setup intercept hooks
	API.SetupInterceptHooks()

	// Setup filters hook
	API.SetupFiltersHook()

	API.App.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		API.App.Dao().DB().NewQuery(`
			DELETE FROM _intercept;
		`).Execute()
		return nil
	})

	// Reset all proxy states and intercept settings during boot up
	API.App.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		log.Println("[Startup] Resetting all proxy states and intercept settings...")

		dao := API.App.Dao()

		// Fetch all proxy records
		proxyRecords, err := dao.FindRecordsByExpr("_proxies")
		if err != nil {
			log.Printf("[Startup] Error fetching proxy records: %v", err)
			return nil
		}

		// Reset intercept to false and state to "" for each proxy
		for _, proxyRecord := range proxyRecords {
			proxyRecord.Set("intercept", false)
			proxyRecord.Set("state", "")

			if err := dao.SaveRecord(proxyRecord); err != nil {
				log.Printf("[Startup] Error updating proxy %s: %v", proxyRecord.Id, err)
			} else {
				log.Printf("[Startup] Reset proxy %s: intercept=false, state=''", proxyRecord.Id)
			}
		}

		log.Printf("[Startup] Successfully reset %d proxy records", len(proxyRecords))
		return nil
	})

	API.Serve()
}
