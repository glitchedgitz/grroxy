package schemas

import (
	"github.com/pocketbase/pocketbase/models/schema"
)

var Sitemap = schema.NewSchema(
	&schema.SchemaField{
		Name:     "path",
		Type:     schema.FieldTypeText,
		Required: true,
	}, &schema.SchemaField{
		Name:     "query",
		Type:     schema.FieldTypeText,
		Required: true,
	}, &schema.SchemaField{
		Name:     "fragment",
		Type:     schema.FieldTypeText,
		Required: true,
	}, &schema.SchemaField{
		Name:     "type",
		Type:     schema.FieldTypeText,
		Required: true,
	},
	&schema.SchemaField{
		Name:     "main_id",
		Type:     schema.FieldTypeRelation,
		Required: true,
		Options: &schema.RelationOptions{
			CollectionId:  "data",
			CascadeDelete: true,
		},
	},
)
