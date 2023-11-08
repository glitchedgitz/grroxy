package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/elazarl/goproxy"
	"github.com/glitchedgitz/grroxy-db/base"
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

func (p *Proxy) OnResponse(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {

	userdata := ctx.UserData.(types.UserData)
	log.Printf("[Response][Intercept][%s]: ResponseUserdata \n", userdata)

	id := userdata.ID
	userdata.HasResponse = true

	if resp == nil {
		return nil
	}

	responseInBytes, err := base.ResponseToByte(resp)
	base.CheckErr("[OnResponse]", err)
	responseInString := string(responseInBytes)

	var title string
	if responseInBytes != nil {
		title, _ = extractTitle(responseInBytes)
	}

	userdata.OriginalResponse = types.ResponseData{
		HasCookies:    len(resp.Cookies()) > 0,
		Title:         title,
		Mimetype:      resp.Header.Get("content-type"),
		StatusCode:    resp.StatusCode,
		ContentLength: len(responseInString),
		Date:          resp.Header.Get("Date"),
		Time:          time.Now().Format(time.RFC3339),
	}

	r_data := Store_Resp{
		Response: responseInString,
	}

	p.grroxydb.Update("_store", id, r_data)

	// var updatedString string
	var edited bool
	// Intercept
	if p.options.Intercept && p.checkFilters(userdata) {

		responseInString, edited = p.interceptWait(userdata, "response", resp.ContentLength)

		if edited {
			userdata.IsResponseEdited = true
		}

		p.grroxydb.Update("_data", userdata.ID, userdata)

		base.CheckErr("Error in reading updated request", err)

	}

	p._responseAddToDB(userdata)
	resp, err = http.ReadResponse(bufio.NewReader(strings.NewReader(fmt.Sprint(responseInString))), ctx.Req)
	base.CheckErr("[onResponse]: ", err)
	ctx.UserData = userdata
	// defer resp.Body.Close()
	return resp
}

func (p *Proxy) _responseAddToDB(userdata types.UserData) {
	p.DBUpdate("_data", userdata.ID, userdata)
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
