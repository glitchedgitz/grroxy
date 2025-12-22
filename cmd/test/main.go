package main

import (
	"encoding/json"
	"fmt"

	"github.com/glitchedgitz/grroxy-db/internal/schemas"
	"github.com/glitchedgitz/grroxy-db/internal/sdk"
	"github.com/glitchedgitz/grroxy-db/internal/types"
	pbTypes "github.com/pocketbase/pocketbase/tools/types"

	"github.com/pocketbase/pocketbase/models"
)

func testPlaygroundAdd(grroxydb *sdk.Client, id string, playgroundAddData string) {
	fmt.Println("PlaygroundAddData: ", playgroundAddData)

	var playgroundAdd types.PlaygroundAdd
	err := json.Unmarshal([]byte(playgroundAddData), &playgroundAdd)
	if err != nil {
		fmt.Println("Error: ", err)
	}
	fmt.Println("playgroundAdd: ", playgroundAdd)
	pgData, err := grroxydb.PlaygroundAddChild(playgroundAdd)
	fmt.Println("Returned data: ", pgData)
	if err != nil {
		fmt.Println("Error: ", err)
	}
}

func main() {
	var grroxydb = sdk.NewClient(
		"http://127.0.0.1:8091",
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

	// Create a new playground
	playgroundNewData := `{
		"name": "test pg",
		"type": "playground",
		"parent_id": "",
		"expanded":   true
	}`

	var playgroundNew types.PlaygroundNew
	json.Unmarshal([]byte(playgroundNewData), &playgroundNew)
	pgData, err := grroxydb.PlaygroundNew(playgroundNew)
	fmt.Println("Returned data: ", pgData)
	if err != nil {
		fmt.Println("Error: ", err)
	}

	id := pgData.(map[string]any)["id"].(string)
	fmt.Println("ID: ", id)

	playgroundAddData := `{
		"parent_id": "` + id + `",
		"items": [
			{
			"name": "test repeater",
			"type": "repeater",
			"tool_data": {
				"url": "http://example.com/test",
				"req": "GET /test HTTP/1.1\nHost: example.com",
				"resp": "HTTP/1.1 200 OK\nContent-Type: text/plain\n\nHello World",
				"method": "GET",
				"path": "/test",
				"headers": {
					"Host": "example.com"
					}
				}
			}
		]
	}`

	testPlaygroundAdd(grroxydb, id, playgroundAddData)

	playgroundAddData2 := `{
		"parent_id": "` + id + `",
		"items": [
		{
			"name": "test repeater",
			"type": "repeater",
			"tool_data": {
				"url": "http://example.com/test",
				"req": "GET /test HTTP/1.1\nHost: example.com",
				"resp": "HTTP/1.1 200 OK\nContent-Type: text/plain\n\nHello World",
				"method": "GET",
				"path": "/test",
				"headers": {
					"Host": "example.com"
					}
				}
			},
			{
			"name": "test note",
			"type": "note",
			"tool_data": {
				"content": "test note"
				}
			}
		]
	}`

	testPlaygroundAdd(grroxydb, id, playgroundAddData2)

	playgroundAddIntruderData := `{
		"parent_id": "` + id + `",
		"items": [
			{
			"name": "test intruder",
			"type": "fuzzer",
			"tool_data": {
				"url": "http://example.com/test",
				"req": "GET /test HTTP/1.1\nHost: example.com",
				"resp": "HTTP/1.1 200 OK\nContent-Type: text/plain\n\nHello World",
				"httpVersionTab": "HTTP/1.1",
				"threads": 40,
				"markers": { "FUZZ": { "name": "FUZZ", "color": "#e5c07b" } },
				"props": {
					"FUZZ": {
						"generator": {
							"title": "Range",
							"name": "Range",
							"props": {
								"from": "1",
								"to": "999"
								},
							"component": "Range",
							"favourite": false,
							"id": "h13NCzug"
							},
						"methods": []
						}
					}
				}
			}
		]
	}`

	testPlaygroundAdd(grroxydb, id, playgroundAddIntruderData)

}
