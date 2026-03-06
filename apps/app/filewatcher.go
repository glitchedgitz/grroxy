package app

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"

	"github.com/fsnotify/fsnotify"
	"github.com/glitchedgitz/pocketbase/apis"
	"github.com/glitchedgitz/pocketbase/core"
	"github.com/labstack/echo/v5"
)

func (backend *Backend) CWDContent(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: "GET",
		Path:   "/api/cwd",
		Handler: func(c echo.Context) error {

			cwd := path.Join(backend.Config.ProjectsDirectory, backend.Config.ProjectID)

			list := []Path{}

			entries, err := os.ReadDir(cwd)
			if err != nil {
				fmt.Println("Error:", err)
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}
			for _, entry := range entries {
				name := entry.Name()

				list = append(list, Path{
					Name:  name,
					Path:  path.Join(cwd, name),
					IsDir: entry.IsDir(),
				})
			}

			jsonData := make(map[string]any)
			jsonData["list"] = list

			json.Marshal(jsonData)

			return c.JSON(http.StatusOK, map[string]interface{}{
				"cwd":  cwd,
				"list": list,
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})

	return nil
}

func (backend *Backend) FileWatcher(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: "POST",
		Path:   "/api/filewatcher",
		Handler: func(c echo.Context) error {

			var data map[string]interface{}
			if err := c.Bind(&data); err != nil {
				return err
			}
			filePath := data["filePath"].(string)

			fmt.Println("filePath", filePath)

			// settingsFilePath := os.Getenv("GRROXY_TEMPLATE_DIR")
			// // If GRROXY_TEMPLATE_DIR isn't configured, skip file watching instead of crashing.
			// if settingsFilePath == "" {
			// 	return c.NoContent(204)
			// }

			c.Response().Header().Set(echo.HeaderContentType, "text/event-stream")
			c.Response().Header().Set(echo.HeaderCacheControl, "no-cache")
			c.Response().Header().Set(echo.HeaderConnection, "keep-alive")

			updateChan := make(chan fsnotify.Event)

			go func() {
				watcher, err := fsnotify.NewWatcher()
				if err != nil {
					log.Fatal(err)
				}
				defer watcher.Close()

				// Create a channel to send updates
				if err := watcher.Add(filePath); err != nil {
					log.Printf("filewatcher: failed to watch %q: %v", filePath, err)
					close(updateChan)
					return
				}
				for {
					select {
					case event := <-watcher.Events:
						// if event.Op&fsnotify.Write == fsnotify.Write {
						log.Println("New File Watcher Event:", event)
						updateChan <- event
						// }
					case <-c.Request().Context().Done():
						close(updateChan)
						return
					}
				}
			}()

			for newSettings := range updateChan {
				data, err := json.Marshal(newSettings)
				if err != nil {
					log.Printf("Failed to marshal settings: %v", err)
					continue
				}
				c.Response().Write([]byte("data: " + string(data) + "\n\n"))
				c.Response().Flush()
			}

			return nil
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})

	return nil
}
