package rawhttp

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/glitchedgitz/grroxy-db/internal/utils"
)

// DumpRequest dumps an HTTP request to a string format.
// It normalizes HTTP proxy requests to use path-only format (like HTTPS requests).
// IMPORTANT: This function restores the request body so it can still be forwarded.
func DumpRequest(req *http.Request, normalizeHTTP bool) string {
	// Read the body first
	originalBody, err := io.ReadAll(req.Body)
	utils.CheckErr("[DumpRequest] Read body error: ", err)

	// Close the original body to release resources
	req.Body.Close()

	// CRITICAL: Restore the body so it can be forwarded
	req.Body = io.NopCloser(bytes.NewReader(originalBody))

	// Determine the URL to use in the request line
	url := req.URL.RequestURI()

	// Normalize HTTP proxy requests if requested
	// HTTP proxy format: "GET http://example.com/path HTTP/1.1"
	// Normalized format:  "GET /path HTTP/1.1"
	if normalizeHTTP && req.URL.Scheme == "http" && req.URL.Host != "" {
		// Use only the path part for HTTP requests
		url = req.URL.Path
		if req.URL.RawQuery != "" {
			url += "?" + req.URL.RawQuery
		}
		if req.URL.Fragment != "" {
			url += "#" + req.URL.Fragment
		}
		if url == "" {
			url = "/"
		}
	}

	// Build request line: METHOD URL PROTO
	finalReq := fmt.Sprintf("%s %s %s\n", req.Method, url, req.Proto)

	// Add Host header if not already in req.Header
	hasHost := false

	for header := range req.Header {
		if strings.EqualFold(header, "Host") {
			hasHost = true
			break
		}
	}

	if !hasHost && req.Host != "" {
		finalReq += fmt.Sprintf("Host: %s\n", req.Host)
	}

	// Add other headers
	for header, values := range req.Header {
		for _, value := range values {
			finalReq += fmt.Sprintf("%s: %s\n", header, value)
		}
	}

	// Add body if present
	if len(originalBody) > 0 {
		finalReq += "\n" + string(originalBody)
	}

	return finalReq
}

func GetHeaders(h http.Header) map[string]string {
	headers := map[string]string{}
	for header, value := range h {
		// header = strings.ReplaceAll(header, "-", "_")
		// header = strings.ToLower(header)
		headers[header] = strings.Join(value, " ///// ")
	}
	return headers
}
