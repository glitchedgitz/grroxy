package migrations

import (
	"log"
	"slices"
	"time"

	"github.com/glitchedgitz/grroxy/internal/types"
	"github.com/glitchedgitz/pocketbase/daos"
	m "github.com/glitchedgitz/pocketbase/migrations"
	"github.com/glitchedgitz/pocketbase/models/schema"
	"github.com/pocketbase/dbx"
)

// generated_by split into fields, see docs/generated_by.md.
//
// For each collection: add the fields, fill them on the existing rows from
// generated_by (which is left as it is), then add the indexes.
type originCollection struct {
	fields  []string
	indexes []string
}

var originCollections = map[string]originCollection{
	"_data": {
		fields: []string{"type", "actor", "proxy", "chat"},
		// each ends in id, so PocketBase's COUNT(DISTINCT id) is answered from the index
		indexes: []string{
			"CREATE INDEX `idx_data_type` ON `_data` (`type`, `index`, `index_minor`, `id`)",
			"CREATE INDEX `idx_data_actor` ON `_data` (`actor`, `index`, `index_minor`, `id`)",
			"CREATE INDEX `idx_data_proxy` ON `_data` (`proxy`, `index`, `index_minor`, `id`)",
			"CREATE INDEX `idx_data_chat` ON `_data` (`chat`, `index`, `index_minor`, `id`)",
		},
	},
	// wiped at startup and tiny, no indexes needed
	"_intercept": {
		fields: []string{"type", "actor", "proxy", "chat"},
	},
	"_websockets": {
		fields: []string{"actor", "proxy", "chat"}, // "type" is already the frame type
		indexes: []string{
			"CREATE INDEX `idx_websockets_proxy` ON `_websockets` (`proxy`, `id`)",
		},
	},
}

func init() {
	m.Register(func(db dbx.Builder) error {
		dao := daos.New(db)

		for name, oc := range originCollections {
			collection, err := dao.FindCollectionByNameOrId(name)
			if err != nil {
				log.Printf("[migration][origin] Error finding %s collection: %v\n", name, err)
				return err
			}

			added := false
			for _, field := range oc.fields {
				if collection.Schema.GetFieldByName(field) != nil {
					log.Printf("[migration][origin] Field %s.%s already exists, skipping\n", name, field)
					continue
				}
				log.Printf("[migration][origin] Adding field: %s.%s\n", name, field)
				collection.Schema.AddField(&schema.SchemaField{
					Name: field,
					Type: schema.FieldTypeText,
				})
				added = true
			}
			if added {
				if err := dao.SaveCollection(collection); err != nil {
					log.Printf("[migration][origin] Error saving %s collection: %v\n", name, err)
					return err
				}
			}

			if err := backfillOrigin(db, name, oc.fields); err != nil {
				return err
			}

			// after the backfill, so the indexes are built once over filled rows
			if len(oc.indexes) > 0 {
				start := time.Now()
				collection.Indexes = append(collection.Indexes, oc.indexes...)
				if err := dao.SaveCollection(collection); err != nil {
					log.Printf("[migration][origin] Error creating %s indexes: %v\n", name, err)
					return err
				}
				log.Printf("[migration][origin] %s: indexes built in %s\n", name, time.Since(start).Round(time.Millisecond))
			}
		}

		log.Println("[migration][origin] Successfully added origin fields")
		return nil
	}, func(db dbx.Builder) error {
		// Rollback: Remove the indexes and fields that were added
		dao := daos.New(db)

		for name, oc := range originCollections {
			collection, err := dao.FindCollectionByNameOrId(name)
			if err != nil {
				// Collection doesn't exist, nothing to rollback
				continue
			}

			collection.Indexes = slices.DeleteFunc(collection.Indexes, func(idx string) bool {
				return slices.Contains(oc.indexes, idx)
			})
			for _, field := range oc.fields {
				if f := collection.Schema.GetFieldByName(field); f != nil {
					log.Printf("[migration][origin] Removing field: %s.%s\n", name, field)
					collection.Schema.RemoveField(f.Id)
				}
			}

			if err := dao.SaveCollection(collection); err != nil {
				log.Printf("[migration][origin] Error saving %s collection during rollback: %v\n", name, err)
				return err
			}
		}

		log.Println("[migration][origin] Rollback completed")
		return nil
	})
}

// backfillOrigin fills the fields on the existing rows, one distinct
// generated_by value at a time, so on _data each UPDATE is served by
// idx_data_generated_by. Unknown values are logged and left empty.
func backfillOrigin(db dbx.Builder, table string, fields []string) error {
	start := time.Now()

	var values []string
	err := db.NewQuery("SELECT DISTINCT [[generated_by]] FROM {{" + table + "}} WHERE [[generated_by]] != ''").Column(&values)
	if err != nil {
		log.Printf("[migration][origin] Error reading %s generated_by values: %v\n", table, err)
		return err
	}

	var total int64
	for _, gb := range values {
		origin, ok := types.ParseGeneratedBy(gb)
		if !ok {
			log.Printf("[migration][origin] %s: unknown generated_by %q, fields left empty\n", table, gb)
			continue
		}

		all := origin.Fields()
		set := dbx.Params{}
		for _, f := range fields {
			set[f] = all[f]
		}

		res, err := db.Update(table, set, dbx.HashExp{"generated_by": gb}).Execute()
		if err != nil {
			log.Printf("[migration][origin] Error filling %s rows of %q: %v\n", table, gb, err)
			return err
		}
		n, _ := res.RowsAffected()
		total += n
	}

	log.Printf("[migration][origin] %s: filled %d rows from %d generated_by values in %s\n", table, total, len(values), time.Since(start).Round(time.Millisecond))
	return nil
}
