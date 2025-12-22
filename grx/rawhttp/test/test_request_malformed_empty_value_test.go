package rawhttp_test

import (
	"testing"

	"github.com/glitchedgitz/grroxy-db/grx/rawhttp"
)

func TestRequestMalformedEmptyValue(t *testing.T) {
	rawRequest := "POST /api/data HTTP/1.1\r\n" +
		"Host: api.example.com\r\n" +
		"Header-With-No-Value:\r\n" +
		"Empty-Value-Header: \r\n" +
		"Content-Type: application/json\r\n" +
		"\r\n" +
		`{"data": "test"}`

	// Parse the request
	parsed := rawhttp.ParseRequest([]byte(rawRequest))

	// Unparse back to raw bytes
	unparsed := rawhttp.UnparseRequest(parsed)

	if string(unparsed) != rawRequest {
		t.Errorf("Roundtrip failed:\nExpected: %q\nGot:      %q", rawRequest, string(unparsed))
	}
}
