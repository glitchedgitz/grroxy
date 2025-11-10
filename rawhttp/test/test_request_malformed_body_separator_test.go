package rawhttp_test

import (
	"testing"

	"github.com/glitchedgitz/grroxy-db/rawhttp"
)

func TestRequestMalformedBodySeparator(t *testing.T) {
	rawRequest := "GET /test HTTP/1.1\r\n" +
		"Content-Type: text/plain\r\n" +
		"Content-Length: 11\r\n" +
		"Last-Header: value\r\n" +
		"Some body text"

	// Parse the request
	parsed := rawhttp.ParseRequest([]byte(rawRequest))

	// Unparse back to raw bytes
	unparsed := rawhttp.UnparseRequest(parsed)

	if string(unparsed) != rawRequest {
		t.Errorf("Roundtrip failed:\nExpected: %q\nGot:      %q", rawRequest, string(unparsed))
	}
}
