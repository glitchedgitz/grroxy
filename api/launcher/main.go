package launcher

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/glitchedgitz/cook/v2/pkg/cook"
	"github.com/glitchedgitz/grroxy-db/config"
	"github.com/glitchedgitz/grroxy-db/process"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/models/schema"
	pbTypes "github.com/pocketbase/pocketbase/tools/types"
)

type Launcher struct {
	App        *pocketbase.PocketBase
	Config     *config.Config
	Cook       *cook.CookGenerator
	CmdChannel chan process.RunCommandData
}

func (launcher *Launcher) Serve() {
	launcher.App.Bootstrap()

	fmt.Printf(`
Application:        http://%s
Database:           http://%s/_/
API:                http://%s/api/
Cert:               http://%s/cacert.crt

Proxy Listening On: %s

	`, launcher.Config.HostAddr, launcher.Config.HostAddr, launcher.Config.HostAddr, launcher.Config.HostAddr, launcher.Config.ProxyAddr)

	// var httpsAddr string

	var httpAddr = launcher.Config.HostAddr
	_, err := apis.Serve(launcher.App, apis.ServeConfig{
		HttpAddr: httpAddr,
	})

	if errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}

// Create Collection with schema in params
func (launcher *Launcher) CreateCollection(collectionName string, dbSchema schema.Schema) error {
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

	if err := launcher.App.Dao().SaveCollection(collection); err != nil {
		return err
	}

	return nil
}
