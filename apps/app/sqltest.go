package app

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/glitchedgitz/pocketbase/apis"
	"github.com/glitchedgitz/pocketbase/core"
	"github.com/glitchedgitz/pocketbase/models"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
)

type TEXTSQL struct {
	SQL string `json:"sql"`
}
type CountResult struct {
	CountOfRows         int `db:"CountOfRows" json:"CountOfRows"`
	CountOfDistinctRows int `db:"CountOfDistinctRows" json:"CountOfDistinctRows"`
}

func (backend *Backend) TextSQL(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: "POST",
		Path:   "/api/sqltest",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil

			if isGuest {
				return c.String(http.StatusForbidden, "")
			}
			var data TEXTSQL
			if err := c.Bind(&data); err != nil {
				return err
			}

			var results sql.Result

			query := backend.App.Dao().DB().NewQuery(data.SQL)
			log.Println("[TextSQL] ", results)

			rows, err := query.Rows()
			if err != nil {
				return apis.NewBadRequestError("Failed", err)
			}

			row := dbx.NullStringMap{}

			resultStr := ""
			for rows.Next() {
				_ = rows.ScanMap(row)
				// log.Println("Scanned SQL:, ", row)
				jsonStr, _ := json.Marshal(row)
				resultStr = resultStr + string(jsonStr) + "\n"
			}

			return c.JSON(http.StatusOK, resultStr)
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})

	return nil
}
