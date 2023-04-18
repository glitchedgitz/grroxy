package migrations

import "github.com/pocketbase/pocketbase/models/schema"

var Sites = schema.NewSchema(
	&schema.SchemaField{
		Name:     "site",
		Type:     schema.FieldTypeText,
		Unique:   true,
		Required: true,
	},
)
