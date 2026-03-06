package schemas

import (
	"github.com/glitchedgitz/pocketbase/models/schema"
)

// WebSocket Messages Collection (_websockets)
//
// UI Table Columns (suggested):
//   index:        Message sequence number within the connection
//   direction:    ← (server-to-client) or → (client-to-server)
//   type:         text, binary, close, ping, pong
//   host:         demo.piesocket.com
//   path:         /v3/channel_123
//   length:       1234 bytes
//   timestamp:    2026-01-24 06:43:29
//   payload:      (expandable/modal for full content)
//
// Filters:
//   - By host
//   - By direction
//   - By type
//   - By connection (proxy_id groups messages for same WS session)

var Websockets = schema.NewSchema(
	// Message sequence within the WebSocket connection (for ordering)
	&schema.SchemaField{
		Name: "index",
		Type: schema.FieldTypeNumber,
	},
	// Host of the WebSocket server (e.g., demo.piesocket.com)
	&schema.SchemaField{
		Name: "host",
		Type: schema.FieldTypeText,
	},
	// Path of the WebSocket endpoint (e.g., /v3/channel_123)
	&schema.SchemaField{
		Name: "path",
		Type: schema.FieldTypeText,
	},
	// Full URL for reference
	&schema.SchemaField{
		Name: "url",
		Type: schema.FieldTypeText,
	},
	// Direction: "send" (client→server) or "recv" (server→client)
	&schema.SchemaField{
		Name: "direction",
		Type: schema.FieldTypeText,
	},
	// Frame type: text, binary, close, ping, pong
	&schema.SchemaField{
		Name: "type",
		Type: schema.FieldTypeText,
	},
	// Is this a binary message? (for quick filtering)
	&schema.SchemaField{
		Name: "is_binary",
		Type: schema.FieldTypeBool,
	},
	// The actual message payload
	&schema.SchemaField{
		Name: "payload",
		Type: schema.FieldTypeText,
	},
	// Payload length in bytes
	&schema.SchemaField{
		Name: "length",
		Type: schema.FieldTypeNumber,
	},
	// When the message was captured
	&schema.SchemaField{
		Name: "timestamp",
		Type: schema.FieldTypeDate,
	},
	// Proxy request ID (e.g., req-00000001) - groups messages by WS session
	&schema.SchemaField{
		Name: "proxy_id",
		Type: schema.FieldTypeText,
	},
	// Link to the main _data record (the HTTP upgrade request)
	&schema.SchemaField{
		Name: "data_index",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "generated_by",
		Type: schema.FieldTypeText,
	},
)
