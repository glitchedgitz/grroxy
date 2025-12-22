package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"

	// "github.com/pocketbase/dbx"

	"github.com/glitchedgitz/cook/v2/pkg/cook"
	"github.com/glitchedgitz/grroxy-db/apps/launcher"
	"github.com/glitchedgitz/grroxy-db/grx/rawproxy"
	"github.com/glitchedgitz/grroxy-db/internal/config"
	_ "github.com/glitchedgitz/grroxy-db/internal/logflags"
	"github.com/glitchedgitz/grroxy-db/internal/process"
	"github.com/glitchedgitz/grroxy-db/internal/utils"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"
	"github.com/spf13/cobra"

	// "github.com/pocketbase/pocketbase/tools/list"
	_ "github.com/glitchedgitz/grroxy-db/cmd/grroxy/migrations"
)

var noProxy bool
var MainHostAddress string = "127.0.0.1:8090"
var MainProxyAddress string = "127.0.0.1:8888"
var showLogs = false

func init() {
	// Ensure timestamps are included in standard log output.
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
}

var launch *launcher.Launcher
var conf config.Config

func setConfig() {

	conf.Initiate()

	caCrtPath := path.Join(conf.ConfigDirectory, "ca.crt")
	caKeyPath := path.Join(conf.ConfigDirectory, "ca.key")

	// If certificates don't exist, generate them using rawproxy
	if !fileExists(caCrtPath) || !fileExists(caKeyPath) {
		_, certPath, _, err := rawproxy.GenerateMITMCA(conf.ConfigDirectory)
		if err != nil {
			log.Printf("[Warning] Failed to generate CA certificate: %v", err)
		} else {
			log.Printf("[Certificate] CA certificate generated at: %s", certPath)
		}
	} else {
		log.Printf("[Certificate] CA certificate already exists at: %s", caCrtPath)
	}
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func initialize() {

	setConfig()

	if !showLogs {
		log.SetOutput(io.Discard)
	}

	startCore()

}

func completionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "completion",
		Short: "This meant to be hidden",
	}
}

var rootCmd = &cobra.Command{
	Use:   "grroxy",
	Short: "grroxy is center of your web hacking operations",
}

func init() {
	completion := completionCommand()

	// mark completion hidden
	completion.Hidden = true
	rootCmd.AddCommand(completion)
}

func main() {

	rootCmd.AddCommand(&cobra.Command{
		Use:   "config",
		Short: "Set config directory",
		Run: func(cmd *cobra.Command, args []string) {
			setConfig()
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "start",
		Short: "Start grroxy",
		Run: func(cmd *cobra.Command, args []string) {
			initialize()
		},
	})

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func startCore() {

	launch = &launcher.Launcher{
		App: pocketbase.NewWithConfig(
			pocketbase.Config{
				ProjectDir:      path.Join(conf.ProjectsDirectory),
				DefaultDataDir:  "grroxy-main",
				HideStartBanner: true,
				// DefaultDev: true,
				// DefaultEncryptionEnv: "hJH#GRJ#HG$JH$54h5kjhHJG#JHG#*&Y&EG#F&GIG@JKGH$JHRGJ##JKJH#JHG",
			},
		),
		Cook:       cook.NewGenerator(),
		Config:     &conf,
		CmdChannel: make(chan process.RunCommandData),
	}

	migratecmd.MustRegister(launch.App, launch.App.RootCmd, migratecmd.Config{})

	go launch.CommandManager()
	launch.App.Bootstrap()

	// Reset project states when the app is terminated
	launch.App.OnBeforeServe().Add(launch.ResetProjectStates)
	launch.App.OnBeforeServe().Add(launch.ResetToolsStates)

	// Adding custom endpoints
	launch.App.OnBeforeServe().Add(launch.API_ListProjects)
	launch.App.OnBeforeServe().Add(launch.API_CreateNewProject)
	launch.App.OnBeforeServe().Add(launch.API_OpenProject)
	launch.App.OnBeforeServe().Add(launch.BindFrontend)
	launch.App.OnBeforeServe().Add(launch.RunCommand)
	launch.App.OnBeforeServe().Add(launch.TextSQL)
	launch.App.OnBeforeServe().Add(launch.SaveFile)
	launch.App.OnBeforeServe().Add(launch.ReadFile)
	launch.App.OnBeforeServe().Add(launch.DownloadCert)
	launch.App.OnBeforeServe().Add(launch.CookSearch)
	launch.App.OnBeforeServe().Add(launch.SearchRegex)
	launch.App.OnBeforeServe().Add(launch.FileWatcher)
	launch.App.OnBeforeServe().Add(launch.TemplatesList)
	launch.App.OnBeforeServe().Add(launch.TemplatesNew)
	launch.App.OnBeforeServe().Add(launch.TemplatesDelete)
	launch.App.OnBeforeServe().Add(launch.Tools)
	launch.App.OnBeforeServe().Add(launch.ToolsServer)

	host, err := utils.CheckAndFindAvailablePort("127.0.0.1:8090")
	if err != nil {
		panic(err)
	}

	// Signal that initialization is complete before starting the server
	fmt.Println("Starting main app at: ", host)

	_, err = apis.Serve(launch.App, apis.ServeConfig{
		HttpAddr: host,
	})

	if errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}
