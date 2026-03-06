package app

import (
	"net/http"
	"path"

	"github.com/glitchedgitz/grroxy/grx/version"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func (backend *Backend) Info(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodGet,
		Path:   "/api/info",
		Handler: func(c echo.Context) error {
			return c.JSON(http.StatusOK, map[string]interface{}{
				"version":    version.CURRENT_BACKEND_VERSION,
				"cwd":        path.Join(backend.Config.ProjectsDirectory, backend.Config.ProjectID),
				"project_id": backend.Config.ProjectID,
				"cache":      backend.Config.CacheDirectory,
				"config":     backend.Config.ConfigDirectory,
				"template":   backend.Config.TemplateDirectory,
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}
