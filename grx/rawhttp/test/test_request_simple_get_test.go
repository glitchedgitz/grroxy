package rawhttp_test

import (
	"testing"

	"github.com/glitchedgitz/grroxy/grx/rawhttp"
)

func TestRequestSimpleGet(t *testing.T) {
	rawRequest := "GET /api/users HTTP/1.1\r\n" +
		"Host: example.com\r\n" +
		"User-Agent: Mozilla/5.0\r\n" +
		"\r\n"

	// Parse the request
	parsed := rawhttp.ParseRequest([]byte(rawRequest))

	// Unparse back to raw bytes
	unparsed := rawhttp.UnparseRequest(parsed)

	if string(unparsed) != rawRequest {
		t.Errorf("Roundtrip failed:\nExpected: %q\nGot:      %q", rawRequest, string(unparsed))
	}
}
