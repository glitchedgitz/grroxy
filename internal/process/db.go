package process

import (
	"log"

	"github.com/glitchedgitz/grroxy/internal/utils"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/models"
)

// ProcessInput represents the input field structure for a process
type ProcessInput struct {
	Completed int    `json:"completed"`
	Total     int    `json:"total"`
	Progress  int    `json:"progress"`
	Message   string `json:"message"`
	Error     string `json:"error"`
}

// ProgressUpdate represents progress information for updating a process
type ProgressUpdate struct {
	Completed int
	Total     int
	Message   string
	Error     string
	State     string
}

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

// CreateProcess creates a new process with progress tracking
func CreateProcess(app *pocketbase.PocketBase, name, description, typz, state string, data map[string]any, customID string) string {
	collection, err := app.Dao().FindCollectionByNameOrId("_processes")
	utils.CheckErr("[CreateProcess][FindCollection]:", err)

	record := models.NewRecord(collection)

	id := customID
	if id == "" {
		id = utils.RandomString(15)
	}

	// Set defaults
	if state == "" {
		state = "running"
	}
	if data == nil {
		data = make(map[string]any)
	}

	record.Set("id", id)
	record.Set("name", name)
	record.Set("description", description)
	record.Set("type", typz)
	record.Set("state", state)
	record.Set("data", data)
	record.Set("input", map[string]interface{}{
		"completed": 0,
		"total":     100,
		"progress":  0,
		"message":   "Starting...",
		"error":     "",
	})
	record.Set("output", map[string]interface{}{})

	err = app.Dao().SaveRecord(record)
	utils.CheckErr("[CreateProcess][SaveRecord]", err)
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

// UpdateProgress updates the progress of a process
func UpdateProgress(app *pocketbase.PocketBase, id string, progress ProgressUpdate) {
	record, err := app.Dao().FindRecordById("_processes", id)
	utils.CheckErr("[UpdateProgress][FindRecord]", err)

	// Calculate progress percentage
	percentage := 0
	if progress.Total > 0 {
		percentage = (progress.Completed * 100) / progress.Total
	}

	record.Set("input", map[string]interface{}{
		"completed": progress.Completed,
		"total":     progress.Total,
		"progress":  percentage,
		"message":   progress.Message,
		"error":     progress.Error,
	})

	// Update state if provided
	if progress.State != "" {
		record.Set("state", progress.State)
	}

	err = app.Dao().SaveRecord(record)
	utils.CheckErr("[UpdateProgress][SaveRecord]", err)
}

// CompleteProcess marks a process as completed
func CompleteProcess(app *pocketbase.PocketBase, id string, message string) {
	if message == "" {
		message = "Completed"
	}

	UpdateProgress(app, id, ProgressUpdate{
		Completed: 100,
		Total:     100,
		Message:   message,
		State:     "completed",
	})
}

// FailProcess marks a process as failed with an error message
func FailProcess(app *pocketbase.PocketBase, id string, errorMsg string) {
	UpdateProgress(app, id, ProgressUpdate{
		Completed: 0,
		Total:     100,
		Message:   "Failed",
		Error:     errorMsg,
		State:     "failed",
	})
}
