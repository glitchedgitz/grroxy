package endpoints

import (
	"github.com/glitchedgitz/grroxy-db/config"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/cmd"
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

	serveCmd := cmd.NewServeCommand(pocketbaseDB.App, true)
	serveCmd.Execute()
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
