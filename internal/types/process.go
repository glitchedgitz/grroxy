package types

import (
	"encoding/json"
	"fmt"

	"github.com/glitchedgitz/grroxy/internal/utils"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/models"
)

type RunCommandData struct {
	ID         string `db:"id,omitempty" json:"id,omitempty"`
	SaveTo     string `db:"save_to,omitempty" json:"save_to,omitempty"`
	Data       string `db:"data,omitempty" json:"data,omitempty"`
	Command    string `db:"command,omitempty" json:"command,omitempty"`
	Collection string `db:"collection,omitempty" json:"collection,omitempty"`
	Filename   string `db:"filename,omitempty" json:"filename,omitempty"`
}

func (d *RunCommandData) Scan(value interface{}) error {
	if value == nil {
		*d = RunCommandData{}
		return nil
	}
	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, d)
	case string:
		return json.Unmarshal([]byte(v), d)
	default:
		return fmt.Errorf("unsupported type: %T", v)
	}
}

func RegisterProcessInDB(app *pocketbase.PocketBase, input, data any, state string) string {
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

func SetProcess(app *pocketbase.PocketBase, id, state string) {
	record, err := app.Dao().FindRecordById("_processes", id)
	utils.CheckErr("", err)

	record.Set("state", state)

	err = app.Dao().SaveRecord(record)
	utils.CheckErr("[RegisterProcessInDB][SaveRecord]", err)
}
