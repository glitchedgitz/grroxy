package api

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func (backend *Backend) DownloadCert(e *core.ServeEvent) error {

	e.Router.AddRoute(echo.Route{
		Method: http.MethodGet,
		Path:   "/cacert.crt",
		Handler: func(c echo.Context) error {
			return c.Attachment(backend.Config.ConfigDirectory, "cacert.crt")
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}
