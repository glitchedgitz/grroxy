package launcher

import (
	"net/http"

	"github.com/glitchedgitz/grroxy-db/grx/frontend"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
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
