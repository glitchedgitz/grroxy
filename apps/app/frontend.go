package app

import (
	"net/http"

	"github.com/glitchedgitz/grroxy/grx/frontend"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func (backend *Backend) BindFrontend(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method:  http.MethodGet,
		Path:    "/*",
		Handler: echo.StaticDirectoryHandler(frontend.DistDirFS, false),
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}
