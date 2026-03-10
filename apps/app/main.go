package app

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/glitchedgitz/cook/v2/pkg/cook"
	"github.com/glitchedgitz/grroxy/internal/config"
	"github.com/glitchedgitz/grroxy/internal/process"
	"github.com/glitchedgitz/pocketbase"
	"github.com/glitchedgitz/pocketbase/apis"
	"github.com/glitchedgitz/pocketbase/models"
	"github.com/glitchedgitz/pocketbase/models/schema"
	pbTypes "github.com/glitchedgitz/pocketbase/tools/types"
	wappalyzer "github.com/glitchedgitz/wappalyzergo"
)

type Backend struct {
	App            *pocketbase.PocketBase
	Config         *config.Config
	Cook           *cook.CookGenerator
	Wappalyzer     *wappalyzer.Wappalyze
	CmdChannel     chan process.RunCommandData
	CounterManager *CounterManager
	XtermManager   *XtermManager
	MCP            *MCP
}

func (backend *Backend) Serve() {
	backend.App.Bootstrap()

	fmt.Printf(`
Application:        http://%s
Database:           http://%s/_/
API:                http://%s/api/
Cert:               http://%s/cacert.crt

	`, backend.Config.HostAddr, backend.Config.HostAddr, backend.Config.HostAddr, backend.Config.HostAddr)

	go backend.CommandManager()

	// var httpsAddr string

	var httpAddr = backend.Config.HostAddr
	_, err := apis.Serve(backend.App, apis.ServeConfig{
		HttpAddr: httpAddr,
	})

	if errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}

// Create Collection with schema in params
func (backend *Backend) CreateCollection(collectionName string, dbSchema schema.Schema) error {
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

	if err := backend.App.Dao().SaveCollection(collection); err != nil {
		return err
	}

	return nil
}
