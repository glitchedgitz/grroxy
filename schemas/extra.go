package schemas

import "github.com/pocketbase/pocketbase/models/schema"

var Attached = schema.NewSchema(
	&schema.SchemaField{
		Name: "label",
		Type: schema.FieldTypeJson,
	},
	&schema.SchemaField{
		Name: "note",
		Type: schema.FieldTypeEditor,
	},
	&schema.SchemaField{
		Name: "extra",
		Type: schema.FieldTypeJson,
	},
)
