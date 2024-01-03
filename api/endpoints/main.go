package endpoints

import (
	"fmt"
	"io"
	"os/exec"
	"sync"

	"github.com/glitchedgitz/grroxy-db/config"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/models/schema"
	pbTypes "github.com/pocketbase/pocketbase/tools/types"
)

type DatabaseAPI struct {
	App        *pocketbase.PocketBase
	Config     *config.Config
	CmdChannel chan RunCommandData
}

func (pocketbaseDB *DatabaseAPI) Serve() {
	pocketbaseDB.App.Bootstrap()

	// os.Args = []string{"grroxy-db", "serve", "--http", "127.0.0.1:8090"}

	// serveCmd := cmd.NewServeCommand(pocketbaseDB.App, true)
	// serveCmd.
	// log.Println("Serving: ", os.Args)

	cmd := exec.Command("grroxy-db", "serve", "--http", "127.0.0.1:8090")

	// err := cmd.Run()
	// log.Println(err)
	// serveCmd.Execute()
	// cmd, _, _ := pocketbaseDB.App.RootCmd.Find([]string{"serve"})
	// cmd.Execute()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("Error creating StdoutPipe:", err)
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Println("Error creating StderrPipe:", err)
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		printOutput(stdout)
	}()

	go func() {
		defer wg.Done()
		printOutput(stderr)
	}()

	err = cmd.Start()
	if err != nil {
		fmt.Println("Error starting command:", err)
		return
	}

	err = cmd.Wait()
	if err != nil {
		fmt.Println("Command finished with error:", err)
	}

	wg.Wait()
}

func printOutput(reader io.Reader) {
	buf := make([]byte, 1024)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			fmt.Print(string(buf[:n]))
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Println("Error reading from pipe:", err)
			break
		}
	}
}

// Create Collection with schema in params
func (pocketbaseDB *DatabaseAPI) CreateCollection(collectionName string, dbSchema schema.Schema) error {
	collection := &models.Collection{
		Name:       collectionName,
		Type:       models.CollectionTypeBase,
		ListRule:   nil,
		ViewRule:   pbTypes.Pointer(""),
		CreateRule: pbTypes.Pointer(""),
		UpdateRule: pbTypes.Pointer(""),
		DeleteRule: nil,
		Schema:     dbSchema,
	}

	if err := pocketbaseDB.App.Dao().SaveCollection(collection); err != nil {
		return err
	}

	return nil
}
