package rawhttp_test

import (
	"testing"

	"github.com/glitchedgitz/grroxy/grx/rawhttp"
)

func TestRequestMalformedNoColon(t *testing.T) {
	rawRequest := "GET /test HTTP/1.1\r\n" +
		"Host: example.com\r\n" +
		"Invalid Header No Colon\r\n" +
		"Valid-Header: value\r\n" +
		"\r\n"

	// Parse the request
	parsed := rawhttp.ParseRequest([]byte(rawRequest))

	// Unparse back to raw bytes
	unparsed := rawhttp.UnparseRequest(parsed)

	if string(unparsed) != rawRequest {
		t.Errorf("Roundtrip failed:\nExpected: %q\nGot:      %q", rawRequest, string(unparsed))
	}
}
