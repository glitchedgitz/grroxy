package schemas

import "github.com/glitchedgitz/pocketbase/models/schema"

var Wordlists = schema.NewSchema(
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
		Name: "data",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 100000,
		},
	},
)
