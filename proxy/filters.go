package proxy

import (
	"fmt"
	"log"

	"github.com/glitchedgitz/grroxy-db/base"
	"github.com/glitchedgitz/grroxy-db/sdk"
	"github.com/glitchedgitz/grroxy-db/types"
)

func (p *Proxy) FiltersManager() {
	p.options.Filters = ""

	collection := sdk.CollectionSet[map[string]any](p.grroxydb, "_ui")
	response, err := collection.List(types.ParamsList{
		Page:    1,
		Size:    1,
		Filters: "unique_id ~ '___INTERCEPT___'",
	})

	p.options.Filters = response.Items[0]["data"].(map[string]any)["filterstring"].(string)
	base.CheckErr("[FiltersManager] Fetching ID", err)

	id := response.Items[0]["id"].(string)

	stream, err := sdk.CollectionSet[any](p.grroxydb, "_ui").Subscribe("_ui/" + id)

	log.Print("Subscribed to _ui/" + id)
	if err != nil {
		log.Fatal(err)
	}

	<-stream.Ready()
	defer stream.Unsubscribe()

	for ev := range stream.Events() {
		log.Print("[Main][FiltersManager]: ", ev.Action, ev.Record)

		p.options.Filters = ev.Record.(map[string]any)["data"].(map[string]any)["filterstring"].(string)

	}
}

func (p *Proxy) checkFilters(userdata types.UserData) bool {
	if p.options.Filters == "" {
		return true
	}

	r, err := p.grroxydb.Create("tmp_intercept", userdata)
	base.CheckErr("[checkFilters][tmp_intercept] Create", err)
	defer p.grroxydb.Delete("tmp_intercept", r.ID)

	filters := fmt.Sprintf("id ~ '%s' && ( %s )", r.ID, p.options.Filters)

	collection := sdk.CollectionSet[types.RealtimeRecord](p.grroxydb, "tmp_intercept")
	response, err := collection.List(types.ParamsList{
		Page:    1,
		Size:    1,
		Filters: filters,
	})

	log.Println("======================== Response ===========================", response)
	base.CheckErr("[tmp_intercept] Getting Response", err)

	return len(response.Items) > 0
}
