package rawhttp_test

import (
	"testing"

	"github.com/glitchedgitz/grroxy-db/grx/rawhttp"
)

func TestRequestMalformedCaseInsensitiveHeaders(t *testing.T) {
	rawRequest := "GET /api/data HTTP/1.1\r\n" +
		"host: example.com\r\n" +
		"HOST: EXAMPLE.COM\r\n" +
		"Content-Type: application/json\r\n" +
		"content-type: text/xml\r\n" +
		"\r\n"

	// Parse the request
	parsed := rawhttp.ParseRequest([]byte(rawRequest))

	// Unparse back to raw bytes
	unparsed := rawhttp.UnparseRequest(parsed)

	if string(unparsed) != rawRequest {
		t.Errorf("Roundtrip failed:\nExpected: %q\nGot:      %q", rawRequest, string(unparsed))
	}
}
