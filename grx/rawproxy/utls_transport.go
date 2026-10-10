package rawproxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/http2"
)

// BrowserFingerprint represents different browser TLS fingerprints to mimic
type BrowserFingerprint int

const (
	FingerprintChrome BrowserFingerprint = iota
	FingerprintFirefox
	FingerprintSafari
	FingerprintEdge
	FingerprintRandom // Randomly pick one
)

// hostProtoCache is a global cache of host → negotiated protocol.
// Once we learn a host speaks "http/1.1" (or "h2"), all future
// UTLSRoundTripper instances for that host reuse the result.
var hostProtoCache sync.Map // map[string]protoCacheEntry  (host:port → entry)

// protoCacheTTL bounds how long a learned protocol is trusted. Entries used to
// live for the whole process lifetime, which made a single wrong downgrade
// permanent: a host pinned to "http/1.1" was never re-probed, so every later
// request took the HTTP/1.1 transport no matter what the server had started
// doing. A TTL makes the cache self-healing - the worst case is one re-probe
// per host per TTL.
const protoCacheTTL = 5 * time.Minute

type protoCacheEntry struct {
	proto     string
	expiresAt time.Time
}

// loadCachedProto returns the cached protocol for addr, or "" if absent or
// expired. Expired entries are evicted on read.
func loadCachedProto(addr string) string {
	v, ok := hostProtoCache.Load(addr)
	if !ok {
		return ""
	}
	entry, ok := v.(protoCacheEntry)
	if !ok || time.Now().After(entry.expiresAt) {
		hostProtoCache.Delete(addr)
		return ""
	}
	return entry.proto
}

// storeCachedProto records a protocol we actually observed working for addr.
func storeCachedProto(addr, proto string) {
	hostProtoCache.Store(addr, protoCacheEntry{
		proto:     proto,
		expiresAt: time.Now().Add(protoCacheTTL),
	})
}

// UTLSRoundTripper is an http.RoundTripper that uses uTLS for TLS connections
// and properly handles HTTP/2 based on ALPN negotiation
type UTLSRoundTripper struct {
	fingerprint    BrowserFingerprint
	serverName     string
	dialer         *net.Dialer
	http2Transport *http2.Transport
	http1Transport *http.Transport
}

// NewUTLSRoundTripper creates a new round tripper with browser fingerprint spoofing
func NewUTLSRoundTripper(serverName string, fingerprint BrowserFingerprint) *UTLSRoundTripper {
	rt := &UTLSRoundTripper{
		fingerprint: fingerprint,
		serverName:  serverName,
		dialer: &net.Dialer{
			Timeout:   15 * time.Second,
			KeepAlive: 30 * time.Second,
		},
	}

	// HTTP/1.1 transport with custom TLS dialer
	rt.http1Transport = &http.Transport{
		DialTLSContext:        rt.dialTLSForHTTP1,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ForceAttemptHTTP2:     false, // We handle HTTP/2 separately
	}

	// HTTP/2 transport with custom TLS dialer
	rt.http2Transport = &http2.Transport{
		DialTLSContext:  rt.dialTLSForHTTP2,
		ReadIdleTimeout: 30 * time.Second,
		PingTimeout:     15 * time.Second,
	}

	return rt
}

// getClientHelloID returns the uTLS ClientHelloID for the specified fingerprint
func (rt *UTLSRoundTripper) getClientHelloID() utls.ClientHelloID {
	switch rt.fingerprint {
	case FingerprintChrome:
		return utls.HelloChrome_Auto
	case FingerprintFirefox:
		return utls.HelloFirefox_Auto
	case FingerprintSafari:
		return utls.HelloSafari_Auto
	case FingerprintEdge:
		return utls.HelloEdge_Auto
	case FingerprintRandom:
		return utls.HelloRandomized
	default:
		return utls.HelloChrome_Auto
	}
}

// dialUTLS creates a uTLS connection with the specified ALPN protocols
func (rt *UTLSRoundTripper) dialUTLS(ctx context.Context, network, addr string, alpnProtos []string) (*utls.UConn, error) {
	// Extract hostname for SNI
	serverName := rt.serverName
	if serverName == "" {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			serverName = addr
		} else {
			serverName = host
		}
	}

	// Dial TCP connection
	tcpConn, err := rt.dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %w", addr, err)
	}

	// Create uTLS config with specified ALPN
	config := &utls.Config{
		ServerName:         serverName,
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
		MaxVersion:         tls.VersionTLS13,
		NextProtos:         alpnProtos,
	}

	// Build the browser ClientHello spec ourselves so we can override its ALPN.
	// A preset ClientHelloID ignores config.NextProtos entirely: the parrot spec
	// hardcodes ALPN to ["h2","http/1.1"] and ALPNExtension.writeToUConn copies
	// that *into* config.NextProtos. Asking for http/1.1-only would silently
	// still advertise h2, the server would pick it, and the HTTP/1.1 transport
	// would then write plaintext requests into an HTTP/2 connection.
	// HelloCustom + ApplyPreset keeps the cipher/extension ordering intact, so
	// the JA3/JA4 fingerprint is unchanged - only the ALPN list narrows.
	spec, err := utls.UTLSIdToSpec(rt.getClientHelloID())
	if err != nil {
		tcpConn.Close()
		return nil, fmt.Errorf("failed to build ClientHello spec for %s: %w", serverName, err)
	}
	for _, ext := range spec.Extensions {
		if alpn, ok := ext.(*utls.ALPNExtension); ok {
			alpn.AlpnProtocols = alpnProtos
		}
	}

	utlsConn := utls.UClient(tcpConn, config, utls.HelloCustom)
	if err := utlsConn.ApplyPreset(&spec); err != nil {
		tcpConn.Close()
		return nil, fmt.Errorf("failed to apply ClientHello spec for %s: %w", serverName, err)
	}

	// Perform TLS handshake
	if err := utlsConn.HandshakeContext(ctx); err != nil {
		tcpConn.Close()
		return nil, fmt.Errorf("uTLS handshake failed for %s: %w", serverName, err)
	}

	// Never hand a mis-negotiated connection back to the caller. The HTTP/1.1 and
	// HTTP/2 transports each assume the protocol they asked for, and neither can
	// detect otherwise: net/http only reads ALPN off a *tls.Conn, and a
	// *utls.UConn is not one, so it would write HTTP/1.1 into an h2 connection
	// and read back a GOAWAY frame as a "malformed HTTP response".
	// An empty NegotiatedProtocol means the server declined ALPN, which is only
	// valid when we were willing to speak HTTP/1.1 anyway.
	negotiated := utlsConn.ConnectionState().NegotiatedProtocol
	if negotiated == "" {
		negotiated = "http/1.1"
	}
	if !slices.Contains(alpnProtos, negotiated) {
		utlsConn.Close()
		return nil, fmt.Errorf("alpn mismatch for %s: requested %v, negotiated %q", serverName, alpnProtos, negotiated)
	}

	return utlsConn, nil
}

// dialTLSForHTTP1 creates a TLS connection for HTTP/1.1 (only requests http/1.1 via ALPN)
func (rt *UTLSRoundTripper) dialTLSForHTTP1(ctx context.Context, network, addr string) (net.Conn, error) {
	return rt.dialUTLS(ctx, network, addr, []string{"http/1.1"})
}

// dialTLSForHTTP2 creates a TLS connection for HTTP/2 (only requests h2 via ALPN)
func (rt *UTLSRoundTripper) dialTLSForHTTP2(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
	return rt.dialUTLS(ctx, network, addr, []string{"h2"})
}

// probeALPN performs a probe connection to determine which protocol the server supports.
// Always returns "h2" or "http/1.1" - a server that declines ALPN speaks HTTP/1.1.
func (rt *UTLSRoundTripper) probeALPN(ctx context.Context, addr string) (string, error) {
	// Try with both protocols to see what server prefers
	conn, err := rt.dialUTLS(ctx, "tcp", addr, []string{"h2", "http/1.1"})
	if err != nil {
		return "", err
	}
	proto := conn.ConnectionState().NegotiatedProtocol
	conn.Close()
	if proto != "h2" {
		proto = "http/1.1"
	}
	return proto, nil
}

// rewindBody restores req.Body so the request can be retried on another
// transport. Requests without a body need nothing; anything else requires
// GetBody, which callers set when they hold the full body in memory.
func rewindBody(req *http.Request) error {
	if req.Body == nil || req.Body == http.NoBody {
		return nil
	}
	if req.GetBody == nil {
		return fmt.Errorf("cannot retry request with unrewindable body")
	}
	body, err := req.GetBody()
	if err != nil {
		return fmt.Errorf("cannot rewind request body: %w", err)
	}
	req.Body = body
	return nil
}

// isNotH2Capable reports whether an http2.Transport error means the server does
// not actually speak HTTP/2, as opposed to an ordinary per-stream or
// connection-recycle failure. Only the former justifies remembering a host as
// HTTP/1.1-only; matching broadly here lets a single bad request disable HTTP/2
// for a host process-wide.
func isNotH2Capable(err error) bool {
	errStr := err.Error()
	return strings.Contains(errStr, "frame header looked like an HTTP/1.1 header") ||
		strings.Contains(errStr, "unexpected HTTP/1")
}

// RoundTrip implements http.RoundTripper
func (rt *UTLSRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Determine target address
	addr := req.URL.Host
	if !hasPort(addr) {
		if req.URL.Scheme == "https" {
			addr += ":443"
		} else {
			addr += ":80"
		}
	}

	// Check global protocol cache for this host
	cachedProto := loadCachedProto(addr)

	// If no cached protocol, probe the server
	if cachedProto == "" {
		ctx, cancel := context.WithTimeout(req.Context(), 10*time.Second)
		proto, err := rt.probeALPN(ctx, addr)
		cancel()
		if err != nil {
			// Fall back to HTTP/1.1 for this request only. Caching a protocol
			// we never actually observed would let one transient failure pin
			// the host to HTTP/1.1 for the rest of the process lifetime.
			log.Printf("[TRANSPORT] Probe of %s failed, using HTTP/1.1 for this request: %v", addr, err)
			cachedProto = "http/1.1"
		} else {
			storeCachedProto(addr, proto)
			cachedProto = proto
			log.Printf("[TRANSPORT] Probed %s → protocol: %s", addr, proto)
		}
	}

	// Use appropriate transport based on negotiated protocol
	if cachedProto == "h2" {
		resp, err := rt.http2Transport.RoundTrip(req)
		if err == nil {
			return resp, nil
		}
		if !isNotH2Capable(err) {
			return nil, err
		}
		// The server advertised h2 via ALPN but answered with HTTP/1.1 framing.
		// Retry over HTTP/1.1, and only remember the downgrade once that retry
		// actually succeeds - otherwise a request that fails for its own reasons
		// would poison the host for every later request.
		// The h2 attempt may have consumed the body, so only retry when we can
		// rewind it.
		if err := rewindBody(req); err != nil {
			return nil, err
		}
		log.Printf("[TRANSPORT] Host %s advertised h2 but answered with HTTP/1.1 framing, retrying over HTTP/1.1 (error: %v)", addr, err)
		resp, err1 := rt.http1Transport.RoundTrip(req)
		if err1 != nil {
			// Surface both: the h2 error says why we downgraded, err1 says why
			// the fallback did not save us. Reporting only the first hides the
			// actual failure from whoever reads the log.
			return nil, fmt.Errorf("h2 failed (%v); HTTP/1.1 retry also failed: %w", err, err1)
		}
		storeCachedProto(addr, "http/1.1")
		log.Printf("[TRANSPORT] Host %s downgraded to HTTP/1.1", addr)
		return resp, nil
	}
	return rt.http1Transport.RoundTrip(req)
}

// hasPort checks if the address already has a port
func hasPort(addr string) bool {
	_, _, err := net.SplitHostPort(addr)
	return err == nil
}

// GetCachedProto returns the cached protocol for a host (e.g. "h2" or "http/1.1").
// Returns empty string if the host hasn't been probed yet.
// This allows other packages to check the actual upstream protocol.
func GetCachedProto(host string) string {
	// Try with :443 suffix (most common for HTTPS)
	if proto := loadCachedProto(host + ":443"); proto != "" {
		return proto
	}
	// Try exact match
	return loadCachedProto(host)
}

// GetUTLSRoundTripper returns a UTLSRoundTripper for the given host
// This is the preferred way to make HTTP requests with browser TLS fingerprint
func GetUTLSRoundTripper(host string, fingerprint BrowserFingerprint) http.RoundTripper {
	return NewUTLSRoundTripper(host, fingerprint)
}

// DialUTLS creates a direct uTLS connection for WebSocket or other raw connections
func DialUTLS(ctx context.Context, addr, serverName string, fingerprint BrowserFingerprint) (net.Conn, error) {
	rt := &UTLSRoundTripper{
		fingerprint: fingerprint,
		serverName:  serverName,
		dialer: &net.Dialer{
			Timeout:   15 * time.Second,
			KeepAlive: 30 * time.Second,
		},
	}
	// For raw connections (WebSocket), use HTTP/1.1 ALPN
	return rt.dialUTLS(ctx, "tcp", addr, []string{"http/1.1"})
}
