package rawhttp_test

import (
	"testing"

	"github.com/glitchedgitz/grroxy-db/rawhttp"
)

func TestRequestMalformedLinebreakMixed(t *testing.T) {
	rawRequest := "GET /test HTTP/1.1\r\n" +
		"Host: example.com\n" +
		"Content-Type: text/plain\r\n" +
		"User-Agent: test\n" +
		"\r\n" +
		"Body content"

	// Parse the request
	parsed := rawhttp.ParseRequest([]byte(rawRequest))

	// Unparse back to raw bytes
	unparsed := rawhttp.UnparseRequest(parsed)

	if string(unparsed) != rawRequest {
		t.Errorf("Roundtrip failed:\nExpected: %q\nGot:      %q", rawRequest, string(unparsed))
	}
}

