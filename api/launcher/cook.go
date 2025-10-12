package launcher

import (
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
)

func (launcher *Launcher) CookSearch(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: "POST",
		Path:   "/api/cook/search",
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

			search := data["search"].(string)
			results, found := launcher.Cook.Search(search)

			jsonData := make(map[string]any)
			jsonData["search"] = search
			jsonData["results"] = results

			if found {
				json.Marshal(jsonData)
				return c.JSON(http.StatusOK, jsonData)
			} else {
				return c.String(http.StatusNotFound, "")
			}

		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(launcher.App),
		},
	})

	return nil
}
