package app

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"time"

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

func (backend *Backend) CWDBrowse(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: "POST",
		Path:   "/api/cwd/browse",
		Handler: func(c echo.Context) error {
			var data map[string]interface{}
			if err := c.Bind(&data); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
			}

			dirPath, _ := data["path"].(string)
			if dirPath == "" {
				dirPath = path.Join(backend.Config.ProjectsDirectory, backend.Config.ProjectID)
			}

			info, err := os.Stat(dirPath)
			if err != nil {
				return c.JSON(http.StatusNotFound, map[string]interface{}{"error": fmt.Sprintf("path not found: %s", dirPath)})
			}
			if !info.IsDir() {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "path is not a directory"})
			}

			entries, err := os.ReadDir(dirPath)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			list := make([]map[string]interface{}, 0, len(entries))
			for _, entry := range entries {
				name := entry.Name()
				entryInfo, _ := entry.Info()
				size := int64(0)
				if entryInfo != nil {
					size = entryInfo.Size()
				}
				list = append(list, map[string]interface{}{
					"name":  name,
					"path":  path.Join(dirPath, name),
					"is_dir": entry.IsDir(),
					"size":  size,
				})
			}

			return c.JSON(http.StatusOK, map[string]interface{}{
				"path": dirPath,
				"list": list,
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})

	return nil
}

func (backend *Backend) CWDReadFile(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: "POST",
		Path:   "/api/cwd/readfile",
		Handler: func(c echo.Context) error {
			var data map[string]interface{}
			if err := c.Bind(&data); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
			}

			filePath, _ := data["path"].(string)
			if filePath == "" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "path is required"})
			}

			info, err := os.Stat(filePath)
			if err != nil {
				return c.JSON(http.StatusNotFound, map[string]interface{}{"error": fmt.Sprintf("file not found: %s", filePath)})
			}
			if info.IsDir() {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "path is a directory, use /api/cwd/browse instead"})
			}

			// Limit file size to 10MB for preview
			if info.Size() > 10*1024*1024 {
				return c.JSON(http.StatusOK, map[string]interface{}{
					"path":    filePath,
					"name":    path.Base(filePath),
					"size":    info.Size(),
					"content": "",
					"binary":  true,
					"message": "File too large to preview (>10MB)",
				})
			}

			content, err := os.ReadFile(filePath)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			// Check if binary by looking for null bytes in first 512 bytes
			isBinary := false
			checkLen := len(content)
			if checkLen > 512 {
				checkLen = 512
			}
			for i := 0; i < checkLen; i++ {
				if content[i] == 0 {
					isBinary = true
					break
				}
			}

			if isBinary {
				return c.JSON(http.StatusOK, map[string]interface{}{
					"path":    filePath,
					"name":    path.Base(filePath),
					"size":    info.Size(),
					"content": "",
					"binary":  true,
					"message": "Binary file cannot be previewed",
				})
			}

			return c.JSON(http.StatusOK, map[string]interface{}{
				"path":    filePath,
				"name":    path.Base(filePath),
				"size":    info.Size(),
				"content": string(content),
				"binary":  false,
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})

	return nil
}

type fileWatcherState struct {
	watcher    *fsnotify.Watcher
	updateChan chan fsnotify.Event
	dirsChan   chan []string
}

func (backend *Backend) FileWatcher(e *core.ServeEvent) error {

	// SSE stream endpoint — watches root + expanded dirs with debouncing
	e.Router.AddRoute(echo.Route{
		Method: "POST",
		Path:   "/api/filewatcher",
		Handler: func(c echo.Context) error {
			var data map[string]interface{}
			if err := c.Bind(&data); err != nil {
				return err
			}
			filePath, _ := data["filePath"].(string)
			if filePath == "" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "filePath is required"})
			}

			c.Response().Header().Set(echo.HeaderContentType, "text/event-stream")
			c.Response().Header().Set(echo.HeaderCacheControl, "no-cache")
			c.Response().Header().Set(echo.HeaderConnection, "keep-alive")

			watcher, err := fsnotify.NewWatcher()
			if err != nil {
				return fmt.Errorf("filewatcher: failed to create watcher: %w", err)
			}

			if err := watcher.Add(filePath); err != nil {
				watcher.Close()
				log.Printf("filewatcher: failed to watch %q: %v", filePath, err)
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			ws := &fileWatcherState{
				watcher:    watcher,
				updateChan: make(chan fsnotify.Event, 100),
				dirsChan:   make(chan []string, 10),
			}

			// Store watcher state so /api/filewatcher/dirs can update it
			backend.mu.Lock()
			backend.fileWatcher = ws
			backend.mu.Unlock()

			ctx := c.Request().Context()

			// Goroutine: read fsnotify events and forward to updateChan
			go func() {
				defer watcher.Close()
				for {
					select {
					case event, ok := <-watcher.Events:
						if !ok {
							return
						}
						ws.updateChan <- event
					case err, ok := <-watcher.Errors:
						if !ok {
							return
						}
						log.Printf("filewatcher error: %v", err)
					case <-ctx.Done():
						return
					}
				}
			}()

			// Goroutine: handle dir updates from /api/filewatcher/dirs
			go func() {
				for {
					select {
					case dirs, ok := <-ws.dirsChan:
						if !ok {
							return
						}
						// Get current watched dirs
						currentDirs := make(map[string]bool)
						for _, d := range ws.watcher.WatchList() {
							currentDirs[d] = true
						}

						// Desired dirs (root + expanded)
						desiredDirs := make(map[string]bool)
						desiredDirs[filePath] = true
						for _, d := range dirs {
							desiredDirs[d] = true
						}

						// Add new dirs
						for d := range desiredDirs {
							if !currentDirs[d] {
								if err := ws.watcher.Add(d); err != nil {
									log.Printf("filewatcher: failed to add %q: %v", d, err)
								}
							}
						}

						// Remove old dirs (except root)
						for d := range currentDirs {
							if !desiredDirs[d] {
								ws.watcher.Remove(d)
							}
						}
					case <-ctx.Done():
						return
					}
				}
			}()

			// Main loop: debounce events and send SSE
			var debounceTimer *time.Timer
			for {
				select {
				case event, ok := <-ws.updateChan:
					if !ok {
						return nil
					}
					// Skip events for hidden dirs (.git, node_modules, etc.)
					baseName := path.Base(event.Name)
					if baseName == ".git" || baseName == "node_modules" || baseName == ".svelte-kit" || baseName == ".DS_Store" || baseName == "__pycache__" {
						continue
					}

					// Debounce: reset timer on each event, fire after 200ms of quiet
					if debounceTimer != nil {
						debounceTimer.Stop()
					}
					debounceTimer = time.AfterFunc(200*time.Millisecond, func() {
						eventData, err := json.Marshal(event)
						if err != nil {
							log.Printf("Failed to marshal event: %v", err)
							return
						}
						c.Response().Write([]byte("data: " + string(eventData) + "\n\n"))
						c.Response().Flush()
					})

				case <-ctx.Done():
					if debounceTimer != nil {
						debounceTimer.Stop()
					}
					backend.mu.Lock()
					backend.fileWatcher = nil
					backend.mu.Unlock()
					return nil
				}
			}
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})

	// Update watched directories endpoint
	e.Router.AddRoute(echo.Route{
		Method: "POST",
		Path:   "/api/filewatcher/dirs",
		Handler: func(c echo.Context) error {
			var data struct {
				Dirs []string `json:"dirs"`
			}
			if err := c.Bind(&data); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
			}

			backend.mu.Lock()
			ws := backend.fileWatcher
			backend.mu.Unlock()

			if ws == nil {
				return c.JSON(http.StatusOK, map[string]interface{}{"status": "no active watcher"})
			}

			ws.dirsChan <- data.Dirs

			return c.JSON(http.StatusOK, map[string]interface{}{"status": "ok", "watching": len(data.Dirs)})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})

	return nil
}
