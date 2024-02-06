package endpoints

import (
	"log"
	"net/http"

	"github.com/glitchedgitz/grroxy-db/schemas"
	"github.com/glitchedgitz/grroxy-db/types"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
)

func (pocketbaseDB *DatabaseAPI) LabelNew(e *core.ServeEvent) error {
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

			mainCollection, err := pocketbaseDB.App.Dao().FindCollectionByNameOrId("_labels")
			if err != nil {
				return err
			}

			record := models.NewRecord(mainCollection)
			record.Set("name", data.Name)
			record.Set("color", data.Color)
			record.Set("type", data.Type)

			if err := pocketbaseDB.App.Dao().SaveRecord(record); err != nil {
				return err
			}

			return c.String(http.StatusOK, "Created")
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(pocketbaseDB.App),
		},
	})
	return nil
}

func (pocketbaseDB *DatabaseAPI) LabelDelete(e *core.ServeEvent) error {
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
				record, err = pocketbaseDB.App.Dao().FindRecordById("_labels", data.ID)
				if err != nil {
					log.Println("Label Delete: ", err)
					return err
				}
			}

			if data.Name != "" {
				record, err = pocketbaseDB.App.Dao().FindFirstRecordByFilter(
					"_labels", "name = {:name}",
					dbx.Params{"name": data.Name},
				)
				if err != nil {
					log.Println("Label Delete: ", err)
					return err
				}
			}

			collection, err = pocketbaseDB.App.Dao().FindCollectionByNameOrId("label_" + record.Id)
			if err != nil {
				log.Println("Label Delete: ", err)
				return err
			}
			if err := pocketbaseDB.App.Dao().DeleteCollection(collection); err != nil {
				log.Println("Label Delete - Collection: ", err)
				return err
			}
			if err := pocketbaseDB.App.Dao().DeleteRecord(record); err != nil {
				log.Println("Label Delete: - Record", err)
				return err
			}

			return c.String(http.StatusOK, "Deleted")
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(pocketbaseDB.App),
		},
	})
	return nil
}

func (pocketbaseDB *DatabaseAPI) LabelAttach(e *core.ServeEvent) error {
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
			mainCollection, err := pocketbaseDB.App.Dao().FindCollectionByNameOrId("_labels")
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

			err = pocketbaseDB.App.Dao().SaveRecord(record)
			// =====================

			// Fetching ID
			labelRecord, err2 := pocketbaseDB.App.Dao().FindFirstRecordByData("_labels", "name", data.Name)

			if err2 != nil {
				log.Println("[LabelNew]: ", err)
				return err
			}

			// If first error is not nil, means row just created, we need to create respective `label_[id]` collection
			var collection = "label_" + labelRecord.Id
			if err == nil {
				// TODO: This is unnecessary todo everytime
				// Create Collection if not exists
				err = pocketbaseDB.CreateCollection(collection, schemas.LabelCollection)
				if err != nil {
					log.Println("[LabelNew]: ", err)
					return err
				}
			}

			// Inserting in the `label_[id]` Collection
			result2, err := pocketbaseDB.App.Dao().DB().Insert(collection, dbx.Params{
				"id":   data.ID,
				"data": data.ID,
			}).Execute()
			if err != nil {
				log.Println("[LabelNew]: ", err)
				return err
			}

			log.Println("[LabelNew]: ", result2)

			// Attaching to the row
			record3, err := pocketbaseDB.App.Dao().FindRecordById("_attached", data.ID)
			if err != nil {
				log.Println("[LabelNew]: ", err)
				return err
			}

			record3.Set("labels", append(record3.GetStringSlice("labels"), labelRecord.Id))

			if err := pocketbaseDB.App.Dao().SaveRecord(record3); err != nil {
				log.Println("[LabelNew]: ", err)
				return err
			}

			if err != nil {
				log.Println("[LabelNew] Error: ", err)
			}

			return c.String(http.StatusOK, "Created")
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(pocketbaseDB.App),
		},
	})
	return nil
}
