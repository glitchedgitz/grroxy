package migrations

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/glitchedgitz/grroxy/internal/types"
	"github.com/glitchedgitz/pocketbase/daos"
	m "github.com/glitchedgitz/pocketbase/migrations"
	"github.com/glitchedgitz/pocketbase/models"
	"github.com/glitchedgitz/pocketbase/models/schema"
	"github.com/glitchedgitz/pocketbase/tools/dbutils"
	"github.com/pocketbase/dbx"
)

// is_static marks css, image, font and video requests (internal/types/static.go),
// so the "No Media/CSS" filter is a column check instead of a join to _req for
// every row, and the views' indexes can answer it.
//
// The field and the index changes go in one SaveCollection: PocketBase drops
// and rebuilds every listed index on each save, and each rebuild reads all of
// _data. Then only the static rows are updated.

// replaced by name; has_params and is_static let the filter be checked on the
// index entries, so the page is an ordered walk and the count never opens a row
var staticIndexes = map[string]struct{ up, down string }{
	"idx_data_proxy": {
		up:   "CREATE INDEX `idx_data_proxy` ON `_data` (`proxy`, `index`, `index_minor`, `has_params`, `is_static`, `id`)",
		down: "CREATE INDEX `idx_data_proxy` ON `_data` (`proxy`, `index`, `index_minor`, `id`)",
	},
	// type second: the AI panel's Sent tab is chat = ? AND type = 'repeater',
	// and most of a chat's rows are proxy captures it drove
	"idx_data_chat": {
		up:   "CREATE INDEX `idx_data_chat` ON `_data` (`chat`, `type`, `index`, `index_minor`, `has_params`, `is_static`, `id`)",
		down: "CREATE INDEX `idx_data_chat` ON `_data` (`chat`, `index`, `index_minor`, `id`)",
	},
	// views with no origin filter (All Requests, typed filters): sorted by
	// index they were a full read and sort of _data; new, so down removes it
	"idx_data_index": {
		up: dataIndexIndex,
	},
}

// also ensured by 1792570000_data_index, for projects that ran this migration
// before it listed the index
const dataIndexIndex = "CREATE INDEX `idx_data_index` ON `_data` (`index`, `index_minor`, `has_params`, `is_static`, `id`)"

// nothing reads generated_by by exact value since the origin fields, and a
// `~` filter can't use it; created as raw SQL, so PocketBase doesn't list it
const generatedByIndex = "CREATE INDEX IF NOT EXISTS idx_data_generated_by ON _data (generated_by)"

// The "No Media/CSS" global filter, as stored in each project's templates
// collection (seeded once by the UI from grroxytemplates.ts).
const noMediaName = "No Media/CSS"

const noMediaOld = `# include anyway if request has params
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

// keep in sync with templates.Ignore in cybernetic-ui grroxytemplates.ts
const noMediaNew = `# include anyway if request has params
has_params = true

# ignore css, images, fonts and videos: is_static is set from req.ext
# when the request is stored, see internal/types/static.go
OR is_static = false

`

func init() {
	m.Register(func(db dbx.Builder) error {
		dao := daos.New(db)

		collection, err := dao.FindCollectionByNameOrId("_data")
		if err != nil {
			log.Printf("[migration][is_static] Error finding _data collection: %v\n", err)
			return err
		}

		if collection.Schema.GetFieldByName("is_static") == nil {
			collection.Schema.AddField(&schema.SchemaField{
				Name: "is_static",
				Type: schema.FieldTypeBool,
			})
		}
		setIndexes(collection, func(d struct{ up, down string }) string { return d.up })

		start := time.Now()
		if err := dao.SaveCollection(collection); err != nil {
			log.Printf("[migration][is_static] Error saving _data collection: %v\n", err)
			return err
		}
		log.Printf("[migration][is_static] _data: field added, indexes rebuilt in %s\n", time.Since(start).Round(time.Millisecond))

		if err := backfillStatic(db); err != nil {
			return err
		}

		if _, err := db.NewQuery("DROP INDEX IF EXISTS idx_data_generated_by").Execute(); err != nil {
			log.Printf("[migration][is_static] Error dropping idx_data_generated_by: %v\n", err)
			return err
		}

		if err := replaceTemplate(db, noMediaOld, noMediaNew); err != nil {
			return err
		}

		log.Println("[migration][is_static] Successfully added is_static")
		return nil
	}, func(db dbx.Builder) error {
		dao := daos.New(db)

		collection, err := dao.FindCollectionByNameOrId("_data")
		if err != nil {
			return nil // nothing to roll back
		}

		setIndexes(collection, func(d struct{ up, down string }) string { return d.down })
		if f := collection.Schema.GetFieldByName("is_static"); f != nil {
			collection.Schema.RemoveField(f.Id)
		}
		if err := dao.SaveCollection(collection); err != nil {
			log.Printf("[migration][is_static] Error saving _data collection during rollback: %v\n", err)
			return err
		}

		if _, err := db.NewQuery(generatedByIndex).Execute(); err != nil {
			return err
		}
		if err := replaceTemplate(db, noMediaNew, noMediaOld); err != nil {
			return err
		}

		log.Println("[migration][is_static] Rollback completed")
		return nil
	})
}

// setIndexes swaps the definitions of the staticIndexes by name, adding any
// that the collection doesn't list; an empty definition removes the index.
func setIndexes(collection *models.Collection, pick func(struct{ up, down string }) string) {
	seen := map[string]bool{}
	kept := collection.Indexes[:0]
	for _, idx := range collection.Indexes {
		name := dbutils.ParseIndex(idx).IndexName
		if d, ok := staticIndexes[name]; ok {
			seen[name] = true
			if idx = pick(d); idx == "" {
				continue
			}
		}
		kept = append(kept, idx)
	}
	collection.Indexes = kept
	for name, d := range staticIndexes {
		if !seen[name] && pick(d) != "" {
			collection.Indexes = append(collection.Indexes, pick(d))
		}
	}
}

// backfillStatic marks the existing static rows from their _req.ext, the
// column the old filter read, with the same case-insensitive substring match.
func backfillStatic(db dbx.Builder) error {
	start := time.Now()

	likes := make([]string, len(types.StaticExtTerms))
	params := dbx.Params{}
	for i, term := range types.StaticExtTerms {
		p := fmt.Sprintf("t%d", i)
		likes[i] = "[[r.ext]] LIKE {:" + p + "}"
		params[p] = "%" + term + "%"
	}

	res, err := db.NewQuery(
		"UPDATE {{_data}} SET [[is_static]] = TRUE WHERE EXISTS (" +
			"SELECT 1 FROM {{_req}} r WHERE [[r.id]] = {{_data}}.[[req]] AND (" + strings.Join(likes, " OR ") + "))",
	).Bind(params).Execute()
	if err != nil {
		log.Printf("[migration][is_static] Error filling is_static: %v\n", err)
		return err
	}

	n, _ := res.RowsAffected()
	log.Printf("[migration][is_static] _data: marked %d static rows in %s\n", n, time.Since(start).Round(time.Millisecond))
	return nil
}

// replaceTemplate rewrites the stored "No Media/CSS" filter when it still
// reads `from`; a copy the user edited is left as it is.
func replaceTemplate(db dbx.Builder, from, to string) error {
	var exists int
	if err := db.NewQuery("SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'templates'").Row(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return nil // the UI seeds it later, from the new text
	}

	var rows []struct {
		ID       string `db:"id"`
		Template string `db:"template"`
	}
	if err := db.NewQuery("SELECT [[id]], [[template]] FROM {{templates}} WHERE [[name]] = {:name}").
		Bind(dbx.Params{"name": noMediaName}).All(&rows); err != nil {
		log.Printf("[migration][is_static] Error reading the %q template: %v\n", noMediaName, err)
		return err
	}

	for _, r := range rows {
		if strings.Join(strings.Fields(r.Template), " ") != strings.Join(strings.Fields(from), " ") {
			log.Printf("[migration][is_static] %q template %s was edited, left as it is\n", noMediaName, r.ID)
			continue
		}
		if _, err := db.Update("templates", dbx.Params{"template": to}, dbx.HashExp{"id": r.ID}).Execute(); err != nil {
			log.Printf("[migration][is_static] Error updating the %q template: %v\n", noMediaName, err)
			return err
		}
	}
	return nil
}
