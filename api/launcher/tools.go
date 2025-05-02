package launcher

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

type ToolsServerResponse struct {
	Path        string `db:"path" json:"path"`
	HostAddress string `db:"hostAddress" json:"hostAddress"`
	ID          string `db:"id" json:"id"`
	Name        string `db:"name" json:"name"`
	Username    string `db:"username" json:"username"`
	Password    string `db:"password" json:"password"`
}

func (launcher *Launcher) ToolsServer(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: "GET",
		Path:   "/api/tool/server",
		Handler: func(c echo.Context) error {
			path := launcher.Config.ConfigDirectory
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
			id := launcher.RegisterProcessInDB(_c, nil,
				"grroxy-tool", "tool-server", schemas.ProcessState.Inqueue)

			go launcher.toolsServerStart(hostAddress, path, name, func() {
				fmt.Println("toolsServerStart closed")
			})

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
