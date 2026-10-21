package migrations_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/glitchedgitz/pocketbase/daos"
	"github.com/glitchedgitz/pocketbase/models"
	"github.com/glitchedgitz/pocketbase/models/schema"
	"github.com/pocketbase/dbx"
)

const staticMigration = "1792561300_is_static.go"

// the "No Media/CSS" filter as the UI seeded it into projects
const storedNoMedia = `# include anyway if request has params
has_params = true

# ignore these
OR NOT (

  # ignore css
    req.ext ~ 'css'   OR

  # ignore images
    req.ext ~ 'jpg'   OR   req.ext ~ 'jpeg' OR
    req.ext ~ 'gif'   OR   req.ext ~ 'png'  OR
    req.ext ~ 'png'   OR   req.ext ~ 'webp' OR
    req.ext ~ 'ico'   OR   req.ext ~ 'svg'  OR

  # ignore fonts
    req.ext ~ 'ttf'   OR   req.ext ~ 'otf'   OR
    req.ext ~ 'woff'  OR   req.ext ~ 'woff2' OR
    req.ext ~ 'eot'   OR   req.ext ~ 'dfont' OR
    req.ext ~ 'pfa'   OR   req.ext ~ 'pfb'   OR
    req.ext ~ 'fon'   OR

  # ignore videos
    req.ext ~ 'mp4'   OR   req.ext ~ 'avi' OR
    req.ext ~ 'mkv'   OR   req.ext ~ 'mov' OR
    req.ext ~ 'webm'  OR   req.ext ~ 'mp3' OR
    req.ext ~ 'wav'
)

`

const editedNoMedia = "has_params = true OR NOT (req.ext ~ 'css')"

type staticRow struct {
	ext       string
	hasParams bool
	static    bool
}

var staticRows = map[string]staticRow{
	"______________1": {".css", false, true},
	"______________2": {".PNG", true, true}, // static, but kept by the filter: it has params
	"______________3": {".js", false, false},
	"______________4": {"", false, false},
	"______________5": {".scss", false, true},
	"______________6": {".woff2", false, true},
	"______________7": {".json", true, false},
}

func filterIDs(t *testing.T, dao *daos.Dao, filter string) []string {
	t.Helper()

	records, err := dao.FindRecordsByFilter("_data", "("+filter+") AND proxy = '_____________17'", "", 0, 0)
	if err != nil {
		t.Fatalf("filter %q: %v", filter, err)
	}
	ids := []string{}
	for _, r := range records {
		ids = append(ids, r.Id)
	}
	slices.Sort(ids)
	return ids
}

func indexSQL(t *testing.T, db *dbx.DB, name string) string {
	t.Helper()

	var sql []string
	if err := db.NewQuery("SELECT sql FROM sqlite_master WHERE type = 'index' AND name = {:n}").Bind(dbx.Params{"n": name}).Column(&sql); err != nil {
		t.Fatal(err)
	}
	if len(sql) == 0 {
		return ""
	}
	// PocketBase writes one column per line
	return strings.Join(strings.Fields(sql[0]), " ")
}

func plan(t *testing.T, db *dbx.DB, query string) string {
	t.Helper()

	var rows []dbx.NullStringMap
	if err := db.NewQuery("EXPLAIN QUERY PLAN " + query).All(&rows); err != nil {
		t.Fatal(err)
	}
	var details []string
	for _, r := range rows {
		details = append(details, r["detail"].String)
	}
	return strings.Join(details, "; ")
}

type templateRow struct {
	ID       string `db:"id"`
	Template string `db:"template"`
}

func templateText(t *testing.T, db *dbx.DB, id string) string {
	t.Helper()

	var r templateRow
	if err := db.Select("id", "template").From("templates").Where(dbx.HashExp{"id": id}).One(&r); err != nil {
		t.Fatal(err)
	}
	return r.Template
}

// An existing project: rows are marked from _req.ext, the stored filter is
// rewritten and keeps matching the same rows, and the views' indexes serve it.
func TestIsStaticMigration(t *testing.T) {
	db := projectBefore(t, staticMigration)
	dao := daos.New(db)

	// the UI creates the global filters' collection
	templates := &models.Collection{
		Name: "templates",
		Type: models.CollectionTypeBase,
		Schema: schema.NewSchema(
			&schema.SchemaField{Name: "name", Type: schema.FieldTypeText},
			&schema.SchemaField{Name: "template", Type: schema.FieldTypeText},
		),
	}
	if err := dao.SaveCollection(templates); err != nil {
		t.Fatal(err)
	}
	for id, text := range map[string]string{
		"defaultnomedia1": strings.ReplaceAll(storedNoMedia, "\n", "\r\n"), // spacing doesn't count
		"editednomedia01": editedNoMedia,
	} {
		if _, err := db.Insert("templates", dbx.Params{"id": id, "name": "No Media/CSS", "template": text}).Execute(); err != nil {
			t.Fatal(err)
		}
	}

	i := 0
	for id, r := range staticRows {
		i++
		if _, err := db.Insert("_req", dbx.Params{"id": id, "ext": r.ext}).Execute(); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Insert("_data", dbx.Params{
			"id": id, "req": id, "index": i, "has_params": r.hasParams,
			"type": "proxy", "proxy": "_____________17",
		}).Execute(); err != nil {
			t.Fatal(err)
		}
	}

	before := filterIDs(t, dao, storedNoMedia)
	if len(before) != 4 { // 2, 3, 4, 7
		t.Fatalf("old filter matched %v, want the 4 non-static or with-params rows", before)
	}

	runner := runMigration(t, db, staticMigration)

	var got []struct {
		ID       string `db:"id"`
		IsStatic bool   `db:"is_static"`
	}
	if err := db.Select("id", "is_static").From("_data").All(&got); err != nil {
		t.Fatal(err)
	}
	for _, r := range got {
		if want := staticRows[r.ID].static; r.IsStatic != want {
			t.Errorf("row %s (ext %q) is_static = %v, want %v", r.ID, staticRows[r.ID].ext, r.IsStatic, want)
		}
	}

	rewritten := templateText(t, db, "defaultnomedia1")
	if strings.Contains(rewritten, "req.ext ~") || !strings.Contains(rewritten, "is_static = false") {
		t.Errorf("default template not rewritten:\n%s", rewritten)
	}
	if got := templateText(t, db, "editednomedia01"); got != editedNoMedia {
		t.Errorf("edited template changed to %q", got)
	}
	if after := filterIDs(t, dao, rewritten); !slices.Equal(after, before) {
		t.Errorf("new filter matches %v, the old one %v", after, before)
	}

	if sql := indexSQL(t, db, "idx_data_proxy"); !strings.Contains(sql, "`has_params`, `is_static`, `id`") {
		t.Errorf("idx_data_proxy = %s", sql)
	}
	if sql := indexSQL(t, db, "idx_data_chat"); !strings.Contains(sql, "`chat`, `type`, `index`") {
		t.Errorf("idx_data_chat = %s", sql)
	}
	for _, name := range []string{"idx_data_type", "idx_data_actor"} {
		if indexSQL(t, db, name) == "" {
			t.Errorf("%s is gone", name)
		}
	}
	if indexSQL(t, db, "idx_data_generated_by") != "" {
		t.Error("idx_data_generated_by still exists")
	}
	if indexSQL(t, db, "idx_data_index") == "" {
		t.Error("_data has no idx_data_index")
	}
	// All Requests: no origin filter, a prefetched page sorted by -index
	if p := plan(t, db, "SELECT [[id]], [[req_json]] FROM {{_data}} ORDER BY [[index]] DESC LIMIT 100 OFFSET 100"); !strings.Contains(p, "idx_data_index") || strings.Contains(p, "TEMP B-TREE") {
		t.Errorf("All Requests page plan = %s, want an ordered walk of idx_data_index", p)
	}

	noMedia := "([[has_params]] = 1 OR [[is_static]] = 0)"
	if p := plan(t, db, "SELECT COUNT(DISTINCT [[id]]) FROM {{_data}} WHERE "+noMedia+" AND [[proxy]] = 'p'"); !strings.Contains(p, "COVERING INDEX idx_data_proxy") {
		t.Errorf("History count plan = %s, want a covering scan of idx_data_proxy", p)
	}
	if p := plan(t, db, "SELECT * FROM {{_data}} WHERE "+noMedia+" AND [[proxy]] = 'p' ORDER BY [[index]] LIMIT 100"); !strings.Contains(p, "idx_data_proxy") || strings.Contains(p, "TEMP B-TREE") {
		t.Errorf("History page plan = %s, want an ordered walk of idx_data_proxy", p)
	}
	if p := plan(t, db, "SELECT COUNT(DISTINCT [[id]]) FROM {{_data}} WHERE [[chat]] = 'c' AND [[type]] = 'repeater'"); !strings.Contains(p, "COVERING INDEX idx_data_chat (chat=? AND type=?)") {
		t.Errorf("Sent tab count plan = %s, want a covering scan of idx_data_chat on chat and type", p)
	}

	// the down migration puts back the old indexes, filter and generated_by index
	if _, err := runner.Down(1); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if columns(t, db, "_data")["is_static"] {
		t.Error("after rollback _data still has is_static")
	}
	if sql := indexSQL(t, db, "idx_data_proxy"); strings.Contains(sql, "is_static") || sql == "" {
		t.Errorf("after rollback idx_data_proxy = %q", sql)
	}
	if indexSQL(t, db, "idx_data_generated_by") == "" {
		t.Error("after rollback idx_data_generated_by is missing")
	}
	if indexSQL(t, db, "idx_data_index") != "" {
		t.Error("after rollback idx_data_index still exists")
	}
	if got := templateText(t, db, "defaultnomedia1"); !strings.Contains(got, "req.ext ~ 'woff2'") {
		t.Errorf("after rollback the default template = %q", got)
	}
}

// A project the UI hasn't opened yet has no templates collection; the UI
// seeds the new filter text itself.
func TestIsStaticMigrationNoTemplates(t *testing.T) {
	db := projectBefore(t, staticMigration)
	runMigration(t, db, staticMigration)

	if !columns(t, db, "_data")["is_static"] {
		t.Error("_data has no is_static column")
	}
}
