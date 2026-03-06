package app

import (
	"net/http"
	"strings"

	"github.com/glitchedgitz/dadql/dadql"
	"github.com/glitchedgitz/pocketbase/apis"
	"github.com/glitchedgitz/pocketbase/core"
	"github.com/glitchedgitz/pocketbase/models"
	"github.com/labstack/echo/v5"
)

type FilterCheckRequest struct {
	Filter  string         `json:"filter"`
	Columns map[string]any `json:"columns"`
}

// FiltersCheck registers the /api/filter/check endpoint.
// It evaluates the provided dadql filter against the given columns map.
func (backend *Backend) FiltersCheck(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/filter/check",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil
			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			var req FilterCheckRequest
			if err := c.Bind(&req); err != nil {
				return err
			}

			req.Filter = strings.TrimSpace(req.Filter)
			if req.Filter == "" {
				return c.JSON(http.StatusBadRequest, map[string]any{
					"error": "filter is required",
				})
			}

			ok, err := dadql.Filter(req.Columns, req.Filter)
			if err != nil {
				return c.JSON(http.StatusOK, map[string]any{
					"ok":    false,
					"error": err.Error(),
				})
			}

			return c.JSON(http.StatusOK, map[string]any{
				"ok":    true,
				"match": ok,
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})

	return nil
}
