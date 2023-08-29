package main

import (
	"bufio"
	"fmt"
	"net/http"
	"net/http/httputil"
	"strings"

	"github.com/elazarl/goproxy"
	"github.com/glitchedgitz/grroxy-db/base"
	"github.com/glitchedgitz/grroxy-db/types"
	"github.com/jpillora/go-tld"
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

func (p *Proxy) OnRequest(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	// rateLimit <- ""
	// defer func() { <-rateLimit }()
	var userdata types.UserData

	if ctx.UserData != nil {
		userdata = ctx.UserData.(types.UserData)
	}

	// Convert request to string
	requestInBytes, err := httputil.DumpRequestOut(req, true)
	base.CheckErr("Req:Dumping Bytes Error", err)
	requestInString := string(requestInBytes)

	requestRateLimit <- 0

	// Initiate variables
	var (
		id      = base.RandomString(15)
		method  = http.MethodGet
		host    = req.URL.Host
		port    = ""
		index   = <-generateIndex
		isHttps = req.URL.Scheme != "https"
	)

	// Set method
	func() {
		if req.Method != "" {
			method = req.Method
		}
	}()

	// Set host and port
	func() {
		if strings.Contains(host, ":") {
			t := strings.Split(host, ":")
			host = t[0]
			port = t[1]
		}

		host = req.URL.Scheme + "://" + host
	}()

	userdata = types.UserData{
		ID:          id,
		Index:       index,
		StoreID:     id,
		ExtraID:     id,
		Host:        host,
		Port:        port,
		IP:          "",
		HasResponse: false,
		OriginalRequest: types.RequestData{
			Method:        method,
			HasCookies:    len(req.Cookies()) > 0,
			HasParams:     len(req.URL.Query()) > 0,
			ContentLength: len(requestInString),
			IsHTTPS:       isHttps,
			Url:           req.URL.RequestURI(),
			Path:          req.URL.Path,
			Query:         req.URL.RawQuery,
			Fragment:      req.URL.RawFragment,
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
		// Labels:           []string{"test"},
	}

	// Add to database
	func() {
		p.DBCreate("_extra", map[string]any{
			"id":    userdata.ID,
			"label": map[string]string{},
			"note":  "",
		})
		p.DBCreate("_store", map[string]string{
			"id":       userdata.ID,
			"request":  requestInString,
			"extra_id": userdata.ID,
		})
		// p.grroxydb.Create("data", userdata)
	}()

	var requestNew *http.Request

	// Intercept
	if p.options.Intercept {

		updatedString, edited := p.interceptWait(userdata, "request", req.ContentLength)

		if edited {
			userdata.IsRequestEdited = true
		}

		p.grroxydb.Create("_data", userdata)

		// Convert string to request
		req.Body.Close()
		requestNew, err = http.ReadRequest(bufio.NewReader(strings.NewReader(fmt.Sprint(updatedString))))
		base.CheckErr("Error in reading updated request", err)

		req.Method = requestNew.Method
		req.Header = requestNew.Header
		req.Body = requestNew.Body
		req.Host = requestNew.Host

		newURL := requestNew.URL
		newURL.Host = req.URL.Host
		newURL.Scheme = req.URL.Scheme
		req.URL = newURL

		// fmt.Println(http.DumpRequestOut())

		// Todo: Set Host, Port and Scheme
		// req.URL.Host // this can include the port also
		// req.URL.Scheme
	}

	p._requestAddToDB(userdata)
	ctx.UserData = userdata
	// ctx.Req = req

	// defer req.Body.Close()
	return req, nil
}

func splitAndJoin(s string) string {
	u, _ := tld.Parse(s)
	arr := strings.Split(u.Subdomain, ".")
	arr = append(arr, u.TLD)
	arr = append(arr, u.Domain)

	arr2 := []string{}
	for i := len(arr); i > 0; i-- {
		arr2 = append(arr2, arr[i-1])
	}

	return strings.Join(arr2, ".")
}

func (p *Proxy) _requestAddToDB(userdata types.UserData) {
	path := userdata.OriginalRequest.Path
	typ := "folder"
	extension := ""

	fmt.Println("[_requestAddToDB]userdata: ", userdata)
	p.grroxydb.Create("_data", userdata)

	u, _ := tld.Parse(userdata.Host)

	p.DBCreate("_sites", map[string]string{
		"site":    userdata.Host,
		"reverse": splitAndJoin(userdata.Host),
		"domain":  u.Domain + "." + u.TLD,
	})

	if path != "" {
		p := strings.Split(path, "/")
		lastfile := p[len(p)-1]

		if strings.Contains(lastfile, ".") {
			l := strings.Split(lastfile, ".")
			extension = l[len(l)-1]
			typ = "file"
		}
	}

	s_data := types.SitemapGet{
		Host:      userdata.Host,
		Path:      userdata.OriginalRequest.Path,
		Query:     userdata.OriginalRequest.Query,
		Fragment:  userdata.OriginalRequest.Fragment,
		Type:      typ,
		MainID:    userdata.ID,
		Extension: extension,
	}

	p.DBNewSitemap(s_data)
}
