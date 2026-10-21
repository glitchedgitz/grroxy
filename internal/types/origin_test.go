package types

import "testing"

// Every generated_by value found in real projects, see docs/generated_by.md.
func TestParseGeneratedBy(t *testing.T) {
	scenarios := []struct {
		gb   string
		want Origin
	}{
		// written by current code
		{"proxy/_____________17", Origin{TypeProxy, ActorUser, "_____________17", ""}},
		{"ai/x6s9c08ux5kd3k5/proxy/_____________17", Origin{TypeProxy, ActorAI, "_____________17", "x6s9c08ux5kd3k5"}},
		{"repeater/hud", Origin{TypeRepeater, ActorUser, "", ""}},
		{"repeater/new-request", Origin{TypeRepeater, ActorUser, "", ""}},
		{"repeater/fetch-urls", Origin{TypeRepeater, ActorUser, "", ""}},
		{"repeater/automate", Origin{TypeAutomate, ActorUser, "", ""}},
		{"repeater/template:j4lwc61eooazk62", Origin{TypeRepeater, ActorTemplate, "", ""}},
		{"repeater/ai/x6s9c08ux5kd3k5", Origin{TypeRepeater, ActorAI, "", "x6s9c08ux5kd3k5"}},
		{"repeater/ai/mcp/h9num2fhd84sbur", Origin{TypeRepeater, ActorAI, "", "h9num2fhd84sbur"}},
		{"repeater/ai/mcp/claudecode", Origin{TypeRepeater, ActorAI, "", ""}},
		{"repeater/chat/x6s9c08ux5kd3k5", Origin{TypeRepeater, ActorUser, "", "x6s9c08ux5kd3k5"}},
		{"hud/fuzzer", Origin{TypeFuzzer, ActorUser, "", ""}},
		{"ai/fuzzer", Origin{TypeFuzzer, ActorAI, "", ""}},
		{"fuzz_2766", Origin{TypeFuzzer, ActorUser, "", ""}},
		{"fuzz_2766__________2766_5", Origin{TypeFuzzer, ActorUser, "", ""}},
		{"manual", Origin{TypeManual, ActorUser, "", ""}},
		{"import/curl", Origin{TypeImport, ActorUser, "", ""}},

		// only in old rows
		{"repeater/template:send_request", Origin{TypeRepeater, ActorTemplate, "", ""}},
		{"ai", Origin{TypeRepeater, ActorAI, "", ""}},
		{"repeater", Origin{TypeRepeater, ActorUser, "", ""}},
		{"hud/repeater", Origin{TypeRepeater, ActorUser, "", ""}},
		{"ai/repeater/wx5sh0jomr23avm", Origin{TypeRepeater, ActorAI, "", "wx5sh0jomr23avm"}},
		{"repeater/import/curl", Origin{TypeImport, ActorUser, "", ""}},
	}

	for _, s := range scenarios {
		t.Run(s.gb, func(t *testing.T) {
			got, ok := ParseGeneratedBy(s.gb)
			if !ok {
				t.Fatalf("ParseGeneratedBy(%q) did not recognise the value", s.gb)
			}
			if got != s.want {
				t.Fatalf("ParseGeneratedBy(%q) = %+v, want %+v", s.gb, got, s.want)
			}
		})
	}
}

// Values that match no known shape come back empty instead of half-parsed, so
// the backfill can log them rather than store a guess.
func TestParseGeneratedByUnknown(t *testing.T) {
	for _, gb := range []string{
		"",
		"proxy",
		"proxy/",
		"ai/chat1/proxy/",
		"ai//proxy/_____________17",
		"repeater/",
		"repeater/template:",
		"repeater/ai/",
		"repeater/ai/mcp/",
		"repeater/something-new",
		"pg_abc123",
		"fuzz_1/extra",
	} {
		t.Run(gb, func(t *testing.T) {
			if got, ok := ParseGeneratedBy(gb); ok || got != (Origin{}) {
				t.Fatalf("ParseGeneratedBy(%q) = %+v, %v; want an empty Origin and false", gb, got, ok)
			}
		})
	}
}
