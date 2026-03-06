package schemas

import (
	"github.com/glitchedgitz/pocketbase/models/schema"
)

var Sites = schema.NewSchema(
	&schema.SchemaField{
		Name:     "host",
		Type:     schema.FieldTypeText,
		Required: true,
	},
	&schema.SchemaField{
		Name: "smartsort",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "domain",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "title",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "status",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "favicon",
		Type: schema.FieldTypeFile,
		Options: &schema.FileOptions{
			MimeTypes: []string{"image/png", "image/jpeg", "image/x-icon"},
			MaxSelect: 1,
			MaxSize:   100000,
		},
	},
	&schema.SchemaField{
		Name: "tech",
		Type: schema.FieldTypeRelation,
		Options: &schema.RelationOptions{
			CollectionId: "_tech",
		},
	},
	&schema.SchemaField{
		Name: "extra",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 100000,
		},
	},
	// added new columns via migration 1766714322_add_hosts_fields.go
	//	 &schema.SchemaField{
	//	 	Name: "labels",
	//	 	Type: schema.FieldTypeRelation,
	//	 	Options: &schema.RelationOptions{
	//	 		CollectionId: "_labels",
	//	 	},
	//	 },
	//	 &schema.SchemaField{
	//	 	Name: "notes",
	//	 	Type: schema.FieldTypeText,
	//	 	Options: &schema.JsonOptions{
	//	 		MaxSize: 1000000,
	//	 	},
	//	 },
)
