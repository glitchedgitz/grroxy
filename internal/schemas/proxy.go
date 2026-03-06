package schemas

import (
	"github.com/glitchedgitz/pocketbase/models/schema"
)

var Proxies = schema.NewSchema(
	&schema.SchemaField{
		Name: "label",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "addr",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "browser",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "intercept",
		Type: schema.FieldTypeBool,
	},
	&schema.SchemaField{
		Name: "state",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "color",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "profile",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "data",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 100000,
		},
	},
)
