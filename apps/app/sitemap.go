package app

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

	"github.com/glitchedgitz/grroxy/internal/schemas"
	"github.com/glitchedgitz/grroxy/internal/types"
	"github.com/glitchedgitz/grroxy/internal/utils"
	"github.com/glitchedgitz/pocketbase/apis"
	"github.com/glitchedgitz/pocketbase/core"
	"github.com/glitchedgitz/pocketbase/models"
	wappalyzer "github.com/glitchedgitz/wappalyzergo"
	"github.com/jpillora/go-tld"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
)

func (backend *Backend) handleSitemapNew(data *types.SitemapGet) error {
	var wg sync.WaitGroup

	var collectionExists = true

	SitemapCollectionName := utils.ParseDatabaseName(data.Host)
	err := backend.CreateCollection(SitemapCollectionName, schemas.Sitemap)

	// Checking error if it is collection already exists
	// This is the error "constraint failed: UNIQUE constraint failed: collections.name (2067)"
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
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

			var parsedDomain string
			var parsedTLD string

			// Insert row in _hosts
			u, err := tld.Parse(data.Host)
			if err != nil {
				log.Println(err)
			} else {
				parsedDomain = u.Domain
				parsedTLD = u.TLD
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

				// Increment counter for this tech
				backend.CounterManager.Increment("tech:"+key, "", "")
			}

			backend.SaveRecordToCollection("_hosts", map[string]interface{}{
				"host":      data.Host,
				"smartsort": utils.SmartSort(data.Host),
				"domain":    parsedDomain + "." + parsedTLD,
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

	return nil
}

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

			if err := c.Bind(&data); err != nil {
				return err
			}
			log.Print("SitemapNew: ", data)

			err := backend.handleSitemapNew(&data)
			if err != nil {
				return err
			}

			return c.String(http.StatusOK, "Created")
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

// buildSitemapTree builds a hierarchical tree structure from flat records
func buildSitemapTree(records []*models.Record, basePath string, host string, maxDepth int) []*types.SitemapNode {
	// Create a map to store all nodes by their path
	nodeMap := make(map[string]*types.SitemapNode)
	pathChildren := make(map[string][]string) // Track children paths for each parent path

	// First pass: create all nodes and track parent-child relationships
	for _, item := range records {
		fullPath := item.GetString("path")
		tmpPath := strings.TrimPrefix(fullPath, basePath)
		tmpPath = strings.TrimPrefix(tmpPath, "/")

		// Skip empty paths
		if tmpPath == "" {
			continue
		}

		// Remove query and fragment for path processing
		var cleanPath string
		if index := strings.IndexAny(tmpPath, "?#"); index != -1 {
			cleanPath = tmpPath[:index]
		} else {
			cleanPath = tmpPath
		}

		// Split path into segments
		segments := strings.Split(cleanPath, "/")

		// Build full path for each segment depth
		for i := 0; i < len(segments); i++ {
			currentSegments := segments[:i+1]
			currentPath := strings.Join(currentSegments, "/")
			fullCurrentPath := basePath + "/" + currentPath

			// Only create node if it doesn't exist
			if _, exists := nodeMap[fullCurrentPath]; !exists {
				title := segments[i]

				nodeMap[fullCurrentPath] = &types.SitemapNode{
					Host:          host,
					Path:          fullCurrentPath,
					Title:         title,
					Type:          item.Get("type"),
					Ext:           item.Get("ext"),
					Query:         item.Get("query"),
					Children:      []*types.SitemapNode{},
					ChildrenCount: 0,
				}

				// Track parent-child relationship
				if i > 0 {
					parentPath := basePath + "/" + strings.Join(segments[:i], "/")
					pathChildren[parentPath] = append(pathChildren[parentPath], fullCurrentPath)
				}
			}
		}
	}

	// Second pass: build tree structure and count children
	for parentPath, childPaths := range pathChildren {
		if parentNode, exists := nodeMap[parentPath]; exists {
			uniqueChildren := make(map[string]bool)
			for _, childPath := range childPaths {
				uniqueChildren[childPath] = true
			}

			for childPath := range uniqueChildren {
				if childNode, exists := nodeMap[childPath]; exists {
					parentNode.Children = append(parentNode.Children, childNode)
				}
			}
			parentNode.ChildrenCount = len(parentNode.Children)
			parentNode.IsFolder = parentNode.ChildrenCount > 0

			// Set IsFolder for all children
			for _, child := range parentNode.Children {
				child.IsFolder = len(child.Children) > 0
			}

			// Sort children: folders first, then files, both alphabetically
			sort.Slice(parentNode.Children, func(i, j int) bool {
				// If one is a folder and the other is a file, folder comes first
				if parentNode.Children[i].IsFolder != parentNode.Children[j].IsFolder {
					return parentNode.Children[i].IsFolder
				}

				// Both are folders or both are files, sort alphabetically
				return parentNode.Children[i].Title < parentNode.Children[j].Title
			})
		}
	}

	// Find root level nodes (direct children of basePath)
	rootNodes := []*types.SitemapNode{}
	for path, node := range nodeMap {
		tmpPath := strings.TrimPrefix(path, basePath)
		tmpPath = strings.TrimPrefix(tmpPath, "/")

		// Check if this is a root level node (no slashes means it's direct child of basePath)
		if !strings.Contains(tmpPath, "/") && tmpPath != "" {
			// Set IsFolder for nodes that weren't processed as parents
			if node.ChildrenCount == 0 {
				node.IsFolder = false
			}
			rootNodes = append(rootNodes, node)
		}
	}

	// Sort root nodes: folders first, then files, both alphabetically
	sort.Slice(rootNodes, func(i, j int) bool {
		// If one is a folder and the other is a file, folder comes first
		if rootNodes[i].IsFolder != rootNodes[j].IsFolder {
			return rootNodes[i].IsFolder
		}

		// Both are folders or both are files, sort alphabetically
		return rootNodes[i].Title < rootNodes[j].Title
	})

	// Apply depth limit if specified (depth > 0)
	if maxDepth > 0 {
		rootNodes = limitTreeDepth(rootNodes, maxDepth, 1)
	}

	return rootNodes
}

// limitTreeDepth recursively limits the tree to specified depth
func limitTreeDepth(nodes []*types.SitemapNode, maxDepth int, currentDepth int) []*types.SitemapNode {
	if currentDepth >= maxDepth {
		// Remove children at max depth
		for _, node := range nodes {
			node.Children = nil
		}
		return nodes
	}

	for _, node := range nodes {
		if len(node.Children) > 0 {
			node.Children = limitTreeDepth(node.Children, maxDepth, currentDepth+1)
		}
	}

	return nodes
}

func (backend *Backend) sitemapFetchLogic(data *types.SitemapFetch) ([]*types.SitemapNode, error) {
	// Set default depth to 1 if not specified (0)
	if data.Depth == 0 {
		data.Depth = 1
	}

	db := utils.ParseDatabaseName(data.Host)
	path := data.Path + `/%`

	var result []*models.Record
	var err error

	dao := backend.App.Dao()

	fmt.Println("db: ", db)
	fmt.Println("path: ", path)
	fmt.Println("depth: ", data.Depth)

	collection, err := dao.FindCollectionByNameOrId(db)
	if err != nil {
		log.Println("Error fetching collection: ", err)
		return nil, fmt.Errorf("host doesn't exist")
	}

	if data.Path == "" {
		result, err = dao.FindRecordsByExpr(collection.Id)
	} else {
		result, err = dao.FindRecordsByFilter(collection.Id, "path ~ {:path}", "path", 0, 0, dbx.Params{
			"path": path,
		})
	}

	if err != nil {
		log.Println("Error fetching records: ", err)
		return nil, fmt.Errorf("failed to fetch records: %w", err)
	}

	// Build tree structure with depth control
	// If depth is -1, pass 0 to buildSitemapTree for unlimited depth
	depthLimit := data.Depth
	if depthLimit == -1 {
		depthLimit = 0
	}
	treeNodes := buildSitemapTree(result, data.Path, data.Host, depthLimit)

	log.Println("[SitemapFetch] Request: ", data)
	log.Println("[SitemapFetch] Response nodes count: ", len(treeNodes))

	return treeNodes, nil
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

			treeNodes, err := backend.sitemapFetchLogic(&data)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{
					"error":   err.Error(),
					"message": err.Error(),
					"data":    []interface{}{},
				})
			}

			return c.JSON(http.StatusOK, treeNodes)
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})

	return nil
}
