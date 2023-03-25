package endpoints

import (
	"log"
	"net/http"

	"github.com/glitchedgitz/grroxy-db/endpoints/api"
	"github.com/glitchedgitz/grroxy-db/types"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/list"
)

func (pocketbaseDB *DatabaseAPI) GetData(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: api.V1.Data.Method,
		Path:   api.V1.Data.Endpoint,
		Handler: func(c echo.Context) error {

			var data Data
			if err := c.Bind(&data); err != nil {
				return err
			}

			ids := data.Ids
			log.Println("Request: ", data)

			var result []types.UserData2

			err := pocketbaseDB.App.Dao().DB().
				Select("*").
				From("data").
				Where(dbx.In(
					"id",
					list.ToInterfaceSlice(ids)...,
				)).
				OrderBy("created desc").
				Limit(data.PerPage).Offset(data.Page * data.PerPage).
				All(&result)

			log.Println("Result: ", result)

			if err != nil {
				apis.NewBadRequestError("Failed to fetch warehouse items", err)
			}

			return c.JSON(http.StatusOK, result)
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(pocketbaseDB.App),
		},
	})

	return nil
}

type Data struct {
	Ids     []string `json:"ids"`
	Page    int64    `json:""`
	PerPage int64    `json:""`
}
