package tools

import (
	"github.com/glitchedgitz/cook/v2/pkg/cook"
	"github.com/glitchedgitz/grroxy-db/internal/config"
	"github.com/glitchedgitz/grroxy-db/internal/process"
	"github.com/pocketbase/pocketbase"
)

type Tools struct {
	App        *pocketbase.PocketBase
	Config     *config.Config
	Cook       *cook.COOK
	CmdChannel chan process.RunCommandData
}
