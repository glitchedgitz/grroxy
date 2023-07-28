package schemas

import "github.com/pocketbase/pocketbase/models/schema"

var Sites = schema.NewSchema(
	&schema.SchemaField{
		Name:     "site",
		Type:     schema.FieldTypeText,
		Required: true,
	},
	&schema.SchemaField{
		Name: "reverse",
		Type: schema.FieldTypeText,
	},
)
