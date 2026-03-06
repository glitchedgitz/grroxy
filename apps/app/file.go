package app

import (
	"log"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/glitchedgitz/grroxy/internal/save"
	"github.com/glitchedgitz/pocketbase/apis"
	"github.com/glitchedgitz/pocketbase/core"
	"github.com/glitchedgitz/pocketbase/models"
	"github.com/labstack/echo/v5"
)

func (backend *Backend) GetFilePath(folder, fileName string) string {
	switch folder {
	case "cache":
		return path.Join(backend.Config.CacheDirectory, fileName)
	case "config":
		return path.Join(backend.Config.ProjectsDirectory, fileName)
	case "cwd":
		cwd, _ := os.Getwd()
		return path.Join(strings.Trim(cwd, " "), fileName)
	default:
		return fileName
	}
}

func (backend *Backend) ReadFile(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/readfile",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil

			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			var data map[string]interface{}
			if err := c.Bind(&data); err != nil {
				return err
			}
			log.Println("[ReadFile]: ", data)
			fileName := data["fileName"].(string)
			fileName = strings.Trim(fileName, " ")
			folder := data["folder"].(string)

			content := save.ReadFile(backend.GetFilePath(folder, fileName))

			return c.JSON(http.StatusOK, map[string]interface{}{
				"filecontent": string(content),
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

func (backend *Backend) SaveFile(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/savefile",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil

			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			var data map[string]interface{}
			if err := c.Bind(&data); err != nil {
				return err
			}
			fileName := data["fileName"].(string)
			fileData := data["fileData"].(string)
			folder := data["folder"].(string)

			filepath := backend.GetFilePath(folder, fileName)

			// Save request_id.txt
			save.WriteFile(filepath, []byte(fileData))

			return c.JSON(http.StatusOK, map[string]interface{}{
				"filepath": filepath,
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}
