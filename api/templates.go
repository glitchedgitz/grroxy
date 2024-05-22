package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

type Path struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
}

func (backend *Backend) ListTemplates(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: "GET",
		Path:   "/api/templates/list",
		Handler: func(c echo.Context) error {

			list := []Path{}

			err := filepath.Walk(`D:\sdks\go\src\github.com\glitchedgitz\grroxy-db\grroxy-templates`, func(path string, info os.FileInfo, err error) error {

				list = append(list, Path{
					Name:  info.Name(),
					Path:  path,
					IsDir: info.IsDir(),
				})

				return nil
			})
			if err != nil {
				fmt.Println("Error:", err)
				return err
			}

			jsonData := make(map[string]any)
			jsonData["list"] = list

			json.Marshal(jsonData)
			return c.JSON(http.StatusOK, jsonData)
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})

	return nil
}
