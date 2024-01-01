package schemas

import "github.com/pocketbase/pocketbase/models/schema"

var Sites = schema.NewSchema(
	&schema.SchemaField{
		Name:     "host",
		Type:     schema.FieldTypeText,
		Required: true,
	},
	&schema.SchemaField{
		Name: "smartsort",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "domain",
		Type: schema.FieldTypeText,
	},
)
