package app

import (
	"log"
	"net/http"

	"github.com/glitchedgitz/grroxy/internal/types"
	"github.com/glitchedgitz/pocketbase/apis"
	"github.com/glitchedgitz/pocketbase/core"
	"github.com/glitchedgitz/pocketbase/models"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
)

func (backend *Backend) LabelNew(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/label/new",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil

			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			var data types.Label
			if err := c.Bind(&data); err != nil {
				return err
			}

			mainCollection, err := backend.App.Dao().FindCollectionByNameOrId("_labels")
			if err != nil {
				return err
			}

			record := models.NewRecord(mainCollection)
			record.Set("name", data.Name)
			record.Set("color", data.Color)
			record.Set("type", data.Type)

			if err := backend.App.Dao().SaveRecord(record); err != nil {
				record, err2 := backend.App.Dao().FindFirstRecordByFilter(
					"_labels", "name = {:name}",
					dbx.Params{"name": data.Name},
				)
				if err2 != nil {
					return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err2.Error()})
				}
				return c.JSON(http.StatusOK, map[string]interface{}{
					"id":            record.Id,
					"alreadyExists": true,
				})
			}

			return c.JSON(http.StatusOK, map[string]interface{}{
				"id":            record.Id,
				"alreadyExists": false,
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

func (backend *Backend) LabelDelete(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/label/delete",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil

			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			var data types.Label
			var err error
			var record *models.Record
			var collection *models.Collection

			if err = c.Bind(&data); err != nil {
				log.Println("Label Delete: ", err)
				return err
			}

			if data.ID != "" {
				record, err = backend.App.Dao().FindRecordById("_labels", data.ID)
				if err != nil {
					log.Println("Label Delete: ", err)
					return err
				}
			}

			if data.Name != "" {
				record, err = backend.App.Dao().FindFirstRecordByFilter(
					"_labels", "name = {:name}",
					dbx.Params{"name": data.Name},
				)
				if err != nil {
					log.Println("Label Delete: ", err)
					return err
				}
			}

			collection, err = backend.App.Dao().FindCollectionByNameOrId("label_" + record.Id)
			if err != nil {
				log.Println("Label Delete: ", err)
				return err
			}
			if err := backend.App.Dao().DeleteCollection(collection); err != nil {
				log.Println("Label Delete - Collection: ", err)
				return err
			}
			if err := backend.App.Dao().DeleteRecord(record); err != nil {
				log.Println("Label Delete: - Record", err)
				return err
			}

			return c.String(http.StatusOK, "Deleted")
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

func (backend *Backend) LabelAttach(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/label/attach",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil

			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			var data types.Label
			if err := c.Bind(&data); err != nil {
				log.Println("[LabelNew]: ", err)
				return err
			}

			// Saving to main collection if doesn't exists
			mainCollection, err := backend.App.Dao().FindCollectionByNameOrId("_labels")
			if err != nil {
				log.Println("[LabelNew]: ", err)
				return err
			}

			record := models.NewRecord(mainCollection)

			// set individual fields
			// or bulk load with record.Load(map[string]any{...})
			record.Set("name", data.Name)
			record.Set("color", data.Color)
			record.Set("type", data.Type)

			err = backend.App.Dao().SaveRecord(record)
			// =====================

			// Fetching ID
			// TOOD: we should have list of labels here with ids, instead of fetching it every time
			labelRecord, err2 := backend.App.Dao().FindFirstRecordByData("_labels", "name", data.Name)

			if err2 != nil {
				log.Println("[LabelNew]: ", err)
				return err
			}

			// // If first error is not nil, means row just created, we need to create respective `label_[id]` collection
			// var collection = "label_" + labelRecord.Id
			// if err == nil {
			// 	// TODO: This is unnecessary todo everytime
			// 	// Create Collection if not exists
			// 	err = backend.CreateCollection(collection, schemas.LabelCollection)
			// 	if err != nil {
			// 		log.Println("[LabelNew]: ", err)
			// 		return err
			// 	}
			// }

			// // Inserting in the `label_[id]` Collection
			// result2, err := backend.App.Dao().DB().Insert(collection, dbx.Params{
			// 	"id":   data.ID,
			// 	"data": data.ID,
			// }).Execute()
			// if err != nil {
			// 	log.Println("[LabelNew]: ", err)
			// 	return err
			// }

			// log.Println("[LabelNew]: ", result2)

			// Attaching to the row
			record3, err := backend.App.Dao().FindRecordById("_attached", data.ID)
			if err != nil {
				log.Println("[LabelNew]: ", err)
				return err
			}

			record3.Set("labels", append(record3.GetStringSlice("labels"), labelRecord.Id))

			if err := backend.App.Dao().SaveRecord(record3); err != nil {
				log.Println("[LabelNew]: ", err)
				return err
			}

			// Increment counter for this label
			backend.CounterManager.Increment("label:"+labelRecord.Id, "_labels", "")

			return c.String(http.StatusOK, "Created")
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}
