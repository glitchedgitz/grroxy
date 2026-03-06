package launcher

import (
	"encoding/json"
	"net/http"
	"regexp"

	"github.com/glitchedgitz/pocketbase/apis"
	"github.com/glitchedgitz/pocketbase/core"
	"github.com/labstack/echo/v5"
)

func (launcher *Launcher) SearchRegex(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: "POST",
		Path:   "/api/regex",
		Handler: func(c echo.Context) error {

			var data map[string]interface{}
			if err := c.Bind(&data); err != nil {
				return err
			}

			// fmt.Println("regex data:", data)

			regex := data["regex"].(string)
			responseBody := data["responseBody"].(string)

			jsonData := make(map[string]any)

			matched, err := regexp.MatchString(regex, responseBody)
			if err != nil {
				jsonData["error"] = err.Error()
				json.Marshal(jsonData)
				return c.JSON(http.StatusOK, jsonData)
			}

			jsonData["matched"] = matched
			json.Marshal(jsonData)
			return c.JSON(http.StatusOK, jsonData)

		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(launcher.App),
		},
	})

	return nil
}
