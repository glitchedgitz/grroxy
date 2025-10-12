package tools_api

import (
	"github.com/glitchedgitz/cook/v2/pkg/cook"
	"github.com/glitchedgitz/grroxy-db/config"
	"github.com/glitchedgitz/grroxy-db/process"
	"github.com/pocketbase/pocketbase"
)

type Tools struct {
	App        *pocketbase.PocketBase
	Config     *config.Config
	Cook       *cook.COOK
	CmdChannel chan process.RunCommandData
}
