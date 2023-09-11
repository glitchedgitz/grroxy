package main

import (
	"log"

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

func main() {

	conf.Initiate()

	// Create an instance of the app structure
	pb = endpoints.DatabaseAPI{
		App: pocketbase.NewWithConfig(
			&pocketbase.Config{
				DefaultDataDir:       "grroxy",
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
	pb.App.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		pb.App.Dao().DB().NewQuery(`
			DELETE FROM _intercept;
		`).Execute()
		return nil
	})

	pb.App.RootCmd.AddCommand(&cobra.Command{
		Use: "list",
		Run: func(cmd *cobra.Command, args []string) {
			conf.ListProjects()
			pb.Serve()
		},
	})

	pb.App.RootCmd.AddCommand(&cobra.Command{
		Use: ".",
		Run: func(cmd *cobra.Command, args []string) {
			conf.OpenCWD()
			pb.Serve()
		},
	})

	pb.App.RootCmd.AddCommand(&cobra.Command{
		Use: "config",
		Run: func(cmd *cobra.Command, args []string) {
			conf.ShowConfig()
		},
	})

	pb.App.RootCmd.AddCommand(&cobra.Command{
		Use: "create",
		Run: func(cmd *cobra.Command, args []string) {
			conf.NewProject()
			pb.Serve()
		},
	})

	if err := pb.App.Start(); err != nil {
		log.Fatal(err)
	}
}
