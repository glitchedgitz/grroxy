package app

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func newTestRequest(method, rawURL string, headers map[string]string, body string) *http.Request {
	u, _ := url.Parse(rawURL)
	req := &http.Request{
		Method: method,
		URL:    u,
		Header: make(http.Header),
		Body:   io.NopCloser(strings.NewReader(body)),
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req
}

func TestApplySetToRequest_Method(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		wantMethod string
	}{
		{"set POST", "req.method", "POST", "POST"},
		{"set DELETE", "req.method", "DELETE", "DELETE"},
		{"set empty", "req.method", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newTestRequest("GET", "http://example.com/path", nil, "")
			applySetToRequest(req, tt.key, tt.value)
			if got := req.Method; got != tt.wantMethod {
				t.Errorf("got method %q, want %q", got, tt.wantMethod)
			}
		})
	}
}

func TestApplySetToRequest_Path(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		wantPath string
	}{
		{"set path", "req.path", "/new/path", "/new/path"},
		{"set empty path", "req.path", "", ""},
		{"set root", "req.path", "/", "/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newTestRequest("GET", "http://example.com/old", nil, "")
			applySetToRequest(req, tt.key, tt.value)
			if got := req.URL.Path; got != tt.wantPath {
				t.Errorf("got path %q, want %q", got, tt.wantPath)
			}
		})
	}
}

func TestApplySetToRequest_URL(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		wantHost string
		wantPath string
	}{
		{"full url", "http://new.com/api", "new.com", "/api"},
		{"path only", "/just/path", "", "/just/path"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newTestRequest("GET", "http://example.com/old", nil, "")
			applySetToRequest(req, "req.url", tt.value)
			if got := req.URL.Host; got != tt.wantHost {
				t.Errorf("got host %q, want %q", got, tt.wantHost)
			}
			if got := req.URL.Path; got != tt.wantPath {
				t.Errorf("got path %q, want %q", got, tt.wantPath)
			}
		})
	}
}

func TestApplySetToRequest_Headers(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		value      string
		wantHeader string
		wantValue  string
	}{
		{"set user-agent", "req.headers.User-Agent", "CustomBot", "User-Agent", "CustomBot"},
		{"set custom header", "req.headers.X-Custom", "myval", "X-Custom", "myval"},
		{"overwrite existing", "req.headers.Content-Type", "text/plain", "Content-Type", "text/plain"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newTestRequest("GET", "http://example.com", map[string]string{"Content-Type": "application/json"}, "")
			applySetToRequest(req, tt.key, tt.value)
			if got := req.Header.Get(tt.wantHeader); got != tt.wantValue {
				t.Errorf("got header %q=%q, want %q", tt.wantHeader, got, tt.wantValue)
			}
		})
	}
}

func TestApplySetToRequest_Body(t *testing.T) {
	tests := []struct {
		name          string
		value         string
		wantLen       int64
	}{
		{"set body", `{"key":"val"}`, 13},
		{"empty body", "", 0},
		{"large body", strings.Repeat("x", 10000), 10000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newTestRequest("POST", "http://example.com", nil, "old body")
			applySetToRequest(req, "req.body", tt.value)
			body, _ := io.ReadAll(req.Body)
			if string(body) != tt.value {
				t.Errorf("got body %q, want %q", string(body), tt.value)
			}
			if req.ContentLength != tt.wantLen {
				t.Errorf("got content-length %d, want %d", req.ContentLength, tt.wantLen)
			}
		})
	}
}

func TestApplySetToRequest_Query(t *testing.T) {
	tests := []struct {
		name      string
		startURL  string
		key       string
		value     string
		wantQuery string
	}{
		{"add query param", "http://example.com/path", "req.query.foo", "bar", "foo=bar"},
		{"add to existing", "http://example.com/path?a=1", "req.query.b", "2", "a=1&b=2"},
		{"overwrite param", "http://example.com/path?foo=old", "req.query.foo", "new", "foo=new"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newTestRequest("GET", tt.startURL, nil, "")
			applySetToRequest(req, tt.key, tt.value)
			if got := req.URL.RawQuery; got != tt.wantQuery {
				t.Errorf("got query %q, want %q", got, tt.wantQuery)
			}
		})
	}
}

func TestApplySetToRequest_UnknownKey(t *testing.T) {
	req := newTestRequest("GET", "http://example.com", nil, "")
	applySetToRequest(req, "req.unknown", "value")
	// Should not panic, no change
	if req.Method != "GET" {
		t.Errorf("method changed unexpectedly")
	}
}

// --- applyDeleteToRequest tests ---

func TestApplyDeleteToRequest_Header(t *testing.T) {
	tests := []struct {
		name        string
		headers     map[string]string
		key         string
		wantAbsent  string
		wantPresent string
	}{
		{
			"delete single header",
			map[string]string{"X-Remove": "yes", "Keep": "yes"},
			"req.headers.X-Remove",
			"X-Remove",
			"Keep",
		},
		{
			"delete nonexistent header",
			map[string]string{"Keep": "yes"},
			"req.headers.X-Gone",
			"X-Gone",
			"Keep",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newTestRequest("GET", "http://example.com", tt.headers, "")
			applyDeleteToRequest(req, tt.key)
			if got := req.Header.Get(tt.wantAbsent); got != "" {
				t.Errorf("header %q should be deleted, got %q", tt.wantAbsent, got)
			}
			if got := req.Header.Get(tt.wantPresent); got == "" {
				t.Errorf("header %q should still exist", tt.wantPresent)
			}
		})
	}
}

func TestApplyDeleteToRequest_HeaderWildcard(t *testing.T) {
	headers := map[string]string{
		"Sec-Fetch-Dest": "document",
		"Sec-Fetch-Mode": "navigate",
		"Sec-Ch-Ua":      "chromium",
		"Accept":         "text/html",
	}
	req := newTestRequest("GET", "http://example.com", headers, "")
	applyDeleteToRequest(req, "req.headers.Sec-*")

	for h := range req.Header {
		if strings.HasPrefix(h, "Sec-") {
			t.Errorf("header %q should have been deleted by wildcard", h)
		}
	}
	if req.Header.Get("Accept") == "" {
		t.Error("Accept header should still exist")
	}
}

func TestApplyDeleteToRequest_Method(t *testing.T) {
	req := newTestRequest("POST", "http://example.com", nil, "")
	applyDeleteToRequest(req, "req.method")
	if req.Method != "GET" {
		t.Errorf("got method %q, want GET", req.Method)
	}
}

func TestApplyDeleteToRequest_Path(t *testing.T) {
	req := newTestRequest("GET", "http://example.com/some/path", nil, "")
	applyDeleteToRequest(req, "req.path")
	if req.URL.Path != "" {
		t.Errorf("got path %q, want empty", req.URL.Path)
	}
}

func TestApplyDeleteToRequest_URL(t *testing.T) {
	req := newTestRequest("GET", "http://example.com/path?q=1", nil, "")
	applyDeleteToRequest(req, "req.url")
	if req.URL.String() != "" {
		t.Errorf("got url %q, want empty", req.URL.String())
	}
}

func TestApplyDeleteToRequest_Body(t *testing.T) {
	req := newTestRequest("POST", "http://example.com", nil, "some body")
	applyDeleteToRequest(req, "req.body")
	body, _ := io.ReadAll(req.Body)
	if string(body) != "" {
		t.Errorf("got body %q, want empty", string(body))
	}
	if req.ContentLength != 0 {
		t.Errorf("got content-length %d, want 0", req.ContentLength)
	}
}

func TestApplyDeleteToRequest_Query(t *testing.T) {
	req := newTestRequest("GET", "http://example.com/path?foo=1&bar=2", nil, "")
	applyDeleteToRequest(req, "req.query.foo")
	if req.URL.Query().Get("foo") != "" {
		t.Error("foo query param should be deleted")
	}
	if req.URL.Query().Get("bar") != "2" {
		t.Error("bar query param should still exist")
	}
}

func TestApplyDeleteToRequest_UnknownKey(t *testing.T) {
	req := newTestRequest("GET", "http://example.com", nil, "")
	applyDeleteToRequest(req, "req.unknown")
	// Should not panic
	if req.Method != "GET" {
		t.Error("method changed unexpectedly")
	}
}
