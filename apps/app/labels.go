package app

import (
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"

	"github.com/glitchedgitz/grroxy/internal/types"
	"github.com/glitchedgitz/pocketbase/apis"
	"github.com/glitchedgitz/pocketbase/core"
	"github.com/glitchedgitz/pocketbase/models"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
)

// A label needs a color and a type, and a caller attaching one rarely cares
// which — these are what the template actions already default to.
const (
	defaultLabelColor = "blue"
	defaultLabelType  = "custom"
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

// AttachLabelRequest is the POST /api/label/attach body.
//
// The label is named, not id'd, and created when it does not exist yet —
// attaching a label the user has not defined first is the common case.
type AttachLabelRequest struct {
	// IDs are the rows to attach to, by record id.
	RowTargets
	// ID is the single row form the endpoint has always taken, kept so the
	// existing callers (frontend sidebar, sdk) keep working.
	ID string `json:"id"`

	Name  string `json:"name"`
	Color string `json:"color"`
	Type  string `json:"type"`
}

type AttachLabelResponse struct {
	Success bool      `json:"success"`
	Label   LabelInfo `json:"label"`
	// Created reports whether the label itself was new.
	Created bool `json:"created"`
	// Attached holds the rows the label was put on by this call,
	// AlreadyAttached the ones that already carried it.
	Attached        []string          `json:"attached"`
	AlreadyAttached []string          `json:"alreadyAttached,omitempty"`
	Skipped         map[string]string `json:"skipped,omitempty"`
}

// LabelAttach attaches a label to one or more rows, creating the label if it is
// new.
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

			var body AttachLabelRequest
			if err := c.Bind(&body); err != nil {
				log.Println("[LabelAttach]: ", err)
				return c.JSON(http.StatusBadRequest, map[string]any{"error": "Invalid request body"})
			}

			resp, skipped, err := backend.attachLabelLogic(body)
			if err != nil {
				log.Println("[LabelAttach]: ", err)
				return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error(), "skipped": skipped})
			}

			return c.JSON(http.StatusOK, resp)
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

// attachLabelLogic is the work behind /api/label/attach and the attachLabel MCP
// tool.
//
// A row that cannot be attached to lands in skipped rather than failing the
// batch — one bad id out of twenty should not cost the other nineteen.
func (backend *Backend) attachLabelLogic(body AttachLabelRequest) (AttachLabelResponse, map[string]string, error) {
	name := strings.TrimSpace(body.Name)
	if name == "" {
		return AttachLabelResponse{}, nil, fmt.Errorf("name is required")
	}

	ids, skipped, err := backend.resolveExtractTargets(attachTargets(body))
	if err != nil {
		return AttachLabelResponse{}, skipped, err
	}
	if skipped == nil {
		skipped = make(map[string]string)
	}

	label, created, err := backend.findOrCreateLabel(name, body.Color, body.Type)
	if err != nil {
		return AttachLabelResponse{}, skipped, err
	}

	dao := backend.App.Dao()

	attached := []string{}
	already := []string{}

	for _, id := range ids {
		// the _attached record shares the row id and is written when the row
		// is stored, so a missing one means the row itself is not there
		record, err := dao.FindRecordById("_attached", id)
		if err != nil || record == nil {
			skipped[id] = "no such row"
			continue
		}

		labels := record.GetStringSlice("labels")
		if slices.Contains(labels, label.Id) {
			already = append(already, id)
			continue
		}

		record.Set("labels", append(labels, label.Id))
		if err := dao.SaveRecord(record); err != nil {
			skipped[id] = err.Error()
			continue
		}

		// only a real attach moves the counter, or re-attaching an existing
		// label would inflate the count the sidebar shows
		if backend.CounterManager != nil {
			backend.CounterManager.Increment("label:"+label.Id, "_labels", "")
		}

		attached = append(attached, id)
	}

	resp := AttachLabelResponse{
		Success:         true,
		Label:           backend.labelInfo(label),
		Created:         created,
		Attached:        attached,
		AlreadyAttached: already,
	}
	if len(skipped) > 0 {
		resp.Skipped = skipped
	}

	return resp, skipped, nil
}

// attachTargets is the rows an attach call names. The endpoint has always taken
// a single "id" and the tools take "ids", so both are accepted and folded into
// one list — resolveExtractTargets drops the duplicate when a caller sends the
// same row in both.
func attachTargets(body AttachLabelRequest) RowTargets {
	targets := body.RowTargets

	if id := strings.TrimSpace(body.ID); id != "" {
		targets.IDs = append(targets.IDs, id)
	}

	return targets
}

// findOrCreateLabel returns the label with this name, creating it when there is
// none. The name is matched case-insensitively so that "SQLi" attaches to an
// existing "sqli" instead of creating a near duplicate.
func (backend *Backend) findOrCreateLabel(name, color, labelType string) (*models.Record, bool, error) {
	dao := backend.App.Dao()

	if record, err := backend.findLabelByName(name); err != nil {
		return nil, false, err
	} else if record != nil {
		return record, false, nil
	}

	if color = strings.TrimSpace(color); color == "" {
		color = defaultLabelColor
	}
	if labelType = strings.TrimSpace(labelType); labelType == "" {
		labelType = defaultLabelType
	}

	collection, err := dao.FindCollectionByNameOrId("_labels")
	if err != nil {
		return nil, false, fmt.Errorf("failed to find the labels collection: %w", err)
	}

	record := models.NewRecord(collection)
	record.Set("name", name)
	record.Set("color", color)
	record.Set("type", labelType)

	if err := dao.SaveRecord(record); err != nil {
		// names are unique, so a save that fails here is most likely a label
		// created in between the lookup and this write
		existing, ferr := backend.findLabelByName(name)
		if ferr != nil || existing == nil {
			return nil, false, fmt.Errorf("failed to create label %q: %w", name, err)
		}
		return existing, false, nil
	}

	return record, true, nil
}

// findLabelByName looks a label up by its exact name, case-insensitively.
// A missing label is not an error — it is what tells the caller to create one.
func (backend *Backend) findLabelByName(name string) (*models.Record, error) {
	records, err := backend.labelsMatchingName(name)
	if err != nil {
		return nil, err
	}

	for _, record := range records {
		if strings.EqualFold(record.GetString("name"), name) {
			return record, nil
		}
	}

	return nil, nil
}

// labelsMatchingName returns the labels whose name contains the given one —
// "~" is a LIKE, so an exact hit and the labels that merely contain it come
// back together and the caller decides which of them it meant.
func (backend *Backend) labelsMatchingName(name string) ([]*models.Record, error) {
	records, err := backend.App.Dao().FindRecordsByFilter(
		"_labels", "name ~ {:name}", "name", 0, 0,
		dbx.Params{"name": name},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to read labels: %w", err)
	}

	return records, nil
}
