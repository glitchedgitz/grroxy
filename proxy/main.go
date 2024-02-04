package proxy

import (
	"log"
	"os"
	"os/signal"
	"path"
	"syscall"
	// "github.com/pocketbase/dbx"
	// "github.com/pocketbase/pocketbase/tools/list"
)

func StartProxy() {

	// create channel

	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Almost never here but panic
		panic(err)
	}
	configDirectory := path.Join(homeDir, ".config", "grroxy")
	OutputDirectory := "grroxy_test"

	proxy, err := NewProxy(&Options{
		Silent:                      false,
		Directory:                   configDirectory,
		CertCacheSize:               256,
		Verbosity:                   false,
		ListenAddrHTTP:              "127.0.0.1:8888",
		ListenAddrSocks5:            "127.0.0.1:10080",
		OutputDirectory:             OutputDirectory,
		RequestDSL:                  "",
		ResponseDSL:                 "",
		UpstreamHTTPProxies:         []string{},
		UpstreamSock5Proxies:        []string{},
		ListenDNSAddr:               "",
		DNSMapping:                  "",
		DNSFallbackResolver:         "",
		RequestMatchReplaceDSL:      "",
		ResponseMatchReplaceDSL:     "",
		DumpRequest:                 false,
		DumpResponse:                false,
		UpstreamProxyRequestsNumber: 1,
		// Elastic:                     &Elastic,
		// Kafka:                       &Kafka,
		Allow:     []string{},
		Deny:      []string{},
		Intercept: true,
		Waiting:   true,
	})

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
