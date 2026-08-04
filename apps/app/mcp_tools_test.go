package app

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/glitchedgitz/grroxy/internal/schemas"
	"github.com/glitchedgitz/pocketbase/models"
	"github.com/glitchedgitz/pocketbase/tools/search"
	pbtypes "github.com/glitchedgitz/pocketbase/tools/types"
)

// sitemapResolver accepts the fields a per-host sitemap collection actually
// exposes: its own columns plus anything reached through the "data" relation.
func sitemapResolver() search.FieldResolver {
	return search.NewSimpleFieldResolver(
		"id", "created", "updated",
		"path", "query", "fragment", "type", "ext",
		`^data\.[\w\.]+$`,
	)
}

func buildFilter(t *testing.T, filter string, resolver search.FieldResolver) error {
	t.Helper()

	_, err := search.FilterData(filter).BuildExpr(resolver)

	return err
}

// --- bug: filters on a sitemap collection were passed through unprefixed ---

func TestPrefixDataFilter(t *testing.T) {
	scenarios := []struct {
		name   string
		filter string
		want   string
	}{
		{"empty", "", ""},
		{"single field", "req.url ~ '.js'", "data.req.url ~ '.js'"},
		{"already prefixed", "data.req.url ~ '.js'", "data.req.url ~ '.js'"},
		{"sitemap own field untouched", "path ~ '/api/%'", "path ~ '/api/%'"},
		{
			"multiple fields",
			"resp.status = 200 AND req.ext = 'js'",
			"data.resp.status = 200 AND data.req.ext = 'js'",
		},
		{
			"nested group",
			"(req.url ~ '.js' OR req.url ~ '.map') AND resp.status = 200",
			"(data.req.url ~ '.js' OR data.req.url ~ '.map') AND data.resp.status = 200",
		},
		{"field name inside quoted text", "req.url ~ 'req.url'", "data.req.url ~ 'req.url'"},
		{"value identifier untouched", "is_https = true", "data.is_https = true"},
		{"regex operand preserved", `req.url ~ /\.js$/`, `data.req.url ~ /\.js$/`},
		{"escaped quote preserved", `req.url ~ 'it\'s'`, `data.req.url ~ 'it\'s'`},
		// unscannable input is passed through so the query reports the real error
		{"unscannable passthrough", "req.url ~ '.js' &&", "req.url ~ '.js' &&"},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			if got := prefixDataFilter(s.filter); got != s.want {
				t.Fatalf("prefixDataFilter(%q) = %q, want %q", s.filter, got, s.want)
			}
		})
	}
}

// The regression itself: an AI supplied filter that names "_data" fields is
// unresolvable against a sitemap collection until it is prefixed, and used to
// fail with "invalid or empty filter expression".
func TestPrefixDataFilterMakesSitemapFiltersResolvable(t *testing.T) {
	filters := []string{
		"req.url ~ '.js'",
		"resp.status = 200 AND req.ext = 'js'",
		"(req.url ~ '.js' OR req.url ~ '.map') AND has_resp = true",
		"host ~ 'example.com'",
	}

	for _, filter := range filters {
		t.Run(filter, func(t *testing.T) {
			if err := buildFilter(t, filter, sitemapResolver()); err == nil {
				t.Fatalf("expected the unprefixed filter %q to be unresolvable", filter)
			}

			prefixed := prefixDataFilter(filter)
			if err := buildFilter(t, prefixed, sitemapResolver()); err != nil {
				t.Fatalf("BuildExpr(%q) = %v, want no error", prefixed, err)
			}
		})
	}
}

// Fields of the sitemap collection itself must keep resolving untouched.
func TestPrefixDataFilterKeepsSitemapOwnFieldsResolvable(t *testing.T) {
	filter := prefixDataFilter("path ~ '/api/' AND ext = 'js'")

	if err := buildFilter(t, filter, sitemapResolver()); err != nil {
		t.Fatalf("BuildExpr(%q) = %v, want no error", filter, err)
	}
}

func TestIsSitemapCollection(t *testing.T) {
	sitemap := &models.Collection{Schema: schemas.Sitemap}
	if !isSitemapCollection(sitemap) {
		t.Fatal("isSitemapCollection(sitemap collection) = false, want true")
	}

	data := &models.Collection{Schema: schemas.Rows}
	if isSitemapCollection(data) {
		t.Fatal("isSitemapCollection(_data collection) = true, want false")
	}
}

// --- bug: headers were never stripped, blowing the result past the token limit ---

// record.Get() hands back a json column as types.JsonRaw, so a plain
// map[string]any type assertion silently never matches. The rows must come out
// of a real record to keep that trap covered.
func dataRecord(t *testing.T, reqJSON string) *models.Record {
	t.Helper()

	collection := &models.Collection{Schema: schemas.Rows}
	collection.Name = "_data"

	record := models.NewRecord(collection)
	record.Set("req_json", reqJSON)

	return record
}

func TestWithoutHeadersStripsRecordJSON(t *testing.T) {
	record := dataRecord(t, `{"url":"/a.js","ext":".js","headers":{"Cookie":"secret","User-Agent":"x"}}`)

	stripped := withoutHeaders(record.Get("req_json"))

	decoded, ok := stripped.(map[string]any)
	if !ok {
		t.Fatalf("withoutHeaders() = %T, want map[string]any", stripped)
	}

	if _, exists := decoded["headers"]; exists {
		t.Fatal("withoutHeaders() kept the headers entry")
	}

	if decoded["url"] != "/a.js" || decoded["ext"] != ".js" {
		t.Fatalf("withoutHeaders() = %v, want the other fields preserved", decoded)
	}
}

// the regression itself: headers must not survive into the marshalled result
func TestWithoutHeadersKeepsMarshalledRowSmall(t *testing.T) {
	record := dataRecord(t, `{"url":"/a.js","headers":{"Cookie":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}`)

	row := map[string]any{"req": withoutHeaders(record.Get("req_json"))}

	encoded, err := json.Marshal(row)
	if err != nil {
		t.Fatalf("json.Marshal() = %v", err)
	}

	if strings.Contains(string(encoded), "headers") || strings.Contains(string(encoded), "Cookie") {
		t.Fatalf("marshalled row still carries the headers: %s", encoded)
	}
}

func TestWithoutHeadersPassesThroughNonObjects(t *testing.T) {
	scenarios := []struct {
		name  string
		value any
	}{
		{"not a json column", "plain string"},
		{"json null", pbtypes.JsonRaw([]byte("null"))},
		{"json array", pbtypes.JsonRaw([]byte(`["a"]`))},
		{"invalid json", pbtypes.JsonRaw([]byte("{oops"))},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			if got := withoutHeaders(s.value); !reflect.DeepEqual(got, s.value) {
				t.Fatalf("withoutHeaders(%v) = %v, want it returned untouched", s.value, got)
			}
		})
	}
}

// --- bug: the _hosts search filter used || which the grammar does not accept ---

func TestHostSearchFilterIsResolvable(t *testing.T) {
	filter, params := hostSearchFilter("tiktokcdn")

	resolver := search.NewSimpleFieldResolver("host", "title", "domain")
	if _, err := search.FilterData(filter).BuildExpr(resolver, params); err != nil {
		t.Fatalf("BuildExpr(%q) = %v, want no error", filter, err)
	}
}

// The grammar in glitchedgitz/dadql only knows AND/OR/NOT, so a C style operator
// anywhere in the filter fails to parse before any field is even resolved.
func TestHostSearchFilterAvoidsCStyleOperators(t *testing.T) {
	filter, _ := hostSearchFilter("tiktokcdn")

	if strings.Contains(filter, "||") || strings.Contains(filter, "&&") {
		t.Fatalf("hostSearchFilter() = %q, must join with the OR/AND keywords", filter)
	}

	// this is what the filter used to look like and why it never returned a host
	resolver := search.NewSimpleFieldResolver("host", "title", "domain")
	broken := "host ~ 'tiktokcdn' || title ~ 'tiktokcdn'"
	if _, err := search.FilterData(broken).BuildExpr(resolver); err == nil {
		t.Fatalf("expected %q to be unparsable", broken)
	}
}

func TestHostSearchFilterEmptySearch(t *testing.T) {
	scenarios := []string{"", "   "}

	for _, s := range scenarios {
		if filter, _ := hostSearchFilter(s); filter != "" {
			t.Fatalf("hostSearchFilter(%q) = %q, want empty so the caller lists every host", s, filter)
		}
	}
}

// The search term is bound as a parameter, so quotes in it cannot break the filter.
func TestHostSearchFilterBindsSearchTerm(t *testing.T) {
	filter, params := hostSearchFilter(`o'brien"`)

	if strings.Contains(filter, "o'brien") {
		t.Fatalf("hostSearchFilter() = %q, want the term bound as a param", filter)
	}

	if got := params["search"]; got != `o'brien"` {
		t.Fatalf("params[search] = %v, want %q", got, `o'brien"`)
	}

	resolver := search.NewSimpleFieldResolver("host", "title", "domain")
	if _, err := search.FilterData(filter).BuildExpr(resolver, params); err != nil {
		t.Fatalf("BuildExpr(%q) = %v, want no error", filter, err)
	}
}
