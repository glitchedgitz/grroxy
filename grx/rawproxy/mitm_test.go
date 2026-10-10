package rawproxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// captureRT records the request the MITM handler builds for the upstream and
// returns a canned response.
type captureRT struct {
	got  *http.Request
	resp *http.Response
}

func (c *captureRT) RoundTrip(req *http.Request) (*http.Response, error) {
	c.got = req
	if c.resp != nil {
		return c.resp, nil
	}
	return &http.Response{
		StatusCode: 200,
		Proto:      "HTTP/2.0",
		ProtoMajor: 2,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader("ok")),
	}, nil
}

func testHandler(rt http.RoundTripper) *mitmHandler {
	return &mitmHandler{
		host:         "example.com",
		connectHost:  "example.com:443",
		roundTripper: rt,
		baseReqID:    "test",
		config: &Config{
			OutputDir:  "",
			ReqCounter: &atomic.Uint64{},
		},
	}
}

// TestUpstreamRequestIsRetryable is the regression guard for the failure where a
// connection worked and then abruptly stopped.
//
// Incoming server requests always have GetBody == nil, and Clone copies that nil
// through. If the handler leaves GetBody nil while setting a non-nil Body, then
// http2's shouldRetryRequest cannot rewind the request and converts a routine
// graceful-shutdown GOAWAY - an error it otherwise classifies as retryable -
// into a hard failure. Upstreams recycle h2 connections on a schedule, so this
// surfaces as "works fine, then suddenly stops".
func TestUpstreamRequestIsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		body     string
		wantLen  int64
		wantBody string
	}{
		{name: "GET with no body", method: "GET", body: "", wantLen: 0},
		{name: "POST with body", method: "POST", body: "foo=bar", wantLen: 7, wantBody: "foo=bar"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := &captureRT{}
			h := testHandler(rt)

			req := httptest.NewRequest(tt.method, "https://example.com/x", strings.NewReader(tt.body))
			h.ServeHTTP(httptest.NewRecorder(), req)

			if rt.got == nil {
				t.Fatal("handler never called the upstream round tripper")
			}
			up := rt.got

			// The invariant: http2 must be able to replay this request.
			retryable := up.Body == nil || up.Body == http.NoBody || up.GetBody != nil
			if !retryable {
				t.Fatal("upstream request is not retryable: Body is non-nil and GetBody is nil; " +
					"a GOAWAY from the upstream will hard-fail instead of being retried")
			}

			if up.ContentLength != tt.wantLen {
				t.Errorf("ContentLength = %d, want %d", up.ContentLength, tt.wantLen)
			}

			// A bodyless request should present as bodyless so http2 takes the
			// direct-reuse path rather than paying for an empty DATA frame.
			if tt.body == "" && up.Body != http.NoBody {
				t.Errorf("bodyless request should use http.NoBody, got %T", up.Body)
			}

			// GetBody must actually reproduce the body, not just be non-nil.
			if up.GetBody != nil {
				rc, err := up.GetBody()
				if err != nil {
					t.Fatalf("GetBody() failed: %v", err)
				}
				got, _ := io.ReadAll(rc)
				if string(got) != tt.wantBody {
					t.Errorf("GetBody() returned %q, want %q", got, tt.wantBody)
				}
			}
		})
	}
}

// TestUpstreamRequestStripsConnectionHeaders checks that headers x/net/http2
// rejects outright are gone, while user-controlled headers survive.
func TestUpstreamRequestStripsConnectionHeaders(t *testing.T) {
	rt := &captureRT{}
	h := testHandler(rt)

	req := httptest.NewRequest("GET", "https://example.com/x", nil)
	req.Header.Set("Connection", "keep-alive, X-Named-By-Connection")
	req.Header.Set("X-Named-By-Connection", "should be removed")
	req.Header.Set("Keep-Alive", "timeout=5")
	req.Header.Set("Upgrade", "h2c")
	req.Header.Set("Proxy-Connection", "keep-alive")
	// These are user-controlled data an interception proxy must forward as sent.
	req.Header.Set("Proxy-Authorization", "Basic dXNlcjpwYXNz")
	req.Header.Set("TE", "trailers")
	req.Header.Set("X-Custom", "keep me")

	h.ServeHTTP(httptest.NewRecorder(), req)

	if rt.got == nil {
		t.Fatal("handler never called the upstream round tripper")
	}

	for _, name := range []string{"Connection", "Keep-Alive", "Upgrade", "Proxy-Connection", "X-Named-By-Connection"} {
		if v := rt.got.Header.Get(name); v != "" {
			t.Errorf("%s should have been stripped, got %q", name, v)
		}
	}
	for name, want := range map[string]string{
		"Proxy-Authorization": "Basic dXNlcjpwYXNz",
		"Te":                  "trailers",
		"X-Custom":            "keep me",
	} {
		if v := rt.got.Header.Get(name); v != want {
			t.Errorf("%s = %q, want %q (must be forwarded as sent)", name, v, want)
		}
	}
}

// TestProtoCacheExpires guards the self-healing behaviour of hostProtoCache. It
// used to have no TTL, so one wrong downgrade pinned a host to HTTP/1.1 for the
// rest of the process lifetime.
func TestProtoCacheExpires(t *testing.T) {
	const addr = "cache-test.example:443"
	hostProtoCache.Delete(addr)
	t.Cleanup(func() { hostProtoCache.Delete(addr) })

	if got := loadCachedProto(addr); got != "" {
		t.Fatalf("expected empty cache, got %q", got)
	}

	storeCachedProto(addr, "http/1.1")
	if got := loadCachedProto(addr); got != "http/1.1" {
		t.Errorf("loadCachedProto = %q, want http/1.1", got)
	}

	// Force expiry rather than sleeping for the real TTL.
	hostProtoCache.Store(addr, protoCacheEntry{proto: "http/1.1"}) // zero expiresAt = already past
	if got := loadCachedProto(addr); got != "" {
		t.Errorf("expired entry returned %q, want it evicted so the host is re-probed", got)
	}
	if _, ok := hostProtoCache.Load(addr); ok {
		t.Error("expired entry should be deleted from the cache on read")
	}
}
