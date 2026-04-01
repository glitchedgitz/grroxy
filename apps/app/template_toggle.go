package app

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/glitchedgitz/pocketbase/apis"
	"github.com/glitchedgitz/pocketbase/core"
	"github.com/labstack/echo/v5"
)

type TemplateGlobalToggleBody struct {
	Enabled bool `json:"enabled"`
}

func (backend *Backend) TemplateGlobalToggle(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/templates/global-toggle",
		Handler: func(c echo.Context) error {
			var body TemplateGlobalToggleBody
			if err := c.Bind(&body); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
			}

			backend.TemplatesEnabled = body.Enabled
			log.Printf("[TemplateGlobalToggle] Templates globally set to: %v", body.Enabled)

			return c.JSON(http.StatusOK, map[string]any{
				"templates_enabled": backend.TemplatesEnabled,
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})

	// GET endpoint to check current state
	e.Router.AddRoute(echo.Route{
		Method: http.MethodGet,
		Path:   "/api/templates/global-status",
		Handler: func(c echo.Context) error {
			return c.JSON(http.StatusOK, map[string]any{
				"templates_enabled": backend.TemplatesEnabled,
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})

	return nil
}

type TemplateToggleBody struct {
	ProxyID string `json:"proxy_id"`
	Enabled bool   `json:"enabled"`
}

func (backend *Backend) TemplateToggle(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/templates/toggle",
		Handler: func(c echo.Context) error {
			var body TemplateToggleBody
			if err := c.Bind(&body); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
			}

			if body.ProxyID == "" {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "proxy_id is required"})
			}

			// Update the proxy record in DB
			record, err := backend.App.Dao().FindRecordById("_proxies", body.ProxyID)
			if err != nil {
				return c.JSON(http.StatusNotFound, map[string]string{"error": fmt.Sprintf("Proxy %s not found", body.ProxyID)})
			}

			// Update run_templates inside the data JSON field
			data, _ := record.Get("data").(map[string]any)
			if data == nil {
				data = make(map[string]any)
			}
			data["run_templates"] = body.Enabled
			record.Set("data", data)
			if err := backend.App.Dao().SaveRecord(record); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}

			// Update the running proxy instance
			ProxyMgr.mu.RLock()
			inst := ProxyMgr.instances[body.ProxyID]
			ProxyMgr.mu.RUnlock()

			if inst != nil && inst.Proxy != nil {
				inst.Proxy.RunTemplates = body.Enabled
				log.Printf("[TemplateToggle] Proxy %s templates set to: %v", body.ProxyID, body.Enabled)
			}

			return c.JSON(http.StatusOK, map[string]any{
				"proxy_id":      body.ProxyID,
				"run_templates": body.Enabled,
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})

	return nil
}

// TemplateReload reloads all templates from the launcher
func (backend *Backend) TemplateReload(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/templates/reload",
		Handler: func(c echo.Context) error {
			backend.Templates.Setup()
			if err := backend.LoadTemplatesFromLauncher(backend.Config.LauncherAddr); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, map[string]any{
				"loaded": len(backend.Templates.Templates),
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

// SetupTemplateHooks monitors run_templates changes on _proxies records
func (backend *Backend) SetupTemplateHooks() error {
	backend.App.OnRecordAfterUpdateRequest("_proxies").Add(func(e *core.RecordUpdateEvent) error {
		proxyDBID := e.Record.Id
		var runTemplates bool
		dataRaw := e.Record.Get("data")
		log.Printf("[TemplateHooks] Proxy %s data raw type: %T value: %v", proxyDBID, dataRaw, dataRaw)
		// PocketBase JSON fields can come as different types
		switch v := dataRaw.(type) {
		case map[string]any:
			runTemplates, _ = v["run_templates"].(bool)
		default:
			// Try JSON unmarshal for types.JsonRaw or string
			var dataMap map[string]any
			if bytes, err := json.Marshal(v); err == nil {
				if err := json.Unmarshal(bytes, &dataMap); err == nil {
					runTemplates, _ = dataMap["run_templates"].(bool)
				}
			}
		}

		ProxyMgr.mu.RLock()
		inst := ProxyMgr.instances[proxyDBID]
		ProxyMgr.mu.RUnlock()

		if inst == nil || inst.Proxy == nil {
			return nil
		}

		inst.Proxy.RunTemplates = runTemplates
		log.Printf("[TemplateHooks] Proxy %s run_templates changed to: %v", proxyDBID, runTemplates)

		return nil
	})

	return nil
}
