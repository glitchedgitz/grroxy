package launcher

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"

	"github.com/glitchedgitz/grroxy/internal/schemas"
	"github.com/glitchedgitz/grroxy/internal/utils"
	"github.com/glitchedgitz/pocketbase"
	"github.com/glitchedgitz/pocketbase/apis"
	"github.com/glitchedgitz/pocketbase/core"
	"github.com/glitchedgitz/pocketbase/models"
	"github.com/labstack/echo/v5"
	"github.com/rs/xid"
)

type ToolsServerResponse struct {
	Path     string `db:"path" json:"path"`
	Host     string `db:"host" json:"host"`
	ID       string `db:"id" json:"id"`
	Name     string `db:"name" json:"name"`
	Username string `db:"username" json:"username"`
	Password string `db:"password" json:"password"`
}

func (launcher *Launcher) GetToolById(id string) (*models.Record, error) {
	record, err := launcher.App.Dao().FindRecordById("_tools", id)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (launcher *Launcher) SetToolData(id, host, state string) (*models.Record, error) {
	record, err := launcher.App.Dao().FindRecordById("_tools", id)
	if err != nil {
		return nil, err
	}
	record.Set("host", host)
	record.Set("state", state)
	if err := launcher.App.Dao().SaveRecord(record); err != nil {
		return nil, err
	}
	return record, nil
}

func (launcher *Launcher) NewTool(data map[string]any) ([]*models.Record, error) {
	collection, err := launcher.App.Dao().FindCollectionByNameOrId("_tools")
	if err != nil {
		return nil, err
	}

	record := models.NewRecord(collection)
	record.Load(data)

	if err := launcher.App.Dao().SaveRecord(record); err != nil {
		return nil, err
	}

	return []*models.Record{record}, nil
}

func (launcher *Launcher) ToolsServer(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: "GET",
		Path:   "/api/tool/server",
		Handler: func(c echo.Context) error {

			var path string
			var hostAddress string
			var name string
			var active bool = false

			var err error

			var toolId string = ""
			var body = make(map[string]any)
			if c.QueryParam("id") != "" {
				body["id"] = c.QueryParam("id")
			} else if err := c.Bind(&body); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			if id_val, ok := body["id"]; ok {
				toolId = id_val.(string)
			}

			if toolId != "" {
				tool, err := launcher.GetToolById(toolId)
				if err != nil {
					return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
				}

				path = tool.Get("path").(string)
				state := tool.Get("state").(string)
				name = tool.Get("name").(string)

				if state == "active" {
					active = true
					hostAddress = tool.Get("host").(string)
				} else {
					active = false
					hostAddress, err = utils.CheckAndFindAvailablePort("127.0.0.1:9000")
					if err != nil {
						return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
					}
				}
			} else {
				path = launcher.Config.ProjectsDirectory
				hostAddress, err = utils.CheckAndFindAvailablePort("127.0.0.1:9000")
				name = xid.New().String()
				tool, err := launcher.NewTool(map[string]any{
					"name": name,
					"path": path,
					"host": hostAddress,
					"creds": map[string]any{
						"username": "new@example.com",
						"password": "1234567890",
					},
				})
				if err != nil {
					return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": "Fail to start new tool"})
				}
				toolId = tool[0].Id
			}

			fmt.Println("name", name)
			fmt.Println("path", path)
			fmt.Println("host", hostAddress)
			fmt.Println("err", err)

			if err != nil {
				return c.String(http.StatusInternalServerError, err.Error())
			}

			_c := "grroxy-tool -path " + path + " -host " + hostAddress + " -name " + name
			launcher.RegisterProcessInDB(
				_c,
				map[string]any{
					"path":     path,
					"host":     hostAddress,
					"name":     name,
					"username": "new@example.com",
					"password": "1234567890",
				},
				"grroxy-tool",
				"tool-server",
				schemas.ProcessState.Inqueue,
			)

			if !active {
				go launcher.toolsServerStart(hostAddress, path, name, func() {
					fmt.Println("toolsServerStart closed")

					launcher.SetToolData(toolId, "", "closed")

				})
			}

			launcher.SetToolData(toolId, hostAddress, "active")

			return c.JSON(http.StatusOK, ToolsServerResponse{
				Path:     path,
				Host:     hostAddress,
				ID:       toolId,
				Name:     name,
				Username: "new@example.com",
				Password: "1234567890",
			})
		},
	})
	return nil
}

func (launcher *Launcher) toolsServerStart(hostAddress, path, name string, onClose func()) {

	cmd := exec.Command("grroxy-tool", "-path", path, "-host", hostAddress, "-name", name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error executing grroxy command: %v\n", err)
		return
	}

	onClose()
}

func (launcher *Launcher) Tools(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: "GET",
		Path:   "/api/tool",
		Handler: func(c echo.Context) error {
			path := c.QueryParam("path")
			// launcher.App.Bootstrap()
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
