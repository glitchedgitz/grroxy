package schemas

import "github.com/pocketbase/pocketbase/models/schema"

var PROCESSES = schema.NewSchema(
	&schema.SchemaField{
		Name: "data",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 100000,
		},
	},
	// running, queue, error, killed, completed
	&schema.SchemaField{
		Name: "state",
		Type: schema.FieldTypeText,
	},
)
