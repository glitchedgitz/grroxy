package rawhttp_test

import (
	"testing"

	"github.com/glitchedgitz/grroxy/grx/rawhttp"
)

func TestResponseMalformedMixedLineEndings(t *testing.T) {
	rawResponse := "HTTP/1.1 200 OK\n" +
		"Content-Type: application/json\n" +
		"Content-Length: 20\n" +
		"\n" +
		`{"status": "ok"}`

	// Parse the response
	parsed := rawhttp.ParseResponse([]byte(rawResponse))

	// Unparse back to raw bytes
	unparsed := rawhttp.UnparseResponse(parsed)

	if string(unparsed) != rawResponse {
		t.Errorf("Roundtrip failed:\nExpected: %q\nGot:      %q", rawResponse, string(unparsed))
	}
}
