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

	"runtime"

	"github.com/glitchedgitz/cook/v2/pkg/cook"
	"github.com/glitchedgitz/grroxy/apps/launcher"
	"github.com/glitchedgitz/grroxy/grx/rawproxy"
	"github.com/glitchedgitz/grroxy/grx/version"
	"github.com/glitchedgitz/grroxy/internal/config"
	_ "github.com/glitchedgitz/grroxy/internal/logflags"
	"github.com/glitchedgitz/grroxy/internal/process"
	"github.com/glitchedgitz/grroxy/internal/updater"
	"github.com/glitchedgitz/grroxy/internal/utils"
	"github.com/glitchedgitz/pocketbase"
	"github.com/glitchedgitz/pocketbase/apis"
	"github.com/glitchedgitz/pocketbase/plugins/migratecmd"
	"github.com/spf13/cobra"

	// "github.com/glitchedgitz/pocketbase/tools/list"
	_ "github.com/glitchedgitz/grroxy/cmd/grroxy/migrations"
)

var noProxy bool
var MainHostAddress string = "127.0.0.1:8090"
var hostFixed bool // if true, skip CheckAndFindAvailablePort
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
	Short: "Center of your web hacking operations",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		printBanner()
	},
	// When running just `grroxy`, show the command structure (help)
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
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

	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start grroxy",
		Run: func(cmd *cobra.Command, args []string) {
			if h, _ := cmd.Flags().GetString("host"); h != "" {
				MainHostAddress = h
				hostFixed = true
			}
			initialize()
		},
	}
	startCmd.Flags().String("host", "", "host address to listen on (default 127.0.0.1:8090)")
	rootCmd.AddCommand(startCmd)

	rootCmd.AddCommand(&cobra.Command{
		Use:   "update [binary]",
		Short: "Update grroxy binaries to the latest release",
		Long: `Update grroxy binaries from GitHub Releases.

Without arguments, updates all binaries (grroxy, grroxy-app, grroxy-tool).
Specify a binary name to update only that one.`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			allBinaries := []string{"grroxy", "grroxy-app", "grroxy-tool"}
			targets := allBinaries
			if len(args) == 1 {
				targets = []string{args[0]}
			}

			token := updater.GetToken()
			if token == "" {
				fmt.Println("Warning: No GitHub token found. Set GITHUB_TOKEN or GH_TOKEN for private repo access.")
			}

			fmt.Println("Checking for updates...")
			release, err := updater.CheckLatestVersion(token)
			if err != nil {
				fmt.Printf("Error checking for updates: %v\n", err)
				os.Exit(1)
			}

			current := version.CURRENT_BACKEND_VERSION
			latest := release.TagName
			fmt.Printf("Current version: v%s\n", current)
			fmt.Printf("Latest version:  %s\n", latest)
			fmt.Printf("Platform:        %s/%s\n", runtime.GOOS, runtime.GOARCH)

			if !updater.NeedsUpdate(current, latest) {
				fmt.Println("\nAlready up to date!")
				return
			}

			fmt.Printf("\nUpdating to %s...\n\n", latest)

			for _, name := range targets {
				// Clean up .old files from previous Windows updates
				if binPath, err := updater.FindBinaryPath(name); err == nil {
					updater.CleanupOldBinaries(binPath)
				}

				asset, err := updater.FindAsset(release, name)
				if err != nil {
					fmt.Printf("  [SKIP] %s: %v\n", name, err)
					continue
				}

				binPath, err := updater.FindBinaryPath(name)
				if err != nil {
					fmt.Printf("  [SKIP] %s: %v\n", name, err)
					continue
				}

				fmt.Printf("  Updating %s (%s)...", name, binPath)
				if err := updater.UpdateBinary(asset.URL, binPath, token); err != nil {
					fmt.Printf(" FAILED: %v\n", err)
					continue
				}
				fmt.Println(" OK")
			}

			fmt.Printf("\nUpdated to %s. Restart grroxy to use the new version.\n", latest)
		},
	})

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func printBanner() {
	fmt.Printf(`
G R R R . . . O X Y           v%s
`, version.CURRENT_BACKEND_VERSION)
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
	launch.App.OnBeforeServe().Add(launch.API_CheckUpdate)
	launch.App.OnBeforeServe().Add(launch.API_DoUpdate)

	host := MainHostAddress
	if !hostFixed {
		var err error
		host, err = utils.CheckAndFindAvailablePort(MainHostAddress)
		if err != nil {
			panic(err)
		}
	}

	// Signal that initialization is complete before starting the server
	fmt.Println("Starting main app at: ", host)

	_, err := apis.Serve(launch.App, apis.ServeConfig{
		HttpAddr: host,
	})

	if errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}
