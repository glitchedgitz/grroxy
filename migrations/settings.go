package migrations

import "github.com/pocketbase/pocketbase/models/schema"

var Settings = schema.NewSchema(
	&schema.SchemaField{
		Name:     "option",
		Type:     schema.FieldTypeText,
		Unique:   true,
		Required: true,
	},
	&schema.SchemaField{
		Name: "value",
		Type: schema.FieldTypeText,
	},
)
