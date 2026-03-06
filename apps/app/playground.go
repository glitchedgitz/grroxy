package app

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/glitchedgitz/grroxy/internal/schemas"
	"github.com/glitchedgitz/grroxy/internal/types"
	"github.com/glitchedgitz/pocketbase/apis"
	"github.com/glitchedgitz/pocketbase/core"
	"github.com/glitchedgitz/pocketbase/models"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
)

const SORT_GAP = 1000

func GetSortOrder(items []*models.Record) int {
	// Calculate new sort order
	newSortOrder := 0
	if items != nil && len(items) > 0 {
		// Find the highest sort order
		maxSortOrder := 0
		for _, item := range items {
			if sortOrder, ok := item.Get("sort_order").(int); ok && sortOrder > maxSortOrder {
				maxSortOrder = sortOrder
			}
		}
		newSortOrder = maxSortOrder + SORT_GAP
	}
	return newSortOrder
}

func (backend *Backend) GetOrCreatePlayground(name string, typeVal string, parentId string) (*models.Record, error) {
	pgRecord, err := backend.GetRecord("_playground", "name = '"+name+"' AND type = '"+typeVal+"' AND parent_id = '"+parentId+"'")
	if err != nil {
		return nil, err
	}

	if pgRecord == nil {
		pgRecord, err = backend.SaveRecordToCollection("_playground", map[string]interface{}{
			"name":       name,
			"type":       typeVal,
			"parent_id":  parentId,
			"sort_order": 0,
			"expanded":   false,
		})
	}

	return pgRecord, nil
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

			log.Println("/api/playground/new")

			var body types.PlaygroundNew
			if err := c.Bind(&body); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			log.Println("pg body", body)

			name := body.Name

			if name == "" {
				name = "New Playground"
			}
			if body.Type == "" {
				body.Type = "playground"
			}

			// Get all top-level items (parent_id is null)
			topLevelItems, err := backend.App.Dao().FindRecordsByFilter("_playground", `parent_id = {:parent_id}`, "sort_order", 0, 0, dbx.Params{
				"parent_id": body.ParentId,
			})

			newSortOrder := GetSortOrder(topLevelItems)

			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			pgRecord, err := backend.SaveRecordToCollection("_playground", map[string]interface{}{
				"name":       name,
				"type":       body.Type,
				"parent_id":  body.ParentId,
				"sort_order": newSortOrder,
				"expanded":   body.Expanded,
			})

			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			return c.JSON(http.StatusOK, pgRecord)
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

func (backend *Backend) PlaygroundAddChild(e *core.ServeEvent) error {

	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/playground/add",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil

			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			log.Println("/api/playground/add")

			var body types.PlaygroundAdd
			if err := c.Bind(&body); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			log.Println("pg body", body)

			fmt.Println("Items: ", body.Items)

			// Get all items under the parent to determine sort order
			existingItems, err := backend.App.Dao().FindRecordsByFilter("_playground", `parent_id = {:parent_id}`, "sort_order", 0, 0, dbx.Params{
				"parent_id": body.ParentId,
			})
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			newSortOrder := GetSortOrder(existingItems)

			records := []*models.Record{}

			// Handle list of items
			for _, item := range body.Items {
				fmt.Println("Items loop ", item)
				newSortOrder += SORT_GAP

				pgRecord, err := backend.SaveRecordToCollection("_playground", map[string]interface{}{
					"name":       item.Name,
					"type":       item.Type,
					"parent_id":  body.ParentId,
					"sort_order": newSortOrder,
					"expanded":   false,
				})

				records = append(records, pgRecord)

				if err != nil {
					return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
				}

				switch item.Type {
				case "repeater":
					fmt.Println("PlaygroundItem: ", item)
					err = backend.RepeaterNew(pgRecord.Id, item)
					if err != nil {
						return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
					}
				case "fuzzer":
					fmt.Println("IntruderRequest: ", item)
					err = backend.IntruderNew(pgRecord.Id, item)
					if err != nil {
						return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
					}
				default:
					fmt.Println("Not Found: ")
				}
			}

			return c.JSON(http.StatusOK, map[string]interface{}{"success": true, "items": records})
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

			var body map[string]interface{}
			if err := c.Bind(&body); err != nil {
				return err
			}

			id, ok := body["id"].(string)
			if !ok || id == "" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Missing or invalid id"})
			}

			// Function to recursively delete children
			var deleteChildren func(parentId string) error
			deleteChildren = func(parentId string) error {
				// Find all children of the current parent
				children, err := backend.App.Dao().FindRecordsByFilter("_playground", `parent_id = {:parent_id}`, "sort_order", 0, 0, dbx.Params{
					"parent_id": parentId,
				})
				if err != nil {
					return err
				}

				// Recursively delete each child
				for _, child := range children {
					// Delete children of this child first
					if err := deleteChildren(child.Id); err != nil {
						return err
					}

					// Delete the child record
					if err := backend.App.Dao().DeleteRecord(child); err != nil {
						return err
					}

					// If the child is a repeater or intruder, delete its associated collection
					childType, _ := child.Get("type").(string)
					switch childType {
					case "repeater":
						if err := backend.RepeaterDelete(child.Id); err != nil {
							return err
						}
					case "fuzzer":
						if err := backend.IntruderDelete(child.Id); err != nil {
							return err
						}
					}
				}
				return nil
			}

			// Get the record to be deleted
			record, err := backend.App.Dao().FindRecordById("_playground", id)
			if err != nil {
				return c.JSON(http.StatusNotFound, map[string]interface{}{"error": "Record not found"})
			}

			// Delete all children first
			if err := deleteChildren(id); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			// Delete the parent record
			if err := backend.App.Dao().DeleteRecord(record); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			// If the parent is a repeater or intruder, delete its associated collection
			recordType, _ := record.Get("type").(string)
			switch recordType {
			case "repeater":
				if err := backend.RepeaterDelete(id); err != nil {
					return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
				}
			case "fuzzer":
				if err := backend.IntruderDelete(id); err != nil {
					return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
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

func (backend *Backend) RepeaterNew(id string, data types.PlaygroundItem) error {

	// Create repeater_[ID] collection if not exists
	collectionName := "repeater_" + id
	err := backend.CreateCollection(collectionName, schemas.RepeaterTabSchema)
	if err != nil {
		// If already exists, ignore error
		if !strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return err
		}
	}

	// Insert row into repeater_[ID]
	_, err = backend.SaveRecordToCollection(collectionName, map[string]any{
		"url":   data.ToolData["url"],
		"req":   data.ToolData["req"],
		"resp":  data.ToolData["resp"],
		"data":  data.ToolData,
		"extra": data.ToolData,
	})

	if err != nil {
		return err
	}

	return nil
}

func (backend *Backend) RepeaterDelete(id string) error {

	record, err := backend.App.Dao().FindRecordById("repeater", id)
	if err != nil {
		return err
	}

	err = backend.App.Dao().DeleteRecord(record)
	if err != nil {
		return err
	}

	// Optionally, delete the associated repeater_[ID] collection
	collectionName := "repeater_" + id
	coll, err := backend.App.Dao().FindCollectionByNameOrId(collectionName)
	if err == nil && coll != nil {
		err = backend.App.Dao().DeleteCollection(coll)
		if err != nil {
			return err
		}
	}

	return nil
}

func (backend *Backend) IntruderNew(id string, data types.PlaygroundItem) error {

	collectionName := "intruder_" + id
	err := backend.CreateCollection(collectionName, schemas.IntruderTabSchema)
	if err != nil && !strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return err
	}

	_, err = backend.SaveRecordToCollection(collectionName, map[string]any{
		"url":     data.ToolData["url"],
		"req":     data.ToolData["req"],
		"payload": data.ToolData["payload"],
	})
	if err != nil {
		return err
	}

	return nil
}

func (backend *Backend) IntruderDelete(id string) error {

	collectionName := "intruder_" + id
	coll, err := backend.App.Dao().FindCollectionByNameOrId(collectionName)
	if err != nil || coll == nil {
		return err
	}

	err = backend.App.Dao().DeleteCollection(coll)
	if err != nil {
		return err
	}

	return nil
}
