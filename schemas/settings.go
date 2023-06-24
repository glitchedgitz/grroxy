package schemas

import "github.com/pocketbase/pocketbase/models/schema"

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
