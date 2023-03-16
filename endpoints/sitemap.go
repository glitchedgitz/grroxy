package endpoints

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/glitchedgitz/grroxy-db/base"
	"github.com/glitchedgitz/grroxy-db/endpoints/api"
	"github.com/glitchedgitz/grroxy-db/types"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models/schema"
	pbTypes "github.com/pocketbase/pocketbase/tools/types"
)

func (pocketbaseDB *DatabaseAPI) SitemapFetch(e *core.ServeEvent) error {
	var _api = api.V1.SitemapFetch
	e.Router.AddRoute(echo.Route{
		Method: _api.Method,
		Path:   _api.Endpoint,
		Handler: func(c echo.Context) error {

			var data types.SitemapFetch
			if err := c.Bind(&data); err != nil {
				return err
			}

			db := base.ParseDatabaseName(data.Host)

			// Regex: '^path/([^/]+\s*)?$'
			regexQuery := fmt.Sprintf(`^%s/([^/]+\s*)?$`, data.Path)

			var result []types.SitemapFetch

			var err error

			if data.Path == "" {
				err = pocketbaseDB.App.Dao().DB().
					Select("*").
					From(db).
					All(&result)
			} else {
				err = pocketbaseDB.App.Dao().DB().Select("*").
					From(db).
					Where(dbx.Like("path", regexQuery)).
					All(&result)
			}

			log.Println("[SitemapFetch] Request: ", data)
			log.Println("[SitemapFetch] Response: ", result)

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

func (pocketbaseDB *DatabaseAPI) SitemapNew(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   api.V1.SitemapNew.Endpoint,
		Handler: func(c echo.Context) error {

			var data types.SitemapGet
			if err := c.Bind(&data); err != nil {
				return err
			}

			fmt.Print("SitemapNew: ", data)
			collection := base.ParseDatabaseName(data.Host)

			err := pocketbaseDB.CreateCollection(collection, schema.NewSchema(
				&schema.SchemaField{
					Name:     "path",
					Type:     schema.FieldTypeText,
					Required: true,
				}, &schema.SchemaField{
					Name:     "type",
					Type:     schema.FieldTypeText,
					Required: true,
				},
				&schema.SchemaField{
					Name:     "mainID",
					Type:     schema.FieldTypeText,
					Required: true,
					Options: &schema.RelationOptions{
						MaxSelect:     pbTypes.Pointer(1),
						CollectionId:  "ae40239d2bc4477",
						CascadeDelete: true,
					},
				},
			))

			// Checking error if it is collection already exists
			// This is the error "constraint failed: UNIQUE constraint failed: _collections.name (2067)"

			if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
				log.Println("collection already exists: ", collection)
			}

			// Inserting data
			result, err := pocketbaseDB.App.Dao().DB().Insert(collection, dbx.Params{
				"id":     data.MainID,
				"path":   data.Path + data.Query + data.Fragment,
				"type":   data.Type,
				"mainID": data.MainID,
			}).Execute()

			log.Println("Executed: ", result)

			if err != nil {
				// return nil
				fmt.Println("Error: ", err)
				// apis.NewBadRequestError("Failed to create collection", err)
			}

			return c.String(http.StatusOK, "Created")
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(pocketbaseDB.App),
		},
	})
	return nil
}
