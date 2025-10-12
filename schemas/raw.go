package schemas

import (
	"github.com/pocketbase/pocketbase/models/schema"
	"github.com/pocketbase/pocketbase/tools/types"
)

var Raw = schema.NewSchema(
	&schema.SchemaField{
		Name: "req",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "resp",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "req_edited",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "resp_edited",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "attached",
		Type: schema.FieldTypeRelation,
		Options: &schema.RelationOptions{
			CollectionId: "_attached",
			MaxSelect:    types.Pointer(1),
		},
	},
)
