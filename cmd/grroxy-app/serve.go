package main

import (
	"log"
	"path"

	// "github.com/pocketbase/dbx"

	"github.com/glitchedgitz/cook/v2/pkg/cook"
	"github.com/glitchedgitz/grroxy-db/api"
	"github.com/glitchedgitz/grroxy-db/proxy"
	wappalyzer "github.com/glitchedgitz/wappalyzergo"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"

	// "github.com/pocketbase/pocketbase/tools/list"
	_ "github.com/glitchedgitz/grroxy-db/cmd/grroxy/migrations"
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
		Cook:       cook.NewWithoutConfig(),
		Wappalyzer: wappalyzerClient,
		Config:     &conf,
		CmdChannel: make(chan api.RunCommandData),
	}

	if !noProxy {

		go proxy.StartProxy(&proxy.Options{
			Silent:                      false,
			Directory:                   path.Join(API.Config.HomeDirectory, ".config", "grroxy"),
			CertCacheSize:               256,
			Verbosity:                   false,
			AppAddress:                  API.Config.HostAddr,
			ListenAddrHTTP:              API.Config.ProxyAddr,
			ListenAddrSocks5:            "127.0.0.1:10080",
			OutputDirectory:             "grroxy_test",
			RequestDSL:                  "",
			ResponseDSL:                 "",
			UpstreamHTTPProxies:         []string{},
			UpstreamSock5Proxies:        []string{},
			ListenDNSAddr:               "",
			DNSMapping:                  "",
			DNSFallbackResolver:         "",
			RequestMatchReplaceDSL:      "",
			ResponseMatchReplaceDSL:     "",
			DumpRequest:                 false,
			DumpResponse:                false,
			UpstreamProxyRequestsNumber: 1,
			// Elastic:                     &Elastic,
			// Kafka:                       &Kafka,
			Allow:     []string{},
			Deny:      []string{},
			Intercept: true,
			Waiting:   true,
		})
	}
	go API.CommandManager()

	migratecmd.MustRegister(API.App, API.App.RootCmd, migratecmd.Config{})

	// Adding custom endpoints
	API.App.OnBeforeServe().Add(API.LabelAttach)
	API.App.OnBeforeServe().Add(API.LabelDelete)
	API.App.OnBeforeServe().Add(API.LabelNew)
	API.App.OnBeforeServe().Add(API.BindFrontend)
	API.App.OnBeforeServe().Add(API.SitemapNew)
	API.App.OnBeforeServe().Add(API.SitemapFetch)
	API.App.OnBeforeServe().Add(API.RunCommand)
	API.App.OnBeforeServe().Add(API.SendRawRequest)
	API.App.OnBeforeServe().Add(API.TextSQL)
	API.App.OnBeforeServe().Add(API.SaveFile)
	API.App.OnBeforeServe().Add(API.ReadFile)
	API.App.OnBeforeServe().Add(API.DownloadCert)
	API.App.OnBeforeServe().Add(API.CookSearch)
	API.App.OnBeforeServe().Add(API.SearchRegex)
	API.App.OnBeforeServe().Add(API.FileWatcher)
	API.App.OnBeforeServe().Add(API.TemplatesList)
	API.App.OnBeforeServe().Add(API.TemplatesNew)
	API.App.OnBeforeServe().Add(API.TemplatesDelete)
	API.App.OnBeforeServe().Add(API.Tools)

	API.App.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		API.App.Dao().DB().NewQuery(`
			DELETE FROM _intercept;
			DELETE FROM tmp_intercept;
		`).Execute()
		return nil
	})

	if launchApp {
		go API.Serve()
		runApp()
	} else {
		API.Serve()
	}
}
