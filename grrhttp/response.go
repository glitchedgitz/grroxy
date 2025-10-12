package grrhttp

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/andybalholm/brotli"
	"github.com/glitchedgitz/grroxy-db/utils"
)

func DecompressResponse(reader io.Reader, contentEncoding string) (io.Reader, error) {
	switch strings.ToLower(contentEncoding) {
	case "gzip", "x-gzip":
		gzReader, err := gzip.NewReader(reader)
		if err != nil {
			// If decompression fails, return the original reader
			return reader, nil
		}
		return gzReader, nil
	case "br", "brotli":
		return brotli.NewReader(reader), nil
	case "deflate":
		// Note: deflate is not implemented here, would need zlib
		// For now, return original reader
		return reader, nil
	default:
		return reader, nil
	}
}

// DumpResponse dumps an HTTP response to a string format.
// It uses "magic" detection to automatically decompress content regardless of headers,
// trying multiple compression formats (gzip, brotli, zlib) to see if any work.
func DumpResponse(resp *http.Response) string {

	var err error
	var bodyReader io.Reader

	// Read the body first so we can try multiple decompression attempts
	originalBody, err := io.ReadAll(resp.Body)
	utils.CheckErr("[DumpResponse] Read body error: ", err)

	// Magic detection: try to decompress regardless of headers
	bodyReader, err = MagicDecompress(bytes.NewReader(originalBody))
	utils.CheckErr("[DumpResponse] Magic decompression error: ", err)

	// Read the decompressed body
	bf := bufio.NewReader(bodyReader)
	respbody, err := io.ReadAll(bf)
	utils.CheckErr("[DumpResponse] Read decompressed body error: ", err)

	// Update content length to reflect decompressed size
	cl := int64(len(respbody))

	finalResp := fmt.Sprintf("%s %s\n", resp.Proto, resp.Status)

	// Build headers, but remove Content-Encoding since we've decompressed
	hasContentLength := false
	for header, value := range resp.Header {
		// Skip Content-Encoding header since we've already decompressed
		if strings.EqualFold(header, "Content-Encoding") {
			continue
		}

		// Update Content-Length to reflect decompressed size
		if strings.EqualFold(header, "Content-Length") {
			finalResp += fmt.Sprintf("%s: %d\n", header, cl)
			hasContentLength = true
		} else {
			for _, val := range value {
				finalResp += fmt.Sprintf("%s: %s\n", header, val)
			}
		}
	}

	// Add Content-Length if it didn't exist in the original response
	if !hasContentLength {
		finalResp += fmt.Sprintf("Content-Length: %d\n", cl)
	}

	finalResp += "\n" + string(respbody)

	return finalResp
}

// MagicDecompress attempts to decompress content using multiple compression formats
// regardless of headers. It tries each format and returns the first successful one.
func MagicDecompress(reader io.Reader) (io.Reader, error) {
	// Read all data first so we can try multiple decompression attempts
	data, err := io.ReadAll(reader)
	if err != nil {
		return reader, err
	}

	// Try gzip first (most common)
	if gzReader, err := gzip.NewReader(bytes.NewReader(data)); err == nil {
		// Test if it's actually valid gzip by trying to read a small amount
		testBuf := make([]byte, 1)
		if _, readErr := gzReader.Read(testBuf); readErr == nil {
			// Success! Return a new reader with the data
			gzReader2, _ := gzip.NewReader(bytes.NewReader(data))
			return gzReader2, nil
		}
	}

	// Try brotli
	if brReader := brotli.NewReader(bytes.NewReader(data)); brReader != nil {
		// Test if it's actually valid brotli by trying to read a small amount
		testBuf := make([]byte, 1)
		if _, readErr := brReader.Read(testBuf); readErr == nil {
			// Success! Return a new reader with the data
			return brotli.NewReader(bytes.NewReader(data)), nil
		}
	}

	// Try zlib/deflate
	if zlibReader, err := zlib.NewReader(bytes.NewReader(data)); err == nil {
		// Test if it's actually valid zlib by trying to read a small amount
		testBuf := make([]byte, 1)
		if _, readErr := zlibReader.Read(testBuf); readErr == nil {
			// Success! Return a new reader with the data
			zlibReader2, _ := zlib.NewReader(bytes.NewReader(data))
			return zlibReader2, nil
		}
	}

	// If no compression worked, return the original data
	return bytes.NewReader(data), nil
}
