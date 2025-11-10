package rawhttp_test

import (
	"testing"

	"github.com/glitchedgitz/grroxy-db/rawhttp"
)

func TestResponseMalformedDuplicateHeaders(t *testing.T) {
	rawResponse := "HTTP/1.1 200 OK\r\n" +
		"Set-Cookie: session=abc123\r\n" +
		"Set-Cookie: theme=dark\r\n" +
		"Set-Cookie: lang=en\r\n" +
		"Content-Type: application/json\r\n" +
		"\r\n" +
		`{"status": "ok"}`

	// Parse the response
	parsed := rawhttp.ParseResponse([]byte(rawResponse))

	// Unparse back to raw bytes
	unparsed := rawhttp.UnparseResponse(parsed)

	if string(unparsed) != rawResponse {
		t.Errorf("Roundtrip failed:\nExpected: %q\nGot:      %q", rawResponse, string(unparsed))
	}
}

