package schemas

import (
	"github.com/glitchedgitz/pocketbase/models/schema"
)

// Templates (_templates collection)
//   name:        "detect-api-keys"
//   title:       "Detect API Keys"
//   description: "Scans responses for leaked API keys"
//   author:      "glitchedgitz"
//   type:        "action"
//   mode:        "all" | "any"
//   hooks:       {"on_response": ["proxy", "repeater"], "request-action-button": [...]}
//   tasks:       [{id: "check-aws", condition: "...", todo: [...], disabled: false}]
//   enabled:     true
//
// Hook types:
//   Proxy hooks (automatic):
//     on_request       — triggers when proxy receives a request
//     on_response      — triggers when proxy receives a response
//     on_new_sitemap   — triggers when a new sitemap entry is created
//   UI hooks (user-triggered):
//     request-action-button   — button shown on request rows in the UI
//     response-action-button  — button shown on response rows in the UI
//     sitemap-action-button   — button shown on sitemap entries in the UI

var Templates = schema.NewSchema(
	&schema.SchemaField{
		Name: "name",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "title",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "description",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "author",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "type",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "mode",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "hooks",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 100000,
		},
	},
	&schema.SchemaField{
		Name: "tasks",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 500000,
		},
	},
	&schema.SchemaField{
		Name: "enabled",
		Type: schema.FieldTypeBool,
	},
	&schema.SchemaField{
		Name: "global",
		Type: schema.FieldTypeBool,
	},
	&schema.SchemaField{
		Name: "is_default",
		Type: schema.FieldTypeBool,
	},
	&schema.SchemaField{
		Name: "archive",
		Type: schema.FieldTypeBool,
	},
	&schema.SchemaField{
		Name: "projects",
		Type: schema.FieldTypeRelation,
		Options: &schema.RelationOptions{
			CollectionId: "_projects",
		},
	},
)
