package schemas

import (
	"github.com/glitchedgitz/pocketbase/models/schema"
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
	}, &schema.SchemaField{
		Name:     "ext",
		Type:     schema.FieldTypeText,
		Required: true,
	}, &schema.SchemaField{
		Name:     "data",
		Type:     schema.FieldTypeRelation,
		Required: true,
		Options: &schema.RelationOptions{
			CollectionId:  "_data",
			CascadeDelete: true,
		},
	},
)
