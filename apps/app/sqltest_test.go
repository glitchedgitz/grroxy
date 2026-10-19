package app

import (
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// Values must come back with the type SQLite stored them with, not as the
// {"String": "1", "Valid": true} wrappers a NullStringMap scan produces.
func TestSQLRowsToJSONKeepsStoredTypes(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows, err := db.Query(`SELECT 5001 AS "index", 1.5 AS ratio, 'proxy/1' AS generated_by,
		NULL AS extra, '{"a":1}' AS req_json, x'6869' AS blob`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	out, err := sqlRowsToJSON(rows)
	if err != nil {
		t.Fatal(err)
	}

	var got []map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output is not a JSON array: %v\n%s", err, out)
	}
	if len(got) != 1 {
		t.Fatalf("got %d rows, want 1", len(got))
	}

	row := got[0]
	want := map[string]any{
		"index":        float64(5001),
		"ratio":        1.5,
		"generated_by": "proxy/1",
		"extra":        nil,
		"req_json":     `{"a":1}`,
		"blob":         "hi",
	}
	for k, v := range want {
		if row[k] != v {
			t.Errorf("%s = %#v, want %#v", k, row[k], v)
		}
	}

	// columns stay in query order
	if strings.Index(out, `"index"`) > strings.Index(out, `"blob"`) {
		t.Errorf("columns out of order:\n%s", out)
	}
}

func TestSQLRowsToJSONEmpty(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows, err := db.Query(`SELECT 1 WHERE 0`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	out, err := sqlRowsToJSON(rows)
	if err != nil {
		t.Fatal(err)
	}
	if out != "[]" {
		t.Fatalf("empty result = %q, want []", out)
	}
}
