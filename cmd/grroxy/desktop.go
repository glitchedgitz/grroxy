package main

import (
	"context"
	"runtime"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	rt "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/glitchedgitz/grroxy-db/cmd/grroxy/frontend"
)

type App struct {
	ctx context.Context
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) GetHost() string {
	return "http://" + MainHostAddress
}

func FindChromePath() string {
	chromePath := ""
	if runtime.GOOS == "darwin" {
		chromePath = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	}
	return chromePath
}

func runApp() {
	frameless := true
	app := NewApp()

	AppMenu := menu.NewMenu()
	if runtime.GOOS == "darwin" {
		AppMenu.Append(menu.AppMenu()) // On macOS platform, this must be done right after `NewMenu()`
	}
	FileMenu := AppMenu.AddSubmenu("File")
	FileMenu.AddText("&Open", keys.CmdOrCtrl("o"), func(_ *menu.CallbackData) {
		// do something
	})
	FileMenu.AddSeparator()
	FileMenu.AddText("Quit", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
		rt.Quit(app.ctx)
	})

	FileMenu.AddSeparator()
	FileMenu.AddText("Fullscreen", keys.CmdOrCtrl("f"), func(_ *menu.CallbackData) {
		rt.WindowFullscreen(app.ctx)
	})

	if runtime.GOOS == "darwin" {
		frameless = false
		AppMenu.Append(menu.EditMenu()) // On macOS platform, EditMenu should be appended to enable Cmd+C, Cmd+V, Cmd+Z... shortcuts
	}

	// conf.Initiate()
	// conf.LoadAppData()
	err := wails.Run(&options.App{
		Title:            "Grroxy",
		Width:            1366,
		Height:           768,
		Frameless:        frameless,
		WindowStartState: options.Maximised,
		Menu:             AppMenu, // reference the menu above
		AssetServer: &assetserver.Options{
			Assets: frontend.DistDirFS,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
