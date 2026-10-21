package schemas

import (
	"github.com/glitchedgitz/pocketbase/models/schema"
	"github.com/glitchedgitz/pocketbase/tools/types"
)

var Intercept = schema.NewSchema(
	&schema.SchemaField{
		Name: "index",
		Type: schema.FieldTypeNumber,
	},
	&schema.SchemaField{
		Name: "host",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "port",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "has_params",
		Type: schema.FieldTypeBool,
	},
	&schema.SchemaField{
		Name: "has_resp",
		Type: schema.FieldTypeBool,
	},
	&schema.SchemaField{
		Name: "is_https",
		Type: schema.FieldTypeBool,
	},
	&schema.SchemaField{
		Name: "is_req_edited",
		Type: schema.FieldTypeBool,
	},
	&schema.SchemaField{
		Name: "is_resp_edited",
		Type: schema.FieldTypeBool,
	},
	&schema.SchemaField{
		Name: "req",
		Type: schema.FieldTypeRelation,
		Options: &schema.RelationOptions{
			CollectionId:  "_req",
			CascadeDelete: true,
			MaxSelect:     types.Pointer(1),
		},
	},
	&schema.SchemaField{
		Name: "resp",
		Type: schema.FieldTypeRelation,
		Options: &schema.RelationOptions{
			CollectionId:  "_resp",
			CascadeDelete: true,
			MaxSelect:     types.Pointer(1),
		},
	},
	&schema.SchemaField{
		Name: "req_edited",
		Type: schema.FieldTypeRelation,
		Options: &schema.RelationOptions{
			CollectionId:  "_req_edited",
			CascadeDelete: true,
			MaxSelect:     types.Pointer(1),
		},
	},
	&schema.SchemaField{
		Name: "resp_edited",
		Type: schema.FieldTypeRelation,
		Options: &schema.RelationOptions{
			CollectionId:  "_resp_edited",
			CascadeDelete: true,
			MaxSelect:     types.Pointer(1),
		},
	},
	&schema.SchemaField{
		Name: "req_json",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 100000,
		},
	},
	&schema.SchemaField{
		Name: "resp_json",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 100000,
		},
	},
	&schema.SchemaField{
		Name: "req_edited_json",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 100000,
		},
	},
	&schema.SchemaField{
		Name: "resp_edited_json",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 100000,
		},
	},
	&schema.SchemaField{
		Name: "attached",
		Type: schema.FieldTypeRelation,
		Options: &schema.RelationOptions{
			CollectionId:  "_attached",
			CascadeDelete: true,
			MaxSelect:     types.Pointer(1),
		},
	},
	&schema.SchemaField{
		Name: "generated_by",
		Type: schema.FieldTypeText,
	},
	// generated_by split into fields, see docs/generated_by.md. Added by
	// migration 1792550000_origin_fields for new and old projects alike, so they
	// stay out of the init schema.
	// &schema.SchemaField{
	// 	Name: "type",
	// 	Type: schema.FieldTypeText,
	// },
	// &schema.SchemaField{
	// 	Name: "actor",
	// 	Type: schema.FieldTypeText,
	// },
	// &schema.SchemaField{
	// 	Name: "proxy",
	// 	Type: schema.FieldTypeText,
	// },
	// &schema.SchemaField{
	// 	Name: "chat",
	// 	Type: schema.FieldTypeText,
	// },
	&schema.SchemaField{
		Name: "extra",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 100000,
		},
	},
	&schema.SchemaField{
		Name: "attached",
		Type: schema.FieldTypeRelation,
		Options: &schema.RelationOptions{
			CollectionId: "_attached",
			MaxSelect:    types.Pointer(1),
		},
	},
	&schema.SchemaField{
		Name: "action",
		Type: schema.FieldTypeText,
	},
)
