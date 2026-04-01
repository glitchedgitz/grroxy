package app

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/glitchedgitz/grroxy/grx/templates"
)

// LoadTemplatesFromLauncher fetches templates from the launcher's API
func (backend *Backend) LoadTemplatesFromLauncher(launcherAddr string) error {
	if launcherAddr == "" {
		log.Println("[TemplateLoader] No launcher address provided, skipping")
		return nil
	}

	url := fmt.Sprintf("http://%s/api/templates/list", launcherAddr)
	log.Printf("[TemplateLoader] Fetching templates from %s", url)

	resp, err := http.Get(url)
	if err != nil {
		log.Printf("[TemplateLoader] Error fetching from launcher: %v", err)
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[TemplateLoader] Error reading response: %v", err)
		return nil
	}

	var result struct {
		List []json.RawMessage `json:"list"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("[TemplateLoader] Error parsing response: %v", err)
		return nil
	}

	for _, raw := range result.List {
		var record map[string]any
		if err := json.Unmarshal(raw, &record); err != nil {
			continue
		}

		enabled, _ := record["enabled"].(bool)
		if !enabled {
			continue
		}

		tmpl := &templates.Template{
			Id: getString(record, "id"),
			Info: templates.Info{
				Title:       getString(record, "title"),
				Description: getString(record, "description"),
				Author:      getString(record, "author"),
			},
			Config: templates.Config{
				Type: getString(record, "type"),
				Mode: getString(record, "mode"),
			},
		}

		// Parse hooks
		if hooksRaw, ok := record["hooks"]; ok && hooksRaw != nil {
			if hooksBytes, err := json.Marshal(hooksRaw); err == nil {
				json.Unmarshal(hooksBytes, &tmpl.Config.Hooks)
			}
		}

		// Parse tasks
		if tasksRaw, ok := record["tasks"]; ok && tasksRaw != nil {
			if tasksBytes, err := json.Marshal(tasksRaw); err == nil {
				json.Unmarshal(tasksBytes, &tmpl.Tasks)
			}
		}

		backend.Templates.LoadTemplate(tmpl)
		log.Printf("[TemplateLoader] Template %s: hooks=%v tasks=%d", tmpl.Id, tmpl.Config.Hooks, len(tmpl.Tasks))
	}

	log.Printf("[TemplateLoader] Loaded %d templates from launcher", len(backend.Templates.Templates))
	return nil
}

// LoadTemplatesEnabledFromLauncher checks both global (_configs) and per-project (_projects) settings
func (backend *Backend) LoadTemplatesEnabledFromLauncher(launcherAddr string) {
	if launcherAddr == "" {
		return
	}

	// Check global setting from _configs
	globalEnabled := false
	globalURL := fmt.Sprintf("http://%s/api/collections/_configs/records?filter=(key='settings.templatesEnabled')", launcherAddr)
	if resp, err := http.Get(globalURL); err == nil {
		defer resp.Body.Close()
		if body, err := io.ReadAll(resp.Body); err == nil {
			var result struct {
				Items []struct {
					Data json.RawMessage `json:"data"`
				} `json:"items"`
			}
			if json.Unmarshal(body, &result) == nil && len(result.Items) > 0 {
				json.Unmarshal(result.Items[0].Data, &globalEnabled)
			}
		}
	}
	log.Printf("[TemplateLoader] Global templates enabled: %v", globalEnabled)

	if !globalEnabled {
		backend.TemplatesEnabled = false
		log.Printf("[TemplateLoader] Templates disabled (global off)")
		return
	}

	// Check per-project setting from _projects
	if backend.Config.ProjectID == "" {
		backend.TemplatesEnabled = globalEnabled
		return
	}

	projectURL := fmt.Sprintf("http://%s/api/collections/_projects/records/%s", launcherAddr, backend.Config.ProjectID)
	if resp, err := http.Get(projectURL); err == nil {
		defer resp.Body.Close()
		if body, err := io.ReadAll(resp.Body); err == nil {
			var record struct {
				Data json.RawMessage `json:"data"`
			}
			if json.Unmarshal(body, &record) == nil {
				var dataMap map[string]any
				if json.Unmarshal(record.Data, &dataMap) == nil {
					if enabled, ok := dataMap["templatesEnabled"].(bool); ok {
						backend.TemplatesEnabled = enabled
						log.Printf("[TemplateLoader] Project templates enabled: %v", enabled)
						return
					}
				}
			}
		}
	}

	// If no per-project setting, inherit global
	backend.TemplatesEnabled = globalEnabled
	log.Printf("[TemplateLoader] Templates enabled (inherited from global): %v", globalEnabled)
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
