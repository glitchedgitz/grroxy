package app

import (
	"testing"

	"github.com/glitchedgitz/grroxy/grx/templates"
)

func newRequestData() map[string]any {
	return map[string]any{
		"method":      "GET",
		"url":         "/api/users?page=1",
		"path":        "/api/users",
		"query":       "page=1",
		"fragment":    "",
		"ext":         "",
		"has_cookies": false,
		"length":      42,
		"headers": [][]string{
			{"Host: ", "example.com"},
			{"User-Agent: ", "Mozilla/5.0"},
			{"Content-Type: ", "application/json"},
		},
		"raw": "GET /api/users?page=1 HTTP/1.1\r\nHost: example.com\r\nUser-Agent: Mozilla/5.0\r\nContent-Type: application/json\r\n\r\n{\"key\":\"value\"}",
	}
}

// --- RequestUpdateKey tests ---

func TestRequestUpdateKey_Method(t *testing.T) {
	data := newRequestData()
	RequestUpdateKey(data, "req.method", "POST")
	if got := data["method"].(string); got != "POST" {
		t.Errorf("got method %q, want POST", got)
	}
}

func TestRequestUpdateKey_URL(t *testing.T) {
	data := newRequestData()
	RequestUpdateKey(data, "req.url", "https://new.com/v2/items?limit=10")
	if got := data["url"].(string); got != "https://new.com/v2/items?limit=10" {
		t.Errorf("got url %q, want full url", got)
	}
	if got := data["path"].(string); got != "/v2/items" {
		t.Errorf("got path %q, want /v2/items", got)
	}
	if got := data["query"].(string); got != "limit=10" {
		t.Errorf("got query %q, want limit=10", got)
	}
}

func TestRequestUpdateKey_Path(t *testing.T) {
	data := newRequestData()
	RequestUpdateKey(data, "req.path", "/new/path")
	if got := data["path"].(string); got != "/new/path" {
		t.Errorf("got path %q, want /new/path", got)
	}
}

func TestRequestUpdateKey_Query(t *testing.T) {
	data := newRequestData()
	RequestUpdateKey(data, "req.query.sort", "asc")
	if got := data["query"].(string); got == "" {
		t.Error("query should not be empty")
	}
}

func TestRequestUpdateKey_Header_Existing(t *testing.T) {
	data := newRequestData()
	RequestUpdateKey(data, "req.headers.User-Agent", "CustomBot")
	headers := data["headers"].([][]string)
	found := false
	for _, h := range headers {
		// Header name may have trailing ": " or ":" depending on format
		if (h[0] == "User-Agent: " || h[0] == "User-Agent:") && h[1] == "CustomBot" {
			found = true
		}
	}
	if !found {
		t.Errorf("User-Agent header not updated, headers: %v", headers)
	}
}

func TestRequestUpdateKey_Header_New(t *testing.T) {
	data := newRequestData()
	RequestUpdateKey(data, "req.headers.X-New", "newval")
	headers := data["headers"].([][]string)
	found := false
	for _, h := range headers {
		if h[0] == "X-New: " && h[1] == "newval" {
			found = true
		}
	}
	if !found {
		t.Errorf("X-New header not added, headers: %v", headers)
	}
}

func TestRequestUpdateKey_Body(t *testing.T) {
	data := newRequestData()
	RequestUpdateKey(data, "req.body", "new body content")
	if got := data["length"].(int); got != 16 {
		t.Errorf("got length %d, want 16", got)
	}
}

func TestRequestUpdateKey_UnknownKey(t *testing.T) {
	data := newRequestData()
	RequestUpdateKey(data, "req.unknown", "value")
	// Should not panic, no unexpected changes
	if data["method"].(string) != "GET" {
		t.Error("method changed unexpectedly")
	}
}

// --- RequestDeleteKey tests ---

func TestRequestDeleteKey_Method(t *testing.T) {
	data := newRequestData()
	data["method"] = "POST"
	RequestDeleteKey(data, "req.method")
	if got := data["method"].(string); got != "GET" {
		t.Errorf("got method %q, want GET", got)
	}
}

func TestRequestDeleteKey_URL(t *testing.T) {
	data := newRequestData()
	RequestDeleteKey(data, "req.url")
	if got := data["url"].(string); got != "" {
		t.Errorf("got url %q, want empty", got)
	}
	if got := data["path"].(string); got != "" {
		t.Errorf("got path %q, want empty", got)
	}
}

func TestRequestDeleteKey_Path(t *testing.T) {
	data := newRequestData()
	RequestDeleteKey(data, "req.path")
	if got := data["path"].(string); got != "" {
		t.Errorf("got path %q, want empty", got)
	}
}

func TestRequestDeleteKey_Query(t *testing.T) {
	data := newRequestData()
	RequestDeleteKey(data, "req.query.page")
	// After deleting, the url should not contain page param
	url := data["url"].(string)
	if url == "/api/users?page=1" {
		t.Error("page query param should be deleted from url")
	}
}

func TestRequestDeleteKey_Header(t *testing.T) {
	data := newRequestData()
	RequestDeleteKey(data, "req.headers.User-Agent")
	headers := data["headers"].([][]string)
	for _, h := range headers {
		if h[0] == "User-Agent:" {
			t.Error("User-Agent header should be deleted")
		}
	}
}

func TestRequestDeleteKey_HeaderWildcard(t *testing.T) {
	data := map[string]any{
		"method": "GET",
		"url":    "/",
		"path":   "/",
		"query":  "",
		"headers": [][]string{
			{"Sec-Fetch-Dest:", "document"},
			{"Sec-Fetch-Mode:", "navigate"},
			{"Sec-Ch-Ua:", "chromium"},
			{"Accept:", "text/html"},
		},
		"raw": "GET / HTTP/1.1\r\n\r\n",
	}
	RequestDeleteKey(data, "req.headers.Sec-*")
	headers := data["headers"].([][]string)
	for _, h := range headers {
		if len(h) > 0 && len(h[0]) > 3 && h[0][:4] == "Sec-" {
			t.Errorf("header %q should have been deleted by wildcard", h[0])
		}
	}
	// Accept should remain
	found := false
	for _, h := range headers {
		if h[0] == "Accept:" {
			found = true
		}
	}
	if !found {
		t.Error("Accept header should still exist")
	}
}

func TestRequestDeleteKey_Body(t *testing.T) {
	data := newRequestData()
	RequestDeleteKey(data, "req.body")
	if got := data["length"].(int); got != 0 {
		t.Errorf("got length %d, want 0", got)
	}
}

// --- RequestReplace tests ---

func TestRequestReplace_Simple(t *testing.T) {
	data := newRequestData()
	RequestReplace(data, "Mozilla/5.0", "CustomBot", false)
	raw := data["raw"].(string)
	if raw == "" {
		t.Fatal("raw should not be empty")
	}
	if !stringContains(raw, "CustomBot") {
		t.Errorf("raw should contain CustomBot after replace, got: %s", raw)
	}
	if stringContains(raw, "Mozilla/5.0") {
		t.Error("raw should not contain Mozilla/5.0 after replace")
	}
}

func TestRequestReplace_Regex(t *testing.T) {
	data := newRequestData()
	RequestReplace(data, `Mozilla/\d+\.\d+`, "Bot/1.0", true)
	raw := data["raw"].(string)
	if raw == "" {
		t.Fatal("raw should not be empty")
	}
}

func TestRequestReplace_NoMatch(t *testing.T) {
	data := newRequestData()
	origMethod := data["method"].(string)
	RequestReplace(data, "NONEXISTENT", "replacement", false)
	if data["method"].(string) != origMethod {
		t.Error("method changed when replace had no match")
	}
}

func TestRequestReplace_InvalidRegex(t *testing.T) {
	data := newRequestData()
	// Invalid regex should not panic
	RequestReplace(data, "[invalid", "replacement", true)
	if data["method"].(string) != "GET" {
		t.Error("method changed on invalid regex")
	}
}

// --- buildRawRequest tests ---

func TestBuildRawRequest_Basic(t *testing.T) {
	data := map[string]any{
		"method": "POST",
		"url":    "/api/test",
		"headers": [][]string{
			{"Host: ", "example.com"},
			{"Content-Type: ", "application/json"},
		},
		"raw": "POST /api/test HTTP/1.1\r\nHost: example.com\r\n\r\n{\"data\":true}",
	}
	result := buildRawRequest(data)
	if result == "" {
		t.Fatal("result should not be empty")
	}
	if !containsString(result, "POST") {
		t.Error("result should contain POST method")
	}
	if !containsString(result, "/api/test") {
		t.Error("result should contain path")
	}
	if !containsString(result, "Host: ") {
		t.Error("result should contain Host header")
	}
}

func TestBuildRawRequest_Defaults(t *testing.T) {
	data := map[string]any{}
	result := buildRawRequest(data)
	if !containsString(result, "GET") {
		t.Error("default method should be GET")
	}
	if !containsString(result, "/ HTTP/1.1") {
		t.Error("default url should be /")
	}
}

func TestBuildRawRequest_NoHeaders(t *testing.T) {
	data := map[string]any{
		"method": "GET",
		"url":    "/test",
	}
	result := buildRawRequest(data)
	if result == "" {
		t.Fatal("result should not be empty")
	}
}

// --- runActions tests ---

func TestRunActions_Set(t *testing.T) {
	data := newRequestData()
	actions := []templates.Action{
		{ActionName: "set", Data: map[string]any{"req.method": "PUT"}},
	}
	result, err := runActions(actions, data)
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(result, "PUT") {
		t.Errorf("result should contain PUT, got: %s", result)
	}
}

func TestRunActions_Delete(t *testing.T) {
	data := newRequestData() // uses "Header: " format (with space), matching rawhttp output
	actions := []templates.Action{
		{ActionName: "delete", Data: map[string]any{"req.headers.User-Agent": nil}},
	}
	_, err := runActions(actions, data)
	if err != nil {
		t.Fatal(err)
	}
	headers := data["headers"].([][]string)
	for _, h := range headers {
		if len(h) >= 1 && stringContains(h[0], "User-Agent") {
			t.Error("User-Agent header should be deleted from headers data")
		}
	}
}

func TestRunActions_Replace(t *testing.T) {
	data := newRequestData()
	actions := []templates.Action{
		{ActionName: "replace", Data: map[string]any{"search": "GET", "value": "POST", "regex": false}},
	}
	result, err := runActions(actions, data)
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(result, "POST") {
		t.Errorf("result should contain POST after replace, got: %s", result)
	}
}

func TestRunActions_MultipleActions(t *testing.T) {
	data := newRequestData()
	actions := []templates.Action{
		{ActionName: "set", Data: map[string]any{"req.method": "DELETE"}},
		{ActionName: "set", Data: map[string]any{"req.headers.X-Custom": "test"}},
	}
	result, err := runActions(actions, data)
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(result, "DELETE") {
		t.Error("result should contain DELETE method")
	}
}

func TestRunActions_Empty(t *testing.T) {
	data := newRequestData()
	result, err := runActions([]templates.Action{}, data)
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Error("result should not be empty even with no actions")
	}
}

func TestRunActions_UnknownAction(t *testing.T) {
	data := newRequestData()
	actions := []templates.Action{
		{ActionName: "unknown_action", Data: map[string]any{"foo": "bar"}},
	}
	// Should not panic
	_, err := runActions(actions, data)
	if err != nil {
		t.Fatal(err)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && stringContains(s, substr))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
