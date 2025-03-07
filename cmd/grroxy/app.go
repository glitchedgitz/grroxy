package main

// import (
// 	"context"
// 	"fmt"
// 	"log"
// 	"os/exec"

// 	"github.com/glitchedgitz/grroxy-db/config"
// 	"github.com/wailsapp/wails/v2/pkg/runtime"
// 	"golang.design/x/hotkey"
// 	"golang.design/x/mainthread"
// )

// // App struct
// type App struct {
// 	ctx context.Context
// }

// // NewApp creates a new App application struct
// func NewApp() *App {
// 	return &App{}
// }

// // startup is called when the app starts. The context is saved
// // so we can call the runtime methods
// func (a *App) startup(ctx context.Context) {
// 	a.ctx = ctx
// 	mainthread.Init(a.RegisterHotKey)
// }

// var Host = "http://127.0.0.1:8090"

// // Greet returns a greeting for the given name
// func (a *App) GetHost() string {
// 	return Host
// }

// // just a wrapper to have access to App functions
// // not necessary if you don't plan to do anything with your App on shortcut use
// func (a *App) RegisterHotKey() {
// 	registerHotkey(a)
// }

// // Run command
// func (a *App) RunCommand(index string) {
// 	log.Println("Projects Index:", index)
// 	cmd := exec.Command("grroxy-desktop", "projects", index, "--app")
// 	if err := cmd.Run(); err != nil {
// 		log.Fatal(err)
// 	}
// }

// // Load Projects
// func (a *App) LoadProjects() config.JSONData {
// 	var conf config.Config
// 	conf.Initiate()
// 	return conf.AppData
// }

// func registerHotkey(a *App) {
// 	// Shortcut Ctrl + Shift + S
// 	// Bring the screen to top
// 	hk := hotkey.New([]hotkey.Modifier{hotkey.ModCtrl, hotkey.ModShift}, hotkey.KeyS)
// 	err := hk.Register()
// 	if err != nil {
// 		return
// 	}

// 	// you have 2 events available - Keyup and Keydown
// 	// you can either or neither, or both
// 	fmt.Printf("hotkey: %v is registered\n", hk)
// 	for {

// 		<-hk.Keydown()
// 		// do anything you want on Key down event
// 		fmt.Printf("hotkey: %v is down\n", hk)

// 		// runtime.BrowserOpenURL(a.ctx, "https://www.google.com")

// 		// runtime.WindowSetAlwaysOnTop(a.ctx, true)
// 		// time.Sleep(1 * time.Second)
// 		// runtime.WindowSetAlwaysOnTop(a.ctx, false)
// 		runtime.WindowShow(a.ctx)
// 	}

// 	// runtime.EventsEmit(a.ctx, "Backend:GlobalHotkeyEvent", time.Now().String())

// 	// hk.Unregister()
// 	// fmt.Printf("hotkey: %v is unregistered\n", hk)

// 	// reattach listener
// 	// registerHotkey(a)
// }
