package schemas

import "github.com/glitchedgitz/pocketbase/models/schema"

var ToolsSchema = schema.NewSchema(
	&schema.SchemaField{
		Name:     "name",
		Type:     schema.FieldTypeText,
		Required: true,
	},
	&schema.SchemaField{
		Name:     "path",
		Type:     schema.FieldTypeText,
		Required: true,
	},
	&schema.SchemaField{
		Name:     "host",
		Type:     schema.FieldTypeText,
		Required: true,
	},
	&schema.SchemaField{
		Name:     "state",
		Type:     schema.FieldTypeText,
		Required: true,
	},
	&schema.SchemaField{
		Name:     "creds",
		Type:     schema.FieldTypeJson,
		Required: true,
	},
	&schema.SchemaField{
		Name: "data",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 100000,
		},
	},
)
