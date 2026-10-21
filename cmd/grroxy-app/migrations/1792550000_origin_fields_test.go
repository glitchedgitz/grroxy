package migrations_test

import (
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/glitchedgitz/grroxy/cmd/grroxy-app/migrations"
	"github.com/glitchedgitz/pocketbase/core"
	"github.com/glitchedgitz/pocketbase/daos"
	pbmigrations "github.com/glitchedgitz/pocketbase/migrations"
	"github.com/glitchedgitz/pocketbase/models/schema"
	"github.com/glitchedgitz/pocketbase/tools/migrate"
	"github.com/pocketbase/dbx"
)

const originMigration = "1792550000_origin_fields.go"

// projectBefore is an existing project: a temp app with every migration
// before the named one applied.
func projectBefore(t *testing.T, migration string) *dbx.DB {
	t.Helper()

	app := core.NewBaseApp(core.BaseAppConfig{DataDir: t.TempDir()})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { app.ResetBootstrapState() })

	var before migrate.MigrationsList
	for _, m := range pbmigrations.AppMigrations.Items() {
		if filepath.Base(m.File) < migration {
			before.Register(m.Up, m.Down, m.File)
		}
	}
	runner, err := migrate.NewRunner(app.DB(), before)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Up(); err != nil {
		t.Fatalf("earlier migrations: %v", err)
	}
	return app.DB()
}

// runMigration applies the named migration, and only it, and returns the
// runner, for Down.
func runMigration(t *testing.T, db *dbx.DB, migration string) *migrate.Runner {
	t.Helper()

	var upTo migrate.MigrationsList
	for _, m := range pbmigrations.AppMigrations.Items() {
		if filepath.Base(m.File) <= migration {
			upTo.Register(m.Up, m.Down, m.File)
		}
	}
	runner, err := migrate.NewRunner(db, upTo)
	if err != nil {
		t.Fatal(err)
	}
	applied, err := runner.Up()
	if err != nil {
		t.Fatalf("%s: %v", migration, err)
	}
	if len(applied) != 1 || filepath.Base(applied[0]) != migration {
		t.Fatalf("applied %v, want only %s", applied, migration)
	}
	return runner
}

func columns(t *testing.T, db *dbx.DB, table string) map[string]bool {
	t.Helper()

	var names []string
	if err := db.NewQuery("SELECT name FROM pragma_table_info({:t})").Bind(dbx.Params{"t": table}).Column(&names); err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, n := range names {
		got[n] = true
	}
	return got
}

func indexes(t *testing.T, db *dbx.DB, table string) map[string]bool {
	t.Helper()

	var names []string
	if err := db.NewQuery("SELECT name FROM pragma_index_list({:t})").Bind(dbx.Params{"t": table}).Column(&names); err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, n := range names {
		got[n] = true
	}
	return got
}

type originRow struct {
	ID          string `db:"id"`
	GeneratedBy string `db:"generated_by"`
	Type        string `db:"type"`
	Actor       string `db:"actor"`
	Proxy       string `db:"proxy"`
	Chat        string `db:"chat"`
}

// An existing project: the migration adds the fields, fills them from
// generated_by without changing it, and builds the indexes the views need.
func TestOriginFieldsMigration(t *testing.T) {
	db := projectBefore(t, originMigration)

	if columns(t, db, "_data")["actor"] {
		t.Fatal("_data has actor before the origin migration ran, internal/schemas must leave it out")
	}

	want := map[string]originRow{
		"______________1": {GeneratedBy: "proxy/_____________17", Type: "proxy", Actor: "user", Proxy: "_____________17"},
		"______________2": {GeneratedBy: "ai/x6s9c08ux5kd3k5/proxy/_____________17", Type: "proxy", Actor: "ai", Proxy: "_____________17", Chat: "x6s9c08ux5kd3k5"},
		"______________3": {GeneratedBy: "repeater/ai/mcp/h9num2fhd84sbur", Type: "repeater", Actor: "ai", Chat: "h9num2fhd84sbur"},
		"______________4": {GeneratedBy: "repeater/automate", Type: "automate", Actor: "user"},
		"______________5": {GeneratedBy: "fuzz_2766__________2766_5", Type: "fuzzer", Actor: "user"},
		"______________6": {GeneratedBy: "something-new"}, // unknown: left empty
		"______________7": {GeneratedBy: ""},
	}
	for id, r := range want {
		if _, err := db.Insert("_data", dbx.Params{"id": id, "generated_by": r.GeneratedBy}).Execute(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Insert("_websockets", dbx.Params{"id": "___________2.1", "generated_by": "ai/x6s9c08ux5kd3k5/proxy/_____________17"}).Execute(); err != nil {
		t.Fatal(err)
	}

	runner := runMigration(t, db, originMigration)

	for table, fields := range map[string][]string{
		"_data":       {"type", "actor", "proxy", "chat"},
		"_intercept":  {"type", "actor", "proxy", "chat"},
		"_websockets": {"actor", "proxy", "chat"},
	} {
		cols := columns(t, db, table)
		for _, f := range fields {
			if !cols[f] {
				t.Errorf("%s has no %s column", table, f)
			}
		}
	}

	var rows []originRow
	if err := db.Select("id", "generated_by", "type", "actor", "proxy", "chat").From("_data").All(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(want) {
		t.Fatalf("_data has %d rows, want %d", len(rows), len(want))
	}
	for _, got := range rows {
		w := want[got.ID]
		w.ID = got.ID
		if got != w {
			t.Errorf("_data row %s = %+v, want %+v", got.ID, got, w)
		}
	}

	var ws originRow
	if err := db.Select("id", "generated_by", "actor", "proxy", "chat").From("_websockets").One(&ws); err != nil {
		t.Fatal(err)
	}
	if ws.Actor != "ai" || ws.Proxy != "_____________17" || ws.Chat != "x6s9c08ux5kd3k5" {
		t.Errorf("_websockets row = %+v, want actor ai, proxy _____________17, chat x6s9c08ux5kd3k5", ws)
	}

	dataIdx := indexes(t, db, "_data")
	for _, idx := range []string{"idx_data_type", "idx_data_actor", "idx_data_proxy", "idx_data_chat", "idx_data_generated_by"} {
		if !dataIdx[idx] {
			t.Errorf("_data has no %s", idx)
		}
	}
	if !indexes(t, db, "_websockets")["idx_websockets_proxy"] {
		t.Error("_websockets has no idx_websockets_proxy")
	}

	// a proxy's History count is answered from the index alone
	var plan []dbx.NullStringMap
	if err := db.NewQuery("EXPLAIN QUERY PLAN SELECT COUNT(DISTINCT [[id]]) FROM {{_data}} WHERE [[proxy]] = {:p}").Bind(dbx.Params{"p": "_____________17"}).All(&plan); err != nil {
		t.Fatal(err)
	}
	if len(plan) == 0 || !strings.Contains(plan[0]["detail"].String, "COVERING INDEX idx_data_proxy") {
		t.Errorf("count by proxy plan = %v, want a covering scan of idx_data_proxy", plan)
	}

	// the down migration removes the fields and indexes, generated_by stays
	if _, err := runner.Down(1); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	cols := columns(t, db, "_data")
	if cols["actor"] || !cols["generated_by"] {
		t.Fatalf("after rollback _data columns = %v, want no actor and generated_by kept", cols)
	}
	dataIdx = indexes(t, db, "_data")
	if dataIdx["idx_data_proxy"] || !dataIdx["idx_data_generated_by"] {
		t.Fatalf("after rollback _data indexes = %v, want no idx_data_proxy and idx_data_generated_by kept", dataIdx)
	}
}

// A project that already has the fields, eg. from an earlier build that only
// added them: adding them is skipped, the rows are still filled and indexed.
func TestOriginFieldsMigrationFieldsExist(t *testing.T) {
	db := projectBefore(t, originMigration)
	dao := daos.New(db)

	collection, err := dao.FindCollectionByNameOrId("_data")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"type", "actor", "proxy", "chat"} {
		collection.Schema.AddField(&schema.SchemaField{Name: f, Type: schema.FieldTypeText})
	}
	if err := dao.SaveCollection(collection); err != nil {
		t.Fatal(err)
	}

	gb := "ai/x6s9c08ux5kd3k5/proxy/_____________17"
	if _, err := db.Insert("_data", dbx.Params{"id": "______________1", "generated_by": gb}).Execute(); err != nil {
		t.Fatal(err)
	}

	runMigration(t, db, originMigration)

	var row originRow
	if err := db.Select("id", "generated_by", "type", "actor", "proxy", "chat").From("_data").One(&row); err != nil {
		t.Fatal(err)
	}
	want := originRow{ID: "______________1", GeneratedBy: gb, Type: "proxy", Actor: "ai", Proxy: "_____________17", Chat: "x6s9c08ux5kd3k5"}
	if row != want {
		t.Errorf("_data row = %+v, want %+v", row, want)
	}
	if !indexes(t, db, "_data")["idx_data_proxy"] {
		t.Error("_data has no idx_data_proxy")
	}
	// the collections that did not have the fields still get them
	if !columns(t, db, "_websockets")["actor"] {
		t.Error("_websockets has no actor column")
	}
}
