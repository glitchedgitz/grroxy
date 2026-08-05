package app

import (
	"reflect"
	"strings"
	"testing"

	"github.com/glitchedgitz/pocketbase/tools/search"
)

// labelsResolver accepts the fields the "_labels" collection exposes.
func labelsResolver() search.FieldResolver {
	return search.NewSimpleFieldResolver("id", "created", "updated", "name", "color", "icon", "type")
}

// rowsResolver accepts the fields a "_data" row filter may name: its own
// columns, anything reached through the req/resp relations, and the labels
// hanging off the "attached" relation.
func rowsResolver() search.FieldResolver {
	return search.NewSimpleFieldResolver(
		"id", "created", "updated",
		"index", "index_minor", "host", "port", "http",
		"has_params", "has_resp", "is_https", "generated_by",
		`^req\.[\w\.]+$`, `^resp\.[\w\.]+$`,
		`^attached\.[\w\.]+$`,
	)
}

// --- label listing ---

func TestLabelListFilterIsResolvable(t *testing.T) {
	filter, params := labelListFilter("sql", "custom")

	if _, err := search.FilterData(filter).BuildExpr(labelsResolver(), params); err != nil {
		t.Fatalf("BuildExpr(%q) = %v, want no error", filter, err)
	}
}

// An empty filter is not a filter — FindRecordsByFilter refuses it, so the
// unnarrowed listing has to be recognisable by the caller.
func TestLabelListFilterEmptyNarrowing(t *testing.T) {
	scenarios := []struct{ search, labelType string }{
		{"", ""},
		{"   ", "  "},
	}

	for _, s := range scenarios {
		if filter, _ := labelListFilter(s.search, s.labelType); filter != "" {
			t.Fatalf("labelListFilter(%q, %q) = %q, want empty so the caller lists every label", s.search, s.labelType, filter)
		}
	}
}

func TestLabelListFilterNarrowsOnEitherField(t *testing.T) {
	scenarios := []struct {
		name              string
		search, labelType string
		want              string
	}{
		{"search only", "sql", "", "name ~ {:search}"},
		{"type only", "", "custom", "type = {:type}"},
		{"both", "sql", "custom", "name ~ {:search} AND type = {:type}"},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			filter, _ := labelListFilter(s.search, s.labelType)
			if filter != s.want {
				t.Fatalf("labelListFilter(%q, %q) = %q, want %q", s.search, s.labelType, filter, s.want)
			}
		})
	}
}

// The name is bound as a parameter, so quotes in it cannot break the filter.
func TestLabelListFilterBindsSearchTerm(t *testing.T) {
	filter, params := labelListFilter(`o'brien"`, "")

	if strings.Contains(filter, "o'brien") {
		t.Fatalf("labelListFilter() = %q, want the term bound as a param", filter)
	}

	if got := params["search"]; got != `o'brien"` {
		t.Fatalf("params[search] = %v, want %q", got, `o'brien"`)
	}

	if _, err := search.FilterData(filter).BuildExpr(labelsResolver(), params); err != nil {
		t.Fatalf("BuildExpr(%q) = %v, want no error", filter, err)
	}
}

// --- rows carrying a label ---

func TestLabelRowsFilterIsResolvable(t *testing.T) {
	scenarios := []struct {
		name        string
		host, extra string
	}{
		{"label only", "", ""},
		{"with schemed host", "https://example.com", ""},
		{"with bare host", "example.com", ""},
		{"with extra filter", "", "resp.status = 500"},
		{"with both", "https://example.com", "req.url ~ '.js' OR resp.status = 200"},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			filter, params := labelRowsFilter("abc123", s.host, s.extra)

			if _, err := search.FilterData(filter).BuildExpr(rowsResolver(), params); err != nil {
				t.Fatalf("BuildExpr(%q) = %v, want no error", filter, err)
			}
		})
	}
}

// A row carries several labels, so the label condition has to be an "any of"
// match — a plain "=" asks for every label on the row to be this one.
func TestLabelRowsFilterMatchesAnyLabel(t *testing.T) {
	filter, _ := labelRowsFilter("abc123", "", "")

	if !strings.HasPrefix(filter, "attached.labels.id ?= ") {
		t.Fatalf("labelRowsFilter() = %q, want it to match any of the row's labels", filter)
	}
}

// The label id is bound, never interpolated — it comes from a name the caller
// supplied.
func TestLabelRowsFilterBindsLabelID(t *testing.T) {
	filter, params := labelRowsFilter(`abc' OR 1=1 --`, "", "")

	if strings.Contains(filter, "1=1") {
		t.Fatalf("labelRowsFilter() = %q, want the id bound as a param", filter)
	}

	if got := params["labelID"]; got != `abc' OR 1=1 --` {
		t.Fatalf("params[labelID] = %v, want the id as given", got)
	}
}

// An OR inside the caller's filter must not widen the label condition into
// "this label, or anything matching the rest".
func TestLabelRowsFilterParenthesisesExtra(t *testing.T) {
	filter, params := labelRowsFilter("abc123", "", "resp.status = 500 OR resp.status = 200")

	if !strings.HasSuffix(filter, "(resp.status = 500 OR resp.status = 200)") {
		t.Fatalf("labelRowsFilter() = %q, want the extra filter parenthesised", filter)
	}

	if _, err := search.FilterData(filter).BuildExpr(rowsResolver(), params); err != nil {
		t.Fatalf("BuildExpr(%q) = %v, want no error", filter, err)
	}
}

// A host with a scheme is matched exactly, a bare one only as a substring —
// stored hosts carry the scheme.
func TestLabelRowsFilterHostMatching(t *testing.T) {
	exact, params := labelRowsFilter("abc123", "https://example.com/some/path", "")
	if !strings.Contains(exact, "host = {:host}") {
		t.Fatalf("labelRowsFilter() = %q, want an exact host match", exact)
	}
	if got := params["host"]; got != "https://example.com" {
		t.Fatalf("params[host] = %v, want the path trimmed off", got)
	}

	partial, params := labelRowsFilter("abc123", "example.com", "")
	if !strings.Contains(partial, "host ~ {:host}") {
		t.Fatalf("labelRowsFilter() = %q, want a substring host match", partial)
	}
	if got := params["host"]; got != "example.com" {
		t.Fatalf("params[host] = %v, want %q", got, "example.com")
	}
}

// --- attaching a label ---

// The endpoint has always taken a single "id" and the tools take "ids"; both
// have to reach the same rows.
func TestAttachTargetsAcceptsBothIDForms(t *testing.T) {
	scenarios := []struct {
		name string
		body AttachLabelRequest
		want []string
	}{
		{
			"legacy single id",
			AttachLabelRequest{ID: "476"},
			[]string{"____________476"},
		},
		{
			"ids list",
			AttachLabelRequest{RowTargets: RowTargets{IDs: []string{"476", "5.11"}}},
			[]string{"____________476", "___________5.11"},
		},
		{
			"both forms",
			AttachLabelRequest{RowTargets: RowTargets{IDs: []string{"5.11"}}, ID: "476"},
			[]string{"___________5.11", "____________476"},
		},
		{
			// the frontend sends the padded id, the AI sends the bare one
			"same row in both forms is attached once",
			AttachLabelRequest{RowTargets: RowTargets{IDs: []string{"476"}}, ID: "____________476"},
			[]string{"____________476"},
		},
	}

	backend := &Backend{}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			ids, _, err := backend.resolveExtractTargets(attachTargets(s.body))
			if err != nil {
				t.Fatalf("resolveExtractTargets() = %v, want no error", err)
			}

			if !reflect.DeepEqual(ids, s.want) {
				t.Fatalf("resolveExtractTargets() = %v, want %v", ids, s.want)
			}
		})
	}
}

// An attach with no row named at all has to fail rather than silently create a
// label attached to nothing.
func TestAttachTargetsWithoutAnyID(t *testing.T) {
	backend := &Backend{}

	if _, _, err := backend.resolveExtractTargets(attachTargets(AttachLabelRequest{Name: "sqli"})); err == nil {
		t.Fatal("resolveExtractTargets() = nil error, want one for an attach that names no row")
	}
}

// The grammar in glitchedgitz/dadql only knows AND/OR/NOT, so a C style operator
// anywhere in the filter fails to parse before any field is even resolved.
func TestLabelFiltersAvoidCStyleOperators(t *testing.T) {
	rows, _ := labelRowsFilter("abc123", "example.com", "resp.status = 200")
	labels, _ := labelListFilter("sql", "custom")

	for _, filter := range []string{rows, labels} {
		if strings.Contains(filter, "||") || strings.Contains(filter, "&&") {
			t.Fatalf("filter %q must join with the OR/AND keywords", filter)
		}
	}
}
