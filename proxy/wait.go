package proxy

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/glitchedgitz/grroxy-db/base"
	"github.com/glitchedgitz/grroxy-db/sdk"
	"github.com/glitchedgitz/grroxy-db/types"
)

func (p *Proxy) interceptWait(userdata *types.UserData, field string, contentLength int64) (string, bool) {
	id := userdata.ID

	originalData := field
	editedData := field + "_edited"

	var wg sync.WaitGroup
	wg.Add(1)

	//Add to database
	p.DBCreate("_intercept", userdata)

	// Realtime Subscription
	stream, err := sdk.CollectionSet[types.RealtimeRecord](p.grroxydb, "_intercept").Subscribe("_intercept/" + id)

	base.CheckErr(fmt.Sprintf("[WaitData][Intercept][%s] Error while creating stream \n", id), err)
	log.Printf("[WaitData][Intercept][%s]: Subcrbied to the record \n", id)

	<-stream.Ready()
	log.Printf("[WaitData][Intercept][%s]: Subcrbie is ready\n", id)

	updatedRow := types.RealtimeRecord{}
	action := ""
	for ev := range stream.Events() {
		log.Printf("[WaitData][Intercept][%s]: %s %v\n", id, ev.Action, ev.Record)

		if ev.Record.Action == "forward" {
			log.Printf("[WaitData][Intercept][%s]: Forwarding WaitData\n", id)
			updatedRow = ev.Record
			action = "forward"
			break
		}
		if ev.Record.Action == "drop" { // GPT4's Idea
			updatedRow = ev.Record
			action = "drop"
			updatedRow.Action = "drop"
			log.Printf("[WaitData][Intercept][%s]: Drop WaitData\n", id)
			break
		}
	}

	stream.Unsubscribe()
	log.Printf("[WaitData][Intercept][%s]: About to Unsubscribe WaitData\n", id)

	p.grroxydb.Delete("_intercept", id)

	if action == "drop" {
		userdata.Action = "drop"
		return "", false
	}

	collection := sdk.CollectionSet[any](p.grroxydb, "_raw")
	updatedData, err := collection.One(updatedRow.ID)
	if err != nil {
		log.Println(err)
	}

	var updatedString string

	log.Println("[onWaitData] Edited WaitData is not empty -----------------------")
	log.Println(updatedData)

	upData := updatedData.(map[string]interface{})
	log.Println("[onWaitData] Updated Data --------------  ", upData)

	edited := false
	if field == "req" && updatedRow.IsReqEdited {
		edited = true
		log.Println("[onWaitData] Request is edited -----------------------")
	} else if field == "resp" && updatedRow.IsRespEdited {
		log.Println("[onWaitData] Response is edited -----------------------")
		edited = true
	}

	if edited {
		updatedString = upData[editedData].(string)
		previousTotalLength := len(upData[originalData].(string))
		newTotalLength := len(updatedString)
		diffLength := newTotalLength - previousTotalLength

		if diffLength < 0 {
			diffLength = diffLength * -1
		}

		log.Println("[previousTotalLength] ", previousTotalLength)
		log.Println("[newTotalLength] ", newTotalLength)
		log.Println("[diffLength] ", diffLength)

		if diffLength != 0 {
			previousContentHeader := "Content-Length: " + fmt.Sprint(contentLength)
			newContentHeader := "Content-Length: " + fmt.Sprint(contentLength+int64(diffLength))
			updatedString = strings.Replace(updatedString, previousContentHeader, newContentHeader, 1)

			previousContentHeader = "Content-Length:" + fmt.Sprint(contentLength)
			newContentHeader = "Content-Length:" + fmt.Sprint(contentLength+int64(diffLength))
			updatedString = strings.Replace(updatedString, previousContentHeader, newContentHeader, 1)
		}

	} else {
		updatedString = upData[originalData].(string)
	}

	return updatedString, edited
}
