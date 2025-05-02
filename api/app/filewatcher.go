package api

import (
	"encoding/json"
	"log"
	"os"

	"github.com/fsnotify/fsnotify"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func (backend *Backend) FileWatcher(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: "GET",
		Path:   "/api/filewatcher",
		Handler: func(c echo.Context) error {

			c.Response().Header().Set(echo.HeaderContentType, "text/event-stream")
			c.Response().Header().Set(echo.HeaderCacheControl, "no-cache")
			c.Response().Header().Set(echo.HeaderConnection, "keep-alive")

			updateChan := make(chan fsnotify.Event)
			
			if os.Getenv("GRROXY_TEMPLATE_DIR") == "" {
				panic("GRROXY_TEMPLATE_DIR environment variable is not set")
			}
			settingsFilePath := os.Getenv("GRROXY_TEMPLATE_DIR")

			go func() {
				watcher, err := fsnotify.NewWatcher()
				if err != nil {
					log.Fatal(err)
				}
				defer watcher.Close()

				// Create a channel to send updates
				watcher.Add(settingsFilePath)
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
