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
	Request string `db:"req" json:"req"`
}
type Store_Resp struct {
	Response string `db:"resp" json:"resp"`
}

// MatchReplaceRequest strings or regex
func (p *Proxy) MatchReplaceResponse(resp string) string {
	// resp.ContentLength = 0

	m := make(map[string]interface{})
	m["resp"] = resp
	if v, err := dsl.EvalExpr(p.options.ResponseMatchReplaceDSL, m); err != nil {
		return resp
	} else {
		return fmt.Sprint(v)
	}
}

func (p *Proxy) OnResponse(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {

	if strings.Contains(resp.Request.URL.Host, "grroxy") {
		return resp
	}

	log.Print("[OnResponse] Starting OnResponse")
	userdata := ctx.UserData.(types.UserData)
	log.Printf("[Response][Intercept][%s]: ResponseUserdata \n", userdata)

	id := userdata.ID
	userdata.HasResp = true

	if resp == nil {
		log.Print("[OnResponse]Returning nil response")
		return nil
	}

	responseInBytes, err := base.ResponseToByte(resp)
	base.CheckErr("[OnResponse]", err)
	responseInString := string(responseInBytes)

	var title string
	if responseInBytes != nil {
		title, _ = extractTitle(responseInBytes)
	}

	userdata.Resp = types.ResponseData{
		HasCookies: len(resp.Cookies()) > 0,
		Title:      title,
		Mime:       resp.Header.Get("content-type"),
		Status:     resp.StatusCode,
		Length:     len(responseInString),
		Date:       resp.Header.Get("Date"),
		Time:       time.Now().Format(time.RFC3339),
	}

	r_data := Store_Resp{
		Response: responseInString,
	}

	p.grroxydb.Update("_raw", id, r_data)

	// var updatedString string
	var edited bool
	// Intercept
	if p.options.Intercept && p.checkFilters(userdata) {

		responseInString, edited = p.interceptWait(userdata, "resp", resp.ContentLength)

		if edited {
			userdata.IsRespEdited = true
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
