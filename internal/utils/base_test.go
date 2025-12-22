package utils

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

func TestResponseToByte(t *testing.T) {
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
			originalData:    "This is a gzip compressed response body for testing",
			statusCode:      200,
			headers: map[string]string{
				"Content-Type": "text/plain",
				"Server":       "test-server",
			},
		},
		{
			name:            "brotli compressed response",
			contentEncoding: "br",
			originalData:    "This is a brotli compressed response body for testing",
			statusCode:      200,
			headers: map[string]string{
				"Content-Type":  "application/json",
				"Cache-Control": "no-cache",
			},
		},
		{
			name:            "uncompressed response",
			contentEncoding: "",
			originalData:    "This is an uncompressed response body for testing",
			statusCode:      200,
			headers: map[string]string{
				"Content-Type": "text/html",
			},
		},
		{
			name:            "x-gzip compressed response",
			contentEncoding: "x-gzip",
			originalData:    "This is an x-gzip compressed response body for testing",
			statusCode:      200,
			headers: map[string]string{
				"Content-Type": "text/xml",
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

			// Test ResponseToByte
			result, err := ResponseToByte(resp)
			if err != nil {
				t.Fatalf("ResponseToByte failed: %v", err)
			}

			resultStr := string(result)

			// Verify the result contains the decompressed data
			if !strings.Contains(resultStr, tt.originalData) {
				t.Errorf("ResponseToByte result does not contain original data. Expected: %s", tt.originalData)
			}

			// Verify Content-Encoding header is removed
			if tt.contentEncoding != "" && strings.Contains(resultStr, "Content-Encoding") {
				t.Errorf("ResponseToByte should remove Content-Encoding header")
			}

			// Verify Content-Length is updated to reflect decompressed size
			expectedLength := len(tt.originalData)
			if !strings.Contains(resultStr, fmt.Sprintf("Content-Length: %d", expectedLength)) {
				t.Errorf("ResponseToByte should update Content-Length to %d", expectedLength)
			}

			// Verify it's a valid HTTP response format
			if !strings.HasPrefix(resultStr, "HTTP/") {
				t.Errorf("ResponseToByte should return a valid HTTP response format")
			}

			t.Logf("ResponseToByte result:\n%s", resultStr)
		})
	}
}

func TestResponseToByteWithChunkedEncoding(t *testing.T) {
	// Test with chunked transfer encoding (no Content-Length)
	originalData := "This is chunked data that should be decompressed by ResponseToByte"

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

	result, err := ResponseToByte(resp)
	if err != nil {
		t.Fatalf("ResponseToByte failed: %v", err)
	}

	resultStr := string(result)

	// Should still decompress even without Content-Length
	if !strings.Contains(resultStr, originalData) {
		t.Errorf("ResponseToByte should decompress chunked data. Expected: %s", originalData)
	}

	// Should remove Content-Encoding header
	if strings.Contains(resultStr, "Content-Encoding") {
		t.Errorf("ResponseToByte should remove Content-Encoding header for chunked responses")
	}

	t.Logf("Chunked response result:\n%s", resultStr)
}

func TestResponseToByteErrorHandling(t *testing.T) {
	// Test with corrupted gzip data
	corruptedData := []byte("This is not valid gzip data")

	resp := &http.Response{
		StatusCode: 200,
		Status:     "200 OK",
		Proto:      "HTTP/1.1",
		Header: http.Header{
			"Content-Encoding": []string{"gzip"},
			"Content-Type":     []string{"text/plain"},
		},
		Body: io.NopCloser(bytes.NewReader(corruptedData)),
	}

	// Should not fail, but should return the original data
	result, err := ResponseToByte(resp)
	if err != nil {
		t.Fatalf("ResponseToByte should handle corrupted gzip data gracefully: %v", err)
	}

	resultStr := string(result)

	// Should contain the original corrupted data
	if !strings.Contains(resultStr, string(corruptedData)) {
		t.Errorf("ResponseToByte should return original data when decompression fails")
	}

	t.Logf("Error handling result:\n%s", resultStr)
}
