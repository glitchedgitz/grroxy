package main

import (
	"log"
	"path"

	// "github.com/pocketbase/dbx"

	"github.com/glitchedgitz/grroxy-db/api/endpoints"
	"github.com/glitchedgitz/grroxy-db/proxy"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"

	// "github.com/pocketbase/pocketbase/tools/list"
	_ "github.com/glitchedgitz/grroxy-db/cmd/grroxy/migrations"
)

func serve() {

	// Create an instance of the app structure
	pb = endpoints.DatabaseAPI{
		App: pocketbase.NewWithConfig(
			pocketbase.Config{
				DefaultDataDir:  "grroxy",
				HideStartBanner: true,
				// DefaultEncryptionEnv: "hJH#GRJ#HG$JH$54h5kjhHJG#JHG#*&Y&EG#F&GIG@JKGH$JHRGJ##JKJH#JHG",
			},
		),
		Config:     &conf,
		CmdChannel: make(chan endpoints.RunCommandData),
	}

	if !noProxy {

		go proxy.StartProxy(&proxy.Options{
			Silent:                      false,
			Directory:                   path.Join(pb.Config.HomeDirectory, ".config", "grroxy"),
			CertCacheSize:               256,
			Verbosity:                   false,
			AppAddress:                  pb.Config.HostAddr,
			ListenAddrHTTP:              pb.Config.ProxyAddr,
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
	go pb.CommandManager()

	migratecmd.MustRegister(pb.App, pb.App.RootCmd, migratecmd.Config{})

	// Adding custom endpoints
	pb.App.OnBeforeServe().Add(pb.LabelAttach)
	pb.App.OnBeforeServe().Add(pb.LabelDelete)
	pb.App.OnBeforeServe().Add(pb.LabelNew)
	pb.App.OnBeforeServe().Add(pb.BindFrontend)
	pb.App.OnBeforeServe().Add(pb.SitemapNew)
	pb.App.OnBeforeServe().Add(pb.SitemapFetch)
	pb.App.OnBeforeServe().Add(pb.RunCommand)
	pb.App.OnBeforeServe().Add(pb.SendRawRequest)
	pb.App.OnBeforeServe().Add(pb.TextSQL)
	pb.App.OnBeforeServe().Add(pb.SaveFile)
	pb.App.OnBeforeServe().Add(pb.ReadFile)
	pb.App.OnBeforeServe().Add(pb.DownloadCert)
	pb.App.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		pb.App.Dao().DB().NewQuery(`
			DELETE FROM _intercept;
			DELETE FROM tmp_intercept;
		`).Execute()
		return nil
	})

	pb.Serve()

	if err := pb.App.Start(); err != nil {
		log.Fatal(err)
	}
}
