package launcher

import (
	"net/http"

	"github.com/glitchedgitz/grroxy-db/browser"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func (launcher *Launcher) DownloadCert(e *core.ServeEvent) error {
	filePath, err := browser.GenerateCert(launcher.Config.ConfigDirectory)
	if err != nil {
		panic(err)
	}

	e.Router.AddRoute(echo.Route{
		Method: http.MethodGet,
		Path:   "/cacert.crt",
		Handler: func(c echo.Context) error {
			return c.Attachment(filePath, "cacert.crt")
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(launcher.App),
		},
	})
	return nil
}
