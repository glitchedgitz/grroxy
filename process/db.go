package process

import (
	"github.com/glitchedgitz/grroxy-db/utils"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/models"
)

func RegisterInDB(app *pocketbase.PocketBase, input, data any, state string) string {
	collection, err := app.Dao().FindCollectionByNameOrId("_processes")
	utils.CheckErr("[RunningCommand][FindCollection]:", err)

	record := models.NewRecord(collection)

	id := utils.RandomString(15)

	record.Set("id", id)
	record.Set("name", "name") // Use command as name
	record.Set("input", input) // Store the input data
	record.Set("data", data)   // Store full command data
	record.Set("state", state)
	record.Set("type", "type") // Store whether it saves to file or collection

	err = app.Dao().SaveRecord(record)
	utils.CheckErr("[RegisterProcessInDB][SaveRecord]", err)
	return id
}

func SetState(app *pocketbase.PocketBase, id, state string) {
	record, err := app.Dao().FindRecordById("_processes", id)
	utils.CheckErr("", err)

	record.Set("state", state)

	err = app.Dao().SaveRecord(record)
	utils.CheckErr("[RegisterProcessInDB][SaveRecord]", err)
} 