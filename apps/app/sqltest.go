package app

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/glitchedgitz/pocketbase/apis"
	"github.com/glitchedgitz/pocketbase/core"
	"github.com/glitchedgitz/pocketbase/models"
	"github.com/labstack/echo/v5"
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

			log.Println("[TextSQL] ", data.SQL)

			rows, err := backend.App.Dao().DB().NewQuery(data.SQL).Rows()
			if err != nil {
				return apis.NewBadRequestError("Failed", err)
			}
			defer rows.Close()

			result, err := sqlRowsToJSON(rows.Rows)
			if err != nil {
				return apis.NewBadRequestError("Failed", err)
			}

			// the test page puts the response straight into an editor, so it
			// stays a string
			return c.JSON(http.StatusOK, result)
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})

	return nil
}

// sqlRowsToJSON renders rows as an indented JSON array, one object per row with
// the columns in query order. Values keep the type SQLite stored them with:
// INTEGER and REAL become numbers, NULL becomes null, TEXT and BLOB strings.
func sqlRowsToJSON(rows *sql.Rows) (string, error) {
	columns, err := rows.Columns()
	if err != nil {
		return "", err
	}

	values := make([]any, len(columns))
	pointers := make([]any, len(columns))
	for i := range values {
		pointers[i] = &values[i]
	}

	var out bytes.Buffer
	out.WriteString("[")

	count := 0
	for rows.Next() {
		if err := rows.Scan(pointers...); err != nil {
			return "", err
		}

		if count > 0 {
			out.WriteString(",")
		}
		out.WriteString("\n  {")

		for i, column := range columns {
			value := values[i]
			if b, ok := value.([]byte); ok {
				value = string(b)
			}

			key, _ := json.Marshal(column)
			val, err := json.Marshal(value)
			if err != nil {
				return "", err
			}

			if i > 0 {
				out.WriteString(",")
			}
			out.WriteString("\n    ")
			out.Write(key)
			out.WriteString(": ")
			out.Write(val)
		}

		out.WriteString("\n  }")
		count++
	}

	if err := rows.Err(); err != nil {
		return "", err
	}

	if count > 0 {
		out.WriteString("\n")
	}
	out.WriteString("]")

	return out.String(), nil
}
