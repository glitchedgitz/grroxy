package app

import (
	"log"

	"github.com/glitchedgitz/pocketbase/models"
)

func (backend *Backend) GetRecord(collectionName string, filter string) (*models.Record, error) {
	r, err := backend.App.Dao().FindFirstRecordByFilter(collectionName, filter)
	return r, err
}

func (backend *Backend) SaveRecordToCollection(collectionName string, data map[string]any) (*models.Record, error) {

	log.Println("SaveRecordToCollection: ", collectionName, data)

	collection, err := backend.App.Dao().FindCollectionByNameOrId(collectionName)
	if err != nil {
		return nil, err
	}

	record := models.NewRecord(collection)

	for key, value := range data {
		record.Set(key, value)
	}

	err = backend.App.Dao().Save(record)

	if err != nil {
		return nil, err
	}

	return record, nil
}
