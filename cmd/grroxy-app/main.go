package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/glitchedgitz/grroxy/apps/app"
	"github.com/glitchedgitz/grroxy/internal/config"
	_ "github.com/glitchedgitz/grroxy/internal/logflags"
	"github.com/glitchedgitz/grroxy/internal/utils"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"
	"github.com/spf13/cobra"
)

var conf config.Config
var API app.Backend

var HostAddress string
var ProjectPath string
var ProxyAddress string // removed, we use api now
var showLogs bool

// func printBanner() {
// 	if !noBanner {
// 		fmt.Fprint(os.Stderr, banner)
// 	}
// }

func init() {
	// Ensure timestamps are included in standard log output.
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
}

func initialize() {

	if !showLogs {
		// log.SetOutput(io.Discard)
	}

	// printBanner()

	var err error
	conf.HostAddr, err = utils.CheckAndFindAvailablePort(HostAddress)
	if err != nil {
		log.Fatalln(err)
	} else {
		if conf.HostAddr != HostAddress {
			fmt.Println("\nInfo: Host address is already in use. Using ", conf.HostAddr)
		}
	}

	// Optional override: if GRROXY_TEMPLATE_DIR isn't set, keep the default config value.
	if templateDir := strings.TrimSpace(os.Getenv("GRROXY_TEMPLATE_DIR")); templateDir != "" {
		conf.TemplateDirectory = templateDir
	}

	conf.Initiate()
}

// while migration we need to trun this on
const MIGRATION_MODE = false

func main() {

	if MIGRATION_MODE {
		pocketbaseApp()
	} else {
		prodApp()
	}
}

func prodApp() {
	flag.StringVar(&HostAddress, "host", "127.0.0.1:8090", "Host address to listen on")
	flag.StringVar(&ProxyAddress, "proxy", "127.0.0.1:8888", "Proxy address to listen on")
	flag.StringVar(&ProjectPath, "path", "", "Project directory path")
	flag.BoolVar(&showLogs, "log", false, "Show debug logs")

	flag.Parse()

	if len(os.Args) > 1 {
		initialize()

		fmt.Println("Initializing done")
		serve(ProjectPath)
	} else {
		fmt.Println("No project path provided")
	}
}

// while migration we need to use this function
func pocketbaseApp() {
	app := pocketbase.New()

	app.RootCmd.AddCommand(&cobra.Command{
		Use: "hello",
		Run: func(cmd *cobra.Command, args []string) {
			log.Println("Hello world!")
		},
	})

	app.RootCmd.PersistentFlags().StringVar(&HostAddress, "host", "127.0.0.1:8090", "")
	app.RootCmd.PersistentFlags().StringVar(&ProxyAddress, "proxy", "127.0.0.1:8888", "")
	app.RootCmd.PersistentFlags().StringVar(&ProjectPath, "path", "", "")
	app.RootCmd.PersistentFlags().BoolVar(&showLogs, "log", false, "")

	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{
		// enable auto creation of migration files when making collection changes in the Admin UI
		// (the isGoRun check is to enable it only during development)
		Automigrate: true,
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
