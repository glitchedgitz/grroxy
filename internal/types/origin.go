package types

import "strings"

// Origin is generated_by split into fields, see docs/generated_by.md.
//
// generated_by packs the tool, the actor and the proxy/chat ids into one
// path-like string ("ai/<chat>/proxy/<pid>"), which can only be filtered by
// substring. The fields can be matched exactly and indexed.
type Origin struct {
	Type  string // proxy, repeater, fuzzer, automate, manual, import
	Actor string // user, ai, template
	Proxy string // padded proxy id, proxy captures only
	Chat  string // AI chat id, when a chat drove the request
}

const (
	TypeProxy    = "proxy"
	TypeRepeater = "repeater"
	TypeFuzzer   = "fuzzer"
	TypeAutomate = "automate"
	TypeManual   = "manual"
	TypeImport   = "import"

	ActorUser     = "user"
	ActorAI       = "ai"
	ActorTemplate = "template"
)

// mcpNoChat is the chat segment MCP sendRequest stamps when the caller passed
// no chat id (Claude Code and other external MCP clients).
const mcpNoChat = "claudecode"

// ParseGeneratedBy splits a stored generated_by value into its fields. It knows
// every value current code writes and the older ones still in projects; for
// anything else it returns an empty Origin and false.
func ParseGeneratedBy(gb string) (Origin, bool) {
	gb = strings.TrimSpace(gb)
	parts := strings.Split(gb, "/")

	switch {
	// proxy/<pid>
	case len(parts) == 2 && parts[0] == "proxy" && parts[1] != "":
		return Origin{Type: TypeProxy, Actor: ActorUser, Proxy: parts[1]}, true

	// ai/<chat>/proxy/<pid>
	case len(parts) == 4 && parts[0] == "ai" && parts[1] != "" && parts[2] == "proxy" && parts[3] != "":
		return Origin{Type: TypeProxy, Actor: ActorAI, Proxy: parts[3], Chat: parts[1]}, true

	// fuzzer: hud/fuzzer, ai/fuzzer, fuzz_<index>[_<resultId>]
	case gb == "hud/fuzzer":
		return Origin{Type: TypeFuzzer, Actor: ActorUser}, true
	case gb == "ai/fuzzer":
		return Origin{Type: TypeFuzzer, Actor: ActorAI}, true
	case strings.HasPrefix(gb, "fuzz_") && len(parts) == 1:
		return Origin{Type: TypeFuzzer, Actor: ActorUser}, true

	// /api/request/add, no repeater prefix
	case gb == "manual":
		return Origin{Type: TypeManual, Actor: ActorUser}, true
	case gb == "import/curl", gb == "repeater/import/curl":
		return Origin{Type: TypeImport, Actor: ActorUser}, true

	// old AI sends: ai, ai/repeater/<chat>
	case gb == "ai":
		return Origin{Type: TypeRepeater, Actor: ActorAI}, true
	case len(parts) == 3 && parts[0] == "ai" && parts[1] == "repeater" && parts[2] != "":
		return Origin{Type: TypeRepeater, Actor: ActorAI, Chat: parts[2]}, true

	// old repeater sends: repeater, hud/repeater
	case gb == "repeater", gb == "hud/repeater":
		return Origin{Type: TypeRepeater, Actor: ActorUser}, true

	case parts[0] == "repeater" && len(parts) >= 2:
		return parseRepeaterSend(parts[1:])
	}

	return Origin{}, false
}

// parseRepeaterSend handles repeater/<what>, what /api/repeater/send stores.
func parseRepeaterSend(what []string) (Origin, bool) {
	switch {
	case len(what) == 1 && (what[0] == "hud" || what[0] == "new-request" || what[0] == "fetch-urls"):
		return Origin{Type: TypeRepeater, Actor: ActorUser}, true

	case len(what) == 1 && what[0] == "automate":
		return Origin{Type: TypeAutomate, Actor: ActorUser}, true

	// template:<templateId>, or the old template:send_request
	case len(what) == 1 && strings.HasPrefix(what[0], "template:") && len(what[0]) > len("template:"):
		return Origin{Type: TypeRepeater, Actor: ActorTemplate}, true

	// ai/mcp/<chat>, ai/mcp/claudecode
	case len(what) == 3 && what[0] == "ai" && what[1] == "mcp" && what[2] != "":
		chat := what[2]
		if chat == mcpNoChat {
			chat = ""
		}
		return Origin{Type: TypeRepeater, Actor: ActorAI, Chat: chat}, true

	// ai/<chat>
	case len(what) == 2 && what[0] == "ai" && what[1] != "":
		return Origin{Type: TypeRepeater, Actor: ActorAI, Chat: what[1]}, true
	}

	return Origin{}, false
}

// Fields returns the record fields for o, for Load / Set on a record.
func (o Origin) Fields() map[string]any {
	return map[string]any{
		"type":  o.Type,
		"actor": o.Actor,
		"proxy": o.Proxy,
		"chat":  o.Chat,
	}
}
