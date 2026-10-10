package rawproxy

import (
	"io"
	"net"
	"net/http"
	"strings"
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

// hopByHopRequestHeaders are the connection-scoped headers we drop on the way
// upstream. Only Connection, Transfer-Encoding and Upgrade are load-bearing:
// x/net/http2 rejects a request carrying them outright
// (internal/httpcommon.checkConnHeaders). Proxy-Connection and Keep-Alive are
// dropped by http2 during encoding anyway; we drop them here so HTTP/1.1
// upstreams see the same request.
//
// We are an intercepting proxy, so the default is to forward what the client
// sent and every removal needs a protocol reason. Deliberately NOT stripped:
//
//	Proxy-Authorization  in MITM mode we are inside a CONNECT tunnel, so this
//	                     was never addressed to us - it would have been consumed
//	                     on the CONNECT line. Deleting it would silently discard
//	                     data the user set on purpose.
//	TE, Trailer          no protocol reason; Go forwards them fine, and
//	                     "TE: trailers" is legal and meaningful over h2.
var hopByHopRequestHeaders = []string{
	"Connection",
	"Proxy-Connection",
	"Keep-Alive",
	"Transfer-Encoding",
	"Upgrade",
}

// hopByHopResponseHeaders are dropped on the way back to the client.
// Transfer-Encoding and Trailer must go because the MITM handler buffers the
// whole body and re-states Content-Length, so the framing the upstream used no
// longer describes what we write.
var hopByHopResponseHeaders = []string{
	"Connection",
	"Proxy-Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}

// stripHopByHopRequest removes connection-scoped headers from a request bound
// upstream. Callers must dump/capture before calling this, so recorded traffic
// still shows exactly what the client sent.
func stripHopByHopRequest(h http.Header) { stripHopByHop(h, hopByHopRequestHeaders) }

// stripHopByHopResponse removes connection-scoped headers from a response bound
// for the client. As above: capture first, strip second.
func stripHopByHopResponse(h http.Header) { stripHopByHop(h, hopByHopResponseHeaders) }

// stripHopByHop deletes names, plus any header named by the Connection header
// itself (RFC 7230 6.1). Order matters: the Connection-named headers are read
// and deleted first, while Connection is still present to name them.
func stripHopByHop(h http.Header, names []string) {
	for _, value := range h.Values("Connection") {
		for _, name := range strings.Split(value, ",") {
			if name = strings.TrimSpace(name); name != "" {
				h.Del(name)
			}
		}
	}
	for _, name := range names {
		h.Del(name)
	}
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
