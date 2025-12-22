package rawhttp

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadFromRequestsFolder(t *testing.T) {
	// Get the requests folder path
	requestsDir := filepath.Join("requests")

	// Test reading simple_get.txt
	simpleGetPath := filepath.Join(requestsDir, "simple_get.txt")
	rawBytes, err := ReadFromFile(simpleGetPath)
	if err != nil {
		t.Fatalf("Failed to read simple_get.txt: %v", err)
	}

	if len(rawBytes) == 0 {
		t.Error("simple_get.txt should not be empty")
	}

	// Verify it contains expected content
	content := string(rawBytes)
	if !strings.Contains(content, "GET / HTTP/1.1") {
		t.Error("simple_get.txt should contain 'GET / HTTP/1.1'")
	}
	if !strings.Contains(content, "Host: example.com") {
		t.Error("simple_get.txt should contain 'Host: example.com'")
	}
}

func TestReadMalformedRequestFromFolder(t *testing.T) {
	requestsDir := filepath.Join("requests")
	malformedPath := filepath.Join(requestsDir, "malformed_headers.txt")

	rawBytes, err := ReadFromFile(malformedPath)
	if err != nil {
		t.Fatalf("Failed to read malformed_headers.txt: %v", err)
	}

	content := string(rawBytes)

	// Verify malformations are preserved
	if !strings.Contains(content, "Invalid Header No Colon") {
		t.Error("malformed_headers.txt should contain 'Invalid Header No Colon'")
	}

	if !strings.Contains(content, "  Leading Spaces: value") {
		t.Error("malformed_headers.txt should preserve leading spaces")
	}

	if !strings.Contains(content, "Trailing Spaces : value") {
		t.Error("malformed_headers.txt should preserve trailing spaces")
	}
}

func TestReadAllRequestFiles(t *testing.T) {
	requestsDir := filepath.Join("requests")

	// List of expected files
	expectedFiles := []string{
		"simple_get.txt",
		"post_with_body.txt",
		"malformed_headers.txt",
		"http_1_0.txt",
		"custom_method.txt",
		"whitespace_issues.txt",
		"lowercase_headers.txt",
	}

	for _, filename := range expectedFiles {
		filepath := filepath.Join(requestsDir, filename)

		// Check if file exists
		if _, err := os.Stat(filepath); os.IsNotExist(err) {
			t.Errorf("Expected file %s does not exist", filepath)
			continue
		}

		// Try to read it
		rawBytes, err := ReadFromFile(filepath)
		if err != nil {
			t.Errorf("Failed to read %s: %v", filename, err)
			continue
		}

		if len(rawBytes) == 0 {
			t.Errorf("File %s is empty", filename)
		}
	}
}

func TestClient_SendFile_FromRequestsFolder(t *testing.T) {
	// Start a simple HTTP server
	server := startTestServer(t, func(conn net.Conn) {
		defer conn.Close()
		buf := make([]byte, 2048)
		conn.Read(buf)
		response := "HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nOK"
		conn.Write([]byte(response))
	})
	defer server.Close()

	addr := server.Addr().String()
	host, port, _ := net.SplitHostPort(addr)

	client := DefaultClient()
	client.config.Timeout = 2 * time.Second

	// Test sending from requests folder
	requestsDir := filepath.Join("requests")
	simpleGetPath := filepath.Join(requestsDir, "simple_get.txt")

	resp, err := client.SendFile(simpleGetPath, host, port, false)
	if err != nil {
		t.Fatalf("SendFile failed: %v", err)
	}

	if resp == nil {
		t.Fatal("Response should not be nil")
	}

	if len(resp.RawBytes) == 0 {
		t.Error("Response should have content")
	}
}
