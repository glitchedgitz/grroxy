package rawhttp_test

import (
	"testing"

	"github.com/glitchedgitz/grroxy-db/rawhttp"
)

func TestResponseMultipleHeaders(t *testing.T) {
	rawResponse := "HTTP/1.1 200 OK\r\n" +
		"Content-Type: application/json\r\n" +
		"Content-Length: 25\r\n" +
		"Cache-Control: no-cache\r\n" +
		"X-Custom-Header: custom-value\r\n" +
		"Set-Cookie: session=abc123\r\n" +
		"\r\n" +
		`{"message": "success"}`

	// Parse the response
	parsed := rawhttp.ParseResponse([]byte(rawResponse))

	// Unparse back to raw bytes
	unparsed := rawhttp.UnparseResponse(parsed)

	if string(unparsed) != rawResponse {
		t.Errorf("Roundtrip failed:\nExpected: %q\nGot:      %q", rawResponse, string(unparsed))
	}
}

