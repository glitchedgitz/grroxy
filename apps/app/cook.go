package app

import (
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
)

func (backend *Backend) CookGenerate(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/cook/generate",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil

			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			type Data struct {
				Pattern []string `json:"pattern"`
			}

			var data Data
			if err := c.Bind(&data); err != nil {
				return err
			}

			results := backend.Cook.Generate(data.Pattern)

			jsonData := make(map[string]any)
			jsonData["results"] = results

			return c.JSON(http.StatusOK, jsonData)
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})

	return nil
}

func (backend *Backend) CookApplyMethods(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/cook/apply",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil

			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			type Data struct {
				Strings []string `json:"strings"`
				Methods []string `json:"methods"`
			}

			var data Data
			if err := c.Bind(&data); err != nil {
				return err
			}

			results, err := backend.Cook.ApplyMethods(data.Strings, data.Methods)
			if err != nil {
				return c.String(http.StatusOK, err.Error())
			}
			return c.JSON(http.StatusOK, map[string]any{"results": results})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

func (backend *Backend) CookSearch(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
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
			results, found := backend.Cook.Search(search)

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
			apis.ActivityLogger(backend.App),
		},
	})

	return nil
}
