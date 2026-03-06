package main

import (
	"log"
	"os"
	"path/filepath"
	"time"

	// "github.com/pocketbase/dbx"

	"github.com/glitchedgitz/cook/v2/pkg/cook"
	"github.com/glitchedgitz/grroxy/apps/app"
	"github.com/glitchedgitz/grroxy/internal/process"
	"github.com/glitchedgitz/pocketbase"
	"github.com/glitchedgitz/pocketbase/core"
	"github.com/glitchedgitz/pocketbase/plugins/migratecmd"
	wappalyzer "github.com/glitchedgitz/wappalyzergo"

	_ "github.com/glitchedgitz/grroxy/cmd/grroxy-app/migrations"
)

func serve(projectPath string) {

	wappalyzerClient, err := wappalyzer.New()
	if err != nil {
		log.Println("Wappylyzer Error: ", err)
	}

	os.MkdirAll(projectPath, 0755)
	os.Chdir(projectPath)

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

	// Info
	API.App.OnBeforeServe().Add(API.Info)
	API.App.OnBeforeServe().Add(API.CWDContent)

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
	API.App.OnBeforeServe().Add(API.ScreenshotProxy)
	API.App.OnBeforeServe().Add(API.ClickProxy)
	API.App.OnBeforeServe().Add(API.GetElementsProxy)
	API.App.OnBeforeServe().Add(API.ListChromeTabs)
	API.App.OnBeforeServe().Add(API.OpenChromeTab)
	API.App.OnBeforeServe().Add(API.NavigateChromeTab)
	API.App.OnBeforeServe().Add(API.ActivateTab)
	API.App.OnBeforeServe().Add(API.CloseTab)
	API.App.OnBeforeServe().Add(API.ReloadTab)
	API.App.OnBeforeServe().Add(API.GoBack)
	API.App.OnBeforeServe().Add(API.GoForward)

	// Other
	API.App.OnBeforeServe().Add(API.AddRequest)
	API.App.OnBeforeServe().Add(API.InterceptEndpoints)
	API.App.OnBeforeServe().Add(API.FiltersCheck)

	// Repeater
	API.App.OnBeforeServe().Add(API.SendRepeater)

	// Modify
	API.App.OnBeforeServe().Add(API.ModifyRequest)

	// Extractor
	API.App.OnBeforeServe().Add(API.ExtractDataEndpoint)

	// Xterm (Terminal)
	API.RegisterXtermRoutes()

	API.App.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		return API.InitializeProxy()
	})

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

	API.App.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		// Setup intercept hooks
		err := API.SetupInterceptHooks()
		if err != nil {
			log.Printf("[Startup] Error setting up intercept hooks: %v", err)
			return err
		}

		// Setup filters hook
		err = API.SetupFiltersHook()
		if err != nil {
			log.Printf("[Startup] Error setting up filters hook: %v", err)
			return err
		}

		// Setup counter manager
		err = API.SetupCounterManager()
		if err != nil {
			log.Printf("[Startup] Error setting up counter manager: %v", err)
			return err
		}

		// Start periodic sync every 1 second
		go func() {
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()

			for range ticker.C {
				if err := API.CounterManager.SyncToDB(); err != nil {
					// log.Printf("[CounterManager] Periodic sync error: %v", err)
				} else {
					// log.Println("[CounterManager] Periodic sync completed")
				}
			}
		}()

		return nil
	})

	API.Serve()
}
