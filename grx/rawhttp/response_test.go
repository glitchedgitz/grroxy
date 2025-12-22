package rawhttp

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
)

func TestDecompressResponse(t *testing.T) {
	tests := []struct {
		name               string
		contentEncoding    string
		originalData       string
		expectDecompressed bool
	}{
		{
			name:               "gzip compression",
			contentEncoding:    "gzip",
			originalData:       "Hello, this is a test message for gzip compression!",
			expectDecompressed: true,
		},
		{
			name:               "brotli compression",
			contentEncoding:    "br",
			originalData:       "Hello, this is a test message for brotli compression!",
			expectDecompressed: true,
		},
		{
			name:               "x-gzip compression",
			contentEncoding:    "x-gzip",
			originalData:       "Hello, this is a test message for x-gzip compression!",
			expectDecompressed: true,
		},
		{
			name:               "no compression",
			contentEncoding:    "",
			originalData:       "Hello, this is uncompressed data!",
			expectDecompressed: false,
		},
		{
			name:               "unknown compression",
			contentEncoding:    "deflate",
			originalData:       "Hello, this is deflate data!",
			expectDecompressed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var compressedReader io.Reader

			// Create compressed data if needed
			if tt.expectDecompressed {
				switch strings.ToLower(tt.contentEncoding) {
				case "gzip", "x-gzip":
					var buf bytes.Buffer
					gw := gzip.NewWriter(&buf)
					gw.Write([]byte(tt.originalData))
					gw.Close()
					compressedReader = bytes.NewReader(buf.Bytes())
				case "br", "brotli":
					var buf bytes.Buffer
					bw := brotli.NewWriter(&buf)
					bw.Write([]byte(tt.originalData))
					bw.Close()
					compressedReader = bytes.NewReader(buf.Bytes())
				default:
					compressedReader = strings.NewReader(tt.originalData)
				}
			} else {
				compressedReader = strings.NewReader(tt.originalData)
			}

			// Test decompression
			decompressedReader, err := DecompressResponse(compressedReader, tt.contentEncoding)
			if err != nil {
				t.Fatalf("DecompressResponse failed: %v", err)
			}

			// Read the decompressed data
			decompressedData, err := io.ReadAll(decompressedReader)
			if err != nil {
				t.Fatalf("Failed to read decompressed data: %v", err)
			}

			// Verify the result
			if string(decompressedData) != tt.originalData {
				t.Errorf("Expected: %s, Got: %s", tt.originalData, string(decompressedData))
			}
		})
	}
}

func TestDumpResponse(t *testing.T) {
	tests := []struct {
		name            string
		contentEncoding string
		originalData    string
		statusCode      int
		headers         map[string]string
	}{
		{
			name:            "gzip compressed response",
			contentEncoding: "gzip",
			originalData:    "This is a gzip compressed response body",
			statusCode:      200,
			headers: map[string]string{
				"Content-Type": "text/plain",
				"Server":       "test-server",
			},
		},
		{
			name:            "brotli compressed response",
			contentEncoding: "br",
			originalData:    "This is a brotli compressed response body",
			statusCode:      200,
			headers: map[string]string{
				"Content-Type":  "application/json",
				"Cache-Control": "no-cache",
			},
		},
		{
			name:            "uncompressed response",
			contentEncoding: "",
			originalData:    "This is an uncompressed response body",
			statusCode:      200,
			headers: map[string]string{
				"Content-Type": "text/html",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create response body
			var bodyReader io.Reader
			if tt.contentEncoding != "" {
				switch strings.ToLower(tt.contentEncoding) {
				case "gzip", "x-gzip":
					var buf bytes.Buffer
					gw := gzip.NewWriter(&buf)
					gw.Write([]byte(tt.originalData))
					gw.Close()
					bodyReader = bytes.NewReader(buf.Bytes())
				case "br", "brotli":
					var buf bytes.Buffer
					bw := brotli.NewWriter(&buf)
					bw.Write([]byte(tt.originalData))
					bw.Close()
					bodyReader = bytes.NewReader(buf.Bytes())
				default:
					bodyReader = strings.NewReader(tt.originalData)
				}
			} else {
				bodyReader = strings.NewReader(tt.originalData)
			}

			// Create HTTP response
			resp := &http.Response{
				StatusCode: tt.statusCode,
				Status:     http.StatusText(tt.statusCode),
				Proto:      "HTTP/1.1",
				Header:     make(http.Header),
				Body:       io.NopCloser(bodyReader),
			}

			// Set headers
			if tt.contentEncoding != "" {
				resp.Header.Set("Content-Encoding", tt.contentEncoding)
			}
			for key, value := range tt.headers {
				resp.Header.Set(key, value)
			}

			// Test DumpResponse
			result := DumpResponse(resp)

			// Verify the result contains the decompressed data
			if !strings.Contains(result, tt.originalData) {
				t.Errorf("DumpResponse result does not contain original data. Expected: %s", tt.originalData)
			}

			// Verify Content-Encoding header is removed
			if tt.contentEncoding != "" && strings.Contains(result, "Content-Encoding") {
				t.Errorf("DumpResponse should remove Content-Encoding header")
			}

			// Verify Content-Length is updated to reflect decompressed size
			expectedLength := len(tt.originalData)
			if !strings.Contains(result, fmt.Sprintf("Content-Length: %d", expectedLength)) {
				t.Errorf("DumpResponse should update Content-Length to %d", expectedLength)
			}

			t.Logf("DumpResponse result:\n%s", result)
		})
	}
}

func TestDumpResponseWithChunkedEncoding(t *testing.T) {
	// Test with chunked transfer encoding (no Content-Length)
	originalData := "This is chunked data that should be decompressed"

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	gw.Write([]byte(originalData))
	gw.Close()

	resp := &http.Response{
		StatusCode: 200,
		Status:     "200 OK",
		Proto:      "HTTP/1.1",
		Header: http.Header{
			"Content-Encoding":  []string{"gzip"},
			"Transfer-Encoding": []string{"chunked"},
			"Content-Type":      []string{"text/plain"},
		},
		Body: io.NopCloser(bytes.NewReader(buf.Bytes())),
	}

	result := DumpResponse(resp)

	// Should still decompress even without Content-Length
	if !strings.Contains(result, originalData) {
		t.Errorf("DumpResponse should decompress chunked data. Expected: %s", originalData)
	}

	t.Logf("Chunked response result:\n%s", result)
}
