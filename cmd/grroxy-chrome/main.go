package main

import (
	"context"
	"log"
	"os"

	"github.com/chromedp/chromedp"
)

func main() {
	dir, err := os.MkdirTemp("", "chromedp-example")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		// chromedp.DisableGPU,
		chromedp.UserDataDir(dir),
		chromedp.Flag("headless", false),
		chromedp.Flag("proxy-server", "http://127.0.0.1:8888"),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	// also set up a custom logger
	taskCtx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
	defer cancel()

	// ensure that the browser process is started
	if err := chromedp.Run(taskCtx); err != nil {
		log.Fatal(err)
	}

	// chromedp.
	// var wg sync.WaitGroup
	// wg.Add(1)
	// wg.Wait()
	// path := filepath.Join(dir, "DevToolsActivePort")
	// bs, err := os.ReadFile(path)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// lines := bytes.Split(bs, []byte("\n"))
	// fmt.Printf("DevToolsActivePort has %d lines\n", len(lines))
}
