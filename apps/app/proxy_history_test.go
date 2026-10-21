package app

import (
	"strings"
	"testing"

	"github.com/glitchedgitz/pocketbase/tools/search"
)

// --- the proxy id ---

// A caller says "1", the rows carry the padded id — padProxyID bridges the
// two, and must not pad an id that is already padded.
func TestPadProxyID(t *testing.T) {
	scenarios := []struct {
		name    string
		proxyID string
		want    string
	}{
		{"empty stays empty", "", ""},
		{"blank is empty", "   ", ""},
		{"bare id is padded", "1", "______________1"},
		{"padded id is left alone", "______________1", "______________1"},
		{"two digit id", "11", "_____________11"},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			if got := padProxyID(s.proxyID); got != s.want {
				t.Fatalf("padProxyID(%q) = %q, want %q", s.proxyID, got, s.want)
			}
		})
	}
}

// FormatStringID pads with strings.Repeat, which panics on a negative count —
// an id at or over the padded width must not reach it.
func TestPadProxyIDOversizedID(t *testing.T) {
	long := strings.Repeat("x", proxyIDWidth+5)

	if got := padProxyID(long); got != long {
		t.Fatalf("padProxyID(%q) = %q, want it handed back unpadded", long, got)
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

// One proxy is an exact match on the proxy field, bound as a param; with no id
// it is every proxy's rows, by type.
func TestProxyHistoryFilterMatchesProxyField(t *testing.T) {
	filter, params := proxyHistoryFilter("1", "", "")
	if filter != "proxy = {:proxy}" || params["proxy"] != "______________1" {
		t.Fatalf("proxyHistoryFilter(\"1\") = %q %v, want proxy = {:proxy} bound to the padded id", filter, params)
	}

	filter, params = proxyHistoryFilter("", "", "")
	if filter != "type = {:type}" || params["type"] != "proxy" {
		t.Fatalf("proxyHistoryFilter(\"\") = %q %v, want type = {:type} bound to proxy", filter, params)
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
