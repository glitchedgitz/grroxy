package api

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/glitchedgitz/grroxy-db/schemas"
	"github.com/glitchedgitz/grroxy-db/types"
	"github.com/glitchedgitz/grroxy-db/utils"
	wappalyzer "github.com/glitchedgitz/wappalyzergo"
	"github.com/jpillora/go-tld"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
)

func (backend *Backend) SitemapNew(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/sitemap/new",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil

			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			var data types.SitemapGet
			var wg sync.WaitGroup

			if err := c.Bind(&data); err != nil {
				return err
			}
			log.Print("SitemapNew: ", data)

			var collectionExists = true

			SitemapCollectionName := utils.ParseDatabaseName(data.Host)
			err := backend.CreateCollection(SitemapCollectionName, schemas.Sitemap)

			// Checking error if it is collection already exists
			// This is the error "constraint failed: UNIQUE constraint failed: collections.name (2067)"
			if err != nil && !strings.Contains(err.Error(), "UNIQUE constraint failed") {
				collectionExists = true
			} else {
				collectionExists = false
			}

			// New Host
			go func() {

				log.Println("Checking: new collection for host: ", SitemapCollectionName)

				if !collectionExists {
					wg.Add(1)
					defer wg.Done()

					var fingerprints map[string]wappalyzer.LogoAndInfo = make(map[string]wappalyzer.LogoAndInfo)
					var respData []byte = []byte("0")
					var status int = 0

					log.Println("sending request to: ", SitemapCollectionName)
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // Timeout after 5 seconds
					defer cancel()                                                           // Cancel the context to release resources

					// Create an HTTP request
					req, err := http.NewRequestWithContext(ctx, "GET", data.Host, nil)
					if err != nil {
						log.Println(err)
					}

					// Perform the HTTP request
					resp, err := http.DefaultClient.Do(req)
					log.Println("got request to: ", SitemapCollectionName)

					log.Println("Checking: wappalyzer for: ", SitemapCollectionName)

					if err != nil {
						log.Println("[http.DefaultClient.Get]: ", err)
					} else {
						respData, err = io.ReadAll(resp.Body) // Ignoring error for example
						if err != nil {
							log.Println(err)
						} else {
							status = resp.StatusCode

							fingerprints = backend.Wappalyzer.FingerprintWithLogoAndInfo(resp.Header, respData)

							fmt.Printf("Wappylyzer Fingerprints %v\n", fingerprints)
						}
					}
					log.Println("Checked: wappalyzer for: ", SitemapCollectionName)

					// Insert row in _hosts
					u, err := tld.Parse(data.Host)
					if err != nil {
						log.Println(err)
					}

					// title, _ := "", ""
					title, _ := utils.ExtractTitle(respData)

					recordIDs := []string{}

					// TODO: Having a array of tech and hosts in the sitemap could save quite a lot of requests

					for key, value := range fingerprints {
						r, err := backend.SaveRecordToCollection("_tech", map[string]interface{}{
							"name":  key,
							"image": value.Logo,
							"extra": map[string]any{
								"category":    value.Cats,
								"description": value.Description,
								"website":     value.Website,
							},
						})
						if err != nil {
							// Most probably it's a duplicate and we can fetch the ID
							r, err = backend.GetRecord("_tech", fmt.Sprintf("name = '%s'", key))
							if err != nil {
								log.Println(err)
							}
						}
						recordIDs = append(recordIDs, r.Id)
					}

					backend.SaveRecordToCollection("_hosts", map[string]interface{}{
						"host":      data.Host,
						"smartsort": utils.SmartSort(data.Host),
						"domain":    u.Domain + "." + u.TLD,
						"status":    status,
						"title":     title,
						"tech":      recordIDs,
					})

				}
				log.Println("Checked: new collection for host: ", SitemapCollectionName)

			}()

			// Inserting endpoint data
			backend.SaveRecordToCollection(SitemapCollectionName, map[string]interface{}{
				"id":       data.Data,
				"path":     data.Path,
				"query":    data.Query,
				"fragment": data.Fragment,
				"type":     data.Type,
				"ext":      data.Ext,
				"data":     data.Data,
			})

			wg.Wait()

			return c.String(http.StatusOK, "Created")
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

func (backend *Backend) SitemapFetch(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/sitemap/fetch",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil

			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			var data types.SitemapFetch
			if err := c.Bind(&data); err != nil {
				return err
			}

			db := utils.ParseDatabaseName(data.Host)

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
				err = backend.App.Dao().DB().NewQuery("SELECT * FROM " + db).All(&result)
			} else {
				err = backend.App.Dao().DB().NewQuery("SELECT * FROM " + db + " WHERE path LIKE '" + regexQuery + "'").All(&result)
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
			apis.ActivityLogger(backend.App),
		},
	})

	return nil
}
