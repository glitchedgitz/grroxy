package main

import (
	"fmt"
	"log"
	"os/exec"

	// "github.com/pocketbase/dbx"

	"github.com/glitchedgitz/grroxy-db/api/endpoints"
	"github.com/glitchedgitz/grroxy-db/config"
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

func serveAndOpen() {
	C2 := exec.Command("grroxy-proxy")
	if !noProxy {
		C2.Start()
	}
	if noUI {
		pb.Serve()
	} else {
		C1 := exec.Command("grroxy-ui")
		go pb.Serve()
		C1.Start()
		C1.Wait()
	}
	if !noProxy {
		C2.Wait()
	}
}

func main() {

	conf.Initiate()

	// Create an instance of the app structure
	pb = endpoints.DatabaseAPI{
		App: pocketbase.NewWithConfig(
			&pocketbase.Config{
				DefaultDataDir: "grroxy",
				// HideStartBanner:      true,
				// DefaultEncryptionEnv: "hJH#GRJ#HG$JH$54h5kjhHJG#JHG#*&Y&EG#F&GIG@JKGH$JHRGJ##JKJH#JHG",
			},
		),
		Config:     &conf,
		CmdChannel: make(chan endpoints.RunCommandData),
	}

	// pb.CmdChannel
	go pb.CommandManager()

	migratecmd.MustRegister(pb.App, pb.App.RootCmd, &migratecmd.Options{
		// enable auto creation of migration files when making collection changes in the Admin UI
		// (the isGoRun check is to enable it only during development)
		// Automigrate: isGoRun,
	})

	// Adding custom endpoints
	pb.App.OnBeforeServe().Add(pb.SitemapNew)
	pb.App.OnBeforeServe().Add(pb.SitemapFetch)
	pb.App.OnBeforeServe().Add(pb.RunCommand)
	pb.App.OnBeforeServe().Add(pb.SendRawRequest)
	pb.App.OnBeforeServe().Add(pb.TextSQL)
	pb.App.OnBeforeServe().Add(pb.SaveFile)
	pb.App.OnBeforeServe().Add(pb.ReadFile)
	pb.App.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		pb.App.Dao().DB().NewQuery(`
			DELETE FROM _intercept;
			DELETE FROM tmp_intercept;
		`).Execute()
		return nil
	})

	pb.App.RootCmd.PersistentFlags().BoolVar(&noUI, "no-ui", false, "A global flag for the application")
	pb.App.RootCmd.PersistentFlags().BoolVar(&noProxy, "no-proxy", false, "A global flag for the application")

	pb.App.RootCmd.AddCommand(&cobra.Command{
		Use: "list",
		Run: func(cmd *cobra.Command, args []string) {
			conf.ListProjects()
			serveAndOpen()
			fmt.Println(noUI)
		},
	})

	pb.App.RootCmd.AddCommand(&cobra.Command{
		Use: ".",
		Run: func(cmd *cobra.Command, args []string) {
			conf.OpenCWD()
			serveAndOpen()
		},
	})

	pb.App.RootCmd.AddCommand(&cobra.Command{
		Use: "config",
		Run: func(cmd *cobra.Command, args []string) {
			conf.ShowConfig()
		},
	})

	pb.App.RootCmd.AddCommand(&cobra.Command{
		Use: "serve",
		Run: func(cmd *cobra.Command, args []string) {
			conf.ShowConfig()
		},
		// ... rest of the command details
	})

	pb.App.RootCmd.AddCommand(&cobra.Command{
		Use: "create",
		Run: func(cmd *cobra.Command, args []string) {
			conf.NewProject()
			serveAndOpen()
		},
	})

	if err := pb.App.Start(); err != nil {
		log.Fatal(err)
	}
}
