package rawhttp_test

import (
	"testing"

	"github.com/glitchedgitz/grroxy-db/grx/rawhttp"
)

func TestResponseMalformedInvalidStatusCode(t *testing.T) {
	rawResponse := "HTTP/1.1 999 Invalid Status\r\n" +
		"Content-Type: text/plain\r\n" +
		"Content-Length: 10\r\n" +
		"\r\n" +
		"Some body"

	// Parse the response
	parsed := rawhttp.ParseResponse([]byte(rawResponse))

	// Unparse back to raw bytes
	unparsed := rawhttp.UnparseResponse(parsed)

	if string(unparsed) != rawResponse {
		t.Errorf("Roundtrip failed:\nExpected: %q\nGot:      %q", rawResponse, string(unparsed))
	}
}
