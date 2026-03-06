package schemas

import "github.com/glitchedgitz/pocketbase/models/schema"

var Attached = schema.NewSchema(
	&schema.SchemaField{
		Name: "labels",
		Type: schema.FieldTypeRelation,
		Options: &schema.RelationOptions{
			CollectionId: "_labels",
		},
	},
	&schema.SchemaField{
		Name: "note",
		Type: schema.FieldTypeEditor,
	},
	&schema.SchemaField{
		Name: "extra",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 100000,
		},
	},
)
