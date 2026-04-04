package launcher

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/glitchedgitz/pocketbase/core"
)

// SetupTemplateHooks watches _templates and _configs for changes and notifies running projects
func (launcher *Launcher) SetupTemplateHooks() error {
	notifyReload := func() {
		go launcher.notifyProjectsReloadTemplates()
	}

	launcher.App.OnRecordAfterCreateRequest("_templates").Add(func(e *core.RecordCreateEvent) error {
		log.Printf("[TemplateHooks] Template created: %s", e.Record.GetString("name"))
		notifyReload()
		return nil
	})

	launcher.App.OnRecordAfterUpdateRequest("_templates").Add(func(e *core.RecordUpdateEvent) error {
		log.Printf("[TemplateHooks] Template updated: %s", e.Record.GetString("name"))
		notifyReload()
		return nil
	})

	launcher.App.OnRecordAfterDeleteRequest("_templates").Add(func(e *core.RecordDeleteEvent) error {
		log.Printf("[TemplateHooks] Template deleted: %s", e.Record.GetString("name"))
		notifyReload()
		return nil
	})

	// Watch _projects for templatesEnabled changes in data field
	launcher.App.OnRecordAfterUpdateRequest("_projects").Add(func(e *core.RecordUpdateEvent) error {
		dataMap := make(map[string]any)
		data := e.Record.Get("data")
		switch v := data.(type) {
		case map[string]any:
			dataMap = v
		case string:
			json.Unmarshal([]byte(v), &dataMap)
		default:
			if raw, err := json.Marshal(v); err == nil {
				json.Unmarshal(raw, &dataMap)
			}
		}
		if len(dataMap) == 0 {
			return nil
		}

		state, _ := dataMap["state"].(string)
		ip, _ := dataMap["ip"].(string)
		if !strings.EqualFold(state, "active") || ip == "" {
			return nil
		}

		enabled, _ := dataMap["templatesEnabled"].(bool)
		log.Printf("[TemplateHooks] Project %s templatesEnabled: %v", e.Record.Id, enabled)

		body, _ := json.Marshal(map[string]bool{"enabled": enabled})
		go func() {
			url := fmt.Sprintf("http://%s/api/templates/global-toggle", ip)
			resp, err := http.Post(url, "application/json", bytes.NewReader(body))
			if err != nil {
				log.Printf("[TemplateHooks] Error toggling project %s: %v", e.Record.Id, err)
				return
			}
			resp.Body.Close()
		}()

		return nil
	})

	log.Println("[TemplateHooks] Template and config change hooks registered")
	return nil
}

// notifyProjectsReloadTemplates calls /api/templates/reload on all running projects
func (launcher *Launcher) notifyProjectsReloadTemplates() {
	launcher.forEachRunningProject(func(ip string, projectId string) {
		url := fmt.Sprintf("http://%s/api/templates/reload", ip)
		resp, err := http.Post(url, "application/json", nil)
		if err != nil {
			log.Printf("[TemplateHooks] Error notifying project %s at %s: %v", projectId, ip, err)
			return
		}
		resp.Body.Close()
		log.Printf("[TemplateHooks] Notified project %s at %s to reload templates", projectId, ip)
	})
}

// forEachRunningProject iterates over active projects and calls fn with their IP and ID
func (launcher *Launcher) forEachRunningProject(fn func(ip string, projectId string)) {
	records, err := launcher.App.Dao().FindRecordsByExpr("_projects")
	if err != nil {
		log.Printf("[TemplateHooks] Error fetching projects: %v", err)
		return
	}

	for _, record := range records {
		dataMap := make(map[string]any)
		data := record.Get("data")
		switch v := data.(type) {
		case map[string]any:
			dataMap = v
		case string:
			json.Unmarshal([]byte(v), &dataMap)
		default:
			// Try marshaling and unmarshaling for types.JsonRaw etc
			if raw, err := json.Marshal(v); err == nil {
				json.Unmarshal(raw, &dataMap)
			}
		}

		state, _ := dataMap["state"].(string)
		ip, _ := dataMap["ip"].(string)

		if !strings.EqualFold(state, "active") || ip == "" {
			continue
		}

		fn(ip, record.Id)
	}
}
