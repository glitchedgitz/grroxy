package app

import (
	"testing"

	"github.com/glitchedgitz/grroxy/internal/types"
)

// What each writer stamps today must parse back to the fields it means, so the
// fields written at capture time and the ones the backfill derives agree.
func TestParseGeneratedByRoundTripsWriters(t *testing.T) {
	rp := &RawProxyWrapper{proxyID: "_____________17"}
	if got, _ := types.ParseGeneratedBy(rp.formatGeneratedBy()); got != (types.Origin{types.TypeProxy, types.ActorUser, "_____________17", ""}) {
		t.Fatalf("plain capture parses to %+v", got)
	}

	rp.AIChatID = "chat1"
	if got, _ := types.ParseGeneratedBy(rp.formatGeneratedBy()); got != (types.Origin{types.TypeProxy, types.ActorAI, "_____________17", "chat1"}) {
		t.Fatalf("AI-owned capture parses to %+v", got)
	}
}

// The fields a capture writes and the ones parsed back from its generated_by
// must agree, or old (backfilled) and new rows would disagree.
func TestProxyOriginMatchesGeneratedBy(t *testing.T) {
	for _, chat := range []string{"", "chat1"} {
		rp := &RawProxyWrapper{proxyID: "_____________17", AIChatID: chat}
		parsed, _ := types.ParseGeneratedBy(rp.formatGeneratedBy())
		if got := rp.origin(); got != parsed {
			t.Fatalf("chat %q: origin() = %+v, parsed generated_by = %+v", chat, got, parsed)
		}
	}
}
