package main

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync"

	"github.com/elazarl/goproxy"
	"github.com/glitchedgitz/grroxy-db/base"
	"github.com/glitchedgitz/grroxy-db/sdk"
	"github.com/glitchedgitz/grroxy-db/types"
	"github.com/projectdiscovery/dsl"
)

// MatchReplaceRequest strings or regex
func (p *Proxy) MatchReplaceRequest(req string) string {
	// lazy mode - ninja level - elaborate
	m := make(map[string]interface{})
	m["request"] = req
	if v, err := dsl.EvalExpr(p.options.RequestMatchReplaceDSL, m); err != nil {
		return req
	} else {
		return fmt.Sprint(v)
	}
}

// check dsl
// if p.options.RequestDSL != "" {
// 	m, _ := mapsutil.HTTPRequesToMap(req)
// 	v, err := dsl.EvalExpr(p.options.RequestDSL, m)
// 	if err != nil {
// 		gologger.Warning().Msgf("Could not evaluate request dsl: %s\n", err)
// 	}
// 	userdata.Match = err == nil && v.(bool)
// }

// Future Idea:
// (May be required by plugins in future)
// So before we send the request to user to edit, changes will be applied by plugins

// perform match and replace
//
//	if p.options.RequestMatchReplaceDSL != "" {
//		reqString = p.MatchReplaceRequest(reqString)
//	}

func (p *Proxy) OnRequest(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {

	var userdata types.UserData

	if ctx.UserData != nil {
		userdata = ctx.UserData.(types.UserData)
	}

	id := base.RandomString(15)

	reqBytes, err := httputil.DumpRequest(req, true)
	if err != nil {
		log.Println("Req:Dumping Bytes Error", err)
	}

	reqDataInString := string(reqBytes)

	Method := "GET"
	if req.Method != "" {
		Method = req.Method
	}

	var host = req.URL.Host
	var port = ""
	params := make(map[string][]string)

	for k, v := range req.URL.Query() {
		params[k] = v
	}
	fmt.Println(params)

	if strings.Contains(host, ":") {
		t := strings.Split(host, ":")
		host = t[0]
		port = t[1]
	}

	isHttps := req.TLS != nil
	if isHttps {
		host = "https://" + host
	} else {
		host = "http://" + host
	}

	userdata = types.UserData{
		ID:   id,
		Host: host,
		Port: port,
		IP:   req.RemoteAddr,
		UrlData: types.UrlData{
			Scheme: req.URL.Scheme,
			Params: params,
			Path:   req.URL.Path,
		},
		HasResponse: false,
		OriginalRequest: types.RequestData{
			AllURL:        *req.URL,
			Url:           req.URL.RequestURI(),
			Method:        Method,
			HasCookies:    len(req.Cookies()) > 0,
			HasParams:     len(req.URL.Query()) > 0,
			ContentLength: 0,
			IsHTTPS:       isHttps,
			Date:          "",
			Time:          "",
		},
		OriginalResponse: types.ResponseData{
			Title:         "",
			Mimetype:      "",
			StatusCode:    0,
			ContentLength: 0,
			HasCookies:    false,
			Date:          "",
			Time:          "",
		},
		IsRequestEdited:  false,
		IsResponseEdited: false,
		Labels:           []string{"test"},
	}

	// To get favicon of sites
	//http://www.google.com/s2/favicons?domain=stackoverflow.com

	// First we need to save the original request
	// p.save.Save("req", userdata) //nolint
	// runtime.LogPrintf(p.options.Ctx, "Intercept is %v", p.options.Intercept)

	if p.options.Intercept {

		var wg sync.WaitGroup
		var updatedReq types.UserData

		wg.Add(1)

		//Add to database
		p.DBCreate("intercept", userdata)

		r_data := map[string]string{
			"id":      userdata.ID,
			"request": reqDataInString,
		}
		p.DBCreate("store", r_data)

		stream, err := sdk.CollectionSet[types.RealtimeRecord](p.grroxydb, "intercept").Subscribe("intercept/" + id)

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
				log.Println("Forwarding Request...")
				updatedRow = ev.Record
			} else if ev.Record.Action == "drop" {
				// GPT4's Idea
				log.Println("Drop Request...")
				return req, goproxy.NewResponse(req, goproxy.ContentTypeText, 444, "")
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

		if updatedRow.IsRequestEdited {
			updatedString = upData["request_edited"].(string)
		} else {
			updatedString = upData["request"].(string)
		}

		requestNew, err := http.ReadRequest(bufio.NewReader(strings.NewReader(fmt.Sprint(updatedString))))

		// Update Host
		if err != nil {
			log.Println("[ERR] Request: -----------------------\n", err)
		}

		ctx.UserData = updatedReq

		req.Body.Close()
		return requestNew, nil
	}

	p._requestAddToDB(userdata)

	ctx.UserData = userdata
	return req, nil
}

func (p *Proxy) _requestAddToDB(userdata types.UserData) {

	// r_data := Store_Req{
	// 	ID:      userdata.ID,
	// 	Request: userdata.Event.Data,
	// }

	s_data := types.SitemapGet{
		Host:     userdata.Host,
		Path:     userdata.OriginalRequest.AllURL.Path,
		Query:    userdata.OriginalRequest.AllURL.RawQuery,
		Fragment: userdata.OriginalRequest.AllURL.RawFragment,
		Type:     "Dummy",
		MainID:   userdata.ID,
	}

	p.grroxydb.Delete("intercept", userdata.ID)
	p.DBCreate("sites", map[string]string{
		"site": userdata.Host,
	})
	p.DBNewSitemap(s_data)
}
