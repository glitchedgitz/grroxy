package app

import (
	"testing"

	"github.com/glitchedgitz/grroxy/internal/types"
)

// A capture carries the proxy, and the chat while one owns the proxy.
func TestProxyOrigin(t *testing.T) {
	rp := &RawProxyWrapper{proxyID: "_____________17"}
	if got := rp.origin(); got != (types.Origin{Type: types.TypeProxy, Actor: types.ActorUser, Proxy: "_____________17"}) {
		t.Fatalf("plain capture origin = %+v", got)
	}

	rp.AIChatID = "chat1"
	if got := rp.origin(); got != (types.Origin{Type: types.TypeProxy, Actor: types.ActorAI, Proxy: "_____________17", Chat: "chat1"}) {
		t.Fatalf("AI-owned capture origin = %+v", got)
	}
}

// New rows store only the fields, except where generated_by says more than they
// can hold: fuzzer and template rows, and values the parser doesn't know.
func TestStoredOrigin(t *testing.T) {
	scenarios := []struct {
		gb       string
		want     types.Origin
		wantKept string
	}{
		{"repeater/hud", types.Origin{Type: types.TypeRepeater, Actor: types.ActorUser}, ""},
		{"repeater/automate", types.Origin{Type: types.TypeAutomate, Actor: types.ActorUser}, ""},
		{"repeater/ai/x6s9c08ux5kd3k5", types.Origin{Type: types.TypeRepeater, Actor: types.ActorAI, Chat: "x6s9c08ux5kd3k5"}, ""},
		{"repeater/ai/mcp/x6s9c08ux5kd3k5", types.Origin{Type: types.TypeRepeater, Actor: types.ActorAI, Chat: "x6s9c08ux5kd3k5"}, ""},
		{"manual", types.Origin{Type: types.TypeManual, Actor: types.ActorUser}, ""},
		{"import/curl", types.Origin{Type: types.TypeImport, Actor: types.ActorUser}, ""},

		// kept
		{"hud/fuzzer", types.Origin{Type: types.TypeFuzzer, Actor: types.ActorUser}, "hud/fuzzer"},
		{"ai/fuzzer", types.Origin{Type: types.TypeFuzzer, Actor: types.ActorAI}, "ai/fuzzer"},
		{"fuzz_2766_5", types.Origin{Type: types.TypeFuzzer, Actor: types.ActorUser}, "fuzz_2766_5"},
		{"repeater/template:j4lwc61eooazk62", types.Origin{Type: types.TypeRepeater, Actor: types.ActorTemplate}, "repeater/template:j4lwc61eooazk62"},
		{"something-new", types.Origin{}, "something-new"},
		{"", types.Origin{}, ""},
	}

	for _, s := range scenarios {
		t.Run(s.gb, func(t *testing.T) {
			got, kept := storedOrigin(s.gb)
			if got != s.want || kept != s.wantKept {
				t.Fatalf("storedOrigin(%q) = %+v, %q; want %+v, %q", s.gb, got, kept, s.want, s.wantKept)
			}
		})
	}
}
