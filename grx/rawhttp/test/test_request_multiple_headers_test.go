package rawhttp_test

import (
	"testing"

	"github.com/glitchedgitz/grroxy/grx/rawhttp"
)

func TestRequestMultipleHeaders(t *testing.T) {
	rawRequest := "GET /api/data HTTP/1.1\r\n" +
		"Host: api.example.com\r\n" +
		"User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64)\r\n" +
		"Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8\r\n" +
		"Accept-Language: en-US,en;q=0.5\r\n" +
		"Accept-Encoding: gzip, deflate, br\r\n" +
		"Connection: keep-alive\r\n" +
		"Cache-Control: no-cache\r\n" +
		"\r\n"

	// Parse the request
	parsed := rawhttp.ParseRequest([]byte(rawRequest))

	// Unparse back to raw bytes
	unparsed := rawhttp.UnparseRequest(parsed)

	if string(unparsed) != rawRequest {
		t.Errorf("Roundtrip failed:\nExpected: %q\nGot:      %q", rawRequest, string(unparsed))
	}
}
