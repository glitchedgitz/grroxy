package schemas

import "github.com/glitchedgitz/pocketbase/models/schema"

var Playground = schema.NewSchema(
	&schema.SchemaField{
		Name:     "name",
		Type:     schema.FieldTypeText,
		Required: true,
	},
	&schema.SchemaField{
		Name: "parent_id",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "original_req_id",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "type",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "expanded",
		Type: schema.FieldTypeBool,
	},
	&schema.SchemaField{
		Name: "state",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "sort_order",
		Type: schema.FieldTypeNumber,
	},
	&schema.SchemaField{
		Name: "data",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 100000,
		},
	},
	&schema.SchemaField{
		Name: "extra",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 100000,
		},
	},
)

var RepeaterTabSchema = schema.NewSchema(
	&schema.SchemaField{
		Name: "url",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "req",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "resp",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name:    "extra",
		Type:    schema.FieldTypeJson,
		Options: &schema.JsonOptions{MaxSize: 100000},
	},
)

var IntruderTabSchema = schema.NewSchema(
	&schema.SchemaField{
		Name: "url",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "req",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "payload",
		Type: schema.FieldTypeText,
	},
)

var IntruderTabResultsSchema = schema.NewSchema(
	&schema.SchemaField{
		Name:    "data",
		Type:    schema.FieldTypeJson,
		Options: &schema.JsonOptions{MaxSize: 100000},
	},
)
