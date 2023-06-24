package endpoints

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"path"
	"strings"

	"github.com/glitchedgitz/grroxy-db/save"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models/schema"
)

// channel to receive commands
var commandChannel = make(chan string)

// loop over commandChannel
func CommandManager() {
	for {
		command := <-commandChannel
		log.Println("Command received: ", command)
		RunningCommand(command)
	}
}

func RunningCommand(command string) {

	log.Println("Running command: ", command)

	cmd := exec.Command("cmd", "/C", command)

	// Create a pipe for the output of the command
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("Error creating stdout pipe:", err)
		return
	}

	// Start the command
	err = cmd.Start()
	if err != nil {
		fmt.Println("Error starting command:", err)
		return
	}

	// Create a scanner to read the output line by line
	scanner := bufio.NewScanner(stdout)

	// Read the output in real-time
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}

	// Wait for the command to finish
	err = cmd.Wait()

	if err != nil {
		fmt.Println("Error waiting for command:", err)
		return
	}
}

func (pocketbaseDB *DatabaseAPI) RunCommand(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/runcommand",
		Handler: func(c echo.Context) error {

			var data map[string]interface{}
			if err := c.Bind(&data); err != nil {
				return err
			}

			filePath := path.Join(pocketbaseDB.Config.CacheDirectory, "request_id.txt")
			wordlistPath := `D:\test\test.txt`

			// Save request_id.txt
			save.WriteFile(filePath, []byte(data["request"].(string)))

			// Create a new database
			collection := "ffuf_test"
			err := pocketbaseDB.CreateCollection(collection, schema.NewSchema(
				&schema.SchemaField{
					Name:     "path",
					Type:     schema.FieldTypeText,
					Required: true,
				}, &schema.SchemaField{
					Name:     "type",
					Type:     schema.FieldTypeText,
					Required: true,
				},
				&schema.SchemaField{
					Name:     "main_id",
					Type:     schema.FieldTypeText,
					Required: true,
				},
			))

			if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
				log.Println("collection already exists: ", collection)
			}

			// ffuf -r request_id.txt -w wordlist.txt
			command := fmt.Sprintf("ffuf -request %s -w %s -json", filePath, wordlistPath)
			RunningCommand(command)

			// send to channel
			// commandChannel <- command

			return c.String(http.StatusOK, "Created")
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(pocketbaseDB.App),
		},
	})
	return nil
}
