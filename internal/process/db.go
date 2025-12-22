package process

import (
	"log"

	"github.com/glitchedgitz/grroxy-db/internal/utils"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/models"
)

func RegisterInDB(app *pocketbase.PocketBase, input, data any, name, typz, state string) string {

	collection, err := app.Dao().FindCollectionByNameOrId("_processes")
	utils.CheckErr("[RunningCommand][FindCollection]:", err)

	record := models.NewRecord(collection)

	id := utils.RandomString(15)

	log.Println("id", id)
	record.Set("id", id)
	log.Println("name", name)
	record.Set("name", name) // Use command as name
	log.Println("input", input)
	record.Set("input", map[string]interface{}{
		"command": input,
	}) // Store the input data
	log.Println("data", data)
	record.Set("data", data) // Store full command data
	log.Println("state", state)
	record.Set("state", state)
	log.Println("typz", typz)
	record.Set("type", typz) // Store whether it saves to file or collection

	err = app.Dao().SaveRecord(record)
	utils.CheckErr("[RegisterProcessInDB][SaveRecord]", err)
	return id
}

func GetProcess(app *pocketbase.PocketBase, id string) (*models.Record, error) {
	record, err := app.Dao().FindRecordById("_processes", id)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func SetState(app *pocketbase.PocketBase, id, state string) {
	record, err := app.Dao().FindRecordById("_processes", id)
	utils.CheckErr("", err)

	record.Set("state", state)

	err = app.Dao().SaveRecord(record)
	utils.CheckErr("[RegisterProcessInDB][SaveRecord]", err)
}
