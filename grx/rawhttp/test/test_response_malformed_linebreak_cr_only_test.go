package rawhttp_test

import (
	"testing"

	"github.com/glitchedgitz/grroxy/grx/rawhttp"
)

func TestResponseMalformedLinebreakCrOnly(t *testing.T) {
	rawResponse := "HTTP/1.1 200 OK\r" +
		"Content-Type: text/plain\r" +
		"Content-Length: 12\r" +
		"\r" +
		"Body content"

	// Parse the response
	parsed := rawhttp.ParseResponse([]byte(rawResponse))

	// Unparse back to raw bytes
	unparsed := rawhttp.UnparseResponse(parsed)

	if string(unparsed) != rawResponse {
		t.Errorf("Roundtrip failed:\nExpected: %q\nGot:      %q", rawResponse, string(unparsed))
	}
}
