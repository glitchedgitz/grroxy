package main

import (
	"embed"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"

	// "github.com/pocketbase/dbx"

	"github.com/glitchedgitz/cook/v2/pkg/cook"
	"github.com/glitchedgitz/grroxy-db/config"
	"github.com/glitchedgitz/grroxy-db/api/launcher"
	"github.com/glitchedgitz/grroxy-db/process"
	"github.com/glitchedgitz/grroxy-db/utils"
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
var showLogs bool
var noBanner bool
var launchApp bool

// func printBanner() {
// 	if !noBanner {
// 		fmt.Fprint(os.Stderr, banner)
// 	}
// }

func init() {
	// log.SetOutput(io.Discard)
}

var launch *launcher.Launcher
var conf config.Config

func setConfig() {
	var err error
	// Probably not used
	conf.HomeDirectory, err = os.UserHomeDir()
	utils.CheckErr("", err)

	conf.CacheDirectory, err = os.UserCacheDir()
	conf.CacheDirectory = path.Join(conf.CacheDirectory, "grroxy")
	os.MkdirAll(conf.CacheDirectory, 0755)
	utils.CheckErr("", err)

	conf.ConfigDirectory, err = os.UserConfigDir()
	conf.ConfigDirectory = path.Join(conf.ConfigDirectory, "grroxy")
	os.MkdirAll(conf.ConfigDirectory, 0755)
	utils.CheckErr("", err)

	fmt.Println("Config directory:", conf.ConfigDirectory)
	fmt.Println("Cache directory:", conf.CacheDirectory)
	fmt.Println("Home directory:", conf.HomeDirectory)
}

func initialize() {

	fmt.Println("Starting grroxyy")
	setConfig()

	// if !showLogs {
	// 	log.SetOutput(io.Discard)
	// }

	startCore()

}

//go:embed all:frontend/dist
var assets embed.FS

func completionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "completion",
		Short: "This meant to be hidden",
	}
}

var rootCmd = &cobra.Command{
	Use:   "grroxy",
	Short: "grroxy is center of your web hacking operations",
	// Run: func(cmd *cobra.Command, args []string) {
	// 	if launchApp {
	// 	} else {
	// 		initialize()
	// 	}
	// },
}

func init() {
	completion := completionCommand()

	// mark completion hidden
	completion.Hidden = true
	rootCmd.AddCommand(completion)
}

func main() {
	// initialize()
	// go initialize()
	// runApp()

	// rootCmd.AddCommand(&cobra.Command{
	// 	Use:   "projects [project index (optional)]",
	// 	Short: "List all projects or open a specific project by index",
	// 	Run: func(cmd *cobra.Command, args []string) {
	// 		// Wait for initialization
	// 		wg.Wait()

	// 		if len(args) > 0 {
	// 			projectIndex, err := strconv.Atoi(args[0])
	// 			if err != nil {
	// 				fmt.Println("Invalid project index:", err)
	// 				return
	// 			}
	// 			launch.OpenProject(projectIndex)
	// 		} else {
	// 			initialize()
	// 			launch.ListProjects()
	// 		}
	// 	},
	// })

	rootCmd.AddCommand(&cobra.Command{
		Use:   "config",
		Short: "Set config directory",
		Run: func(cmd *cobra.Command, args []string) {
			// fmt.Println("Config")
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

	rootCmd.AddCommand(&cobra.Command{
		Use: "app",
		Run: func(cmd *cobra.Command, args []string) {
			go initialize()
			runApp()
		},
	})

	// rootCmd.AddCommand(&cobra.Command{
	// 	Use: "create [project name]",
	// 	Run: func(cmd *cobra.Command, args []string) {
	// 		initialize()

	// 		// printBanner()
	// 		projectName := "Project"

	// 		if len(args) > 0 && args[0] != "." {
	// 			projectName = strings.Join([]string(args), " ")
	// 		}

	// 		projectData, err := launch.CreateNewProject(projectName)

	// 		if err != nil {
	// 			fmt.Println("Error creating project:", err)
	// 			return
	// 		}

	// 		fmt.Println("Project created successfully:", projectData)
	// 	},
	// })

	// rootCmd.AddCommand(&cobra.Command{
	// 	Use: "resume",
	// 	Run: func(cmd *cobra.Command, args []string) {
	// 		initialize()
	// 		// conf.OpenProject(0)
	// 	}})

	// rootCmd.PersistentFlags().StringVar(&MainHostAddress, "host", "127.0.0.1:8090", "")
	// rootCmd.PersistentFlags().StringVar(&MainProxyAddress, "proxy", "127.0.0.1:8888", "")
	// rootCmd.PersistentFlags().BoolVar(&noProxy, "no-proxy", false, "")
	// rootCmd.PersistentFlags().BoolVar(&noBanner, "no-banner", false, "")
	// rootCmd.PersistentFlags().BoolVar(&showLogs, "verbose", false, "")
	// rootCmd.PersistentFlags().BoolVar(&launchApp, "app", false, "")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func startCore() {
	// Remove the defer since we want to control when we signal completion
	// defer wg.Done()

	launch = &launcher.Launcher{
		App: pocketbase.NewWithConfig(
			pocketbase.Config{
				ProjectDir:      path.Join(conf.ConfigDirectory),
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

	// Adding custom endpoints
	launch.App.OnBeforeServe().Add(launch.API_ListProjects)
	launch.App.OnBeforeServe().Add(launch.API_CreateNewProject)
	launch.App.OnBeforeServe().Add(launch.API_OpenProject)
	launch.App.OnBeforeServe().Add(launch.BindFrontend)
	launch.App.OnBeforeServe().Add(launch.RunCommand)
	launch.App.OnBeforeServe().Add(launch.SendRawRequest)
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
	fmt.Println("Starting core at: ", host)

	_, err = apis.Serve(launch.App, apis.ServeConfig{
		HttpAddr: host,
	})

	if errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}
