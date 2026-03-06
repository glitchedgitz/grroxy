package schemas

import "github.com/glitchedgitz/pocketbase/models/schema"

var UI = schema.NewSchema(
	&schema.SchemaField{
		Name:     "unique_id",
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
