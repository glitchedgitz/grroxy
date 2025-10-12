package api

import (
	"log"
	"net/http"
	"path"

	"github.com/glitchedgitz/grroxy-db/browser"
	"github.com/glitchedgitz/grroxy-db/proxy"
	"github.com/glitchedgitz/grroxy-db/utils"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
)

var PROXY *proxy.Proxy

type ProxyBody struct {
	HTTP    string `json:"http,omitempty"`
	Browser string `json:"browser,omitempty"`
}

func (backend *Backend) StartProxy(e *core.ServeEvent) error {

	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/proxy/start",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil

			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			log.Println("/api/proxy/start begins")

			var body ProxyBody
			if err := c.Bind(&body); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			log.Println("/api/proxy/start body", body)

			availableHost, err := utils.CheckAndFindAvailablePort(body.HTTP)

			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			if availableHost != body.HTTP {
				return c.JSON(http.StatusOK, map[string]interface{}{"error": "port not available", "availableHost": availableHost})
			}

			options := &proxy.Options{
				Silent:                      false,
				Directory:                   path.Join(backend.Config.HomeDirectory, ".config", "grroxy"),
				CertCacheSize:               256,
				Verbosity:                   false,
				AppAddress:                  backend.Config.HostAddr,
				ListenAddrHTTP:              body.HTTP,
				ListenAddrSocks5:            "127.0.0.1:10080",
				OutputDirectory:             "grroxy_test",
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
			}

			if PROXY != nil {
				PROXY.Stop()
			}

			PROXY, err = proxy.NewProxy(options)

			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			go PROXY.RunProxy()

			if body.Browser != "" {
				certPath := path.Join(backend.Config.ConfigDirectory, "cacert.crt")
				go func() {
					err := browser.LaunchBrowser(body.Browser, body.HTTP, certPath)
					if err != nil {
						log.Println("Error launching browser:", err)
					}
				}()
			}

			record, err := backend.App.Dao().FindRecordById("_settings", "PROXY__________")
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			record.Set("value", body.HTTP)
			if err := backend.App.Dao().SaveRecord(record); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			return c.JSON(http.StatusOK, map[string]any{"message": "Proxy started"})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

func (backend *Backend) StopProxy(e *core.ServeEvent) error {

	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/proxy/stop",
		Handler: func(c echo.Context) error {

			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil

			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			PROXY.Stop()

			record, err := backend.App.Dao().FindRecordById("_settings", "PROXY__________")
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			record.Set("value", "")
			if err := backend.App.Dao().SaveRecord(record); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			return c.JSON(http.StatusOK, map[string]any{"message": "Proxy stopped"})
		},
	})
	return nil
}
