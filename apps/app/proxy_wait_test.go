package app

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"testing"
)

// TestSyncContentLength covers the Content-Length rewrite applied to an edited
// raw message before it is forwarded upstream.
func TestSyncContentLength(t *testing.T) {
	tests := []struct {
		name string
		// contentLength is what the caller knew before the edit.
		contentLength int64
		in            string
		wantHeader    string // expected Content-Length value after the rewrite
		wantBody      string
	}{
		{
			name:          "canonical casing is rewritten",
			contentLength: 5,
			in:            "POST /a HTTP/1.1\r\nHost: x\r\nContent-Length: 5\r\n\r\nfoo=barbaz",
			wantHeader:    "10",
			wantBody:      "foo=barbaz",
		},
		{
			// Regression: detection was case-insensitive but the rewrite was a
			// literal strings.Replace on "Content-Length: <old>". A lowercase
			// header was found (so no header was appended) yet never updated,
			// and http.ReadRequest later truncated the body to the stale length.
			name:          "lowercase casing is rewritten",
			contentLength: 3,
			in:            "POST /a HTTP/2\ncontent-length: 3\nhost: x\n\nfoo=barbaz",
			wantHeader:    "10",
			wantBody:      "foo=barbaz",
		},
		{
			name:          "mixed casing is rewritten",
			contentLength: 3,
			in:            "POST /a HTTP/1.1\r\nHost: x\r\nCONTENT-LENGTH: 3\r\n\r\nfoo=barbaz",
			wantHeader:    "10",
			wantBody:      "foo=barbaz",
		},
		{
			name:          "no space after colon is rewritten",
			contentLength: 3,
			in:            "POST /a HTTP/1.1\r\nHost: x\r\nContent-Length:3\r\n\r\nfoo=barbaz",
			wantHeader:    "10",
			wantBody:      "foo=barbaz",
		},
		{
			// User added a body but deleted the header: one must be appended,
			// or the body is announced as absent and dropped upstream.
			name:          "missing header is appended",
			contentLength: 0,
			in:            "POST /a HTTP/2\nhost: x\n\nfoo=bar",
			wantHeader:    "7",
			wantBody:      "foo=bar",
		},
		{
			name:          "body shrunk to empty",
			contentLength: 10,
			in:            "POST /a HTTP/1.1\r\nHost: x\r\nContent-Length: 10\r\n\r\n",
			wantHeader:    "0",
			wantBody:      "",
		},
		{
			name:          "already correct is left alone",
			contentLength: 7,
			in:            "POST /a HTTP/1.1\r\nHost: x\r\nContent-Length: 7\r\n\r\nfoo=bar",
			wantHeader:    "7",
			wantBody:      "foo=bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := syncContentLength(tt.in, tt.contentLength, tt.in)

			// Parse the result the same way the forwarding path does, so the
			// test fails if the declared length disagrees with the real body.
			req, err := parseEditedRequest(got)
			if err != nil {
				t.Fatalf("parseEditedRequest(%q) failed: %v", got, err)
			}
			if h := req.Header.Get("Content-Length"); h != tt.wantHeader {
				t.Errorf("Content-Length header = %q, want %q\nresult:\n%s", h, tt.wantHeader, got)
			}
			body, _ := io.ReadAll(req.Body)
			if string(body) != tt.wantBody {
				t.Errorf("forwarded body = %q, want %q\nresult:\n%s", body, tt.wantBody, got)
			}
		})
	}
}

// TestSyncContentLengthPassthrough checks that a message with no header/body
// separator is returned untouched rather than mangled.
func TestSyncContentLengthPassthrough(t *testing.T) {
	in := "GET /a HTTP/1.1\r\nHost: x\r\n"
	if got := syncContentLength(in, 0, in); got != in {
		t.Errorf("got %q, want it unchanged (%q)", got, in)
	}
}

// TestParseEditedRequestTolerance covers the shapes the intercept editor
// actually produces, which http.ReadRequest alone rejects.
func TestParseEditedRequestTolerance(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		wantProto string
		wantBody  string
	}{
		{
			name:      "HTTP/2 request line gains a minor version",
			in:        "GET /a HTTP/2\nHost: x\n\n",
			wantProto: "HTTP/2.0",
		},
		{
			name:      "no trailing blank line after headers",
			in:        "GET /a HTTP/1.1\r\nHost: x\r\n",
			wantProto: "HTTP/1.1",
		},
		{
			// A body added without a Content-Length must still be forwarded.
			name:      "body without Content-Length is preserved",
			in:        "POST /a HTTP/1.1\r\nHost: x\r\n\r\nfoo=bar",
			wantProto: "HTTP/1.1",
			wantBody:  "foo=bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := parseEditedRequest(tt.in)
			if err != nil {
				t.Fatalf("parseEditedRequest(%q) failed: %v", tt.in, err)
			}
			if req.Proto != tt.wantProto {
				t.Errorf("Proto = %q, want %q", req.Proto, tt.wantProto)
			}
			body, _ := io.ReadAll(req.Body)
			if string(body) != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
			if tt.wantBody != "" && req.ContentLength != int64(len(tt.wantBody)) {
				t.Errorf("ContentLength = %d, want %d", req.ContentLength, len(tt.wantBody))
			}
		})
	}
}

// TestParseEditedRequestPreservesBinaryBody guards the forwarding path against
// rawhttp.ParseRequest's MagicDecompress, which sniffs by trying each codec.
// Brotli has no magic number: it accepts arbitrary input, reads the leading
// bytes as a stream header and returns the rest as literal data. Around 6% of
// random binary bodies come back with their first bytes silently eaten, which
// would corrupt uploads, protobuf and any encrypted payload on the way upstream.
func TestParseEditedRequestPreservesBinaryBody(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 2000; i++ {
		body := make([]byte, 1+r.Intn(48))
		r.Read(body)

		raw := fmt.Sprintf("POST /a HTTP/1.1\r\nHost: x\r\nContent-Length: %d\r\n\r\n%s",
			len(body), body)

		req, err := parseEditedRequest(raw)
		if err != nil {
			t.Fatalf("parseEditedRequest failed for %x: %v", body, err)
		}
		got, _ := io.ReadAll(req.Body)
		if !bytes.Equal(got, body) {
			t.Fatalf("body altered in transit:\n sent %x\n  got %x", body, got)
		}
	}
}

// A body that really is compressed must also survive untouched - we forward
// bytes, we do not re-encode them.
func TestParseEditedRequestDoesNotDecompressBody(t *testing.T) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	zw.Write([]byte("this is the plaintext nobody upstream should see decoded"))
	zw.Close()
	compressed := buf.Bytes()

	raw := fmt.Sprintf("POST /a HTTP/1.1\r\nHost: x\r\nContent-Encoding: gzip\r\nContent-Length: %d\r\n\r\n%s",
		len(compressed), compressed)

	req, err := parseEditedRequest(raw)
	if err != nil {
		t.Fatalf("parseEditedRequest failed: %v", err)
	}
	got, _ := io.ReadAll(req.Body)
	if !bytes.Equal(got, compressed) {
		t.Errorf("gzip body was altered:\n sent %x\n  got %x", compressed, got)
	}
}

// TestParseEditedRequestPreservesHeaderOrder guards against the parser
// reordering or dropping repeated headers, which matters when the point of the
// tool is to send exactly what the user typed.
func TestParseEditedRequestPreservesHeaderOrder(t *testing.T) {
	in := "GET /a HTTP/1.1\r\nHost: x\r\nCookie: a=1\r\nCookie: b=2\r\n\r\n"
	req, err := parseEditedRequest(in)
	if err != nil {
		t.Fatalf("parseEditedRequest failed: %v", err)
	}
	got := req.Header.Values("Cookie")
	if len(got) != 2 || got[0] != "a=1" || got[1] != "b=2" {
		t.Errorf("Cookie headers = %v, want [a=1 b=2]", got)
	}
}

// sanity check that the fixtures above are things http.ReadRequest alone would
// have choked on, i.e. that parseEditedRequest is earning its keep.
func TestReadRequestRejectsWhatWeTolerate(t *testing.T) {
	for _, raw := range []string{
		"GET /a HTTP/2\nHost: x\n\n",
		"GET /a HTTP/1.1\r\nHost: x\r\n",
	} {
		if _, err := http.ReadRequest(bufio.NewReader(strings.NewReader(raw))); err == nil {
			t.Errorf("http.ReadRequest(%q) unexpectedly succeeded; fixture no longer proves tolerance", raw)
		}
	}
}
