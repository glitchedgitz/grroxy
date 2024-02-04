package endpoints

import (
	"net/http"

	"github.com/glitchedgitz/grroxy-db/api/frontend"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func (pocketbaseDB *DatabaseAPI) BindFrontend(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method:  http.MethodGet,
		Path:    "/*",
		Handler: echo.StaticDirectoryHandler(frontend.DistDirFS, false),
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(pocketbaseDB.App),
		},
	})
	return nil
}
