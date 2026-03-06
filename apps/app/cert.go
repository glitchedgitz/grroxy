package app

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/glitchedgitz/pocketbase/apis"
	"github.com/glitchedgitz/pocketbase/core"
	"github.com/labstack/echo/v5"
)

// DownloadCert serves the unified CA certificate (ca.crt) for download
// Certificate is generated once at app startup, always at the same location
func (backend *Backend) DownloadCert(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodGet,
		Path:   "/cacert.crt",
		Handler: func(c echo.Context) error {
			// Certificate is always at this fixed location (generated at startup)
			certPath := filepath.Join(backend.Config.ConfigDirectory, "ca.crt")

			// Verify certificate exists
			if _, err := os.Stat(certPath); os.IsNotExist(err) {
				log.Printf("[Certificate] ERROR: Certificate not found at %s", certPath)
				return c.JSON(http.StatusInternalServerError, map[string]string{
					"error": "Certificate not found. Please restart the application.",
				})
			}

			log.Printf("[Certificate] Serving: %s", certPath)
			return c.Attachment(certPath, "grroxy-ca.crt")
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}
