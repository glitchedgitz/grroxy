package schemas

import "github.com/glitchedgitz/pocketbase/models/schema"

var Counter = schema.NewSchema(
	&schema.SchemaField{
		Name:     "counter_key",
		Type:     schema.FieldTypeText,
		Required: true,
	},
	&schema.SchemaField{
		Name:     "collection",
		Type:     schema.FieldTypeText,
		Required: false,
	},
	&schema.SchemaField{
		Name:     "filter",
		Type:     schema.FieldTypeText,
		Required: false,
	},
	&schema.SchemaField{
		Name:     "count",
		Type:     schema.FieldTypeNumber,
		Required: false,
	},
	&schema.SchemaField{
		Name:     "load_on_startup",
		Type:     schema.FieldTypeBool,
		Required: false,
	},
)
