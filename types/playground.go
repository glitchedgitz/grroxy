package types

type PlaygroundNew struct {
	Name     string `json:"name,omitempty"`
	ParentId string `json:"parent_id"`
	Type     string `json:"type,omitempty"`
	Expanded bool   `json:"expanded,omitempty"`
}

type PlaygroundAdd struct {
	ParentId string           `json:"parent_id"`
	Items    []PlaygroundItem `json:"items"`
}

type PlaygroundItem struct {
	Name        string         `json:"name,omitempty"`
	Original_Id string         `json:"original_id,omitempty"`
	Type        string         `json:"type,omitempty"`
	ToolData    map[string]any `json:"tool_data,omitempty"`
}

type NewRepeaterRequest struct {
	URL   string         `json:"url,omitempty"`
	Req   string         `json:"req,omitempty"`
	Resp  string         `json:"resp,omitempty"`
	Data  map[string]any `json:"data,omitempty"`
	Extra map[string]any `json:"extra,omitempty"`
}

type NewIntruderRequest struct {
	ID      string `json:"id,omitempty"`
	URL     string `json:"url,omitempty"`
	Req     string `json:"req,omitempty"`
	Payload string `json:"payload,omitempty"`
}
