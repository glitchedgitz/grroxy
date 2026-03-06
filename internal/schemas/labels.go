package schemas

import "github.com/glitchedgitz/pocketbase/models/schema"

// const LABELS_COLLECTION_SCHEMA = {
//     "name": LABELS,
//     "type": "base",
//     "system": false,
//     "schema": [
//         {
//             "name": "name",
//             "type": "text",
//             "system": false,
//             "required": true,
//             "options": {
//                 "min": null,
//                 "max": null,
//                 "pattern": ""
//             }
//         },
//         {
//             "name": "color",
//             "type": "text",
//             "system": false,
//             "required": false,
//             "options": {
//                 "min": 7,
//                 "max": 9,
//                 "pattern": "#(\\w)*"
//             }
//         },
//         {
//             "name": "type",
//             "type": "select",
//             "system": false,
//             "required": false,
//             "options": {
//                 "maxSelect": 1,
//                 "values": [
//                     LABEL_TYPES.DEFAULT,
//                     LABEL_TYPES.EXTENSION,
//                     LABEL_TYPES.SEVERITY,
//                     LABEL_TYPES.TECH,
//                     LABEL_TYPES.FOLDER,
//                 ]
//             }
//         },
//         {
//             "name": "data",
//             "type": "json",
//             "system": false,
//             "required": false,
//             "options": {
//                 maxSize: 100000
//             }
//         }
//     ],
//     "indexes": [
//         "CREATE UNIQUE INDEX `idx_Af9m6z1` ON `labels` (`name`)"
//     ],
// }

var Labels = schema.NewSchema(
	&schema.SchemaField{
		Name:     "name",
		Type:     schema.FieldTypeText,
		Required: true,
	},
	&schema.SchemaField{
		Name:     "color",
		Type:     schema.FieldTypeText,
		Required: true,
	},
	&schema.SchemaField{
		Name:     "icon",
		Type:     schema.FieldTypeText,
		Required: false,
	},
	&schema.SchemaField{
		Name:     "type",
		Type:     schema.FieldTypeText,
		Required: true,
	},
	&schema.SchemaField{
		Name: "extra",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 100000,
		},
	},
)

var Tech = schema.NewSchema(
	&schema.SchemaField{
		Name:     "name",
		Type:     schema.FieldTypeText,
		Required: true,
	},
	&schema.SchemaField{
		Name:     "image",
		Type:     schema.FieldTypeText,
		Required: false,
	},
	&schema.SchemaField{
		Name:     "color",
		Type:     schema.FieldTypeText,
		Required: false,
	},
	&schema.SchemaField{
		Name:     "category",
		Type:     schema.FieldTypeText,
		Required: false,
	},
	&schema.SchemaField{
		Name: "extra",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 100000,
		},
	},
)

var LabelCollection = schema.NewSchema(
	&schema.SchemaField{
		Name:     "data",
		Type:     schema.FieldTypeRelation,
		Required: true,
		Options: &schema.RelationOptions{
			CollectionId:  "_data",
			CascadeDelete: true,
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
