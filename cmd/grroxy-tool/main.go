package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/glitchedgitz/grroxy-db/api/tools"
	"github.com/glitchedgitz/grroxy-db/config"
	"github.com/glitchedgitz/grroxy-db/process"
	"github.com/glitchedgitz/grroxy-db/utils"

	_ "github.com/glitchedgitz/grroxy-db/cmd/grroxy-tool/migrations"
)

var conf config.Config

func initialize() {

	fmt.Println("Starting grroxyy")

	// if !showLogs {
	// 	log.SetOutput(io.Discard)
	// }

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

func main() {

	initialize()

	var host string
	var path string
	var name string

	flag.StringVar(&host, "host", "127.0.0.1:8090", "Host address to listen on")
	flag.StringVar(&path, "path", ".", "Project directory path")
	flag.StringVar(&name, "name", "grroxy-tool", "tool name")
	flag.Parse()

	backend := tools_api.Tools{
		App: pocketbase.NewWithConfig(
			pocketbase.Config{
				ProjectDir:      path,
				DefaultDataDir:  name,
				HideStartBanner: true,
			},
		),
		Config:     &conf,
		CmdChannel: make(chan process.RunCommandData),
	}

	backend.App.OnBeforeServe().Add(backend.RunCommand)

	backend.App.Bootstrap()
	go backend.CommandManager()

	_, err := apis.Serve(backend.App, apis.ServeConfig{
		HttpAddr: host,
	})

	if errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}
