package api

import (
	"net/http"
	"strings"

	"github.com/glitchedgitz/grroxy-db/schemas"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
)

type PlaygroundData struct {
	Type string `json:"type,omitempty"`
	Data struct {
		Host string                 `json:"host"`
		Port string                 `json:"port"`
		Req  map[string]interface{} `json:"req"`
		// Add other fields as needed
	} `json:"data"`
}

type NewRepeaterRequest struct {
	URL   string                 `json:"url"`
	Data  string                 `json:"data"`
	Req   string                 `json:"req"`
	Resp  string                 `json:"resp"`
	Extra map[string]interface{} `json:"extra"`
}

type NewIntruderRequest struct {
	ID      string `json:"id"`
	URL     string `json:"url"`
	Req     string `json:"req"`
	Payload string `json:"payload"`
}

func (backend *Backend) PlaygroundNew(e *core.ServeEvent) error {

	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/playground/new",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil

			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			var data PlaygroundData
			if err := c.Bind(&data); err != nil {
				return err
			}

			host := data.Data.Host
			port := data.Data.Port
			url := ""
			if u, ok := data.Data.Req["url"].(string); ok {
				url = u
			}

			titleFolder := host + ":" + port
			titlePG := url

			// Check if folder exists
			folderFilter := "name = '" + titleFolder + "' AND type = 'folder' AND parent_id = null"
			folderRecord, _ := backend.GetRecord("playground", folderFilter)
			if folderRecord == nil {
				folderRecord, _ = backend.SaveRecordToCollection("playground", map[string]interface{}{
					"name":       titleFolder,
					"type":       "folder",
					"parent_id":  nil,
					"sort_order": 0,
					"expanded":   false,
				})
			}

			// Check if playground exists under this folder
			pgFilter := "name = '" + titlePG + "' AND type = 'playground' AND parent_id = '" + folderRecord.Id + "'"
			pgRecord, _ := backend.GetRecord("playground", pgFilter)
			if pgRecord == nil {
				pgRecord, _ = backend.SaveRecordToCollection("playground", map[string]interface{}{
					"name":       titlePG,
					"type":       "playground",
					"parent_id":  folderRecord.Id,
					"sort_order": 0,
					"expanded":   true,
				})
			}

			return c.JSON(http.StatusOK, pgRecord)
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

func (backend *Backend) PlaygroundInitiate(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/playground/initiate",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil
			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			var data PlaygroundData
			if err := c.Bind(&data); err != nil {
				return err
			}

			typeVal := data.Type
			host := data.Data.Host
			port := data.Data.Port
			url := ""
			if u, ok := data.Data.Req["url"].(string); ok {
				url = u
			}

			titleFolder := host + ":" + port
			titlePG := url

			// Check if folder exists
			folderFilter := "name = '" + titleFolder + "' AND type = 'folder' AND parent_id = null"
			folderRecord, _ := backend.GetRecord("playground", folderFilter)
			if folderRecord == nil {
				folderRecord, _ = backend.SaveRecordToCollection("playground", map[string]interface{}{
					"name":       titleFolder,
					"type":       "folder",
					"parent_id":  nil,
					"sort_order": 0,
					"expanded":   false,
				})
			}

			// Check if playground exists under this folder
			pgFilter := "name = '" + titlePG + "' AND type = 'playground' AND parent_id = '" + folderRecord.Id + "'"
			pgRecord, _ := backend.GetRecord("playground", pgFilter)
			if pgRecord == nil {
				pgRecord, _ = backend.SaveRecordToCollection("playground", map[string]interface{}{
					"name":       titlePG,
					"type":       "playground",
					"parent_id":  folderRecord.Id,
					"sort_order": 0,
					"expanded":   true,
				})
			}

			// Create the child record under the playground
			childData := map[string]interface{}{
				"name":       typeVal,
				"type":       typeVal,
				"parent_id":  pgRecord.Id,
				"sort_order": 0,
				"expanded":   false,
				"data":       data.Data, // store the original data if needed
			}
			childRecord, _ := backend.SaveRecordToCollection("playground", childData)

			return c.JSON(http.StatusOK, childRecord)
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

func (backend *Backend) PlaygroundDelete(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/playground/delete",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil
			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			var data map[string]interface{}
			if err := c.Bind(&data); err != nil {
				return err
			}

			id, ok := data["id"].(string)
			if !ok || id == "" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Missing or invalid id"})
			}

			record, err := backend.App.Dao().FindRecordById("playground", id)
			if err != nil {
				return c.JSON(http.StatusNotFound, map[string]interface{}{"error": "Record not found"})
			}

			err = backend.App.Dao().DeleteRecord(record)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			return c.JSON(http.StatusOK, map[string]interface{}{"success": true, "id": id})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

func (backend *Backend) RepeaterNew(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/repeater/new",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil
			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			var data NewRepeaterRequest
			if err := c.Bind(&data); err != nil {
				return err
			}

			// Create main repeater record
			repeaterRecord, err := backend.SaveRecordToCollection("repeater", map[string]interface{}{
				"name": "Repeater - " + data.URL,
				"data": data.Data,
			})
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			// Create repeater_[ID] collection if not exists
			collectionName := "repeater_" + repeaterRecord.Id
			err = backend.CreateCollection(collectionName, schemas.RepeaterTabSchema)
			if err != nil {
				// If already exists, ignore error
				if !strings.Contains(err.Error(), "UNIQUE constraint failed") {
					return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
				}
			}

			// Insert row into repeater_[ID]
			_, err = backend.SaveRecordToCollection(collectionName, map[string]interface{}{
				"url":   data.URL,
				"req":   data.Req,
				"resp":  data.Resp,
				"extra": data.Extra,
			})
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			return c.JSON(http.StatusOK, repeaterRecord)
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

func (backend *Backend) RepeaterDelete(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/repeater/delete",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil
			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			var data map[string]interface{}
			if err := c.Bind(&data); err != nil {
				return err
			}

			id, ok := data["id"].(string)
			if !ok || id == "" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Missing or invalid id"})
			}

			record, err := backend.App.Dao().FindRecordById("repeater", id)
			if err != nil {
				return c.JSON(http.StatusNotFound, map[string]interface{}{"error": "Record not found"})
			}

			err = backend.App.Dao().DeleteRecord(record)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			// Optionally, delete the associated repeater_[ID] collection
			collectionName := "repeater_" + id
			coll, err := backend.App.Dao().FindCollectionByNameOrId(collectionName)
			if err == nil && coll != nil {
				err = backend.App.Dao().DeleteCollection(coll)
				if err != nil {
					return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": "Deleted record but failed to delete collection: " + err.Error()})
				}
			}

			return c.JSON(http.StatusOK, map[string]interface{}{"success": true, "id": id})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

func (backend *Backend) IntruderNew(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/intruder/new",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil
			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			var data NewIntruderRequest
			if err := c.Bind(&data); err != nil {
				return err
			}

			if data.ID == "" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Missing or invalid id"})
			}

			collectionName := "intruder_" + data.ID
			err := backend.CreateCollection(collectionName, schemas.IntruderTabSchema)
			if err != nil && !strings.Contains(err.Error(), "UNIQUE constraint failed") {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			_, err = backend.SaveRecordToCollection(collectionName, map[string]interface{}{
				"url":     data.URL,
				"req":     data.Req,
				"payload": data.Payload,
			})
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			return c.JSON(http.StatusOK, map[string]interface{}{"success": true, "id": data.ID})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

func (backend *Backend) IntruderDelete(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/intruder/delete",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil
			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			var data map[string]interface{}
			if err := c.Bind(&data); err != nil {
				return err
			}

			id, ok := data["id"].(string)
			if !ok || id == "" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Missing or invalid id"})
			}

			collectionName := "intruder_" + id
			coll, err := backend.App.Dao().FindCollectionByNameOrId(collectionName)
			if err != nil || coll == nil {
				return c.JSON(http.StatusNotFound, map[string]interface{}{"error": "Collection not found"})
			}

			err = backend.App.Dao().DeleteCollection(coll)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			return c.JSON(http.StatusOK, map[string]interface{}{"success": true, "id": id})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}
