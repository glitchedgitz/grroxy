package migrations

import "github.com/pocketbase/pocketbase/models/schema"

var Intercept = schema.NewSchema(
	&schema.SchemaField{
		Name: "host",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "ip",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "port",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "url_data",
		Type: schema.FieldTypeJson,
	},
	&schema.SchemaField{
		Name: "original_request",
		Type: schema.FieldTypeJson,
	},
	&schema.SchemaField{
		Name: "original_response",
		Type: schema.FieldTypeJson,
	},
	&schema.SchemaField{
		Name: "has_response",
		Type: schema.FieldTypeBool,
	},
	&schema.SchemaField{
		Name: "is_request_edited",
		Type: schema.FieldTypeBool,
	},
	&schema.SchemaField{
		Name: "is_response_edited",
		Type: schema.FieldTypeBool,
	},
	&schema.SchemaField{
		Name: "edited_request",
		Type: schema.FieldTypeJson,
	},
	&schema.SchemaField{
		Name: "edited_response",
		Type: schema.FieldTypeJson,
	},
	&schema.SchemaField{
		Name: "labels",
		Type: schema.FieldTypeJson,
	},
	&schema.SchemaField{
		Name: "action",
		Type: schema.FieldTypeText,
	},
)
