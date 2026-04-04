package launcher

import (
	"fmt"
	"log"
	"net/http"

	"github.com/glitchedgitz/grroxy/grx/templates"
	"github.com/glitchedgitz/grroxy/grx/templates/actions"
	"github.com/glitchedgitz/pocketbase/apis"
	"github.com/glitchedgitz/pocketbase/core"
	"github.com/glitchedgitz/pocketbase/models"
	"github.com/labstack/echo/v5"
	"gopkg.in/yaml.v2"
)

func (launcher *Launcher) TemplatesList(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: "GET",
		Path:   "/api/templates/list",
		Handler: func(c echo.Context) error {
			records, err := launcher.App.Dao().FindRecordsByExpr("_templates")
			if err != nil {
				log.Printf("[TemplatesList] Error: %v", err)
				return c.JSON(http.StatusOK, map[string]any{"list": []any{}})
			}

			list := make([]map[string]any, 0, len(records))
			for _, record := range records {
				list = append(list, map[string]any{
					"id":          record.Id,
					"name":        record.GetString("name"),
					"title":       record.GetString("title"),
					"description": record.GetString("description"),
					"author":      record.GetString("author"),
					"type":        record.GetString("type"),
					"mode":        record.GetString("mode"),
					"hooks":       record.Get("hooks"),
					"tasks":       record.Get("tasks"),
					"enabled":     record.GetBool("enabled"),
					"global":      record.GetBool("global"),
					"is_default":  record.GetBool("is_default"),
					"archive":     record.GetBool("archive"),
				})
			}

			return c.JSON(http.StatusOK, map[string]any{"list": list})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(launcher.App),
		},
	})

	return nil
}

func (launcher *Launcher) TemplatesNew(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: "POST",
		Path:   "/api/templates/new",
		Handler: func(c echo.Context) error {
			var data map[string]any
			if err := c.Bind(&data); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
			}

			collection, err := launcher.App.Dao().FindCollectionByNameOrId("_templates")
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}

			record := models.NewRecord(collection)
			for key, value := range data {
				record.Set(key, value)
			}

			if err := launcher.App.Dao().SaveRecord(record); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}

			return c.JSON(http.StatusOK, map[string]any{
				"id": record.Id,
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(launcher.App),
		},
	})

	return nil
}

func (launcher *Launcher) TemplatesDelete(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: "DELETE",
		Path:   "/api/templates/:template",
		Handler: func(c echo.Context) error {
			id := c.PathParam("template")

			record, err := launcher.App.Dao().FindRecordById("_templates", id)
			if err != nil {
				return c.JSON(http.StatusNotFound, map[string]string{"error": "Template not found"})
			}

			if err := launcher.App.Dao().DeleteRecord(record); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}

			return c.JSON(http.StatusOK, map[string]string{"success": "true"})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(launcher.App),
		},
	})

	return nil
}

func (launcher *Launcher) TemplatesInfo(e *core.ServeEvent) error {
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
			apis.ActivityLogger(launcher.App),
		},
	})

	return nil
}

func (launcher *Launcher) TemplatesCheck(e *core.ServeEvent) error {
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
						errors = append(errors, "Unknown hook '"+h+"' in group '"+hookGroup+"'")
					}
				}
			}

			if len(tmpl.Config.Hooks) == 0 {
				warnings = append(warnings, "No hooks defined — template won't run automatically")
			}

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
							errors = append(errors, fmt.Sprintf("Task '%s', todo %d: unknown action '%s'", task.Id, j, actionName))
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
			apis.ActivityLogger(launcher.App),
		},
	})

	return nil
}
