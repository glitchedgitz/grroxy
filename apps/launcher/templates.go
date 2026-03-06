package launcher

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/glitchedgitz/grroxy/internal/save"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

type Path struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
}

type TemplateInfo struct {
	Name    string `json:"name,omitempty"`
	Content string `json:"content,omitempty"`
}

func (launcher *Launcher) TemplatesList(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: "GET",
		Path:   "/api/templates/list",
		Handler: func(c echo.Context) error {

			list := []Path{}

			entries, err := os.ReadDir(launcher.Config.TemplateDirectory)
			if err != nil {
				fmt.Println("Error:", err)
				return err
			}
			for _, entry := range entries {
				name := entry.Name()
				if strings.HasPrefix(name, ".") {
					continue
				}
				if entry.IsDir() {
					continue
				}
				lower := strings.ToLower(name)
				if !(strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml")) {
					continue
				}
				list = append(list, Path{
					Name:  name,
					Path:  path.Join(launcher.Config.TemplateDirectory, name),
					IsDir: false,
				})
			}

			jsonData := make(map[string]any)
			jsonData["list"] = list

			json.Marshal(jsonData)
			return c.JSON(http.StatusOK, jsonData)
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(launcher.App),
		},
	})

	return nil
}

func (launcher *Launcher) TemplatesNew(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: "POST",
		Path:   "/api/templates/new",
		Handler: func(c echo.Context) error {

			var data TemplateInfo
			if err := c.Bind(&data); err != nil {
				return err
			}

			filepath := path.Join(launcher.Config.TemplateDirectory, data.Name)

			save.WriteFile(filepath, []byte(data.Content))

			return c.JSON(http.StatusOK, map[string]interface{}{
				"filepath": filepath,
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(launcher.App),
		},
	})

	return nil
}

func (launcher *Launcher) TemplatesDelete(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: "DELETE",
		Path:   "/api/templates/:template",
		Handler: func(c echo.Context) error {

			file := c.PathParam("template")

			filepath := path.Join(launcher.Config.TemplateDirectory, file)

			err := os.Remove(filepath)
			if err != nil {
				return c.String(http.StatusInternalServerError, "Error deleting file")
			}

			return c.String(http.StatusOK, "")
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(launcher.App),
		},
	})

	return nil
}
