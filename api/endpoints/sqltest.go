package endpoints

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

type TEXTSQL struct {
	SQL string `json:"sql"`
}
type CountResult struct {
	CountOfRows         int `db:"CountOfRows" json:"CountOfRows"`
	CountOfDistinctRows int `db:"CountOfDistinctRows" json:"CountOfDistinctRows"`
}

func (pocketbaseDB *DatabaseAPI) TextSQL(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: "POST",
		Path:   "api/sqltest",
		Handler: func(c echo.Context) error {

			var data TEXTSQL
			if err := c.Bind(&data); err != nil {
				return err
			}

			var s string
			// var result CountResult

			// err := pocketbaseDB.App.Dao().DB().NewQuery(data.SQL).One(&result)
			// err2 := pocketbaseDB.App.Dao().DB().Select("count(*)").From("sites").Row(&t2)
			err := pocketbaseDB.App.Dao().DB().NewQuery(data.SQL).Row(&s)
			// err2 := pocketbaseDB.App.Dao().DB().NewQuery("Select count(*) from data").All(&t2)

			// fmt.Println("t1", t1)
			// fmt.Println("t2", t2)

			if err != nil {
				apis.NewBadRequestError("Failed to fetch warehouse items", err)
			}

			return c.JSON(http.StatusOK, s)
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(pocketbaseDB.App),
		},
	})

	return nil
}
