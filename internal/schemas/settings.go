package schemas

import "github.com/glitchedgitz/pocketbase/models/schema"

var Settings = schema.NewSchema(
	&schema.SchemaField{
		Name:     "option",
		Type:     schema.FieldTypeText,
		Required: true,
	},
	&schema.SchemaField{
		Name: "value",
		Type: schema.FieldTypeText,
	},
)

var ConfigSchema = schema.NewSchema(
	&schema.SchemaField{
		Name:     "key",
		Type:     schema.FieldTypeText,
		Required: true,
	},
	&schema.SchemaField{
		Name: "data",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 100000,
		},
	},
)

// SettingsConfigKey is the _configs row holding the global UI settings blob.
const SettingsConfigKey = "settings"

// DefaultSettings is the seed value for the `settings` row in _configs. The
// frontend reads this blob on boot and only overrides a field when it is
// present, so anything missing here falls back to the frontend's own default.
var DefaultSettings = map[string]any{
	"aiPanel":           true,
	"autoRemoveHeaders": false,
	"autoZoomWide":      true,
	"flipSidebar":       true,
	"hideRightSidebar":  false,
	"minimizedDecoder":  false,
	"sidebarOnLeft":     true,
	"zoom":              100,
	"zoomSlider":        true,
}
