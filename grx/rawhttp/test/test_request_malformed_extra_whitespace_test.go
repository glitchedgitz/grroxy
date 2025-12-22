package rawhttp_test

import (
	"testing"

	"github.com/glitchedgitz/grroxy-db/grx/rawhttp"
)

func TestRequestMalformedExtraWhitespace(t *testing.T) {
	rawRequest := "GET /test HTTP/1.1\r\n" +
		"Host: example.com\r\n" +
		"Header-With-Tabs:\tvalue\t\r\n" +
		"Header-With-Mixed:   value  	\r\n" +
		"\r\n" +
		"\r\n" +
		"Header-After-Empty: value\r\n" +
		"\r\n"

	// Parse the request
	parsed := rawhttp.ParseRequest([]byte(rawRequest))

	// Unparse back to raw bytes
	unparsed := rawhttp.UnparseRequest(parsed)

	if string(unparsed) != rawRequest {
		t.Errorf("Roundtrip failed:\nExpected: %q\nGot:      %q", rawRequest, string(unparsed))
	}
}
