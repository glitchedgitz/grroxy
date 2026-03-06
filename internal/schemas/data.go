package schemas

import (
	"github.com/glitchedgitz/pocketbase/models/schema"
	"github.com/glitchedgitz/pocketbase/tools/types"
)

//   id :              ______________1
//   index :           1
//   host :            www.example.com
//   port :            80
//   has_resp:         false
//   has_params:       true
//   length:           1000
//   is_https:         true
//   http:             HTTP/1.1
//   proxy_id:         req-00000001
//   is_req_edited:    false
//   is_resp_edited:   false
//   req:              (relation to _req)
//   resp:             (relation to _resp)
//   req_edited:       (relation to _req_edited)
//   resp_edited:      (relation to _resp_edited)
//   req_json:         (json of req)
//   resp_json:        (json of resp)
//   req_edited_json:  (json of req_edited)
//   resp_edited_json: (json of resp_edited)
//   attached:         (relation to _attached)

var Rows = schema.NewSchema(
	&schema.SchemaField{
		Name: "index",
		Type: schema.FieldTypeNumber,
	},
	&schema.SchemaField{
		Name: "index_minor",
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
		Name: "has_params",
		Type: schema.FieldTypeBool,
	},
	&schema.SchemaField{
		Name: "has_resp",
		Type: schema.FieldTypeBool,
	},
	&schema.SchemaField{
		Name: "is_https",
		Type: schema.FieldTypeBool,
	},
	&schema.SchemaField{
		Name: "http",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "proxy_id",
		Type: schema.FieldTypeText,
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
		Name: "req",
		Type: schema.FieldTypeRelation,
		Options: &schema.RelationOptions{
			CollectionId:  "_req",
			CascadeDelete: true,
			MaxSelect:     types.Pointer(1),
		},
	},
	&schema.SchemaField{
		Name: "resp",
		Type: schema.FieldTypeRelation,
		Options: &schema.RelationOptions{
			CollectionId:  "_resp",
			CascadeDelete: true,
			MaxSelect:     types.Pointer(1),
		},
	},
	&schema.SchemaField{
		Name: "req_edited",
		Type: schema.FieldTypeRelation,
		Options: &schema.RelationOptions{
			CollectionId:  "_req_edited",
			CascadeDelete: true,
			MaxSelect:     types.Pointer(1),
		},
	},
	&schema.SchemaField{
		Name: "resp_edited",
		Type: schema.FieldTypeRelation,
		Options: &schema.RelationOptions{
			CollectionId:  "_resp_edited",
			CascadeDelete: true,
			MaxSelect:     types.Pointer(1),
		},
	},
	&schema.SchemaField{
		Name: "req_json",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 100000,
		},
	},
	&schema.SchemaField{
		Name: "resp_json",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 100000,
		},
	},
	&schema.SchemaField{
		Name: "req_edited_json",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 100000,
		},
	},
	&schema.SchemaField{
		Name: "resp_edited_json",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 100000,
		},
	},
	&schema.SchemaField{
		Name: "generated_by",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "extra",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 100000,
		},
	},
	&schema.SchemaField{
		Name: "attached",
		Type: schema.FieldTypeRelation,
		Options: &schema.RelationOptions{
			CollectionId:  "_attached",
			CascadeDelete: true,
			MaxSelect:     types.Pointer(1),
		},
	},
	&schema.SchemaField{
		Name: "action",
		Type: schema.FieldTypeText,
	},
)

// Request Data (_req collection)
//   method:      GET
//   url:         "https://www.example.com/path?query=value#fragment"
//   path:        "/path"
//   query:       "query=value"
//   fragment:    "fragment"
//   ext:         ".html"
//   has_cookies: true
//   length:      1000
//   headers:     {"Host": "www.example.com", "User-Agent": "Mozilla/5.0"}
//   raw:         "GET /path?query=value HTTP/1.1\r\nHost: www.example.com\r\n..."

var RequestData = schema.NewSchema(
	&schema.SchemaField{
		Name: "method",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "url",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "path",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "query",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "fragment",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "ext",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "has_cookies",
		Type: schema.FieldTypeBool,
	},
	&schema.SchemaField{
		Name: "length",
		Type: schema.FieldTypeNumber,
	},
	&schema.SchemaField{
		Name: "headers",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 100000,
		},
	},
	&schema.SchemaField{
		Name: "raw",
		Type: schema.FieldTypeText,
	},
)

// Response Data (_resp collection)
//   title:       ""
//   mime:        ""
//   status:      0
//   length:      0
//   has_cookies: false
//   headers:     {"Content-Type": "text/html"}
//   raw:         "HTTP/1.1 200 OK\r\nContent-Type: text/html\r\n..."

var ResponseData = schema.NewSchema(
	&schema.SchemaField{
		Name: "title",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "mime",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "status",
		Type: schema.FieldTypeNumber,
	},
	&schema.SchemaField{
		Name: "length",
		Type: schema.FieldTypeNumber,
	},
	&schema.SchemaField{
		Name: "has_cookies",
		Type: schema.FieldTypeBool,
	},
	&schema.SchemaField{
		Name: "headers",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 100000,
		},
	},
	&schema.SchemaField{
		Name: "raw",
		Type: schema.FieldTypeText,
	},
)
