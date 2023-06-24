package schemas

import "github.com/pocketbase/pocketbase/models/schema"

var Store = schema.NewSchema(
	&schema.SchemaField{
		Name: "request",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "response",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "request_edited",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "response_edited",
		Type: schema.FieldTypeText,
	},
)
