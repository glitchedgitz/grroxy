package rawhttp

import (
	"strings"
	"testing"
	"time"
)

// TestHTTP2Request tests sending a request using HTTP/2
func TestHTTP2Request(t *testing.T) {
	// Skip if in CI or if we can't reach the internet
	if testing.Short() {
		t.Skip("Skipping HTTP/2 test in short mode")
	}

	client := NewClient(Config{
		Timeout:            10 * time.Second,
		InsecureSkipVerify: true,
	})

	// Create a simple HTTP request (HTTP/1.x format will be converted to HTTP/2)
	rawRequest := "GET / HTTP/1.1\r\nHost: www.google.com\r\nUser-Agent: grroxy-test\r\n\r\n"

	req := Request{
		RawBytes: []byte(rawRequest),
		Host:     "www.google.com",
		Port:     "443",
		UseTLS:   true,
		UseHTTP2: true,
		Timeout:  10 * time.Second,
	}

	resp, err := client.Send(req)
	if err != nil {
		t.Fatalf("Failed to send HTTP/2 request: %v", err)
	}

	if resp.StatusCode == 0 {
		t.Error("Expected non-zero status code")
	}

	if len(resp.RawBytes) == 0 {
		t.Error("Expected non-empty response")
	}

	// Check that response indicates HTTP/2
	respStr := string(resp.RawBytes)
	if !strings.Contains(respStr, "HTTP/2") {
		t.Errorf("Expected HTTP/2 in response, got: %s", respStr[:100])
	}

	t.Logf("HTTP/2 Request successful!")
	t.Logf("Status Code: %d", resp.StatusCode)
	t.Logf("Status: %s", resp.Status)
	t.Logf("Response Time: %v", resp.ResponseTime)
	t.Logf("Response Length: %d bytes", len(resp.RawBytes))
}

// TestHTTP2VsHTTP1 compares HTTP/2 and HTTP/1.x responses
func TestHTTP2VsHTTP1(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping comparison test in short mode")
	}

	client := NewClient(Config{
		Timeout:            10 * time.Second,
		InsecureSkipVerify: true,
	})

	rawRequest := "GET / HTTP/1.1\r\nHost: www.cloudflare.com\r\nUser-Agent: grroxy-test\r\n\r\n"

	// Test HTTP/1.1
	req1 := Request{
		RawBytes: []byte(rawRequest),
		Host:     "www.cloudflare.com",
		Port:     "443",
		UseTLS:   true,
		UseHTTP2: false,
		Timeout:  10 * time.Second,
	}

	resp1, err1 := client.Send(req1)
	if err1 != nil {
		t.Logf("HTTP/1.1 request failed (expected on some servers): %v", err1)
	} else {
		t.Logf("HTTP/1.1 - Status: %d, Time: %v", resp1.StatusCode, resp1.ResponseTime)
	}

	// Test HTTP/2
	req2 := Request{
		RawBytes: []byte(rawRequest),
		Host:     "www.cloudflare.com",
		Port:     "443",
		UseTLS:   true,
		UseHTTP2: true,
		Timeout:  10 * time.Second,
	}

	resp2, err2 := client.Send(req2)
	if err2 != nil {
		t.Fatalf("HTTP/2 request failed: %v", err2)
	}

	t.Logf("HTTP/2 - Status: %d, Time: %v", resp2.StatusCode, resp2.ResponseTime)

	if resp2.StatusCode == 0 {
		t.Error("Expected non-zero status code for HTTP/2")
	}
}

// TestHTTP2WithoutTLS tests that HTTP/2 requires TLS
func TestHTTP2WithoutTLS(t *testing.T) {
	client := DefaultClient()

	req := Request{
		RawBytes: []byte("GET / HTTP/1.1\r\nHost: example.com\r\n\r\n"),
		Host:     "example.com",
		Port:     "80",
		UseTLS:   false,
		UseHTTP2: true,
		Timeout:  5 * time.Second,
	}

	_, err := client.Send(req)
	if err == nil {
		t.Error("Expected error when using HTTP/2 without TLS")
	}

	if !strings.Contains(err.Error(), "requires TLS") {
		t.Errorf("Expected 'requires TLS' error, got: %v", err)
	}
}
