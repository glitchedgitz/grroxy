package api

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"

	"github.com/glitchedgitz/grroxy-db/schemas"
	"github.com/glitchedgitz/grroxy-db/utils"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/rs/xid"
)

// func (backend *Backend) ToolsManager() {

// 		var grroxydb = sdk.NewClient(
// 		"http://"+options.AppAddress,
// 		sdk.WithAdminEmailPassword("new@example.com", "1234567890"))

// 	stream, err := sdk.CollectionSet[any](launcher.toolsSdks[0], "_process").Subscribe("_process")

// 	log.Print("Subscribed to setting")
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	<-stream.Ready()
// 	defer stream.Unsubscribe()

// 	for ev := range stream.Events() {
// 		log.Print("[Main][InterceptManager]: ", ev.Action, ev.Record)

// 		// extract the value field from ev.Record using type assertion
// 		value, ok := ev.Record.(map[string]interface{})["value"].(string)
// 		if !ok {
// 			log.Print("invalid value field type")
// 			continue
// 		}

// 		if value == "false" {
// 			backend.Intercept = false
// 			collection := sdk.CollectionSet[types.RealtimeRecord](backend.grroxydb, "_intercept")
// 			response, err := collection.List(types.ParamsList{
// 				Page: 1, Size: 1000, Sort: "created",
// 			})

// 			if err != nil {
// 				log.Fatal(err)
// 			}

// 			var wg sync.WaitGroup

// 			wg.Add(len(response.Items))

// 			// update each record action to forward
// 			for _, record := range response.Items {
// 				go func(r types.RealtimeRecord) {
// 					r.Action = "forward"
// 					p.grroxydb.Update("_intercept", r.ID, r)
// 					wg.Done()
// 				}(record)
// 			}
// 			wg.Wait()
// 		} else {
// 			p.options.Intercept = true
// 		}
// 	}
// }

type ToolsServerResponse struct {
	Path        string `db:"path" json:"path"`
	HostAddress string `db:"hostAddress" json:"hostAddress"`
	ID          string `db:"id" json:"id"`
	Name        string `db:"name" json:"name"`
	Username    string `db:"username" json:"username"`
	Password    string `db:"password" json:"password"`
}

type login struct {
	Host     string `json:"host"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (backend *Backend) ToolsServer(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: "GET",
		Path:   "/api/tool/server",
		Handler: func(c echo.Context) error {
			path := backend.Config.ConfigDirectory
			hostAddress, err := utils.CheckAndFindAvailablePort("127.0.0.1:8090")
			name := xid.New().String()

			fmt.Println("name", name)
			fmt.Println("path", path)
			fmt.Println("hostAddress", hostAddress)
			fmt.Println("err", err)

			if err != nil {
				return c.String(http.StatusInternalServerError, err.Error())
			}

			_c := "grroxy-tool -path " + path + " -host " + hostAddress + " -name " + name

			id := backend.RegisterProcessInDB(
				_c,
				map[string]any{
					"path":        path,
					"hostAddress": hostAddress,
					"name":        name,
					"username":    "new@example.com",
					"password":    "1234567890",
				},
				"grroxy-tool",
				"tool-server",
				schemas.ProcessState.Inqueue,
			)

			go backend.toolsServerStart(hostAddress, path, name, func() {
				fmt.Println("toolsServerStart closed")
			})

			// backend.ToolLoginAndSubscribe(id, login{
			// 	Host:     hostAddress,
			// 	Username: "new@example.com",
			// 	Password: "1234567890",
			// })

			return c.JSON(http.StatusOK, ToolsServerResponse{
				Path:        path,
				HostAddress: hostAddress,
				ID:          id,
				Name:        name,
				Username:    "new@example.com",
				Password:    "1234567890",
			})
		},
	})
	return nil
}

func (backend *Backend) toolsServerStart(hostAddress, path, name string, onClose func()) {
	cmd := exec.Command("grroxy-tool", "-path", path, "-host", hostAddress, "-name", name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error executing grroxy command: %v\n", err)
		return
	}

	onClose()
}

func (backend *Backend) Tools(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: "GET",
		Path:   "/api/tool",
		Handler: func(c echo.Context) error {
			path := c.QueryParam("path")
			// backend.App.Bootstrap()
			hostAddress, err := utils.CheckAndFindAvailablePort("127.0.0.1:8090")

			fmt.Println("path", path)
			fmt.Println("hostAddress", hostAddress)
			fmt.Println("err", err)

			var NewApp = pocketbase.NewWithConfig(
				pocketbase.Config{
					ProjectDir:      path,
					DefaultDataDir:  "weird",
					HideStartBanner: true,
					// DefaultDev: true,
					// DefaultEncryptionEnv: "hJH#GRJ#HG$JH$54h5kjhHJG#JHG#*&Y&EG#F&GIG@JKGH$JHRGJ##JKJH#JHG",
				},
			)

			NewApp.Bootstrap()

			if err != nil {
				return c.String(http.StatusInternalServerError, err.Error())
			}

			_, err = apis.Serve(NewApp, apis.ServeConfig{
				HttpAddr: hostAddress,
			})

			fmt.Println("err", err)

			if err != nil {
				return c.String(http.StatusInternalServerError, err.Error())
			}

			return c.String(http.StatusOK, fmt.Sprintf("Path parameter: %s", path))
		},
	})
	return nil
}
