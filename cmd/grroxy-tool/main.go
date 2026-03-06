package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/glitchedgitz/grroxy/apps/tools"
	"github.com/glitchedgitz/grroxy/internal/config"
	"github.com/glitchedgitz/grroxy/internal/process"
	"github.com/glitchedgitz/grroxy/internal/utils"
	"github.com/glitchedgitz/pocketbase"
	"github.com/glitchedgitz/pocketbase/apis"

	_ "github.com/glitchedgitz/grroxy/cmd/grroxy-tool/migrations"
)

var conf config.Config

func initialize() {

	conf.Initiate()
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

	// Resolve the project path to an absolute path
	projectPath, err := filepath.Abs(path)
	utils.CheckErr("Failed to resolve project path", err)

	// Change working directory to the project directory
	err = os.Chdir(projectPath)
	utils.CheckErr("Failed to change working directory to project path", err)

	fmt.Println("Working directory changed to:", projectPath)

	backend := tools.Tools{
		App: pocketbase.NewWithConfig(
			pocketbase.Config{
				ProjectDir:      projectPath,
				DefaultDataDir:  name,
				HideStartBanner: true,
			},
		),
		Config:     &conf,
		CmdChannel: make(chan process.RunCommandData),
	}

	backend.App.OnBeforeServe().Add(backend.RunCommand)

	// SDK Login
	backend.App.OnBeforeServe().Add(backend.LoginSDKEndpoint)

	// Fuzzer
	backend.App.OnBeforeServe().Add(backend.StartFuzzer)
	backend.App.OnBeforeServe().Add(backend.StopFuzzer)

	backend.App.Bootstrap()
	go backend.CommandManager()

	_, err = apis.Serve(backend.App, apis.ServeConfig{
		HttpAddr: host,
	})

	if errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}
