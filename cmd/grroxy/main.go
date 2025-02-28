package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	// "github.com/pocketbase/dbx"

	"github.com/glitchedgitz/grroxy-db/api"
	"github.com/glitchedgitz/grroxy-db/utils"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"
	"github.com/spf13/cobra"

	// "github.com/pocketbase/pocketbase/tools/list"
	_ "github.com/glitchedgitz/grroxy-db/cmd/grroxy/migrations"
)

var App *pocketbase.PocketBase
var API api.Backend
var noUI bool
var noProxy bool
var MainHostAddress string
var MainProxyAddress string
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

func initialize() {

	if !showLogs {
		log.SetOutput(io.Discard)
	}

	var err error

	// Probably not used
	HomeDirectory, err = os.UserHomeDir()
	utils.CheckErr("", err)

	CacheDirectory, err = os.UserCacheDir()
	CacheDirectory = path.Join(CacheDirectory, "grroxy")
	os.MkdirAll(CacheDirectory, 0755)
	utils.CheckErr("", err)

	ConfigDirectory, err = os.UserConfigDir()
	ConfigDirectory = path.Join(ConfigDirectory, "grroxy")
	os.MkdirAll(ConfigDirectory, 0755)
	utils.CheckErr("", err)

}

func main() {

	fmt.Println("Starting grroxyy")
	go startCore()

	var rootCmd = &cobra.Command{
		Use:   "grroxyy",
		Short: "grroxyy is center of your web hacking operations",
	}

	rootCmd.AddCommand(&cobra.Command{
		Use:   "projects [project index (optional)]",
		Short: "List all projects or open a specific project by index",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) > 0 {
				projectIndex, err := strconv.Atoi(args[0])
				if err != nil {
					fmt.Println("Invalid project index:", err)
					return
				}
				openProject(projectIndex)
			} else {
				initialize()
				listProjects()
			}
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use: "config",
		Run: func(cmd *cobra.Command, args []string) {
			initialize()
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use: "create [project name]",
		Run: func(cmd *cobra.Command, args []string) {
			initialize()

			// printBanner()
			projectName := "Project"
			if len(args) > 0 && args[0] != "." {
				projectName = strings.Join([]string(args), " ")
			}
			createNewProject(projectName)
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use: "resume",
		Run: func(cmd *cobra.Command, args []string) {
			initialize()
			// conf.OpenProject(0)
		}})

	rootCmd.PersistentFlags().StringVar(&MainHostAddress, "host", "127.0.0.1:8090", "")
	rootCmd.PersistentFlags().StringVar(&MainProxyAddress, "proxy", "127.0.0.1:8888", "")
	rootCmd.PersistentFlags().BoolVar(&noProxy, "no-proxy", false, "")
	rootCmd.PersistentFlags().BoolVar(&noBanner, "no-banner", false, "")
	rootCmd.PersistentFlags().BoolVar(&showLogs, "verbose", false, "")
	rootCmd.PersistentFlags().BoolVar(&launchApp, "app", false, "")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func startCore() {
	App = pocketbase.NewWithConfig(
		pocketbase.Config{
			ProjectDir:      "D:\\test\\main",
			DefaultDataDir:  "grroxy-main",
			HideStartBanner: true,
			// DefaultDev: true,
			// DefaultEncryptionEnv: "hJH#GRJ#HG$JH$54h5kjhHJG#JHG#*&Y&EG#F&GIG@JKGH$JHRGJ##JKJH#JHG",
		},
	)

	migratecmd.MustRegister(App, App.RootCmd, migratecmd.Config{})

	App.Bootstrap()

	host, err := utils.CheckAndFindAvailablePort("127.0.0.1:8090")
	if err != nil {
		panic(err)
	}

	fmt.Println("Starting core at: ", host)

	_, err = apis.Serve(App, apis.ServeConfig{
		HttpAddr: host,
	})

	if errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}
