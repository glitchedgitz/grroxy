package app

import (
	"fmt"
	"log"
	"strings"

	"github.com/glitchedgitz/grroxy/grx/templates"
	"github.com/glitchedgitz/grroxy/grx/templates/actions"
	"github.com/glitchedgitz/pocketbase/models"
)

// ExecuteTemplateActions processes the results from Templates.Run()
// and dispatches each action to its handler.
func (backend *Backend) ExecuteTemplateActions(results []templates.Action, data map[string]any) {
	rowID, _ := data["id"].(string)

	for _, action := range results {
		log.Printf("[TemplateActions] Executing action: %s for row: %s", action.ActionName, rowID)

		switch action.ActionName {
		case actions.CreateLabel:
			backend.executeCreateLabel(action.Data, rowID)
		case actions.SendRequest:
			backend.executeSendRequest(action.Data, data)
		default:
			log.Printf("[TemplateActions] Unknown action: %s", action.ActionName)
		}
	}
}

func (backend *Backend) executeCreateLabel(data map[string]any, rowID string) {
	name, _ := data["name"].(string)
	if name == "" {
		log.Println("[TemplateActions][CreateLabel] Missing name")
		return
	}

	color, _ := data["color"].(string)
	if color == "" {
		color = "blue"
	}

	labelType, _ := data["type"].(string)
	if labelType == "" {
		labelType = "custom"
	}

	// Step 1: Create label in _labels if it doesn't exist
	collection, err := backend.App.Dao().FindCollectionByNameOrId("_labels")
	if err != nil {
		log.Printf("[TemplateActions][CreateLabel] Error finding labels collection: %v", err)
		return
	}

	record := models.NewRecord(collection)
	record.Set("name", name)
	record.Set("color", color)
	record.Set("type", labelType)
	backend.App.Dao().SaveRecord(record) // ignore duplicate error

	// Step 2: Find the label record to get its ID
	labelRecord, err := backend.App.Dao().FindFirstRecordByData("_labels", "name", name)
	if err != nil {
		log.Printf("[TemplateActions][CreateLabel] Error finding label record: %v", err)
		return
	}

	// Step 3: Attach label to the row via _attached
	if rowID == "" {
		log.Println("[TemplateActions][CreateLabel] No row ID, skipping attach")
		return
	}

	attachedRecord, err := backend.App.Dao().FindRecordById("_attached", rowID)
	if err != nil {
		log.Printf("[TemplateActions][CreateLabel] Error finding attached record %s: %v", rowID, err)
		return
	}

	existingLabels := attachedRecord.GetStringSlice("labels")
	// Check if already attached
	for _, l := range existingLabels {
		if l == labelRecord.Id {
			return // already attached
		}
	}

	attachedRecord.Set("labels", append(existingLabels, labelRecord.Id))
	if err := backend.App.Dao().SaveRecord(attachedRecord); err != nil {
		log.Printf("[TemplateActions][CreateLabel] Error attaching label: %v", err)
		return
	}

	// Increment counter
	if backend.CounterManager != nil {
		backend.CounterManager.Increment("label:"+labelRecord.Id, "_labels", "")
	}

	log.Printf("[TemplateActions][CreateLabel] Attached label '%s' to row %s", name, rowID)
}

func (backend *Backend) executeSendRequest(actionData map[string]any, requestData map[string]any) {
	// Build the raw request from the original request data + overrides from the action
	host, _ := requestData["host"].(string)
	port, _ := requestData["port"].(string)
	tls, _ := requestData["tls"].(bool)
	rawReq, _ := requestData["raw"].(string)

	if host == "" || rawReq == "" {
		log.Println("[TemplateActions][SendRequest] Missing host or raw request data")
		return
	}

	// Apply overrides from action data (req.method, req.headers.X, req.body)
	for key, value := range actionData {
		strVal := fmt.Sprint(value)
		if key == "req.method" {
			// Replace method in first line
			lines := strings.SplitN(rawReq, "\n", 2)
			if len(lines) >= 2 {
				parts := strings.SplitN(lines[0], " ", 2)
				if len(parts) == 2 {
					rawReq = strVal + " " + parts[1] + "\n" + lines[1]
				}
			}
		} else if strings.HasPrefix(key, "req.headers.") {
			headerName := strings.TrimPrefix(key, "req.headers.")
			// TODO: modify header in rawReq
			_ = headerName
		} else if key == "req.body" {
			// TODO: modify body in rawReq
			_ = strVal
		}
	}

	index, _ := requestData["index"].(float64)

	go func() {
		_, err := backend.sendRepeaterLogic(&RepeaterSendRequest{
			Host:        host,
			Port:        port,
			TLS:         tls,
			Request:     rawReq,
			Timeout:     10,
			Index:       index,
			GeneratedBy: "template:send_request",
		})
		if err != nil {
			log.Printf("[TemplateActions][SendRequest] Error: %v", err)
			return
		}
		log.Printf("[TemplateActions][SendRequest] Sent request to %s", host)
	}()
}
