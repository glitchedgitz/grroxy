package rawhttp

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// ReadFromFile reads a raw HTTP request from a file.
// It preserves all formatting, including malformed headers, whitespace, and line endings.
// Only minimal parsing is performed to extract the target host if needed.
func ReadFromFile(filepath string) ([]byte, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return data, nil
}

// ReadFromReader reads a raw HTTP request from an io.Reader.
// It preserves all formatting, including malformed headers, whitespace, and line endings.
func ReadFromReader(reader io.Reader) ([]byte, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read from reader: %w", err)
	}
	return data, nil
}

// ReadFromString reads a raw HTTP request from a string.
// It preserves all formatting, including malformed headers, whitespace, and line endings.
func ReadFromString(rawRequest string) []byte {
	return []byte(rawRequest)
}

// ExtractHost attempts to extract the host from a raw HTTP request.
// This is a minimal parser that only tries to find the Host header.
// It returns an empty string if no host is found.
// This is optional and only used for convenience - the raw bytes are preserved as-is.
func ExtractHost(rawRequest []byte) string {
	scanner := bufio.NewScanner(strings.NewReader(string(rawRequest)))

	for scanner.Scan() {
		line := scanner.Text()

		// Check for Host header (case-insensitive, but preserve any malformations)
		if len(line) > 5 {
			// Try to find "Host:" or "host:" or any case variation
			lineLower := strings.ToLower(line)
			if strings.HasPrefix(lineLower, "host:") {
				// Extract host value (everything after "host:")
				host := strings.TrimSpace(line[5:])
				// Remove any port if present (for basic parsing)
				if idx := strings.Index(host, ":"); idx > 0 {
					host = host[:idx]
				}
				return host
			}
		}

		// Stop if we hit an empty line (end of headers)
		if strings.TrimSpace(line) == "" {
			break
		}
	}

	return ""
}