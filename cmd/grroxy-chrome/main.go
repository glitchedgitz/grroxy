package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	_ "github.com/glitchedgitz/grroxy-db/logflags"
)

func main() {

	if len(os.Args) < 2 {
		fmt.Println("grroxy-chrome [proxy-address]: Required proxy address")
		os.Exit(0)
	}

	address := os.Args[1] // "http://127.0.0.1:8888"
	dir, err := os.MkdirTemp("", "chromedp-example")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserDataDir(dir),
		chromedp.Flag("headless", false),
		chromedp.Flag("proxy-server", address),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	// also set up a custom logger
	taskCtx, taskCancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
	defer taskCancel()

	// ensure that the browser process is started
	if err := chromedp.Run(taskCtx); err != nil {
		log.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		chromedp.ListenBrowser(taskCtx, func(ev interface{}) {
			switch ev.(type) {
			case *target.EventTargetDestroyed:
				log.Println("[Chrome browser] Browser tab closed, shutting down...")
				taskCancel()
				wg.Done()
			}
		})
	}()

	wg.Wait()
}
