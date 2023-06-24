package main

import (
	"log"
	"sync"

	"github.com/glitchedgitz/grroxy-db/base"
	"github.com/glitchedgitz/grroxy-db/sdk"
	"github.com/glitchedgitz/grroxy-db/types"
)

// {
//     "collectionId": "494feqlwo6b1vds",
//     "collectionName": "settings",
//     "created": "2023-04-13 04:57:40.591Z",
//     "id": "___intercept___",
//     "option": "Intercept",
//     "updated": "2023-04-13 10:26:31.332Z",
//     "value": "falsetest"
// }

func (p *Proxy) InterceptManager() {

	p.options.Intercept = true

	stream, err := sdk.CollectionSet[any](p.grroxydb, "settings").Subscribe("settings/" + base.AddUnderscore("INTERCEPT"))

	log.Print("Subscribed to setting")
	if err != nil {
		log.Fatal(err)
	}

	<-stream.Ready()
	defer stream.Unsubscribe()

	for ev := range stream.Events() {
		log.Print("[Main][InterceptManager]: ", ev.Action, ev.Record)

		// extract the value field from ev.Record using type assertion
		value, ok := ev.Record.(map[string]interface{})["value"].(string)
		if !ok {
			log.Print("invalid value field type")
			continue
		}

		if value == "false" {
			p.options.Intercept = false
			collection := sdk.CollectionSet[types.RealtimeRecord](p.grroxydb, "intercept")
			response, err := collection.List(types.ParamsList{
				Page: 1, Size: 1000, Sort: "created",
			})

			if err != nil {
				log.Fatal(err)
			}

			var wg sync.WaitGroup

			wg.Add(len(response.Items))

			// update each record action to forward
			for _, record := range response.Items {
				go func(r types.RealtimeRecord) {
					r.Action = "forward"
					p.grroxydb.Update("intercept", r.ID, r)
					wg.Done()
				}(record)
			}
			wg.Wait()
		} else {
			p.options.Intercept = true
		}
	}
}
