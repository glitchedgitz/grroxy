package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/glitchedgitz/grroxy-db/api/app"
	"github.com/glitchedgitz/grroxy-db/config"
	"github.com/glitchedgitz/grroxy-db/utils"
)

var conf config.Config
var API api.Backend

// var noUI bool
var noProxy bool
var HostAddress string
var ProjectPath string
var ProxyAddress string
var showLogs bool

// var noBanner bool
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
	conf.ProxyAddr = ProxyAddress

	if os.Getenv("GRROXY_TEMPLATE_DIR") == "" {
		panic("GRROXY_TEMPLATE_DIR environment variable is not set")
	}
	conf.TemplateDirectory = os.Getenv("GRROXY_TEMPLATE_DIR")

	conf.Initiate()
}

func main() {

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

	// var rootCmd = &cobra.Command{
	// 	Use:   "grroxy",
	// 	Short: "grroxy is center of your web hacking operations",
	// }

	// rootCmd.AddCommand(&cobra.Command{
	// 	Use:   "projects [project index (optional)]",
	// 	Short: "List all projects or open a specific project by index",
	// 	Run: func(cmd *cobra.Command, args []string) {
	// 		initialize()
	// 		if len(args) > 0 {
	// 			index, err := strconv.Atoi(args[0])

	// 			if err != nil {
	// 				fmt.Println("Invalid project index:", args[0])
	// 				return
	// 			}

	// 			conf.OpenProject(index)
	// 		} else {
	// 			conf.ListProjects()
	// 		}
	// 		serve()
	// 	},
	// })

	// rootCmd.AddCommand(&cobra.Command{
	// 	Use: "config",
	// 	Run: func(cmd *cobra.Command, args []string) {
	// 		initialize()
	// 		conf.ShowConfig()
	// 	},
	// })

	// rootCmd.AddCommand(&cobra.Command{
	// 	Use: "create [project name]",
	// 	Run: func(cmd *cobra.Command, args []string) {
	// 		initialize()

	// 		// printBanner()
	// 		projectName := "Project"
	// 		if len(args) > 0 && args[0] != "." {
	// 			projectName = strings.Join([]string(args), " ")
	// 		}
	// 		conf.NewProject(projectName)
	// 		serve()
	// 	},
	// })

	// rootCmd.AddCommand(&cobra.Command{
	// 	Use: "resume",
	// 	Run: func(cmd *cobra.Command, args []string) {
	// 		initialize()
	// 		conf.OpenProject(0)
	// 		serve()
	// 	}})

	// rootCmd.PersistentFlags().StringVar(&HostAddress, "host", "127.0.0.1:8090", "")
	// rootCmd.PersistentFlags().StringVar(&ProxyAddress, "proxy", "127.0.0.1:8888", "")
	// rootCmd.PersistentFlags().BoolVar(&noProxy, "no-proxy", false, "")
	// rootCmd.PersistentFlags().BoolVar(&noBanner, "no-banner", false, "")
	// rootCmd.PersistentFlags().BoolVar(&showLogs, "verbose", false, "")
	// rootCmd.PersistentFlags().BoolVar(&launchApp, "app", false, "")

	// if err := rootCmd.Execute(); err != nil {
	// 	fmt.Println(err)
	// 	os.Exit(1)
	// }
}
