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
	host, _ := requestData["host"].(string)
	port, _ := requestData["port"].(string)
	useTLS, _ := requestData["is_https"].(bool)

	// Strip scheme from host — sendRepeaterLogic expects bare hostname
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")

	// raw may be in userdata directly or in the req record
	rawReq, _ := requestData["raw"].(string)

	// If raw is empty, try to fetch from DB
	if rawReq == "" {
		if reqID, ok := requestData["req"].(string); ok && reqID != "" {
			log.Printf("[TemplateActions][SendRequest] Fetching raw from _req record: %s", reqID)
			record, err := backend.App.Dao().FindRecordById("_req", reqID)
			if err != nil {
				log.Printf("[TemplateActions][SendRequest] Error fetching _req: %v", err)
			} else {
				rawReq = record.GetString("raw")
				log.Printf("[TemplateActions][SendRequest] Fetched raw (len=%d)", len(rawReq))
			}
		} else {
			log.Printf("[TemplateActions][SendRequest] No req ID in data. Keys: %v", func() []string {
				keys := make([]string, 0, len(requestData))
				for k := range requestData {
					keys = append(keys, k)
				}
				return keys
			}())
		}
	}

	if host == "" || rawReq == "" {
		log.Printf("[TemplateActions][SendRequest] Missing host=%q or raw request (len=%d)", host, len(rawReq))
		return
	}

	// Fix the raw request for direct replay:
	// 1. Replace Host header with actual target host
	// 2. Remove proxy-specific headers (Proxy-Connection, etc.)
	rawReq = fixRawRequestForReplay(rawReq, host, port)

	// Apply overrides from action data
	for key, value := range actionData {
		strVal := fmt.Sprint(value)
		if key == "req.method" {
			lines := strings.SplitN(rawReq, "\r\n", 2)
			if len(lines) >= 2 {
				parts := strings.SplitN(lines[0], " ", 2)
				if len(parts) == 2 {
					rawReq = strVal + " " + parts[1] + "\r\n" + lines[1]
				}
			}
		} else if strings.HasPrefix(key, "req.headers.") {
			headerName := strings.TrimPrefix(key, "req.headers.")
			rawReq = replaceHeaderInRaw(rawReq, headerName, strVal)
		} else if key == "req.body" {
			parts := strings.SplitN(rawReq, "\r\n\r\n", 2)
			rawReq = parts[0] + "\r\n\r\n" + strVal
		}
	}

	index, _ := requestData["index"].(float64)

	go func() {
		if port == "" {
			if useTLS {
				port = "443"
			} else {
				port = "80"
			}
		}
		log.Printf("[TemplateActions][SendRequest] Sending to %s:%s TLS=%v rawLen=%d", host, port, useTLS, len(rawReq))
		scheme := "http"
		if useTLS {
			scheme = "https"
		}
		url := scheme + "://" + host
		if port != "" && port != "80" && port != "443" {
			url += ":" + port
		}

		_, err := backend.sendRepeaterLogic(&RepeaterSendRequest{
			Host:        host,
			Port:        port,
			TLS:         useTLS,
			Request:     rawReq,
			Timeout:     10,
			Index:       index,
			Url:         url,
			GeneratedBy: "template:send_request",
		})
		if err != nil {
			log.Printf("[TemplateActions][SendRequest] Error: %v", err)
			return
		}
		log.Printf("[TemplateActions][SendRequest] Sent request to %s", host)
	}()
}

// fixRawRequestForReplay cleans up a proxy-captured raw request for direct replay:
// - Replaces Host header with the actual target host
// - Removes proxy-specific headers (Proxy-Connection, Proxy-Authorization)
func fixRawRequestForReplay(rawReq string, host string, port string) string {
	// Normalize line endings to \r\n
	rawReq = strings.ReplaceAll(rawReq, "\r\n", "\n")
	rawReq = strings.ReplaceAll(rawReq, "\n", "\r\n")

	// Ensure request ends with \r\n\r\n
	if !strings.HasSuffix(rawReq, "\r\n\r\n") {
		if strings.HasSuffix(rawReq, "\r\n") {
			rawReq += "\r\n"
		} else {
			rawReq += "\r\n\r\n"
		}
	}

	// Build correct Host value
	hostValue := host
	if port != "" && port != "80" && port != "443" {
		hostValue = host + ":" + port
	}

	lines := strings.Split(rawReq, "\r\n")
	var result []string

	for _, line := range lines {
		lower := strings.ToLower(line)

		// Replace Host header
		if strings.HasPrefix(lower, "host:") {
			result = append(result, "Host: "+hostValue)
			continue
		}

		// Remove proxy-specific headers
		if strings.HasPrefix(lower, "proxy-connection:") ||
			strings.HasPrefix(lower, "proxy-authorization:") {
			continue
		}

		result = append(result, line)
	}

	return strings.Join(result, "\r\n")
}

// replaceHeaderInRaw replaces or adds a header in a raw HTTP request string
func replaceHeaderInRaw(rawReq string, headerName string, value string) string {
	lines := strings.Split(rawReq, "\r\n")
	found := false
	lowerName := strings.ToLower(headerName)

	for i, line := range lines {
		if strings.HasPrefix(strings.ToLower(line), lowerName+":") {
			lines[i] = headerName + ": " + value
			found = true
			break
		}
	}

	if !found {
		// Insert before the empty line (end of headers)
		for i, line := range lines {
			if line == "" {
				lines = append(lines[:i+1], lines[i:]...)
				lines[i] = headerName + ": " + value
				break
			}
		}
	}

	return strings.Join(lines, "\r\n")
}
