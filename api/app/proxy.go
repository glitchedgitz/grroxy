package api

import (
	"log"
	"net/http"
	"path"

	"github.com/glitchedgitz/grroxy-db/browser"
	"github.com/glitchedgitz/grroxy-db/utils"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
)

var PROXY *RawProxyWrapper

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

			// Stop existing proxy if running
			if PROXY != nil {
				PROXY.Stop()
			}

			// Create new rawproxy wrapper
			configDir := path.Join(backend.Config.HomeDirectory, ".config", "grroxy")

			// Disable file captures by passing empty string (we save to database instead)
			// To enable file captures for testing, uncomment the line below:
			// outputDir := path.Join(backend.Config.HomeDirectory, ".config", "grroxy", "captures")
			outputDir := "" // Empty = disabled

			PROXY, err = NewRawProxyWrapper(body.HTTP, configDir, outputDir, backend)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			// Load initial intercept filters
			if err := backend.loadInterceptFilters(); err != nil {
				log.Printf("[StartProxy] Warning: Failed to load intercept filters: %v", err)
			}

			// Start the proxy
			if err := PROXY.RunProxy(); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			if body.Browser != "" {
				// Use the certificate path from the rawproxy
				certPath := PROXY.GetCertPath()
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

			if PROXY != nil {
				if err := PROXY.Stop(); err != nil {
					log.Printf("[WARN] Error stopping proxy: %v", err)
				}
			}

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
