package rawhttp

import (
	"os"
	"strings"
	"testing"
)

func TestReadFromString(t *testing.T) {
	rawRequest := "GET / HTTP/1.1\r\nHost: example.com\r\n\r\n"
	result := ReadFromString(rawRequest)

	if string(result) != rawRequest {
		t.Errorf("Expected %q, got %q", rawRequest, string(result))
	}
}

func TestReadFromFile(t *testing.T) {
	// Create a temporary file
	tmpfile, err := os.CreateTemp("", "rawhttp_test_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	testContent := "GET /test HTTP/1.1\r\nHost: example.com\r\nX-Test: value\r\n\r\nbody content"
	if _, err := tmpfile.WriteString(testContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpfile.Close()

	// Read from file
	result, err := ReadFromFile(tmpfile.Name())
	if err != nil {
		t.Fatalf("ReadFromFile failed: %v", err)
	}

	if string(result) != testContent {
		t.Errorf("Expected %q, got %q", testContent, string(result))
	}
}

func TestReadFromFile_NonExistent(t *testing.T) {
	_, err := ReadFromFile("/nonexistent/file/path")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

func TestReadFromReader(t *testing.T) {
	testContent := "POST /api HTTP/1.1\r\nHost: api.example.com\r\nContent-Length: 11\r\n\r\nhello world"
	reader := strings.NewReader(testContent)

	result, err := ReadFromReader(reader)
	if err != nil {
		t.Fatalf("ReadFromReader failed: %v", err)
	}

	if string(result) != testContent {
		t.Errorf("Expected %q, got %q", testContent, string(result))
	}
}

func TestExtractHost(t *testing.T) {
	tests := []struct {
		name         string
		rawRequest   string
		expectedHost string
	}{
		{
			name:         "Standard Host header",
			rawRequest:   "GET / HTTP/1.1\r\nHost: example.com\r\n\r\n",
			expectedHost: "example.com",
		},
		{
			name:         "Host with port",
			rawRequest:   "GET / HTTP/1.1\r\nHost: example.com:8080\r\n\r\n",
			expectedHost: "example.com",
		},
		{
			name:         "Host header with spaces",
			rawRequest:   "GET / HTTP/1.1\r\nHost:  example.com  \r\n\r\n",
			expectedHost: "example.com",
		},
		{
			name:         "Lowercase host header",
			rawRequest:   "GET / HTTP/1.1\r\nhost: test.example.com\r\n\r\n",
			expectedHost: "test.example.com",
		},
		{
			name:         "Mixed case host header",
			rawRequest:   "GET / HTTP/1.1\r\nHoSt: mixed.example.com\r\n\r\n",
			expectedHost: "mixed.example.com",
		},
		{
			name:         "No Host header",
			rawRequest:   "GET / HTTP/1.1\r\nX-Other: value\r\n\r\n",
			expectedHost: "",
		},
		{
			name:         "Host header after other headers",
			rawRequest:   "GET / HTTP/1.1\r\nX-First: value\r\nHost: last.example.com\r\n\r\n",
			expectedHost: "last.example.com",
		},
		{
			name:         "Empty request",
			rawRequest:   "",
			expectedHost: "",
		},
		{
			name:         "Malformed header (no colon)",
			rawRequest:   "GET / HTTP/1.1\r\nHost example.com\r\n\r\n",
			expectedHost: "",
		},
		{
			name:         "Host header in body",
			rawRequest:   "GET / HTTP/1.1\r\nHost: header.example.com\r\n\r\nHost: body.example.com",
			expectedHost: "header.example.com", // Should find header, not body
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractHost([]byte(tt.rawRequest))
			if result != tt.expectedHost {
				t.Errorf("Expected host %q, got %q", tt.expectedHost, result)
			}
		})
	}
}
