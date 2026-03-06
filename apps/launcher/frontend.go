package launcher

import (
	"net/http"

	"github.com/glitchedgitz/grroxy/grx/frontend"
	"github.com/glitchedgitz/pocketbase/apis"
	"github.com/glitchedgitz/pocketbase/core"
	"github.com/labstack/echo/v5"
)

func (launcher *Launcher) BindFrontend(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method:  http.MethodGet,
		Path:    "/*",
		Handler: echo.StaticDirectoryHandler(frontend.DistDirFS, false),
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(launcher.App),
		},
	})
	return nil
}
