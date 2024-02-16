package endpoints

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"runtime"

	"github.com/glitchedgitz/grroxy-db/base"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
)

// loop over commandChannel
func (pocketbaseDB *DatabaseAPI) CommandManager() {
	// log.Println("[CommandManager Stared]")
	for c := range pocketbaseDB.CmdChannel {
		log.Println("Command received: ", c)
		if c.SaveTo == "collection" {
			pocketbaseDB.RunningCommandSaveToCollection(c.ID, c.Command, c.Collection)
		} else {
			pocketbaseDB.RunningCommand(c.ID, c.Command, c.Filename)
		}
	}
}

var process = struct {
	inqueue   string
	running   string
	completed string
	killed    string
}{
	inqueue:   "In Queue",
	running:   "Running",
	completed: "Completed",
	killed:    "Killed",
}

func (pocketbaseDB *DatabaseAPI) SetProcess(id, state string) {
	record, err := pocketbaseDB.App.Dao().FindRecordById("_processes", id)
	base.CheckErr("", err)

	record.Set("state", state)

	err = pocketbaseDB.App.Dao().SaveRecord(record)
	base.CheckErr("[RegisterProcessInDB][SaveRecord]", err)
}

func (pocketbaseDB *DatabaseAPI) RegisterProcessInDB(data, state string) string {
	collection, err := pocketbaseDB.App.Dao().FindCollectionByNameOrId("_processes")
	base.CheckErr("[RunningCommand][FindCollection]:", err)

	record := models.NewRecord(collection)

	id := base.RandomString(15)

	record.Set("id", id)
	record.Set("data", data)
	record.Set("state", state)

	err = pocketbaseDB.App.Dao().SaveRecord(record)
	base.CheckErr("[RegisterProcessInDB][SaveRecord]", err)
	return id
}

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

func (pocketbaseDB *DatabaseAPI) RunCommand(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/runcommand",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil

			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			var data RunCommandData
			if err := c.Bind(&data); err != nil {
				return err
			}

			log.Println("[RunCommand]: ", data)

			id := pocketbaseDB.RegisterProcessInDB(data.Data, process.inqueue)

			data.ID = id

			// send to channel
			pocketbaseDB.CmdChannel <- data

			return c.JSON(http.StatusOK, map[string]interface{}{
				"id": id,
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(pocketbaseDB.App),
		},
	})
	return nil
}

func (pocketbaseDB *DatabaseAPI) RunningCommand(id string, command string, filename string) {

	pocketbaseDB.SetProcess(id, process.running)
	var cmd *exec.Cmd
	saveToFile := filename != ""

	var useBash = runtime.GOOS != "windows"

	// if saveToFile {
	// 	command = command + " > " + c.Filename
	// }
	// cmd = exec.Command("cmd", "/C", command)

	if saveToFile {
		command = command + " > " + filename
	}

	if useBash {
		cmd = exec.Command("bash", "-c", command)
	} else {
		cmd = exec.Command("cmd", "/C", command)
	}

	log.Println("[RunningCommand] ", cmd)

	// Create a pipe for the output of the command
	_, err := cmd.StdoutPipe()
	if err != nil {
		log.Println("Error creating stdout pipe:", err)
		pocketbaseDB.SetProcess(id, fmt.Sprintf("%v error", err))
		return
	}

	// Start the command
	err = cmd.Start()
	if err != nil {
		log.Println("Error starting command:", err)
		pocketbaseDB.SetProcess(id, fmt.Sprintf("%v error", err))
		return
	}

	// Wait for the command to finish
	err = cmd.Wait()

	if err != nil {
		log.Println("Error waiting for command:", err)
		pocketbaseDB.SetProcess(id, fmt.Sprintf("%v error", err))
		return
	}

	pocketbaseDB.SetProcess(id, process.completed)
}

func (pocketbaseDB *DatabaseAPI) RunningCommandSaveToCollection(id, command, collectionName string) {

	pocketbaseDB.SetProcess(id, process.running)

	log.Println("RunningCommand: ", command)
	var cmd *exec.Cmd

	var useBash = runtime.GOOS != "windows"

	if useBash {
		cmd = exec.Command("bash", "-c", command)
	} else {
		cmd = exec.Command("cmd", "/C", command)
	}

	// Create a pipe for the output of the command
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Println("Error creating stdout pipe:", err)
		pocketbaseDB.SetProcess(id, fmt.Sprintf("%v error", err))
		return
	}

	// Start the command
	err = cmd.Start()
	if err != nil {
		log.Println("Error starting command:", err)
		pocketbaseDB.SetProcess(id, fmt.Sprintf("%v error", err))
		return
	}

	// Create a scanner to read the output line by line
	scanner := bufio.NewScanner(stdout)

	collection, err := pocketbaseDB.App.Dao().FindCollectionByNameOrId(collectionName)
	base.CheckErr("[RunningCommand][FindCollection]:", err)

	// Read the output in real-time
	for scanner.Scan() {
		jsonrow := scanner.Text()
		log.Println("[RunningCommand][Scanner]: ", jsonrow)

		record := models.NewRecord(collection)
		record.Set("data", jsonrow)
		err = pocketbaseDB.App.Dao().SaveRecord(record)
		base.CheckErr("[RunningCommand][SaveRecord]:", err)
	}

	// Wait for the command to finish
	err = cmd.Wait()

	if err != nil {
		log.Println("Error waiting for command:", err)
		pocketbaseDB.SetProcess(id, fmt.Sprintf("%v error", err))
		return
	}

	pocketbaseDB.SetProcess(id, process.completed)

}
