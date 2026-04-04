package launcher

import (
	"net/http"

	"github.com/glitchedgitz/grroxy/grx/version"
	"github.com/glitchedgitz/pocketbase/core"
	"github.com/labstack/echo/v5"
)

func (launcher *Launcher) Version(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodGet,
		Path:   "/api/version",
		Handler: func(c echo.Context) error {
			return c.JSON(http.StatusOK, map[string]interface{}{
				"version": version.RELEASED_APP_VERSION,
			})
		},
	})
	return nil
}
