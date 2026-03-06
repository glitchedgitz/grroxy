package rawhttp_test

import (
	"testing"

	"github.com/glitchedgitz/grroxy/grx/rawhttp"
)

func TestResponseMalformedMissingVersion(t *testing.T) {
	rawResponse := "200 OK\r\n" +
		"Content-Type: application/json\r\n" +
		"Content-Length: 25\r\n" +
		"\r\n" +
		`{"message": "test"}`

	// Parse the response
	parsed := rawhttp.ParseResponse([]byte(rawResponse))

	// Unparse back to raw bytes
	unparsed := rawhttp.UnparseResponse(parsed)

	if string(unparsed) != rawResponse {
		t.Errorf("Roundtrip failed:\nExpected: %q\nGot:      %q", rawResponse, string(unparsed))
	}
}
