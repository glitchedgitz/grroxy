package proxy

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	// "github.com/pocketbase/dbx"
	// "github.com/pocketbase/pocketbase/tools/list"
)

func StartProxy(options *Options) {

	proxy, err := NewProxy(options)

	if err != nil {
		panic(err)
	}

	go func() {
		c := make(chan os.Signal, 1) //added size 1 to channel buffer
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-c
			log.Println("\r- Ctrl+C pressed in Terminal")
			proxy.Stop()
			os.Exit(0)
		}()
	}()

	proxy.RunProxy()

}
