package rawproxy

import (
	"io"
	"net"
	"net/http"
	"time"
)

// Shared transport for connection pooling (plain HTTP only)
// For HTTPS, we use uTLS transports created per-host to mimic browser fingerprints
var sharedHTTPTransport = &http.Transport{
	Proxy:                 nil,
	ForceAttemptHTTP2:     false, // HTTP/1.1 only for plain HTTP
	MaxIdleConns:          100,
	MaxIdleConnsPerHost:   10,
	IdleConnTimeout:       90 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
	ResponseHeaderTimeout: 30 * time.Second,
	DialContext: (&net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
}

// GetTransportForHost returns an appropriate transport for the given scheme and host
// For HTTPS, it creates a uTLS round tripper to mimic browser TLS fingerprint
// For HTTP, it uses the shared plain HTTP transport
func GetTransportForHost(scheme, host string) http.RoundTripper {
	if scheme == "https" {
		// Use uTLS round tripper with Chrome fingerprint for HTTPS
		return GetUTLSRoundTripper(host, FingerprintChrome)
	}
	return sharedHTTPTransport
}

// copyHeader copies HTTP headers from src to dst
func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

// cloneResponseMeta creates a clone of a response with a new body
func cloneResponseMeta(src *http.Response, body io.ReadCloser) *http.Response {
	c := new(http.Response)
	*c = *src
	c.Body = body
	return c
}
