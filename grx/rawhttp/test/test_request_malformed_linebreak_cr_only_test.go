package rawhttp_test

import (
	"testing"

	"github.com/glitchedgitz/grroxy/grx/rawhttp"
)

func TestRequestMalformedLinebreakCrOnly(t *testing.T) {
	rawRequest := "GET /test HTTP/1.1\r" +
		"Host: example.com\r" +
		"Content-Type: text/plain\r" +
		"\r" +
		"Body content"

	// Parse the request
	parsed := rawhttp.ParseRequest([]byte(rawRequest))

	// Unparse back to raw bytes
	unparsed := rawhttp.UnparseRequest(parsed)

	if string(unparsed) != rawRequest {
		t.Errorf("Roundtrip failed:\nExpected: %q\nGot:      %q", rawRequest, string(unparsed))
	}
}
