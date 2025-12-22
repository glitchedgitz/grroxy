package rawhttp_test

import (
	"testing"

	"github.com/glitchedgitz/grroxy-db/grx/rawhttp"
)

func TestResponseMalformedInvalidContentLength(t *testing.T) {
	rawResponse := "HTTP/1.1 200 OK\r\n" +
		"Content-Type: application/json\r\n" +
		"Content-Length: not-a-number\r\n" +
		"\r\n" +
		`{"actual": "body"}`

	// Parse the response
	parsed := rawhttp.ParseResponse([]byte(rawResponse))

	// Unparse back to raw bytes
	unparsed := rawhttp.UnparseResponse(parsed)

	if string(unparsed) != rawResponse {
		t.Errorf("Roundtrip failed:\nExpected: %q\nGot:      %q", rawResponse, string(unparsed))
	}
}
