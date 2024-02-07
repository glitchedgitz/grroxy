package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	// "github.com/pocketbase/dbx"

	"github.com/glitchedgitz/grroxy-db/api/endpoints"
	"github.com/glitchedgitz/grroxy-db/config"
	"github.com/glitchedgitz/grroxy-db/proxy"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"
	"github.com/spf13/cobra"

	// "github.com/pocketbase/pocketbase/tools/list"
	_ "github.com/glitchedgitz/grroxy-db/cmd/grroxy-db/migrations"
)

var conf config.Config
var pb endpoints.DatabaseAPI
var noUI bool
var noProxy bool
var showLogs bool
var noBanner bool

func serveAndOpen() {
	if !noProxy {
		go proxy.StartProxy()
		// C2.Start()
	}
	pb.Serve()
}

func checkVerbose() {
	if showLogs {
		log.SetOutput(os.Stderr)
	}
}

func printBanner() {
	if !noBanner {
		fmt.Fprint(os.Stderr, banner)
	}
}

func init() {
	log.SetOutput(io.Discard)
}

func main() {
	conf.Initiate()

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

	go pb.CommandManager()

	migratecmd.MustRegister(pb.App, pb.App.RootCmd, migratecmd.Config{
	})

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

	pb.App.RootCmd.SetHelpTemplate(commandsUsage)
	pb.App.RootCmd.SetUsageTemplate(commandsUsage)

	// pb.App.RootCmd.PersistentFlags().BoolVar(&noUI, "no-ui", false, "A global flag for the application")
	pb.App.RootCmd.PersistentFlags().BoolVar(&noProxy, "no-proxy", false, "")
	pb.App.RootCmd.PersistentFlags().BoolVar(&noBanner, "no-banner", false, "")

	pb.App.RootCmd.AddCommand(&cobra.Command{
		Use: "list",
		Run: func(cmd *cobra.Command, args []string) {
			printBanner()
			conf.ListProjects()
			serveAndOpen()
			fmt.Println(noUI)
			checkVerbose()
		},
	})

	pb.App.RootCmd.AddCommand(&cobra.Command{
		Use: ".",
		Run: func(cmd *cobra.Command, args []string) {
			conf.OpenCWD()
			serveAndOpen()
			checkVerbose()
		},
	})

	pb.App.RootCmd.AddCommand(&cobra.Command{
		Use: "config",
		Run: func(cmd *cobra.Command, args []string) {
			conf.ShowConfig()
			checkVerbose()
		},
	})

	pb.App.RootCmd.AddCommand(&cobra.Command{
		Use: "create",
		Run: func(cmd *cobra.Command, args []string) {
			printBanner()
			projectName := "Project"
			if len(args) > 0 && args[0] != "." {
				projectName = strings.Join([]string(args), " ")
			}
			conf.NewProject(projectName)
			serveAndOpen()
			checkVerbose()
		},
	})

	if err := pb.App.Start(); err != nil {
		log.Fatal(err)
	}
}
