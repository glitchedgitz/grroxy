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

		// Find the _processes collection
		collection, err := dao.FindCollectionByNameOrId("_processes")
		if err != nil {
			log.Printf("[migration][processes] Error finding _processes collection: %v\n", err)
			return err
		}

		// Create a map of existing field names for quick lookup
		existingFields := make(map[string]bool)
		for _, field := range collection.Schema.Fields() {
			existingFields[field.Name] = true
		}

		// Add parent_id field if it doesn't exist
		if !existingFields["parent_id"] {
			log.Println("[migration][processes] Adding field: parent_id")
			collection.Schema.AddField(&schema.SchemaField{
				Name: "parent_id",
				Type: schema.FieldTypeText,
			})
		}

		// Add generated_by field if it doesn't exist
		if !existingFields["generated_by"] {
			log.Println("[migration][processes] Adding field: generated_by")
			collection.Schema.AddField(&schema.SchemaField{
				Name: "generated_by",
				Type: schema.FieldTypeText,
			})
		}

		// Add created_by field if it doesn't exist
		if !existingFields["created_by"] {
			log.Println("[migration][processes] Adding field: created_by")
			collection.Schema.AddField(&schema.SchemaField{
				Name: "created_by",
				Type: schema.FieldTypeText,
			})
		}

		// Save the updated collection
		if err := dao.SaveCollection(collection); err != nil {
			log.Printf("[migration][processes] Error saving _processes collection: %v\n", err)
			return err
		}

		log.Println("[migration][processes] Successfully updated _processes collection schema")
		return nil
	}, func(db dbx.Builder) error {
		// Rollback: Remove the fields that were added
		dao := daos.New(db)

		collection, err := dao.FindCollectionByNameOrId("_processes")
		if err != nil {
			// Collection doesn't exist, nothing to rollback
			return nil
		}

		currentSchema := collection.Schema

		// Remove parent_id field if it exists
		if field := currentSchema.GetFieldByName("parent_id"); field != nil {
			log.Println("[migration][processes] Removing field: parent_id")
			currentSchema.RemoveField(field.Id)
		}

		// Remove generated_by field if it exists
		if field := currentSchema.GetFieldByName("generated_by"); field != nil {
			log.Println("[migration][processes] Removing field: generated_by")
			currentSchema.RemoveField(field.Id)
		}

		// Remove created_by field if it exists
		if field := currentSchema.GetFieldByName("created_by"); field != nil {
			log.Println("[migration][processes] Removing field: created_by")
			currentSchema.RemoveField(field.Id)
		}

		// Save the updated collection
		if err := dao.SaveCollection(collection); err != nil {
			log.Printf("[migration][processes] Error saving _processes collection during rollback: %v\n", err)
			return err
		}

		log.Println("[migration][processes] Rollback completed")
		return nil
	})
}
