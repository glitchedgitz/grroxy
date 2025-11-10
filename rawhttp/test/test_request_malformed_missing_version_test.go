package rawhttp_test

import (
	"testing"

	"github.com/glitchedgitz/grroxy-db/rawhttp"
)

func TestRequestMalformedMissingVersion(t *testing.T) {
	rawRequest := "GET /api/users\r\n" +
		"Host: api.example.com\r\n" +
		"User-Agent: test-client\r\n" +
		"\r\n"

	// Parse the request
	parsed := rawhttp.ParseRequest([]byte(rawRequest))

	// Unparse back to raw bytes
	unparsed := rawhttp.UnparseRequest(parsed)

	if string(unparsed) != rawRequest {
		t.Errorf("Roundtrip failed:\nExpected: %q\nGot:      %q", rawRequest, string(unparsed))
	}
}
