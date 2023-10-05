package main

import (
	"github.com/glitchedgitz/grroxy-db/schemas"
	"github.com/glitchedgitz/grroxy-db/sdk"
	pbTypes "github.com/pocketbase/pocketbase/tools/types"

	"github.com/pocketbase/pocketbase/models"
)

func main() {
	var grroxydb = sdk.NewClient(
		"http://127.0.0.1:8090",
		sdk.WithAdminEmailPassword("new@example.com", "1234567890"))

	grroxydb.CreateCollection(models.Collection{
		Name:       "plugin_tmp_intercept",
		Type:       models.CollectionTypeBase,
		ListRule:   pbTypes.Pointer(""),
		ViewRule:   pbTypes.Pointer(""),
		CreateRule: pbTypes.Pointer(""),
		UpdateRule: pbTypes.Pointer(""),
		DeleteRule: nil,
		Schema:     schemas.Intercept,
	})
}
