package app

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"runtime"

	"github.com/glitchedgitz/grroxy-db/internal/process"
	"github.com/glitchedgitz/grroxy-db/internal/schemas"
	"github.com/glitchedgitz/grroxy-db/internal/utils"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
)

// loop over commandChannel
func (backend *Backend) CommandManager() {
	// log.Println("[CommandManager Stared]")
	for c := range backend.CmdChannel {
		log.Println("Command received: ", c)
		if c.SaveTo == "collection" {
			backend.RunningCommandSaveToCollection(c.ID, c.Command, c.Collection)
		} else {
			backend.RunningCommand(c.ID, c.Command, c.Filename)
		}
	}
}

func (backend *Backend) SetProcess(id, state string) {
	process.SetState(backend.App, id, state)
}

func (backend *Backend) RegisterProcessInDB(input, data any, name, typz, state string) string {
	return process.RegisterInDB(backend.App, input, data, name, typz, state)
}

func (backend *Backend) RunCommand(e *core.ServeEvent) error {
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

			var data process.RunCommandData
			if err := c.Bind(&data); err != nil {
				return err
			}

			log.Println("[RunCommand]: ", data)

			id := backend.RegisterProcessInDB(data.Data, data, data.Command, "command", schemas.ProcessState.Inqueue)

			data.ID = id

			// send to channel
			backend.CmdChannel <- data

			return c.JSON(http.StatusOK, map[string]interface{}{
				"id": id,
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

func (backend *Backend) RunningCommand(id string, command string, filename string) {

	backend.SetProcess(id, schemas.ProcessState.Running)
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
		backend.SetProcess(id, fmt.Sprintf("%v error", err))
		return
	}

	// Start the command
	err = cmd.Start()
	if err != nil {
		log.Println("Error starting command:", err)
		backend.SetProcess(id, fmt.Sprintf("%v error", err))
		return
	}

	// Wait for the command to finish
	err = cmd.Wait()

	if err != nil {
		log.Println("Error waiting for command:", err)
		backend.SetProcess(id, fmt.Sprintf("%v error", err))
		return
	}

	backend.SetProcess(id, schemas.ProcessState.Completed)
}

func (backend *Backend) RunningCommandSaveToCollection(id, command, collectionName string) {

	backend.SetProcess(id, schemas.ProcessState.Running)

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
		backend.SetProcess(id, fmt.Sprintf("%v error", err))
		return
	}

	// Start the command
	err = cmd.Start()
	if err != nil {
		log.Println("Error starting command:", err)
		backend.SetProcess(id, fmt.Sprintf("%v error", err))
		return
	}

	// Create a scanner to read the output line by line
	scanner := bufio.NewScanner(stdout)

	collection, err := backend.App.Dao().FindCollectionByNameOrId(collectionName)
	utils.CheckErr("[RunningCommand][FindCollection]:", err)

	// Read the output in real-time
	for scanner.Scan() {
		jsonrow := scanner.Text()
		log.Println("[RunningCommand][Scanner]: ", jsonrow)

		record := models.NewRecord(collection)
		record.Set("data", jsonrow)
		err = backend.App.Dao().SaveRecord(record)
		utils.CheckErr("[RunningCommand][SaveRecord]:", err)
	}

	// Wait for the command to finish
	err = cmd.Wait()

	if err != nil {
		log.Println("Error waiting for command:", err)
		backend.SetProcess(id, fmt.Sprintf("%v error", err))
		return
	}

	backend.SetProcess(id, schemas.ProcessState.Completed)

}
