package migrations

import (
	"log"
	"time"

	"github.com/glitchedgitz/pocketbase/daos"
	m "github.com/glitchedgitz/pocketbase/migrations"
	"github.com/glitchedgitz/pocketbase/tools/dbutils"
	"github.com/pocketbase/dbx"
)

// idx_data_index for projects that ran 1792561300_is_static before it listed
// the index. Everywhere else that migration already built it and this does
// nothing, so _data's indexes aren't rebuilt a second time.
func init() {
	m.Register(func(db dbx.Builder) error {
		dao := daos.New(db)

		collection, err := dao.FindCollectionByNameOrId("_data")
		if err != nil {
			log.Printf("[migration][data_index] Error finding _data collection: %v\n", err)
			return err
		}

		for _, idx := range collection.Indexes {
			if dbutils.ParseIndex(idx).IndexName == "idx_data_index" {
				return nil
			}
		}

		start := time.Now()
		collection.Indexes = append(collection.Indexes, dataIndexIndex)
		if err := dao.SaveCollection(collection); err != nil {
			log.Printf("[migration][data_index] Error creating idx_data_index: %v\n", err)
			return err
		}
		log.Printf("[migration][data_index] _data: indexes rebuilt in %s\n", time.Since(start).Round(time.Millisecond))
		return nil
	}, func(db dbx.Builder) error {
		// the index belongs to 1792561300_is_static, whose down removes it
		return nil
	})
}
