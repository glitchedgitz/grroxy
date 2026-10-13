package app

import (
	"strings"
	"testing"

	"github.com/glitchedgitz/pocketbase/tools/search"
)

// --- the generated_by tag ---

// A caller says "1", the rows are stamped with the padded id — the tag has to
// bridge the two, and must not pad an id that is already padded.
func TestProxyHistoryTag(t *testing.T) {
	scenarios := []struct {
		name    string
		proxyID string
		want    string
	}{
		{"every proxy", "", "proxy/"},
		{"blank is every proxy", "   ", "proxy/"},
		{"bare id is padded", "1", "proxy/______________1"},
		{"padded id is left alone", "______________1", "proxy/______________1"},
		{"two digit id", "11", "proxy/_____________11"},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			if got := proxyHistoryTag(s.proxyID); got != s.want {
				t.Fatalf("proxyHistoryTag(%q) = %q, want %q", s.proxyID, got, s.want)
			}
		})
	}
}

// FormatStringID pads with strings.Repeat, which panics on a negative count —
// an id at or over the padded width must not reach it.
func TestProxyHistoryTagOversizedID(t *testing.T) {
	long := strings.Repeat("x", proxyIDWidth+5)

	if got := proxyHistoryTag(long); got != "proxy/"+long {
		t.Fatalf("proxyHistoryTag(%q) = %q, want it handed back unpadded", long, got)
	}
}

// --- the row filter ---

func TestProxyHistoryFilterIsResolvable(t *testing.T) {
	scenarios := []struct {
		name              string
		proxyID, host, ex string
	}{
		{"every proxy", "", "", ""},
		{"one proxy", "1", "", ""},
		{"with schemed host", "1", "https://example.com", ""},
		{"with bare host", "1", "example.com", ""},
		{"with extra filter", "1", "", "resp.status = 500"},
		{"with everything", "1", "https://example.com", "req.url ~ '.js' OR resp.status = 200"},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			filter, params := proxyHistoryFilter(s.proxyID, s.host, s.ex)

			if _, err := search.FilterData(filter).BuildExpr(rowsResolver(), params); err != nil {
				t.Fatalf("BuildExpr(%q) = %v, want no error", filter, err)
			}
		})
	}
}

// The tag is bound, never interpolated: a padded id is all underscores, and "_"
// is a single-character wildcard in LIKE. Bound values get escaped, so
// "proxy/______________1" cannot also match "proxy/_____________11".
func TestProxyHistoryFilterBindsTag(t *testing.T) {
	filter, params := proxyHistoryFilter("1", "", "")

	if strings.Contains(filter, "proxy/") {
		t.Fatalf("proxyHistoryFilter() = %q, want the tag bound as a param", filter)
	}

	if got := params["proxy"]; got != "proxy/______________1" {
		t.Fatalf("params[proxy] = %v, want %q", got, "proxy/______________1")
	}
}

// An OR inside the caller's filter must not widen the proxy condition into
// "this proxy, or anything matching the rest".
func TestProxyHistoryFilterParenthesisesExtra(t *testing.T) {
	filter, params := proxyHistoryFilter("1", "", "resp.status = 500 OR resp.status = 200")

	if !strings.HasSuffix(filter, "(resp.status = 500 OR resp.status = 200)") {
		t.Fatalf("proxyHistoryFilter() = %q, want the extra filter parenthesised", filter)
	}

	if _, err := search.FilterData(filter).BuildExpr(rowsResolver(), params); err != nil {
		t.Fatalf("BuildExpr(%q) = %v, want no error", filter, err)
	}
}

// A host with a scheme is matched exactly, a bare one only as a substring —
// stored hosts carry the scheme.
func TestProxyHistoryFilterHostMatching(t *testing.T) {
	exact, params := proxyHistoryFilter("1", "https://example.com/some/path", "")
	if !strings.Contains(exact, "host = {:host}") {
		t.Fatalf("proxyHistoryFilter() = %q, want an exact host match", exact)
	}
	if got := params["host"]; got != "https://example.com" {
		t.Fatalf("params[host] = %v, want the path trimmed off", got)
	}

	partial, params := proxyHistoryFilter("1", "example.com", "")
	if !strings.Contains(partial, "host ~ {:host}") {
		t.Fatalf("proxyHistoryFilter() = %q, want a substring host match", partial)
	}
	if got := params["host"]; got != "example.com" {
		t.Fatalf("params[host] = %v, want %q", got, "example.com")
	}
}

// The filter always carries the proxy condition, so it is never the empty
// string FindRecordsByFilter refuses.
func TestProxyHistoryFilterIsNeverEmpty(t *testing.T) {
	if filter, _ := proxyHistoryFilter("", "", ""); filter == "" {
		t.Fatal("proxyHistoryFilter() = \"\", want a filter FindRecordsByFilter accepts")
	}
}

// The grammar only understands AND/OR/NOT, not && / ||.
func TestProxyHistoryFilterAvoidsCStyleOperators(t *testing.T) {
	filter, _ := proxyHistoryFilter("1", "example.com", "resp.status = 200")

	if strings.Contains(filter, "&&") || strings.Contains(filter, "||") {
		t.Fatalf("proxyHistoryFilter() = %q, want AND/OR keywords", filter)
	}
}

// --- ordering ---

func TestProxyHistorySort(t *testing.T) {
	scenarios := []struct {
		order     string
		wantSort  string
		wantOrder string
	}{
		{"", "-index,-index_minor", "newest"},
		{"newest", "-index,-index_minor", "newest"},
		{"oldest", "index,index_minor", "oldest"},
		{"  OLDEST  ", "index,index_minor", "oldest"},
		{"nonsense", "-index,-index_minor", "newest"},
	}

	for _, s := range scenarios {
		t.Run(s.order, func(t *testing.T) {
			sort, order := proxyHistorySort(s.order)
			if sort != s.wantSort || order != s.wantOrder {
				t.Fatalf("proxyHistorySort(%q) = (%q, %q), want (%q, %q)", s.order, sort, order, s.wantSort, s.wantOrder)
			}
		})
	}
}
