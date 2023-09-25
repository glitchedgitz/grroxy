package endpoints

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/glitchedgitz/grroxy-db/save"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func (pocketbaseDB *DatabaseAPI) ReadFile(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/readfile",
		Handler: func(c echo.Context) error {

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
				filePath = path.Join(pocketbaseDB.Config.CacheDirectory, fileName)
			} else if from == "config" {
				log.Println("config")
				filePath = path.Join(pocketbaseDB.Config.ConfigDirectory, fileName)
			} else {
				log.Println("cwd")
				cwd, _ = os.Getwd()
				filePath = path.Join(strings.Trim(cwd, " "), fileName)
			}

			fmt.Println("Filepath: ", filePath)
			content1 := save.ReadFile(path.Join(strings.Trim(cwd, " "), "api.motos.net.urls"))
			fmt.Println(string(content1))
			fmt.Println(os.Getwd())
			fmt.Println(pocketbaseDB.Config.CWDirectory)

			content := save.ReadFile(filePath)

			return c.JSON(http.StatusOK, map[string]interface{}{
				"filecontent": string(content),
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(pocketbaseDB.App),
		},
	})
	return nil
}

func (pocketbaseDB *DatabaseAPI) SaveFile(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/savefile",
		Handler: func(c echo.Context) error {

			var data map[string]interface{}
			if err := c.Bind(&data); err != nil {
				return err
			}
			log.Println("[SaveFile]: ", data)

			fileName := data["fileName"].(string)
			fileData := data["fileData"].(string)

			filePath := path.Join(pocketbaseDB.Config.CacheDirectory, fileName)

			// Save request_id.txt
			save.WriteFile(filePath, []byte(fileData))

			return c.JSON(http.StatusOK, map[string]interface{}{
				"filepath": filePath,
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(pocketbaseDB.App),
		},
	})
	return nil
}
