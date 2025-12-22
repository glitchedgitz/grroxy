package rawhttp

import (
	"bytes"
	"io"
	"strconv"
	"strings"
)

// ParsedRequest is a minimal parsed shape for a raw HTTP request
type ParsedRequest struct {
	Method        string
	URL           string
	HTTPVersion   string
	Headers       [][]string // Array of [key, value] pairs
	Body          string
	LineBreak     string
	BodySeparator string // The body separator found in the original (e.g., "\r\n\r\n", "\n\n", or empty if none)
}

// ParsedResponse is a minimal parsed shape for a raw HTTP response
type ParsedResponse struct {
	Version       string
	Status        int
	StatusFull    string
	Headers       [][]string // Array of [key, value] pairs
	Body          string
	LineBreak     string
	BodySeparator string // The body separator found in the original (e.g., "\r\n\r\n", "\n\n", or empty if none)
}

// ParseRequest performs a tolerant, minimal parse of a raw HTTP request.
// It extracts method, URL, HTTP version, headers (as array of [key, value] pairs), and body.
func ParseRequest(raw []byte) ParsedRequest {
	method, url, httpVersion := "", "", ""
	headers := [][]string{}
	body := ""
	lineBreak := detectLineBreak(raw)

	bodySeparator := ""
	if len(raw) == 0 {
		return ParsedRequest{Method: method, URL: url, HTTPVersion: httpVersion, Headers: headers, Body: body, LineBreak: lineBreak, BodySeparator: bodySeparator}
	}

	// Detect header/body separator (prefer \r\n\r\n, fallback to \n\n)
	sep := []byte(lineBreak + lineBreak)
	idx := bytes.Index(raw, sep)
	if idx < 0 {
		sep = []byte("\n\n")
		idx = bytes.Index(raw, sep)
	}

	headerPart := raw
	bodyBytes := []byte{}
	if idx >= 0 {
		bodySeparator = string(sep)
		headerPart = raw[:idx]
		bodyBytes = raw[idx+len(sep):]
	} else {
		// No body separator found - check if the header part ends with a line break
		// This helps us preserve trailing newlines when unparsing
		// Note: headerPart is the same as raw when there's no separator
		if len(headerPart) > 0 {
			if len(headerPart) >= 2 && headerPart[len(headerPart)-2] == '\r' && headerPart[len(headerPart)-1] == '\n' {
				bodySeparator = "\r\n"
			} else if len(headerPart) > 0 && headerPart[len(headerPart)-1] == '\n' {
				bodySeparator = "\n"
			}
		}
	}

	// Decompress body if needed
	body = decompressBody(bodyBytes)

	// Split header lines by either CRLF or LF
	headerText := string(headerPart)
	lines := splitLines(headerText)
	if len(lines) == 0 {
		return ParsedRequest{Method: method, URL: url, HTTPVersion: httpVersion, Headers: headers, Body: body, LineBreak: lineBreak, BodySeparator: bodySeparator}
	}

	// Parse request line: METHOD SP URL SP HTTP/X.Y
	reqLine := lines[0]
	if reqLine != "" {
		parts := strings.Fields(reqLine)
		// Handle missing method case: if first part starts with /, it's a URL (method is missing)
		if len(parts) >= 1 {
			firstPart := parts[0]
			if strings.HasPrefix(firstPart, "/") {
				// Missing method - first part is URL
				url = firstPart
				if len(parts) >= 2 {
					httpVersion = parts[1]
				}
			} else {
				// Normal case: METHOD URL VERSION
				method = firstPart
				if len(parts) >= 2 {
					url = parts[1]
				}
				if len(parts) >= 3 {
					httpVersion = parts[2]
				}
			}
		}
	}

	// Parse headers (lines after request line until empty)
	for i := 1; i < len(lines); i++ {
		line := lines[i]
		if line == "" {
			break
		}
		// Check if line has a colon (header separator)
		colonIdx := strings.IndexByte(line, ':')
		// Support simple header folding: if line starts with space or tab AND has no colon, append to previous header
		if (len(line) > 0) && (line[0] == ' ' || line[0] == '\t') && colonIdx < 0 {
			// Append to the last header value if there is one
			if len(headers) > 0 {
				headers[len(headers)-1][1] += " " + line
			}
			continue
		}
		// If line has a colon, parse it as a header (keep raw key and value)
		if colonIdx >= 0 {
			key := line[:colonIdx]
			val := ""
			if colonIdx+1 < len(line) {
				val = line[colonIdx+1:]
			}
			headers = append(headers, []string{key + ":", val})
		} else {
			// If line has no colon, save entire line as key with empty value (malformed header)
			headers = append(headers, []string{line, ""})
		}
	}

	return ParsedRequest{Method: method, URL: url, HTTPVersion: httpVersion, Headers: headers, Body: body, LineBreak: lineBreak, BodySeparator: bodySeparator}
}

// ParseResponse performs a tolerant, minimal parse of a raw HTTP response.
// It extracts version, numeric status, full status line, headers (as array of [key, value] pairs), and body.
func ParseResponse(raw []byte) ParsedResponse {
	version := ""
	status := 0
	statusFull := ""
	headers := [][]string{}
	body := ""
	lineBreak := detectLineBreak(raw)

	bodySeparator := ""
	if len(raw) == 0 {
		return ParsedResponse{Version: version, Status: status, StatusFull: statusFull, Headers: headers, Body: body, LineBreak: lineBreak, BodySeparator: bodySeparator}
	}

	// Detect header/body separator (prefer lineBreak+lineBreak, fallback to \r\n\r\n, \n\n, \r\r)
	sep := []byte(lineBreak + lineBreak)
	idx := bytes.Index(raw, sep)
	if idx < 0 {
		sep = []byte("\r\n\r\n")
		idx = bytes.Index(raw, sep)
	}
	if idx < 0 {
		sep = []byte("\n\n")
		idx = bytes.Index(raw, sep)
	}
	if idx < 0 {
		sep = []byte("\r\r")
		idx = bytes.Index(raw, sep)
	}

	headerPart := raw
	bodyBytes := []byte{}
	if idx >= 0 {
		bodySeparator = string(sep)
		headerPart = raw[:idx]
		bodyBytes = raw[idx+len(sep):]
	} else {
		// No body separator found - check if the header part ends with a line break
		// This helps us preserve trailing newlines when unparsing
		// Note: headerPart is the same as raw when there's no separator
		if len(headerPart) > 0 {
			if len(headerPart) >= 2 && headerPart[len(headerPart)-2] == '\r' && headerPart[len(headerPart)-1] == '\n' {
				bodySeparator = "\r\n"
			} else if len(headerPart) > 0 && headerPart[len(headerPart)-1] == '\n' {
				bodySeparator = "\n"
			}
		}
	}

	// Decompress body if needed
	body = decompressBody(bodyBytes)

	headerText := string(headerPart)
	lines := splitLines(headerText)
	if len(lines) == 0 {
		return ParsedResponse{Version: version, Status: status, StatusFull: statusFull, Headers: headers, Body: body, LineBreak: lineBreak, BodySeparator: bodySeparator}
	}

	// Parse status line: HTTP/X.Y SP 3DIGIT SP REASON
	statusLine := lines[0]
	statusFull = statusLine
	if statusLine != "" {
		parts := strings.Fields(statusLine)
		// Handle missing version case: if first part is a number, treat as status code
		if len(parts) >= 1 {
			firstPart := parts[0]
			if code, err := strconv.Atoi(firstPart); err == nil {
				// First part is a number - missing version, this is the status code
				status = code
				if len(parts) >= 2 {
					// parts[1] would be the reason phrase
				}
			} else {
				// Normal case: VERSION STATUS REASON
				version = firstPart
				if len(parts) >= 2 {
					if code, err := strconv.Atoi(parts[1]); err == nil {
						status = code
					}
				}
			}
		}
	}

	// Headers
	for i := 1; i < len(lines); i++ {
		line := lines[i]
		if line == "" {
			break
		}
		// Check if line has a colon (header separator)
		colonIdx := strings.IndexByte(line, ':')
		// Support simple header folding: if line starts with space or tab AND has no colon, append to previous header
		if (len(line) > 0) && (line[0] == ' ' || line[0] == '\t') && colonIdx < 0 {
			// Append to the last header value if there is one
			if len(headers) > 0 {
				headers[len(headers)-1][1] += " " + line
			}
			continue
		}
		// If line has a colon, parse it as a header (keep raw key and value)
		if colonIdx >= 0 {
			key := line[:colonIdx]
			val := ""
			if colonIdx+1 < len(line) {
				val = line[colonIdx+1:]
			}
			headers = append(headers, []string{key + ":", val})
		} else {
			// If line has no colon, save entire line as key with empty value (malformed header)
			headers = append(headers, []string{line, ""})
		}
	}

	return ParsedResponse{Version: version, Status: status, StatusFull: statusFull, Headers: headers, Body: body, LineBreak: lineBreak, BodySeparator: bodySeparator}
}

// detectLineBreak detects the line break style used in the raw HTTP message.
// It finds the first \n and checks if the previous byte is \r.
// Returns "\r\n" (CRLF), "\n" (LF), "\r" (CR), or "" if none detected.
func detectLineBreak(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}

	// Find the first \n and check if previous byte is \r
	for i := 0; i < len(raw); i++ {
		if raw[i] == '\n' {
			// Check previous byte if it exists
			if i > 0 && raw[i-1] == '\r' {
				return "\r\n"
			}
			return "\n"
		}
	}

	// If no \n found, check for \r (old Mac style)
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
	decompressedReader, err := MagicDecompress(bytes.NewReader(bodyBytes))
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
	// Split on CRLF first, then LF, then CR (old Mac style)
	// strings.Split will keep empty last item if trailing newline; that's fine
	if strings.Contains(s, "\r\n") {
		return strings.Split(s, "\r\n")
	}
	if strings.Contains(s, "\n") {
		return strings.Split(s, "\n")
	}
	if strings.Contains(s, "\r") {
		return strings.Split(s, "\r")
	}
	return []string{s}
}

// GetHeaderValue returns the first header value matching the key (case-insensitive)
// Returns raw key and value without trimming
func GetHeaderValue(headers [][]string, key string) (string, bool) {
	keyLower := strings.ToLower(key)
	for _, header := range headers {
		if len(header) >= 2 && strings.ToLower(header[0]) == keyLower {
			return header[1], true
		}
	}
	return "", false
}

// UnparseRequest converts a ParsedRequest back into raw HTTP request bytes.
// It uses the LineBreak field to determine line endings.
func UnparseRequest(pr ParsedRequest) []byte {
	var buf bytes.Buffer
	lineBreak := pr.LineBreak

	// Write request line: METHOD SP URL SP HTTPVERSION
	reqLine := ""
	if pr.Method != "" {
		reqLine = pr.Method + " " + pr.URL
	} else {
		reqLine = pr.URL
	}
	if pr.HTTPVersion != "" {
		reqLine += " " + pr.HTTPVersion
	}
	buf.WriteString(reqLine)
	buf.WriteString(lineBreak)

	// Write headers
	for i, header := range pr.Headers {
		if len(header) >= 2 {
			buf.WriteString(header[0] + header[1])
			isLastHeader := i == len(pr.Headers)-1
			if isLastHeader {
				// For the last header:
				// - If BodySeparator is a full separator ("\r\n\r\n" or "\n\n"), don't write line break (separator includes it)
				// - If BodySeparator is just a trailing newline ("\r\n" or "\n"), don't write line break (separator includes it)
				// - If BodySeparator is empty and Body is empty, don't write line break (original had no trailing newline)
				// - If BodySeparator is empty but Body exists, write line break (need separator for body)
				if pr.BodySeparator == "" && pr.Body == "" {
					// No separator and no body - original had no trailing newline
					// Don't write line break
				} else if pr.BodySeparator != "" {
					// BodySeparator exists (full separator or just trailing newline) - it includes the line break
					// Don't write line break here
				} else {
					// BodySeparator is empty but Body exists - need to write line break for body separator
					buf.WriteString(lineBreak)
				}
			} else {
				// Not the last header - always write line break
				buf.WriteString(lineBreak)
			}
		}
	}

	// Empty line separating headers and body
	if pr.BodySeparator != "" {
		// BodySeparator can be:
		// - "\r\n\r\n" or "\n\n" (full body separator)
		// - "\r\n" or "\n" (just trailing newline after last header)
		// Write it as-is
		buf.WriteString(pr.BodySeparator)
	} else if pr.Body != "" {
		// No separator in original, but we have a body, so add a line break
		buf.WriteString(lineBreak)
	}

	// Write body
	if pr.Body != "" {
		buf.WriteString(pr.Body)
	}

	return buf.Bytes()
}

// UnparseResponse converts a ParsedResponse back into raw HTTP response bytes.
// It uses the LineBreak field to determine line endings.
func UnparseResponse(pr ParsedResponse) []byte {
	var buf bytes.Buffer
	lineBreak := pr.LineBreak

	// Write status line: VERSION SP STATUS SP REASON
	statusLine := ""
	if pr.StatusFull != "" {
		// Use StatusFull if provided
		statusLine = pr.StatusFull
	} else {
		// Construct from Version and Status
		if pr.Version != "" {
			statusLine = pr.Version
			if pr.Status > 0 {
				statusLine += " " + strconv.Itoa(pr.Status)
			}
		} else if pr.Status > 0 {
			// If no version but has status, just write status
			statusLine = strconv.Itoa(pr.Status)
		}
		// If both are empty, statusLine remains empty (preserves empty status line)
	}
	buf.WriteString(statusLine)
	buf.WriteString(lineBreak)

	// Write headers
	for i, header := range pr.Headers {
		if len(header) >= 2 {
			buf.WriteString(header[0] + header[1])
			isLastHeader := i == len(pr.Headers)-1
			if isLastHeader {
				// For the last header:
				// - If BodySeparator is a full separator ("\r\n\r\n" or "\n\n"), don't write line break (separator includes it)
				// - If BodySeparator is just a trailing newline ("\r\n" or "\n"), don't write line break (separator includes it)
				// - If BodySeparator is empty and Body is empty, don't write line break (original had no trailing newline)
				// - If BodySeparator is empty but Body exists, write line break (need separator for body)
				if pr.BodySeparator == "" && pr.Body == "" {
					// No separator and no body - original had no trailing newline
					// Don't write line break
				} else if pr.BodySeparator != "" {
					// BodySeparator exists (full separator or just trailing newline) - it includes the line break
					// Don't write line break here
				} else {
					// BodySeparator is empty but Body exists - need to write line break for body separator
					buf.WriteString(lineBreak)
				}
			} else {
				// Not the last header - always write line break
				buf.WriteString(lineBreak)
			}
		}
	}

	// Empty line separating headers and body
	if pr.BodySeparator != "" {
		// BodySeparator can be:
		// - "\r\n\r\n" or "\n\n" (full body separator)
		// - "\r\n" or "\n" (just trailing newline after last header)
		// Write it as-is
		buf.WriteString(pr.BodySeparator)
	} else if pr.Body != "" {
		// No separator in original, but we have a body, so add a line break
		buf.WriteString(lineBreak)
	}

	// Write body
	if pr.Body != "" {
		buf.WriteString(pr.Body)
	}

	return buf.Bytes()
}
