package rawhttp_test

import (
	"testing"

	"github.com/glitchedgitz/grroxy-db/grx/rawhttp"
)

func TestResponseMalformedLinebreakMixed(t *testing.T) {
	rawResponse := "HTTP/1.1 200 OK\r\n" +
		"Content-Type: text/plain\n" +
		"Content-Length: 12\r\n" +
		"X-Custom: value\n" +
		"\r\n" +
		"Body content"

	// Parse the response
	parsed := rawhttp.ParseResponse([]byte(rawResponse))

	// Unparse back to raw bytes
	unparsed := rawhttp.UnparseResponse(parsed)

	if string(unparsed) != rawResponse {
		t.Errorf("Roundtrip failed:\nExpected: %q\nGot:      %q", rawResponse, string(unparsed))
	}
}
