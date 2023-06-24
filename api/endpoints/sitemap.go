package endpoints

import (
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"

	"github.com/glitchedgitz/grroxy-db/api"
	"github.com/glitchedgitz/grroxy-db/base"
	"github.com/glitchedgitz/grroxy-db/schemas"
	"github.com/glitchedgitz/grroxy-db/types"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/list"
)

func _getFirstFolder(path string) string {
	firstSlash := strings.Index(path, "/")
	if firstSlash != -1 {
		return path[:firstSlash]
	}
	return ""
}

type UserData struct {
	ID               string `db:"id" json:"id"`
	Host             string `db:"host" json:"host"`
	IP               string `db:"ip" json:"ip"`
	Port             string `db:"port" json:"port"`
	UrlData          string `db:"url_data" json:"url_data"`
	OriginalRequest  string `db:"original_request" json:"original_request"`
	OriginalResponse string `db:"original_response" json:"original_response"`
	HasResponse      bool   `db:"has_response" json:"has_response"`
	IsRequestEdited  bool   `db:"is_request_edited" json:"is_request_edited"`
	IsResponseEdited bool   `db:"is_response_edited" json:"is_response_edited"`
	EditedRequest    string `db:"edited_request" json:"edited_request"`
	EditedResponse   string `db:"edited_response" json:"edited_response"`
	// Labels           []string     `db:"labels" json:"labels"`
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

			var err error

			type MainIDPath struct {
				MainID string `db:"main_id"`
				Path   string `db:"path"`
			}

			var mainIDPathResults []MainIDPath
			if data.Path == "" || data.Path == "/" {
				err = pocketbaseDB.App.Dao().DB().Select("main_id", "path").From(db).All(&mainIDPathResults)
			} else {
				regexQuery := data.Path + `/%`

				err = pocketbaseDB.App.Dao().DB().NewQuery("SELECT main_id,path FROM " + db + " WHERE path LIKE '" + regexQuery + "'").All(&mainIDPathResults)
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

			// var tmpResults []UserData
			var results []types.UserData

			// tmp:= pocketbaseDB.App.Dao().DB().NewQuery().Execute()
			err = pocketbaseDB.App.Dao().DB().
				Select("*").
				From("data").
				Where(dbx.In(
					"id",
					list.ToInterfaceSlice(mainIDs)...,
				)).
				// OrderBy("created desc").
				// Limit(data.PerPage).
				// Offset((data.Page - 1) * data.PerPage).
				All(&results)

			// for i, result := range tmpResults {
			// 	if err := json.Unmarshal(result.Userdata, &result.OriginalRequest); err != nil {
			// 		// handle error
			// 	}
			// }

			log.Println("[SitemapRows] Request: ", data)
			log.Println("[SitemapRows] Response: ", results)

			if err != nil {
				apis.NewBadRequestError("Failed to fetch warehouse items", err)
			}

			return c.JSON(http.StatusOK, results)
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
			// regexQuery := fmt.Sprintf(`^%s/([^/]+\s*)?$`, data.Path)

			// Simplier for noeWHERE path LIKE '/s/%'
			regexQuery := data.Path + `/%`

			var result []types.SitemapFetchResponse
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

			err := pocketbaseDB.CreateCollection(collection, schemas.Sitemap)

			// Checking error if it is collection already exists
			// This is the error "constraint failed: UNIQUE constraint failed: collections.name (2067)"

			if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
				log.Println("collection already exists: ", collection)
			}

			// Inserting data
			result, err := pocketbaseDB.App.Dao().DB().Insert(collection, dbx.Params{
				"id":       data.MainID,
				"path":     data.Path,
				"query":    data.Query,
				"fragment": data.Fragment,
				"type":     data.Type,
				"main_id":  data.MainID,
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
