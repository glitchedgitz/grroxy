package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	// "github.com/pocketbase/dbx"

	"github.com/glitchedgitz/grroxy-db/api"
	"github.com/glitchedgitz/grroxy-db/base"
	"github.com/glitchedgitz/grroxy-db/config"
	"github.com/spf13/cobra"

	// "github.com/pocketbase/pocketbase/tools/list"
	_ "github.com/glitchedgitz/grroxy-db/cmd/grroxy/migrations"
)

var conf config.Config
var API api.Backend
var noUI bool
var noProxy bool
var HostAddress string
var ProxyAddress string
var showLogs bool
var noBanner bool

func printBanner() {
	if !noBanner {
		fmt.Fprint(os.Stderr, banner)
	}
}

func init() {
	// log.SetOutput(io.Discard)
}

func initialize() {

	if !showLogs {
		log.SetOutput(io.Discard)
	}

	printBanner()

	var err error
	conf.HostAddr, err = base.CheckAndFindAvailablePort(HostAddress)
	if err != nil {
		log.Fatalln(err)
	} else {
		if conf.HostAddr != HostAddress {
			fmt.Println("\nInfo: Host address is already in use. Using ", conf.HostAddr)
		}
	}
	conf.ProxyAddr = ProxyAddress
	conf.Initiate()
}

func main() {

	var rootCmd = &cobra.Command{
		Use:   "grroxy",
		Short: "grroxy is center of your web hacking operations",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(commandsUsage)
		},
	}

	rootCmd.AddCommand(&cobra.Command{
		Use: "list",
		Run: func(cmd *cobra.Command, args []string) {
			initialize()
			conf.ListProjects()
			serve()
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use: "config",
		Run: func(cmd *cobra.Command, args []string) {
			initialize()
			conf.ShowConfig()
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use: "create",
		Run: func(cmd *cobra.Command, args []string) {
			initialize()

			printBanner()
			projectName := "Project"
			if len(args) > 0 && args[0] != "." {
				projectName = strings.Join([]string(args), " ")
			}
			conf.NewProject(projectName)
			serve()
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use: "resume",
		Run: func(cmd *cobra.Command, args []string) {
			initialize()
			conf.OpenProject(0)
			serve()
		}})

	rootCmd.PersistentFlags().StringVar(&HostAddress, "host", "127.0.0.1:8090", "")
	rootCmd.PersistentFlags().StringVar(&ProxyAddress, "proxy", "127.0.0.1:8888", "")
	rootCmd.PersistentFlags().BoolVar(&noProxy, "no-proxy", false, "")
	rootCmd.PersistentFlags().BoolVar(&noBanner, "no-banner", false, "")
	rootCmd.PersistentFlags().BoolVar(&showLogs, "verbose", false, "")

	rootCmd.SetHelpTemplate(commandsUsage)
	rootCmd.SetUsageTemplate(commandsUsage)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
