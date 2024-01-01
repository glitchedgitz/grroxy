package endpoints

import (
	"log"
	"net/http"
	"sort"
	"strings"

	"github.com/glitchedgitz/grroxy-db/base"
	"github.com/glitchedgitz/grroxy-db/schemas"
	"github.com/glitchedgitz/grroxy-db/types"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func (pocketbaseDB *DatabaseAPI) SitemapNew(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/sitemap/new",
		Handler: func(c echo.Context) error {

			var data types.SitemapGet
			if err := c.Bind(&data); err != nil {
				return err
			}

			log.Print("SitemapNew: ", data)
			collection := base.ParseDatabaseName(data.Host)

			err := pocketbaseDB.CreateCollection(collection, schemas.Sitemap)

			// Checking error if it is collection already exists
			// This is the error "constraint failed: UNIQUE constraint failed: collections.name (2067)"

			if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
				log.Println("collection already exists: ", collection)
			}

			// Inserting data
			result, err := pocketbaseDB.App.Dao().DB().Insert(collection, dbx.Params{
				"id":       data.Data,
				"path":     data.Path,
				"query":    data.Query,
				"fragment": data.Fragment,
				"type":     data.Type,
				"ext":      data.Ext,
				"data":     data.Data,
			}).Execute()

			log.Println("Executed: ", result)

			if err != nil {
				// return nil
				log.Println("Error: ", err)
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

func (pocketbaseDB *DatabaseAPI) SitemapFetch(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/sitemap/fetch",
		Handler: func(c echo.Context) error {

			var data types.SitemapFetch
			if err := c.Bind(&data); err != nil {
				return err
			}

			db := base.ParseDatabaseName(data.Host)

			// Regex: '^path/([^/]+\s*)?$'
			// regexQuery := fmt.Sprintf(`^%s/([^/]+\s*)?$`, data.Path)

			// Simplier for noeWHERE path LIKE '/s/%'
			regexQuery := data.Path + `/%`

			var result []types.SitemapGet
			// var tmpResult []map[string]interface{}
			uniqueMap := make(map[string]map[string]interface{})
			var titles []string
			var err error

			if data.Path == "" {
				err = pocketbaseDB.App.Dao().DB().NewQuery("SELECT * FROM " + db).All(&result)
			} else {
				err = pocketbaseDB.App.Dao().DB().NewQuery("SELECT * FROM " + db + " WHERE path LIKE '" + regexQuery + "'").All(&result)
			}

			for _, item := range result {
				tmpPath := strings.TrimPrefix(item.Path, data.Path)
				tmpPath = strings.TrimPrefix(tmpPath, "/")

				var part string
				if index := strings.IndexAny(tmpPath, "?#"); index != -1 {
					part = tmpPath[:index]
				} else {
					part = tmpPath
				}

				title := strings.Split(part, "/")[0]

				if _, exists := uniqueMap[title]; !exists {
					uniqueMap[title] = map[string]interface{}{
						"host":  data.Host,
						"path":  data.Path + "/" + title,
						"type":  item.Type,
						"title": title,
						"ext":   item.Ext,
						"query": item.Query,
					}
					titles = append(titles, title)
				}
			}

			sort.Strings(titles)
			var tmpResult2 []map[string]interface{}
			for _, title := range titles {
				tmpResult2 = append(tmpResult2, uniqueMap[title])
			}
			log.Println("[SitemapFetch] Request: ", data)
			log.Println("[SitemapFetch] Response: ", tmpResult2)

			if err != nil {
				apis.NewBadRequestError("Failed to fetch warehouse items", err)
			}

			return c.JSON(http.StatusOK, tmpResult2)
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(pocketbaseDB.App),
		},
	})

	return nil
}
