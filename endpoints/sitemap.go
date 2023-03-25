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
	"github.com/pocketbase/pocketbase/tools/list"
	pbTypes "github.com/pocketbase/pocketbase/tools/types"
)

func _getFirstFolder(path string) string {
	firstSlash := strings.Index(path, "/")
	if firstSlash != -1 {
		return path[:firstSlash]
	}
	return ""
}

func (pocketbaseDB *DatabaseAPI) SitemapRows(e *core.ServeEvent) error {
	var _api = api.V1.SitemapRows
	e.Router.AddRoute(echo.Route{
		Method: _api.Method,
		Path:   _api.Endpoint,
		Handler: func(c echo.Context) error {

			var data types.SitemapRows
			if err := c.Bind(&data); err != nil {
				return err
			}

			log.Println("[SitemapRows] data: ", data)

			db := base.ParseDatabaseName(data.Host)

			var result []types.UserData2
			var err error

			type MainIDPath struct {
				MainID string `db:"mainID"`
				Path   string `db:"path"`
			}

			var mainIDPathResults []MainIDPath
			if data.Path == "" || data.Path == "/" {
				err = pocketbaseDB.App.Dao().DB().Select("mainID", "path").From(db).All(&mainIDPathResults)
			} else {
				regexQuery := fmt.Sprintf(`^%s/([^/]+\s*)?$`, data.Path)
				err = pocketbaseDB.App.Dao().DB().Select("mainID", "path").From(db).Where(dbx.Like("path", regexQuery)).All(&mainIDPathResults)
			}

			log.Println("[SitemapRows] mainIDPathResults: ", mainIDPathResults)
			if err != nil {
				apis.NewBadRequestError("Failed to fetch warehouse items", err)
			}

			uniqueFolders := make(map[string]bool)
			folders := []string{}
			mainIDs := []string{}

			for _, result := range mainIDPathResults {
				folder := _getFirstFolder(result.Path)
				mainIDs = append(mainIDs, result.MainID)
				if _, ok := uniqueFolders[folder]; ok {
					continue
				}
				uniqueFolders[folder] = true
				folders = append(folders, folder)
			}

			log.Println("[SitemapRows] folders: ", folders)
			log.Println("[SitemapRows] mainIDs: ", mainIDs)

			err = pocketbaseDB.App.Dao().DB().
				Select("*").
				From("data").
				Where(dbx.In(
					"id",
					list.ToInterfaceSlice(mainIDs)...,
				)).
				OrderBy("created desc").
				Limit(data.PerPage).
				Offset((data.Page - 1) * data.PerPage).
				All(&result)

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

			var result []types.SitemapFetchResponse

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
			// This is the error "constraint failed: UNIQUE constraint failed: collections.name (2067)"

			if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
				log.Println("collection already exists: ", collection)
			}

			if data.Query != "" {
				data.Query = "?" + data.Query
			}
			if data.Fragment != "" {
				data.Fragment = "#" + data.Fragment
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
