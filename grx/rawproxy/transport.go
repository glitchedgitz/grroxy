package rawproxy

import (
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"time"
)

// Shared transport for connection pooling
var sharedTransport = &http.Transport{
	Proxy:                 nil,
	ForceAttemptHTTP2:     true, // Allow automatic HTTP/2 negotiation
	MaxIdleConns:          100,
	MaxIdleConnsPerHost:   10,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   15 * time.Second, // Increase timeout for better compatibility
	ExpectContinueTimeout: 1 * time.Second,
	ResponseHeaderTimeout: 30 * time.Second, // Add response timeout
	DialContext: (&net.Dialer{
		Timeout:   10 * time.Second, // Increase dial timeout
		KeepAlive: 30 * time.Second,
	}).DialContext,
	TLSClientConfig: &tls.Config{
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12, // Ensure modern TLS
		MaxVersion:         tls.VersionTLS13,
		ServerName:         "", // Will be set per request
	},
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
