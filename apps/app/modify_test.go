package app

import (
	"strings"
	"testing"

	"github.com/glitchedgitz/grroxy/grx/rawhttp"
	"github.com/glitchedgitz/grroxy/grx/templates"
)

// These tests cover the request rebuild path behind /api/request/modify — the
// one the "AutoRemove useless headers" toggle drives. They exercise both CRLF
// and LF requests: the parser accepts either, so the rebuild has to as well.

// lineBreaks is the set of line endings every round-trip test runs against.
var lineBreaks = map[string]string{"CRLF": "\r\n", "LF": "\n"}

// rawRequest joins lines with lb, mirroring how a request sits in the editor.
func rawRequest(lb string, lines ...string) string {
	return strings.Join(lines, lb)
}

// recordFrom builds the same map the /api/request/modify handler builds.
func recordFrom(raw string) map[string]any {
	p := rawhttp.ParseRequest([]byte(raw))
	return map[string]any{
		"method":       p.Method,
		"http_version": p.HTTPVersion,
		"url":          p.URL,
		"path":         p.URL,
		"query":        "",
		"fragment":     "",
		"headers":      p.Headers,
		"length":       len(raw),
		"raw":          raw,
	}
}

// postRequest is a representative request with headers worth stripping and a
// body worth keeping.
func postRequest(lb string) string {
	return rawRequest(lb,
		"POST /login?next=/home HTTP/1.1",
		"Host: example.com",
		"Sec-Fetch-Mode: cors",
		"Sec-Ch-Ua: chromium",
		"Accept-Encoding: gzip, deflate",
		"Accept-Language: en-US",
		"Content-Type: application/json",
		"",
		`{"user":"admin","pass":"hunter2"}`,
	)
}

// --- splitRawRequest ---

func TestSplitRawRequest(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		wantHead string
		wantSep  string
		wantBody string
	}{
		{
			name:     "crlf request",
			raw:      "POST / HTTP/1.1\r\nHost: a\r\n\r\nbody",
			wantHead: "POST / HTTP/1.1\r\nHost: a",
			wantSep:  "\r\n\r\n",
			wantBody: "body",
		},
		{
			name:     "lf request",
			raw:      "POST / HTTP/1.1\nHost: a\n\nbody",
			wantHead: "POST / HTTP/1.1\nHost: a",
			wantSep:  "\n\n",
			wantBody: "body",
		},
		{
			name:     "no separator",
			raw:      "GET / HTTP/1.1\r\nHost: a",
			wantHead: "GET / HTTP/1.1\r\nHost: a",
			wantSep:  "",
			wantBody: "",
		},
		{
			name:     "empty request",
			raw:      "",
			wantHead: "",
			wantSep:  "",
			wantBody: "",
		},
		{
			name:     "empty body after separator",
			raw:      "GET / HTTP/1.1\r\nHost: a\r\n\r\n",
			wantHead: "GET / HTTP/1.1\r\nHost: a",
			wantSep:  "\r\n\r\n",
			wantBody: "",
		},
		{
			name:     "body containing blank lines is kept whole",
			raw:      "POST / HTTP/1.1\r\nHost: a\r\n\r\nfirst\r\n\r\nsecond",
			wantHead: "POST / HTTP/1.1\r\nHost: a",
			wantSep:  "\r\n\r\n",
			wantBody: "first\r\n\r\nsecond",
		},
		{
			name:     "crlf wins when body itself uses lf",
			raw:      "POST / HTTP/1.1\r\nHost: a\r\n\r\nline\n\nline",
			wantHead: "POST / HTTP/1.1\r\nHost: a",
			wantSep:  "\r\n\r\n",
			wantBody: "line\n\nline",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			head, sep, body := splitRawRequest(tt.raw)
			if head != tt.wantHead {
				t.Errorf("head = %q, want %q", head, tt.wantHead)
			}
			if sep != tt.wantSep {
				t.Errorf("sep = %q, want %q", sep, tt.wantSep)
			}
			if body != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
			if head+sep+body != tt.raw {
				t.Errorf("head+sep+body = %q, want %q", head+sep+body, tt.raw)
			}
		})
	}
}

// --- buildRawRequest ---

// A parse-then-rebuild with no actions must return the request untouched.
// This is the invariant the AutoRemove toggle relies on for every field it
// isn't deleting.
func TestBuildRawRequest_RoundTripIsLossless(t *testing.T) {
	for name, lb := range lineBreaks {
		t.Run(name, func(t *testing.T) {
			raw := postRequest(lb)
			if got := buildRawRequest(recordFrom(raw)); got != raw {
				t.Errorf("round-trip changed the request:\n got: %q\nwant: %q", got, raw)
			}
		})
	}
}

// The regression: an LF request lost its entire body because the rebuild
// split on a hardcoded "\r\n\r\n".
func TestBuildRawRequest_KeepsBody(t *testing.T) {
	const body = `{"user":"admin","pass":"hunter2"}`

	for name, lb := range lineBreaks {
		t.Run(name, func(t *testing.T) {
			data := recordFrom(postRequest(lb))
			RequestDeleteKey(data, "req.headers.Sec-*")

			got := buildRawRequest(data)
			if !strings.Contains(got, body) {
				t.Errorf("body dropped after deleting headers:\n%q", got)
			}
			if strings.Contains(got, "Sec-Fetch-Mode") {
				t.Errorf("Sec-* header survived deletion:\n%q", got)
			}
		})
	}
}

func TestBuildRawRequest_KeepsLineBreak(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string // line break the rebuild must use
	}{
		{"crlf with body", "POST / HTTP/1.1\r\nHost: a\r\n\r\nbody", "\r\n"},
		{"lf with body", "POST / HTTP/1.1\nHost: a\n\nbody", "\n"},
		{"crlf without body", "GET / HTTP/1.1\r\nHost: a", "\r\n"},
		{"lf without body", "GET / HTTP/1.1\nHost: a", "\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildRawRequest(recordFrom(tt.raw))
			if !strings.HasPrefix(got, "GET / HTTP/1.1"+tt.want) &&
				!strings.HasPrefix(got, "POST / HTTP/1.1"+tt.want) {
				t.Errorf("request line does not end with %q:\n%q", tt.want, got)
			}
			if tt.want == "\n" && strings.Contains(got, "\r\n") {
				t.Errorf("LF request was rewritten to CRLF:\n%q", got)
			}
		})
	}
}

// The rebuild used to hardcode HTTP/1.1, silently downgrading the request line.
func TestBuildRawRequest_KeepsHTTPVersion(t *testing.T) {
	for _, version := range []string{"HTTP/1.1", "HTTP/1.0", "HTTP/2"} {
		t.Run(version, func(t *testing.T) {
			raw := "GET / " + version + "\r\nHost: a\r\n\r\n"
			got := buildRawRequest(recordFrom(raw))
			if !strings.HasPrefix(got, "GET / "+version+"\r\n") {
				t.Errorf("version %q not preserved:\n%q", version, got)
			}
		})
	}
}

func TestBuildRawRequest_Defaults(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
		want string
	}{
		{"empty record", map[string]any{}, "GET / HTTP/1.1\r\n\r\n"},
		{"blank method", map[string]any{"method": ""}, " / HTTP/1.1\r\n\r\n"},
		{"blank url falls back to slash", map[string]any{"url": ""}, "GET / HTTP/1.1\r\n\r\n"},
		{"blank version falls back to 1.1", map[string]any{"http_version": ""}, "GET / HTTP/1.1\r\n\r\n"},
		{"wrong types are ignored", map[string]any{"method": 42, "headers": "nope"}, "GET / HTTP/1.1\r\n\r\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildRawRequest(tt.data); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// A body with blank lines in it (multipart, form data, pasted text) must not be
// truncated at the first blank line.
func TestBuildRawRequest_KeepsBodyWithBlankLines(t *testing.T) {
	for name, lb := range lineBreaks {
		t.Run(name, func(t *testing.T) {
			body := rawRequest(lb,
				"--boundary",
				`Content-Disposition: form-data; name="a"`,
				"",
				"value-a",
				"--boundary--",
			)
			raw := rawRequest(lb, "POST /upload HTTP/1.1", "Host: example.com", "", body)

			data := recordFrom(raw)
			RequestDeleteKey(data, "req.headers.Nonexistent")

			got := buildRawRequest(data)
			if !strings.HasSuffix(got, body) {
				t.Errorf("body truncated:\n got: %q\nwant suffix: %q", got, body)
			}
		})
	}
}

// Rebuilding is idempotent — toggling AutoRemove repeatedly must not keep
// growing or shrinking the request.
func TestBuildRawRequest_Idempotent(t *testing.T) {
	raws := []string{
		postRequest("\r\n"),
		postRequest("\n"),
		"GET / HTTP/1.1\r\nHost: a",
		"GET / HTTP/1.1\nHost: a",
		"GET / HTTP/1.1\r\nHost: a\r\n\r\n",
	}

	for _, raw := range raws {
		once := buildRawRequest(recordFrom(raw))
		twice := buildRawRequest(recordFrom(once))
		if once != twice {
			t.Errorf("rebuild not idempotent for %q:\n once: %q\ntwice: %q", raw, once, twice)
		}
	}
}

func TestBuildRawRequest_DuplicateHeadersPreserved(t *testing.T) {
	raw := "GET / HTTP/1.1\r\nCookie: a=1\r\nCookie: b=2\r\nHost: x\r\n\r\n"
	got := buildRawRequest(recordFrom(raw))
	if strings.Count(got, "Cookie:") != 2 {
		t.Errorf("expected both Cookie headers, got:\n%q", got)
	}
}

// --- RequestDeleteKey ---

func TestRequestDeleteKey_Headers(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		gone    []string
		present []string
	}{
		{
			name:    "exact match",
			key:     "req.headers.Accept-Encoding",
			gone:    []string{"Accept-Encoding"},
			present: []string{"Host", "Sec-Fetch-Mode", "Content-Type"},
		},
		{
			name:    "wildcard prefix",
			key:     "req.headers.Sec-*",
			gone:    []string{"Sec-Fetch-Mode", "Sec-Ch-Ua"},
			present: []string{"Host", "Accept-Encoding", "Content-Type"},
		},
		{
			name:    "unknown header is a no-op",
			key:     "req.headers.X-Does-Not-Exist",
			present: []string{"Host", "Sec-Fetch-Mode", "Accept-Encoding", "Content-Type"},
		},
		{
			name:    "partial name does not match without wildcard",
			key:     "req.headers.Sec-",
			present: []string{"Sec-Fetch-Mode", "Sec-Ch-Ua"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := recordFrom(postRequest("\r\n"))
			RequestDeleteKey(data, tt.key)
			got := buildRawRequest(data)

			for _, h := range tt.gone {
				if strings.Contains(got, h+":") {
					t.Errorf("header %q should be gone:\n%q", h, got)
				}
			}
			for _, h := range tt.present {
				if !strings.Contains(got, h+":") {
					t.Errorf("header %q should have survived:\n%q", h, got)
				}
			}
		})
	}
}

func TestRequestDeleteKey_AllDuplicates(t *testing.T) {
	data := recordFrom("GET / HTTP/1.1\r\nCookie: a=1\r\nCookie: b=2\r\nHost: x\r\n\r\n")
	RequestDeleteKey(data, "req.headers.Cookie")
	if got := buildRawRequest(data); strings.Contains(got, "Cookie") {
		t.Errorf("all Cookie headers should be deleted:\n%q", got)
	}
}

func TestRequestDeleteKey_Body(t *testing.T) {
	for name, lb := range lineBreaks {
		t.Run(name, func(t *testing.T) {
			data := recordFrom(postRequest(lb))
			RequestDeleteKey(data, "req.body")

			if got := data["length"].(int); got != 0 {
				t.Errorf("length = %d, want 0", got)
			}
			got := buildRawRequest(data)
			if strings.Contains(got, "hunter2") {
				t.Errorf("body should be gone:\n%q", got)
			}
			if !strings.Contains(got, "Host:") {
				t.Errorf("headers should survive body deletion:\n%q", got)
			}
		})
	}
}

func TestRequestDeleteKey_Method(t *testing.T) {
	data := recordFrom(postRequest("\r\n"))
	RequestDeleteKey(data, "req.method")
	if got := data["method"].(string); got != "GET" {
		t.Errorf("method = %q, want GET", got)
	}
}

func TestRequestDeleteKey_QueryParam(t *testing.T) {
	data := recordFrom("GET /search?q=a&page=2 HTTP/1.1\r\nHost: x\r\n\r\n")
	RequestDeleteKey(data, "req.query.page")

	url := data["url"].(string)
	if strings.Contains(url, "page=") {
		t.Errorf("page param should be gone, url = %q", url)
	}
	if !strings.Contains(url, "q=a") {
		t.Errorf("q param should survive, url = %q", url)
	}
}

func TestRequestDeleteKey_URLAndPath(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		wantURL  string
		wantPath string
	}{
		{"delete url", "req.url", "", ""},
		{"delete path", "req.path", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := recordFrom("GET /api/users?page=1 HTTP/1.1\r\nHost: x\r\n\r\n")
			RequestDeleteKey(data, tt.key)

			if got := data["path"].(string); got != tt.wantPath {
				t.Errorf("path = %q, want %q", got, tt.wantPath)
			}
			// url is cleared outright for req.url; for req.path only the path
			// segment is stripped.
			if tt.key == "req.url" {
				if got := data["url"].(string); got != tt.wantURL {
					t.Errorf("url = %q, want %q", got, tt.wantURL)
				}
			}
		})
	}
}

// --- RequestUpdateKey ---

func TestRequestUpdateKey_ExistingHeader(t *testing.T) {
	data := recordFrom(postRequest("\r\n"))
	RequestUpdateKey(data, "req.headers.Content-Type", "text/plain")

	got := buildRawRequest(data)
	if !strings.Contains(got, "text/plain") {
		t.Errorf("Content-Type not updated:\n%q", got)
	}
	if strings.Contains(got, "application/json") {
		t.Errorf("old Content-Type still present:\n%q", got)
	}
	if strings.Count(got, "Content-Type") != 1 {
		t.Errorf("expected exactly one Content-Type header:\n%q", got)
	}
}

func TestRequestUpdateKey_NewHeader(t *testing.T) {
	data := recordFrom(postRequest("\r\n"))
	RequestUpdateKey(data, "req.headers.X-Trace", "abc123")

	if got := buildRawRequest(data); !strings.Contains(got, "X-Trace: abc123") {
		t.Errorf("new header not appended:\n%q", got)
	}
}

func TestRequestUpdateKey_Body(t *testing.T) {
	for name, lb := range lineBreaks {
		t.Run(name, func(t *testing.T) {
			data := recordFrom(postRequest(lb))
			RequestUpdateKey(data, "req.body", "replaced")

			if got := data["length"].(int); got != len("replaced") {
				t.Errorf("length = %d, want %d", got, len("replaced"))
			}
			got := buildRawRequest(data)
			if !strings.HasSuffix(got, "replaced") {
				t.Errorf("body not replaced:\n%q", got)
			}
			if strings.Contains(got, "hunter2") {
				t.Errorf("old body still present:\n%q", got)
			}
		})
	}
}

func TestRequestUpdateKey_Path(t *testing.T) {
	data := recordFrom("GET /api/users?page=1 HTTP/1.1\r\nHost: x\r\n\r\n")
	RequestUpdateKey(data, "req.path", "/new/path")

	if got := data["path"].(string); got != "/new/path" {
		t.Errorf("path = %q, want /new/path", got)
	}
	if got := data["url"].(string); !strings.HasPrefix(got, "/new/path") {
		t.Errorf("url = %q, want it to start with /new/path", got)
	}
}

func TestRequestUpdateKey_QueryParam(t *testing.T) {
	data := recordFrom("GET /search?q=a HTTP/1.1\r\nHost: x\r\n\r\n")
	RequestUpdateKey(data, "req.query.sort", "asc")

	url := data["url"].(string)
	if !strings.Contains(url, "sort=asc") {
		t.Errorf("url = %q, want it to contain sort=asc", url)
	}
	if !strings.Contains(url, "q=a") {
		t.Errorf("url = %q, want existing q param preserved", url)
	}
	if got := data["query"].(string); !strings.Contains(got, "sort=asc") {
		t.Errorf("query = %q, want it to contain sort=asc", got)
	}
}

func TestRequestUpdateKey_UnknownKeyIsNoOp(t *testing.T) {
	raw := postRequest("\r\n")
	data := recordFrom(raw)
	RequestUpdateKey(data, "req.unknown", "value")

	if got := buildRawRequest(data); got != raw {
		t.Errorf("unknown key changed the request:\n got: %q\nwant: %q", got, raw)
	}
}

func TestRequestUpdateKey_MethodAndURL(t *testing.T) {
	data := recordFrom(postRequest("\r\n"))
	RequestUpdateKey(data, "req.method", "PUT")
	RequestUpdateKey(data, "req.url", "/v2/login?next=/home")

	got := buildRawRequest(data)
	if !strings.HasPrefix(got, "PUT /v2/login?next=/home HTTP/1.1\r\n") {
		t.Errorf("request line not updated:\n%q", got)
	}
}

// --- RequestReplace ---

func TestRequestReplace_InBody(t *testing.T) {
	for name, lb := range lineBreaks {
		t.Run(name, func(t *testing.T) {
			data := recordFrom(postRequest(lb))
			RequestReplace(data, "hunter2", "REDACTED", false)

			got := buildRawRequest(data)
			if !strings.Contains(got, "REDACTED") {
				t.Errorf("replacement missing from body:\n%q", got)
			}
			if strings.Contains(got, "hunter2") {
				t.Errorf("original value still present:\n%q", got)
			}
			if !strings.Contains(got, "Host: example.com") {
				t.Errorf("headers lost during replace:\n%q", got)
			}
		})
	}
}

func TestRequestReplace_Regex(t *testing.T) {
	data := recordFrom(postRequest("\r\n"))
	RequestReplace(data, `"pass":"[^"]*"`, `"pass":"***"`, true)

	if got := buildRawRequest(data); !strings.Contains(got, `"pass":"***"`) {
		t.Errorf("regex replacement did not apply:\n%q", got)
	}
}

func TestRequestReplace_NoMatchLeavesRequestIntact(t *testing.T) {
	raw := postRequest("\r\n")
	data := recordFrom(raw)
	RequestReplace(data, "NONEXISTENT", "replacement", false)

	if got := buildRawRequest(data); got != raw {
		t.Errorf("no-match replace modified the request:\n got: %q\nwant: %q", got, raw)
	}
}

func TestRequestReplace_InvalidRegexLeavesRequestIntact(t *testing.T) {
	raw := postRequest("\r\n")
	data := recordFrom(raw)
	RequestReplace(data, "[unclosed", "x", true)

	if got := buildRawRequest(data); got != raw {
		t.Errorf("invalid regex modified the request:\n got: %q\nwant: %q", got, raw)
	}
}

// Replace runs against the current rebuilt state, so a header deleted by an
// earlier task must not reappear.
func TestRequestReplace_AfterDelete(t *testing.T) {
	data := recordFrom(postRequest("\r\n"))
	RequestDeleteKey(data, "req.headers.Sec-*")
	RequestReplace(data, "example.com", "target.com", false)

	got := buildRawRequest(data)
	if strings.Contains(got, "Sec-Fetch-Mode") {
		t.Errorf("deleted header came back after replace:\n%q", got)
	}
	if !strings.Contains(got, "Host: target.com") {
		t.Errorf("replacement not applied:\n%q", got)
	}
	if !strings.Contains(got, "hunter2") {
		t.Errorf("body lost after delete+replace:\n%q", got)
	}
}

// --- runActions: the exact task list the AutoRemove toggle sends ---

// autoRemoveTasks mirrors removeUselessHeaders() in TwinEditors2.svelte.
func autoRemoveTasks() []templates.Action {
	return []templates.Action{{
		ActionName: "delete",
		Data: map[string]any{
			"req.headers.Sec-*":                     "",
			"req.headers.Accept-Encoding":           "",
			"req.headers.Accept-Language":           "",
			"req.headers.Upgrade-Insecure-Requests": "",
			"req.headers.Priority":                  "",
			"req.headers.Connection":                "",
		},
	}}
}

func TestRunActions_AutoRemoveUselessHeaders(t *testing.T) {
	for name, lb := range lineBreaks {
		t.Run(name, func(t *testing.T) {
			raw := rawRequest(lb,
				"POST /login HTTP/1.1",
				"Host: example.com",
				"Connection: keep-alive",
				"Sec-Fetch-Mode: cors",
				"Accept-Encoding: gzip",
				"Accept-Language: en-US",
				"Priority: u=1",
				"Upgrade-Insecure-Requests: 1",
				"Content-Type: application/json",
				"",
				`{"user":"admin","pass":"hunter2"}`,
			)

			got, err := runActions(autoRemoveTasks(), recordFrom(raw))
			if err != nil {
				t.Fatal(err)
			}

			for _, h := range []string{
				"Connection", "Sec-Fetch-Mode", "Accept-Encoding",
				"Accept-Language", "Priority", "Upgrade-Insecure-Requests",
			} {
				if strings.Contains(got, h+":") {
					t.Errorf("%q should have been removed:\n%q", h, got)
				}
			}
			for _, h := range []string{"Host: example.com", "Content-Type: application/json"} {
				if !strings.Contains(got, h) {
					t.Errorf("%q should have survived:\n%q", h, got)
				}
			}
			if !strings.HasSuffix(got, `{"user":"admin","pass":"hunter2"}`) {
				t.Errorf("body was trimmed:\n%q", got)
			}
		})
	}
}

// Toggling AutoRemove off then on again must converge, not keep editing.
func TestRunActions_AutoRemoveIsIdempotent(t *testing.T) {
	for name, lb := range lineBreaks {
		t.Run(name, func(t *testing.T) {
			once, err := runActions(autoRemoveTasks(), recordFrom(postRequest(lb)))
			if err != nil {
				t.Fatal(err)
			}
			twice, err := runActions(autoRemoveTasks(), recordFrom(once))
			if err != nil {
				t.Fatal(err)
			}
			if once != twice {
				t.Errorf("not idempotent:\n once: %q\ntwice: %q", once, twice)
			}
		})
	}
}

func TestRunActions_NoTasksLeavesRequestUnchanged(t *testing.T) {
	for name, lb := range lineBreaks {
		t.Run(name, func(t *testing.T) {
			raw := postRequest(lb)
			got, err := runActions(nil, recordFrom(raw))
			if err != nil {
				t.Fatal(err)
			}
			if got != raw {
				t.Errorf("no-op run changed the request:\n got: %q\nwant: %q", got, raw)
			}
		})
	}
}

func TestRunActions_UnknownActionIsIgnored(t *testing.T) {
	raw := postRequest("\r\n")
	got, err := runActions([]templates.Action{
		{ActionName: "not_a_real_action", Data: map[string]any{"foo": "bar"}},
	}, recordFrom(raw))
	if err != nil {
		t.Fatal(err)
	}
	if got != raw {
		t.Errorf("unknown action changed the request:\n got: %q\nwant: %q", got, raw)
	}
}

func TestRunActions_SetThenDelete(t *testing.T) {
	data := recordFrom(postRequest("\r\n"))
	got, err := runActions([]templates.Action{
		{ActionName: "set", Data: map[string]any{"req.headers.X-Trace": "abc"}},
		{ActionName: "delete", Data: map[string]any{"req.headers.Sec-*": ""}},
	}, data)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "X-Trace: abc") {
		t.Errorf("set header missing:\n%q", got)
	}
	if strings.Contains(got, "Sec-Ch-Ua") {
		t.Errorf("deleted header present:\n%q", got)
	}
	if !strings.Contains(got, "hunter2") {
		t.Errorf("body lost:\n%q", got)
	}
}

// A request with no body at all must not gain one.
func TestRunActions_BodylessRequest(t *testing.T) {
	for name, lb := range lineBreaks {
		t.Run(name, func(t *testing.T) {
			raw := rawRequest(lb, "GET /health HTTP/1.1", "Host: example.com", "Sec-Fetch-Mode: cors", "", "")

			got, err := runActions(autoRemoveTasks(), recordFrom(raw))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(got, "Sec-Fetch-Mode") {
				t.Errorf("Sec-* header survived:\n%q", got)
			}
			want := rawRequest(lb, "GET /health HTTP/1.1", "Host: example.com", "", "")
			if got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}
