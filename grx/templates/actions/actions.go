package actions

// Actions:
// Perform some action after filter match, in templates

const (
	CreateLabel = "create_label"

	Replace = "replace" // modify request/response
	Set     = "set"     // modify request/response
	Delete  = "delete"  // delete request/response

	SendRequest = "send_request" // send a modified copy of the request
)

// ActionInfo describes an action with its keys and description
type ActionInfo struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Keys        []KeyInfo `json:"keys"`
}

// ModeInfo describes a template execution mode
type ModeInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ModeRegistry returns available modes
var ModeRegistry = []ModeInfo{
	{Name: "any", Description: "Stop after the first matching task"},
	{Name: "all", Description: "Run all matching tasks"},
}

// KeyInfo describes a key for an action
type KeyInfo struct {
	Name        string `json:"name"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

// HookInfo describes a hook group with its hooks
type HookInfo struct {
	Group       string         `json:"group"`
	Description string         `json:"description"`
	Hooks       []HookItemInfo `json:"hooks"`
}

// HookItemInfo describes a single hook
type HookItemInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ValidActions lists all supported action names (for /check validation)
var ValidActions = map[string][]string{
	CreateLabel: {"name", "color", "type", "icon"},
	Set:         {},
	Replace:     {"search", "value", "regex"},
	Delete:      {},
	SendRequest: {"req.method", "req.headers", "req.body"},
}

// ValidHooks lists all supported hook types (for /check validation)
var ValidHooks = map[string][]string{
	"proxy":                 {"request", "response", "before_request"},
	"request-action-button": {},
	"response-action-button": {},
}

// ActionRegistry returns full action metadata
var ActionRegistry = []ActionInfo{
	{
		Name:        CreateLabel,
		Description: "Create a label and attach it to the request row",
		Keys: []KeyInfo{
			{Name: "name", Required: true, Description: "Label name. Supports {{variables}}"},
			{Name: "color", Required: false, Description: "Label color (blue, red, green, yellow, orange, purple, pink, ignore). Default: blue"},
			{Name: "type", Required: false, Description: "Label category (extension, mime, endpoint, custom). Default: custom"},
			{Name: "icon", Required: false, Description: "Icon name for the label"},
		},
	},
	{
		Name:        Set,
		Description: "Set or modify request/response fields. Keys are field paths like req.method, req.headers.X-Custom, req.body, req.path, req.query.param",
		Keys: []KeyInfo{
			{Name: "req.method", Required: false, Description: "Set HTTP method (GET, POST, PUT, etc.)"},
			{Name: "req.path", Required: false, Description: "Set request path"},
			{Name: "req.url", Required: false, Description: "Set full request URL"},
			{Name: "req.body", Required: false, Description: "Set request body"},
			{Name: "req.headers.<name>", Required: false, Description: "Set a request header value"},
			{Name: "req.query.<param>", Required: false, Description: "Set a query parameter value"},
		},
	},
	{
		Name:        Delete,
		Description: "Remove request/response fields. Keys are field paths. Supports wildcard header deletion with * suffix (e.g. req.headers.Sec-*)",
		Keys: []KeyInfo{
			{Name: "req.method", Required: false, Description: "Reset method to GET"},
			{Name: "req.path", Required: false, Description: "Clear the request path"},
			{Name: "req.url", Required: false, Description: "Clear the full URL"},
			{Name: "req.body", Required: false, Description: "Clear the request body"},
			{Name: "req.headers.<name>", Required: false, Description: "Remove a header. Use * suffix for wildcard (e.g. Sec-*)"},
			{Name: "req.query.<param>", Required: false, Description: "Remove a query parameter"},
		},
	},
	{
		Name:        Replace,
		Description: "Search and replace in the raw request/response. Supports string and regex replacement",
		Keys: []KeyInfo{
			{Name: "search", Required: true, Description: "Search string or regex pattern"},
			{Name: "value", Required: true, Description: "Replacement string"},
			{Name: "regex", Required: false, Description: "Set to true for regex mode. Default: false"},
		},
	},
	{
		Name:        SendRequest,
		Description: "Send a modified copy of the current request using the repeater logic. Response is saved to DB",
		Keys: []KeyInfo{
			{Name: "req.method", Required: false, Description: "Override the HTTP method"},
			{Name: "req.headers", Required: false, Description: "Override request headers (map)"},
			{Name: "req.body", Required: false, Description: "Override request body"},
		},
	},
}

// HookRegistry returns full hook metadata
var HookRegistry = []HookInfo{
	{
		Group:       "proxy",
		Description: "Hooks that run automatically on proxy traffic",
		Hooks: []HookItemInfo{
			{Name: "before_request", Description: "Runs before sending request upstream. Synchronous — can modify the request (set/delete/replace actions apply to the live http.Request)"},
			{Name: "request", Description: "Runs after request is saved to DB. Async — used for labeling, notifications, send_request"},
			{Name: "response", Description: "Runs after response is saved to DB. Async — used for labeling based on response data (mime, status, headers)"},
		},
	},
	{
		Group:       "request-action-button",
		Description: "Button shown on request rows — user clicks to run the action on a specific request",
		Hooks:       []HookItemInfo{},
	},
	{
		Group:       "response-action-button",
		Description: "Button shown on response rows — user clicks to run the action on a specific response",
		Hooks:       []HookItemInfo{},
	},
}
