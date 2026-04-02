package migrations

import (
	"encoding/json"
	"log"

	"github.com/glitchedgitz/grroxy/grx/templates"
	"github.com/glitchedgitz/grroxy/grx/templates/defaults"
	"github.com/glitchedgitz/grroxy/internal/schemas"
	"github.com/glitchedgitz/pocketbase/daos"
	m "github.com/glitchedgitz/pocketbase/migrations"
	"github.com/glitchedgitz/pocketbase/models"
	pbTypes "github.com/glitchedgitz/pocketbase/tools/types"
	"github.com/pocketbase/dbx"
	"gopkg.in/yaml.v2"
)

func init() {
	m.Register(func(db dbx.Builder) error {
		dao := daos.New(db)

		collection := &models.Collection{
			Name:       "_templates",
			Type:       models.CollectionTypeBase,
			ListRule:   pbTypes.Pointer(""),
			ViewRule:   pbTypes.Pointer(""),
			CreateRule: pbTypes.Pointer(""),
			UpdateRule: pbTypes.Pointer(""),
			DeleteRule: pbTypes.Pointer(""),
			Schema:     schemas.Templates,
		}

		collection.SetId("_templates")

		if err := dao.SaveCollection(collection); err != nil {
			log.Println("[migration][templates] Error creating collection: ", err)
			return err
		}

		if _, err := dao.DB().NewQuery("CREATE UNIQUE INDEX idx_templates_name ON _templates (name);").Execute(); err != nil {
			log.Printf("[migration][templates] Error creating index: %v\n", err)
			return err
		}

		// Seed default templates from embedded YAML files
		entries, err := defaults.FS.ReadDir(".")
		if err != nil {
			log.Printf("[migration][templates] Error reading embedded defaults: %v", err)
			return nil
		}

		for _, entry := range entries {
			if entry.IsDir() || entry.Name() == "embed.go" {
				continue
			}

			data, err := defaults.FS.ReadFile(entry.Name())
			if err != nil {
				log.Printf("[migration][templates] Error reading %s: %v", entry.Name(), err)
				continue
			}

			var tmpl templates.Template
			if err := yaml.Unmarshal(data, &tmpl); err != nil {
				log.Printf("[migration][templates] Error parsing %s: %v", entry.Name(), err)
				continue
			}

			record := models.NewRecord(collection)
			record.Set("name", tmpl.Id)
			record.Set("title", tmpl.Info.Title)
			record.Set("description", tmpl.Info.Description)
			record.Set("author", tmpl.Info.Author)
			record.Set("type", tmpl.Config.Type)
			record.Set("mode", tmpl.Config.Mode)
			record.Set("hooks", tmpl.Config.Hooks)
			record.Set("tasks", tmpl.Tasks)
			record.Set("enabled", true)
			record.Set("global", true)
			record.Set("is_default", true)
			record.Set("archive", false)

			if err := dao.SaveRecord(record); err != nil {
				log.Printf("[migration][templates] Error seeding %s: %v", tmpl.Id, err)
			} else {
				log.Printf("[migration][templates] Seeded default template: %s", tmpl.Id)
			}
		}

		// Enable templatesEnabled on all existing projects
		projects, err := dao.FindRecordsByExpr("_projects")
		if err == nil {
			for _, project := range projects {
				existing := make(map[string]any)
				if raw, err := json.Marshal(project.Get("data")); err == nil {
					json.Unmarshal(raw, &existing)
				}
				if _, ok := existing["templatesEnabled"]; !ok {
					existing["templatesEnabled"] = true
					project.Set("data", existing)
					if err := dao.SaveRecord(project); err != nil {
						log.Printf("[migration][templates] Error enabling templates for project %s: %v", project.Id, err)
					} else {
						log.Printf("[migration][templates] Enabled templates for project: %s", project.Id)
					}
				}
			}
		}

		// Seed global templates toggle in _configs
		configCollection, err := dao.FindCollectionByNameOrId("_configs")
		if err == nil {
			configRecord := models.NewRecord(configCollection)
			configRecord.Set("key", "settings.templatesEnabled")
			configRecord.Set("data", true)
			if err := dao.SaveRecord(configRecord); err != nil {
				log.Printf("[migration][templates] Error seeding config: %v", err)
			} else {
				log.Println("[migration][templates] Seeded settings.templatesEnabled = true")
			}
		}

		log.Println("[migration][templates] Successfully created _templates collection with defaults")
		return nil
	}, func(db dbx.Builder) error {
		dao := daos.New(db)

		collection, err := dao.FindCollectionByNameOrId("_templates")
		if err != nil {
			return nil // collection doesn't exist, nothing to revert
		}

		return dao.DeleteCollection(collection)
	})
}
