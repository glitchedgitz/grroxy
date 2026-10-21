package migrations_test

import (
	"slices"
	"testing"

	"github.com/glitchedgitz/pocketbase/daos"
	"github.com/glitchedgitz/pocketbase/tools/dbutils"
)

const dataIndexMigration = "1792570000_data_index.go"

func listedIndexes(t *testing.T, dao *daos.Dao) []string {
	t.Helper()

	collection, err := dao.FindCollectionByNameOrId("_data")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, idx := range collection.Indexes {
		names = append(names, dbutils.ParseIndex(idx).IndexName)
	}
	return names
}

// A project that ran 1792561300_is_static before it listed idx_data_index
// gets the index.
func TestDataIndexMigrationAddsMissing(t *testing.T) {
	db := projectBefore(t, dataIndexMigration)
	dao := daos.New(db)

	collection, err := dao.FindCollectionByNameOrId("_data")
	if err != nil {
		t.Fatal(err)
	}
	collection.Indexes = slices.DeleteFunc(collection.Indexes, func(idx string) bool {
		return dbutils.ParseIndex(idx).IndexName == "idx_data_index"
	})
	if err := dao.SaveCollection(collection); err != nil {
		t.Fatal(err)
	}
	if indexSQL(t, db, "idx_data_index") != "" {
		t.Fatal("idx_data_index still exists after removing it")
	}

	runMigration(t, db, dataIndexMigration)

	if indexSQL(t, db, "idx_data_index") == "" {
		t.Error("_data has no idx_data_index")
	}
	if !slices.Contains(listedIndexes(t, dao), "idx_data_index") {
		t.Error("idx_data_index is not in _data's collection.Indexes")
	}
}

// Where 1792561300_is_static already built it, nothing changes.
func TestDataIndexMigrationAlreadyThere(t *testing.T) {
	db := projectBefore(t, dataIndexMigration)
	dao := daos.New(db)
	before := listedIndexes(t, dao)

	runMigration(t, db, dataIndexMigration)

	if after := listedIndexes(t, dao); !slices.Equal(after, before) {
		t.Errorf("_data indexes changed from %v to %v", before, after)
	}
}
