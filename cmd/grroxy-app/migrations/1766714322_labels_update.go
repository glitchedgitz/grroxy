package migrations

import (
	"log"

	"github.com/glitchedgitz/pocketbase/daos"
	m "github.com/glitchedgitz/pocketbase/migrations"
	"github.com/glitchedgitz/pocketbase/models/schema"
	"github.com/pocketbase/dbx"
)

func init() {
	m.Register(func(db dbx.Builder) error {
		dao := daos.New(db)

		// Find the _hosts collection
		collection, err := dao.FindCollectionByNameOrId("_hosts")
		if err != nil {
			log.Printf("[migration][hosts] Error finding _hosts collection: %v\n", err)
			return err
		}

		// Create a map of existing field names for quick lookup
		existingFields := make(map[string]bool)
		for _, field := range collection.Schema.Fields() {
			existingFields[field.Name] = true
		}

		// Add labels field if it doesn't exist
		if !existingFields["labels"] {
			log.Println("[migration][hosts] Adding field: labels")
			collection.Schema.AddField(&schema.SchemaField{
				Name: "labels",
				Type: schema.FieldTypeRelation,
				Options: &schema.RelationOptions{
					CollectionId: "_labels",
				},
			})
		}

		// Add notes field if it doesn't exist
		if !existingFields["notes"] {
			log.Println("[migration][hosts] Adding field: notes")
			collection.Schema.AddField(&schema.SchemaField{
				Name: "notes",
				Type: schema.FieldTypeJson,
				Options: &schema.JsonOptions{
					MaxSize: 1000000,
				},
			})
		}

		// Save the updated collection
		if err := dao.SaveCollection(collection); err != nil {
			log.Printf("[migration][hosts] Error saving _hosts collection: %v\n", err)
			return err
		}

		log.Println("[migration][hosts] Successfully updated _hosts collection schema")

		// Find the _labels collection
		labelsCollection, err := dao.FindCollectionByNameOrId("_labels")
		if err != nil {
			log.Printf("[migration][labels] Error finding _labels collection: %v\n", err)
			return err
		}

		// Create a map of existing field names for quick lookup
		existingLabelsFields := make(map[string]bool)
		for _, field := range labelsCollection.Schema.Fields() {
			existingLabelsFields[field.Name] = true
		}

		// Add applied_for field if it doesn't exist
		if !existingLabelsFields["applied_for"] {
			log.Println("[migration][labels] Adding field: applied_for")
			labelsCollection.Schema.AddField(&schema.SchemaField{
				Name: "applied_for",
				Type: schema.FieldTypeJson,
				Options: &schema.JsonOptions{
					MaxSize: 100000,
				},
			})
		}

		// Save the updated collection
		if err := dao.SaveCollection(labelsCollection); err != nil {
			log.Printf("[migration][labels] Error saving _labels collection: %v\n", err)
			return err
		}

		log.Println("[migration][labels] Successfully updated _labels collection schema")
		return nil
	}, func(db dbx.Builder) error {
		// Rollback: Remove the fields that were added
		dao := daos.New(db)

		collection, err := dao.FindCollectionByNameOrId("_hosts")
		if err != nil {
			// Collection doesn't exist, nothing to rollback
			return nil
		}

		currentSchema := collection.Schema

		// Remove labels field if it exists
		if field := currentSchema.GetFieldByName("labels"); field != nil {
			log.Println("[migration][hosts] Removing field: labels")
			currentSchema.RemoveField(field.Id)
		}

		// Remove notes field if it exists
		if field := currentSchema.GetFieldByName("notes"); field != nil {
			log.Println("[migration][hosts] Removing field: notes")
			currentSchema.RemoveField(field.Id)
		}

		// Save the updated collection
		if err := dao.SaveCollection(collection); err != nil {
			log.Printf("[migration][hosts] Error saving _hosts collection during rollback: %v\n", err)
			return err
		}

		log.Println("[migration][hosts] Rollback completed")

		// Rollback _labels collection
		labelsCollection, err := dao.FindCollectionByNameOrId("_labels")
		if err != nil {
			// Collection doesn't exist, nothing to rollback
			return nil
		}

		// Remove applied_for field if it exists
		if field := labelsCollection.Schema.GetFieldByName("applied_for"); field != nil {
			log.Println("[migration][labels] Removing field: applied_for")
			labelsCollection.Schema.RemoveField(field.Id)
		}

		// Save the updated collection
		if err := dao.SaveCollection(labelsCollection); err != nil {
			log.Printf("[migration][labels] Error saving _labels collection during rollback: %v\n", err)
			return err
		}

		log.Println("[migration][labels] Rollback completed")
		return nil
	})
}
