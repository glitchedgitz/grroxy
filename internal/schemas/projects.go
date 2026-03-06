package schemas

import "github.com/glitchedgitz/pocketbase/models/schema"

var Projects = schema.NewSchema(
	&schema.SchemaField{
		Name: "name",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "path",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "data",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 100000,
		},
	},
	&schema.SchemaField{
		Name: "version",
		Type: schema.FieldTypeText,
	},
)
