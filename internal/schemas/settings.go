package schemas

import "github.com/glitchedgitz/pocketbase/models/schema"

var Settings = schema.NewSchema(
	&schema.SchemaField{
		Name:     "option",
		Type:     schema.FieldTypeText,
		Required: true,
	},
	&schema.SchemaField{
		Name: "value",
		Type: schema.FieldTypeText,
	},
)

var ConfigSchema = schema.NewSchema(
	&schema.SchemaField{
		Name:     "key",
		Type:     schema.FieldTypeText,
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
