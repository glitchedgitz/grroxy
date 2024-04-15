package api

import (
	"log"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/glitchedgitz/grroxy-db/save"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
)

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
			from := data["from"].(string)

			filePath := fileName
			cwd := ""
			if from == "cache" {
				log.Println("cache")
				filePath = path.Join(backend.Config.CacheDirectory, fileName)
			} else if from == "config" {
				log.Println("config")
				filePath = path.Join(backend.Config.ConfigDirectory, fileName)
			} else {
				log.Println("cwd")
				cwd, _ = os.Getwd()
				filePath = path.Join(strings.Trim(cwd, " "), fileName)
			}

			content := save.ReadFile(filePath)

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

			filePath := path.Join(backend.Config.CacheDirectory, fileName)

			// Save request_id.txt
			save.WriteFile(filePath, []byte(fileData))

			return c.JSON(http.StatusOK, map[string]interface{}{
				"filepath": filePath,
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}
