package schemas

import (
	"github.com/glitchedgitz/pocketbase/models/schema"
)

// Fuzzer Results (fuzzer_<id> collection)
//   fuzzer_id:      The ID of the fuzzer process
//   raw_request:    The raw HTTP request that was sent
//   raw_response:   The raw HTTP response received
//   time:           Response time in nanoseconds
//   markers:        Map of marker names to the values that were used
//   req_method:     HTTP method (from parsed request)
//   req_url:        Request URL (from parsed request)
//   req_version:    HTTP version (from parsed request)
//   req_headers:    Request headers as JSON (from parsed request)
//   resp_version:   HTTP version (from parsed response)
//   resp_status:    HTTP status code (from parsed response)
//   resp_status_full: Full status line (from parsed response)
//   resp_headers:   Response headers as JSON (from parsed response)
//   resp_length:    Response length in bytes

var Fuzzer = schema.NewSchema(
	&schema.SchemaField{
		Name: "fuzzer_id",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "raw_request",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "raw_response",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "time",
		Type: schema.FieldTypeNumber,
	},
	&schema.SchemaField{
		Name: "markers",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 10000,
		},
	},
	&schema.SchemaField{
		Name: "req_method",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "req_url",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "req_version",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "req_headers",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 100000,
		},
	},
	&schema.SchemaField{
		Name: "resp_version",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "resp_status",
		Type: schema.FieldTypeNumber,
	},
	&schema.SchemaField{
		Name: "resp_status_full",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "resp_headers",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 100000,
		},
	},
	&schema.SchemaField{
		Name: "resp_length",
		Type: schema.FieldTypeNumber,
	},
)
