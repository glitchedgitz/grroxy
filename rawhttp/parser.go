package rawhttp

import (
	"bytes"
	"io"
	"strconv"
	"strings"

	"github.com/glitchedgitz/grroxy-db/grrhttp"
)

// ParsedRequest is a minimal parsed shape for a raw HTTP request
type ParsedRequest struct {
	Method      string
	URL         string
	HTTPVersion string
	Headers     map[string]string
	Body        string
	LineBreak   string
}

// ParsedResponse is a minimal parsed shape for a raw HTTP response
type ParsedResponse struct {
	Version    string
	Status     int
	StatusFull string
	Headers    map[string]string
	Body       string
	LineBreak  string
}

// ParseRequest performs a tolerant, minimal parse of a raw HTTP request.
// It extracts method, URL, HTTP version, headers (collapsed by first key occurrence, case-insensitive), and body.
func ParseRequest(raw []byte) ParsedRequest {
	method, url, httpVersion := "", "", ""
	headers := make(map[string]string)
	body := ""
	lineBreak := detectLineBreak(raw)

	if len(raw) == 0 {
		return ParsedRequest{Method: method, URL: url, HTTPVersion: httpVersion, Headers: headers, Body: body, LineBreak: lineBreak}
	}

	// Detect header/body separator (prefer \r\n\r\n, fallback to \n\n)
	sep := []byte("\r\n\r\n")
	idx := bytes.Index(raw, sep)
	if idx < 0 {
		sep = []byte("\n\n")
		idx = bytes.Index(raw, sep)
	}

	headerPart := raw
	bodyBytes := []byte{}
	if idx >= 0 {
		headerPart = raw[:idx]
		bodyBytes = raw[idx+len(sep):]
	}

	// Decompress body if needed
	body = decompressBody(bodyBytes)

	// Split header lines by either CRLF or LF
	headerText := string(headerPart)
	lines := splitLines(headerText)
	if len(lines) == 0 {
		return ParsedRequest{Method: method, URL: url, HTTPVersion: httpVersion, Headers: headers, Body: body, LineBreak: lineBreak}
	}

	// Parse request line: METHOD SP URL SP HTTP/X.Y
	reqLine := strings.TrimSpace(lines[0])
	if reqLine != "" {
		parts := strings.Fields(reqLine)
		if len(parts) >= 1 {
			method = parts[0]
		}
		if len(parts) >= 2 {
			url = parts[1]
		}
		if len(parts) >= 3 {
			httpVersion = parts[2]
		}
	}

	// Parse headers (lines after request line until empty)
	for i := 1; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			break
		}
		// Support simple header folding: if line starts with space or tab, append to previous header
		if (len(line) > 0) && (line[0] == ' ' || line[0] == '\t') {
			// Try to append to the last inserted header key
			// Find last key; map has no order, so we cannot reliably append.
			// Skip folding to keep parser minimal and deterministic.
			continue
		}
		if idx := strings.IndexByte(line, ':'); idx >= 0 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			// Use case-insensitive keys by normalizing to lower case
			lk := strings.ToLower(key)
			if existing, ok := headers[lk]; ok && existing != "" {
				headers[lk] = existing + ", " + val
			} else {
				headers[lk] = val
			}
		}
	}

	return ParsedRequest{Method: method, URL: url, HTTPVersion: httpVersion, Headers: headers, Body: body, LineBreak: lineBreak}
}

// ParseResponse performs a tolerant, minimal parse of a raw HTTP response.
// It extracts version, numeric status, full status line, headers (case-insensitive), and body.
func ParseResponse(raw []byte) ParsedResponse {
	version := ""
	status := 0
	statusFull := ""
	headers := make(map[string]string)
	body := ""
	lineBreak := detectLineBreak(raw)

	if len(raw) == 0 {
		return ParsedResponse{Version: version, Status: status, StatusFull: statusFull, Headers: headers, Body: body, LineBreak: lineBreak}
	}

	// Detect header/body separator
	sep := []byte("\r\n\r\n")
	idx := bytes.Index(raw, sep)
	if idx < 0 {
		sep = []byte("\n\n")
		idx = bytes.Index(raw, sep)
	}

	headerPart := raw
	bodyBytes := []byte{}
	if idx >= 0 {
		headerPart = raw[:idx]
		bodyBytes = raw[idx+len(sep):]
	}

	// Decompress body if needed
	body = decompressBody(bodyBytes)

	headerText := string(headerPart)
	lines := splitLines(headerText)
	if len(lines) == 0 {
		return ParsedResponse{Version: version, Status: status, StatusFull: statusFull, Headers: headers, Body: body, LineBreak: lineBreak}
	}

	// Parse status line: HTTP/X.Y SP 3DIGIT SP REASON
	statusLine := strings.TrimSpace(lines[0])
	statusFull = statusLine
	if statusLine != "" {
		parts := strings.Fields(statusLine)
		if len(parts) >= 1 {
			version = parts[0]
		}
		if len(parts) >= 2 {
			if code, err := strconv.Atoi(parts[1]); err == nil {
				status = code
			}
		}
	}

	// Headers
	for i := 1; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			break
		}
		if (len(line) > 0) && (line[0] == ' ' || line[0] == '\t') {
			continue
		}
		if idx := strings.IndexByte(line, ':'); idx >= 0 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			lk := strings.ToLower(key)
			if existing, ok := headers[lk]; ok && existing != "" {
				headers[lk] = existing + ", " + val
			} else {
				headers[lk] = val
			}
		}
	}

	return ParsedResponse{Version: version, Status: status, StatusFull: statusFull, Headers: headers, Body: body, LineBreak: lineBreak}
}

// detectLineBreak detects the line break style used in the raw HTTP message.
// Returns "\r\n" (CRLF), "\n" (LF), "\r" (CR), or "" if none detected.
func detectLineBreak(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}

	// Look for CRLF first (most common in HTTP)
	if bytes.Contains(raw, []byte("\r\n")) {
		return "\r\n"
	}

	// Look for LF (Unix-style)
	if bytes.Contains(raw, []byte("\n")) {
		return "\n"
	}

	// Look for CR (old Mac-style, rare in HTTP)
	if bytes.Contains(raw, []byte("\r")) {
		return "\r"
	}

	return ""
}

// decompressBody attempts to decompress the body using MagicDecompress.
// If decompression fails or body is empty, returns the original body as string.
func decompressBody(bodyBytes []byte) string {
	if len(bodyBytes) == 0 {
		return ""
	}

	// Try to decompress using MagicDecompress
	decompressedReader, err := grrhttp.MagicDecompress(bytes.NewReader(bodyBytes))
	if err != nil {
		// If decompression fails, return original body
		return string(bodyBytes)
	}

	// Read the decompressed body
	decompressedBytes, err := io.ReadAll(decompressedReader)
	if err != nil {
		// If reading fails, return original body
		return string(bodyBytes)
	}

	return string(decompressedBytes)
}

func splitLines(s string) []string {
	// Split on CRLF first, then normalize LF
	// strings.Split will keep empty last item if trailing newline; that's fine
	if strings.Contains(s, "\r\n") {
		return strings.Split(s, "\r\n")
	}
	return strings.Split(s, "\n")
}
