package launcher

import (
	"log"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/glitchedgitz/grroxy-db/internal/save"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
)

func (launcher *Launcher) GetFilePath(folder, fileName string) string {
	switch folder {
	case "cache":
		return path.Join(launcher.Config.CacheDirectory, fileName)
	case "config":
		return path.Join(launcher.Config.ProjectsDirectory, fileName)
	case "cwd":
		cwd, _ := os.Getwd()
		return path.Join(strings.Trim(cwd, " "), fileName)
	default:
		return fileName
	}
}

func (launcher *Launcher) ReadFile(e *core.ServeEvent) error {
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

			content := save.ReadFile(launcher.GetFilePath(folder, fileName))

			return c.JSON(http.StatusOK, map[string]interface{}{
				"filecontent": string(content),
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(launcher.App),
		},
	})
	return nil
}

func (launcher *Launcher) SaveFile(e *core.ServeEvent) error {
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

			filepath := launcher.GetFilePath(folder, fileName)

			// Save request_id.txt
			save.WriteFile(filepath, []byte(fileData))

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
