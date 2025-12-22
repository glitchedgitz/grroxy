package rawhttp_test

import (
	"testing"

	"github.com/glitchedgitz/grroxy-db/grx/rawhttp"
)

func TestResponseNotFound(t *testing.T) {
	rawResponse := "HTTP/1.1 404 Not Found\r\n" +
		"Content-Type: text/html\r\n" +
		"Content-Length: 13\r\n" +
		"\r\n" +
		"Not Found"

	// Parse the response
	parsed := rawhttp.ParseResponse([]byte(rawResponse))

	// Unparse back to raw bytes
	unparsed := rawhttp.UnparseResponse(parsed)

	if string(unparsed) != rawResponse {
		t.Errorf("Roundtrip failed:\nExpected: %q\nGot:      %q", rawResponse, string(unparsed))
	}
}
