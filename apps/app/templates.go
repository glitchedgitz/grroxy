package app

import (
	"fmt"
	"net/http"

	"github.com/glitchedgitz/grroxy/grx/templates"
	"github.com/glitchedgitz/grroxy/grx/templates/actions"
	"github.com/glitchedgitz/pocketbase/apis"
	"github.com/glitchedgitz/pocketbase/core"
	"github.com/glitchedgitz/pocketbase/models"
	"github.com/labstack/echo/v5"
	"gopkg.in/yaml.v2"
)

type Path struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
}

func (backend *Backend) TemplatesList(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: "GET",
		Path:   "/api/templates/list",
		Handler: func(c echo.Context) error {
			if backend.Templates == nil {
				return c.JSON(http.StatusOK, map[string]any{"list": []any{}})
			}

			list := make([]map[string]any, 0, len(backend.Templates.Templates))
			for _, tmpl := range backend.Templates.Templates {
				list = append(list, map[string]any{
					"id":          tmpl.Id,
					"name":        tmpl.Id,
					"title":       tmpl.Info.Title,
					"description": tmpl.Info.Description,
					"author":      tmpl.Info.Author,
					"type":        tmpl.Config.Type,
					"mode":        tmpl.Config.Mode,
					"hooks":       tmpl.Config.Hooks,
					"tasks":       tmpl.Tasks,
				})
			}

			return c.JSON(http.StatusOK, map[string]any{"list": list})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})

	return nil
}

func (backend *Backend) TemplatesNew(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: "POST",
		Path:   "/api/templates/new",
		Handler: func(c echo.Context) error {
			var data map[string]any
			if err := c.Bind(&data); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
			}

			collection, err := backend.App.Dao().FindCollectionByNameOrId("_templates")
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}

			record := models.NewRecord(collection)
			for key, value := range data {
				record.Set(key, value)
			}

			if err := backend.App.Dao().SaveRecord(record); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}

			// Reload into the running engine if enabled
			if record.GetBool("enabled") {
				backend.LoadTemplatesFromLauncher(backend.Config.LauncherAddr)
			}

			return c.JSON(http.StatusOK, map[string]any{
				"id": record.Id,
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})

	return nil
}

func (backend *Backend) TemplatesDelete(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: "DELETE",
		Path:   "/api/templates/:template",
		Handler: func(c echo.Context) error {
			id := c.PathParam("template")

			record, err := backend.App.Dao().FindRecordById("_templates", id)
			if err != nil {
				return c.JSON(http.StatusNotFound, map[string]string{"error": "Template not found"})
			}

			if err := backend.App.Dao().DeleteRecord(record); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}

			// Remove from running engine
			backend.Templates.RemoveTemplate(id)

			return c.JSON(http.StatusOK, map[string]string{"success": "true"})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})

	return nil
}

func (backend *Backend) TemplatesCheck(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: "POST",
		Path:   "/api/templates/check",
		Handler: func(c echo.Context) error {
			var body struct {
				Yaml string `json:"yaml"`
			}
			if err := c.Bind(&body); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]any{"error": "Invalid request body"})
			}

			var tmpl struct {
				Id     string `yaml:"id"`
				Config struct {
					Hooks map[string][]string `yaml:"hooks"`
				} `yaml:"config"`
				Tasks []struct {
					Id   string                      `yaml:"id"`
					Todo []map[string]map[string]any `yaml:"todo"`
				} `yaml:"tasks"`
			}

			if err := yaml.Unmarshal([]byte(body.Yaml), &tmpl); err != nil {
				return c.JSON(http.StatusOK, map[string]any{
					"valid":  false,
					"errors": []string{"YAML parse error: " + err.Error()},
				})
			}

			errors := []string{}
			warnings := []string{}

			if tmpl.Id == "" {
				errors = append(errors, "Missing 'id' field")
			}

			// Validate hooks
			for hookGroup, hookList := range tmpl.Config.Hooks {
				validHooks, exists := actions.ValidHooks[hookGroup]
				if !exists {
					errors = append(errors, "Unknown hook group: "+hookGroup)
					continue
				}
				for _, h := range hookList {
					found := false
					for _, v := range validHooks {
						if v == h {
							found = true
							break
						}
					}
					if !found {
						errors = append(errors, "Unknown hook '"+h+"' in group '"+hookGroup+"'. Valid: "+joinStrings(validHooks))
					}
				}
			}

			if len(tmpl.Config.Hooks) == 0 {
				warnings = append(warnings, "No hooks defined — template won't run automatically")
			}

			// Validate tasks
			if len(tmpl.Tasks) == 0 {
				errors = append(errors, "No tasks defined")
			}

			for i, task := range tmpl.Tasks {
				if task.Id == "" {
					warnings = append(warnings, fmt.Sprintf("Task %d has no id", i))
				}
				for j, todoItem := range task.Todo {
					for actionName, data := range todoItem {
						_, exists := actions.ValidActions[actionName]
						if !exists {
							errors = append(errors, fmt.Sprintf("Task '%s', todo %d: unknown action '%s'. Valid: %s", task.Id, j, actionName, joinStrings(validActionNames())))
							continue
						}
						if actionName == "create_label" {
							if _, ok := data["name"]; !ok {
								errors = append(errors, fmt.Sprintf("Task '%s', todo %d: 'create_label' requires 'name'", task.Id, j))
							}
						}
						if actionName == "replace" {
							for _, key := range []string{"search", "value"} {
								if _, ok := data[key]; !ok {
									errors = append(errors, fmt.Sprintf("Task '%s', todo %d: 'replace' requires '%s'", task.Id, j, key))
								}
							}
						}
					}
				}
			}

			return c.JSON(http.StatusOK, map[string]any{
				"valid":    len(errors) == 0,
				"errors":   errors,
				"warnings": warnings,
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})

	return nil
}

func (backend *Backend) TemplatesInfo(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: "GET",
		Path:   "/api/templates/info",
		Handler: func(c echo.Context) error {
			return c.JSON(http.StatusOK, map[string]any{
				"actions":   actions.ActionRegistry,
				"hooks":     actions.HookRegistry,
				"modes":     actions.ModeRegistry,
				"reference": templates.TemplateReference,
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})

	return nil
}

// TemplateActionButtons returns templates with request-action-button or response-action-button hooks
func (backend *Backend) TemplateActionButtons(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/templates/action-buttons",
		Handler: func(c echo.Context) error {
			var body struct {
				Hook string `json:"hook"`
			}
			if err := c.Bind(&body); err != nil {
				body.Hook = "request-action-button"
			}
			hookType := body.Hook
			if hookType == "" {
				hookType = "request-action-button"
			}

			if backend.Templates == nil {
				return c.JSON(http.StatusOK, map[string]any{"list": []any{}})
			}

			list := make([]map[string]any, 0)
			for _, tmpl := range backend.Templates.Templates {
				if _, ok := tmpl.Config.Hooks[hookType]; ok {
					list = append(list, map[string]any{
						"id":          tmpl.Id,
						"title":       tmpl.Info.Title,
						"description": tmpl.Info.Description,
						"tasks":       tmpl.Tasks,
						"mode":        tmpl.Config.Mode,
					})
				}
			}

			return c.JSON(http.StatusOK, map[string]any{"list": list})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})

	return nil
}

// TemplateRunAction executes a specific template's tasks on provided request data
func (backend *Backend) TemplateRunAction(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: "POST",
		Path:   "/api/templates/run-action",
		Handler: func(c echo.Context) error {
			var body struct {
				TemplateID string         `json:"template_id"`
				Request    string         `json:"request"`
				Url        string         `json:"url"`
				RowID      string         `json:"row_id"`
				Data       map[string]any `json:"data"`
			}
			if err := c.Bind(&body); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
			}

			if backend.Templates == nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Templates not initialized"})
			}

			tmpl, ok := backend.Templates.Templates[body.TemplateID]
			if !ok {
				return c.JSON(http.StatusNotFound, map[string]string{"error": "Template not found"})
			}

			// Build data map for template execution
			data := body.Data
			if data == nil {
				data = make(map[string]any)
			}
			if body.RowID != "" {
				data["id"] = body.RowID
			}

			// Run the template's tasks with ParseTemplateActions
			results, err := templates.ParseTemplateActions(tmpl.Tasks, data, tmpl.Config.Mode)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}

			// Separate modification actions from side-effect actions
			var modifyTasks []templates.Action
			var sideEffects []templates.Action
			for _, action := range results {
				switch action.ActionName {
				case actions.Set, actions.Delete, actions.Replace:
					modifyTasks = append(modifyTasks, action)
				default:
					sideEffects = append(sideEffects, action)
				}
			}

			// Apply modification actions to the raw request if provided
			var modifiedRequest string
			if body.Request != "" && len(modifyTasks) > 0 {
				modifiedRequest, _ = runActions(modifyTasks, map[string]any{
					"method":  "",
					"url":     body.Url,
					"path":    "",
					"query":   "",
					"headers": [][]string{},
					"raw":     body.Request,
				})
			}

			// Execute side-effect actions (create_label, send_request, etc.)
			if len(sideEffects) > 0 {
				go backend.ExecuteTemplateActions(sideEffects, data)
			}

			return c.JSON(http.StatusOK, map[string]any{
				"request":      modifiedRequest,
				"actions_run":  len(results),
				"modified":     len(modifyTasks) > 0,
				"side_effects": len(sideEffects),
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})

	return nil
}

func validActionNames() []string {
	names := make([]string, 0, len(actions.ValidActions))
	for k := range actions.ValidActions {
		names = append(names, k)
	}
	return names
}

func joinStrings(s []string) string {
	result := ""
	for i, v := range s {
		if i > 0 {
			result += ", "
		}
		result += v
	}
	return result
}
