package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync"
	"time"

	"github.com/elazarl/goproxy"
	"github.com/glitchedgitz/grroxy-db/sdk"
	"github.com/glitchedgitz/grroxy-db/types"
	"github.com/projectdiscovery/dsl"
	"golang.org/x/net/html"
)

type Store_Req struct {
	ID      string `db:"id" json:"id"`
	Request string `db:"request" json:"request"`
}
type Store_Resp struct {
	Response string `db:"response" json:"response"`
}

// MatchReplaceRequest strings or regex
func (p *Proxy) MatchReplaceResponse(resp string) string {
	// resp.ContentLength = 0

	m := make(map[string]interface{})
	m["response"] = resp
	if v, err := dsl.EvalExpr(p.options.ResponseMatchReplaceDSL, m); err != nil {
		return resp
	} else {
		return fmt.Sprint(v)
	}
}

// if p.options.ResponseDSL != "" && !userdata.Match {
// 	m, _ := mapsutil.HTTPResponseToMap(resp)
// 	v, err := dsl.EvalExpr(p.options.ResponseDSL, m)
// 	if err != nil {
// 		gologger.Warning().Msgf("Could not evaluate response dsl: %s\n", err)
// 	}
// 	userdata.Match = err == nil && v.(bool)
// }

// perform match and replace
// if p.options.ResponseMatchReplaceDSL != "" {
// 	respString = p.MatchReplaceResponse(respString)
// }

func (p *Proxy) OnResponse(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {

	userdata := ctx.UserData.(types.UserData)
	userdata.HasResponse = true

	if resp == nil {
		return nil
	}

	respBytes, err := httputil.DumpResponse(resp, true)
	if err != nil {
		log.Println("Resp: Dumping Bytes Error", err)
	}
	respDataInString := string(respBytes)

	// respID := base.RandomString(15)

	// currentTime := time.Now()

	var title string
	if respBytes != nil {
		title, _ = extractTitle(respBytes)
	}

	userdata.OriginalResponse = types.ResponseData{
		HasCookies:    len(resp.Cookies()) > 0,
		Title:         title,
		Mimetype:      resp.Header.Get("content-type"),
		StatusCode:    resp.StatusCode,
		ContentLength: resp.ContentLength,
		Date:          resp.Header.Get("Date"),
		Time:          time.Now().Format(time.RFC3339),
	}

	// First we need to save the original response
	// p.save.Save("resp", userdata) //nolint
	// runtime.LogPrintf(p.options.Ctx, "Intercept is %v", p.options.Intercept)
	r_data := Store_Resp{
		Response: respDataInString,
	}
	p.grroxydb.Update("store", userdata.ID, r_data)

	if p.options.Intercept {

		var wg sync.WaitGroup
		// var updatedResp types.UserData

		wg.Add(1)

		//Add to database
		p.DBCreate("intercept", userdata)

		stream, err := sdk.CollectionSet[types.RealtimeRecord](p.grroxydb, "intercept").Subscribe("intercept/" + userdata.ID)

		log.Print("Subcrbied to the record")
		if err != nil {
			log.Fatal(err)
		}

		<-stream.Ready()

		log.Print("Subcrbie is ready")

		updatedRow := types.RealtimeRecord{}

		for ev := range stream.Events() {
			log.Print(ev.Action, ev.Record)
			if ev.Record.Action == "forward" {
				log.Println("Forwarding response...")
				updatedRow = ev.Record
			} else if ev.Record.Action == "drop" {
				// GPT4's Idea
				log.Println("Drop response...")
				return goproxy.NewResponse(ctx.Req, goproxy.ContentTypeText, 444, "")
			} else {
				continue
			}
			break
		}

		stream.Unsubscribe()
		log.Print("Unsubscribed")

		p.grroxydb.Delete("intercept", userdata.ID)
		p.grroxydb.Create("data", userdata)

		collection := sdk.CollectionSet[any](p.grroxydb, "store")
		updatedData, err := collection.One(updatedRow.ID)
		if err != nil {
			log.Println(err)
		}

		var updatedString string

		log.Println("Edited Request is not empty -----------------------")
		log.Println(updatedData)

		upData := updatedData.(map[string]interface{})
		log.Println("Updated Data --------------  ", upData)

		if updatedRow.IsResponseEdited {
			updatedString = upData["response_edited"].(string)
		} else {
			updatedString = upData["response"].(string)
		}

		respNew, err := http.ReadResponse(bufio.NewReader(strings.NewReader(fmt.Sprint(updatedString))), nil)

		if err != nil {
			log.Println("Error in reading response", err)
		}

		ctx.UserData = userdata
		defer resp.Body.Close()
		return respNew
	}

	p._responseAddToDB(userdata)

	ctx.UserData = userdata
	return resp
}

func (p *Proxy) _responseAddToDB(userdata types.UserData) {
	p.DBUpdate("data", userdata.ID, userdata)
}

func extractTitle(respByte []byte) (string, string) {

	title := ""
	favicon := ""

	z := html.NewTokenizer(bytes.NewReader(respByte))

	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}

		t := z.Token()

		if t.Type == html.StartTagToken {
			if t.Data == "title" {
				if z.Next() == html.TextToken {
					title = strings.TrimSpace(z.Token().Data)
					break
				}
			}
			// else if t.Data == "link" {
			// 	if z.Next() == html.TextToken {
			// 		favicon = strings.TrimSpace(z.Token().Data)
			// 		break
			// 	}
			// }
		}
	}
	return title, favicon
}

// respNew, err := http.ReadResponse(bufio.NewReader(strings.NewReader(fmt.Sprint(updatedResp.Event.Data))), nil)
// if err == io.ErrUnexpectedEOF {
// 	respNew, err = http.ReadResponse(bufio.NewReader(strings.NewReader(fmt.Sprint(updatedResp.Event.Data+"\n\n"))), nil)
// 	if err != nil {
// 		log.Fatalln("Response: ", err)
// 	}
// }
