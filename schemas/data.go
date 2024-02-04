package schemas

import (
	"github.com/pocketbase/pocketbase/models/schema"
	"github.com/pocketbase/pocketbase/tools/types"
)

var Rows = schema.NewSchema(
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
		Name: "req",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 100000,
		},
	},
	&schema.SchemaField{
		Name: "resp",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 100000,
		},
	},
	&schema.SchemaField{
		Name: "has_resp",
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
		Name: "req_edited",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 100000,
		},
	},
	&schema.SchemaField{
		Name: "resp_edited",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 100000,
		},
	},
	&schema.SchemaField{
		Name:     "raw",
		Type:     schema.FieldTypeRelation,
		Required: true,
		Options: &schema.RelationOptions{
			CollectionId:  "_raw",
			CascadeDelete: true,
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
)
