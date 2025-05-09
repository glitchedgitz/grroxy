package proxy

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"strings"

	"github.com/elazarl/goproxy"
	"github.com/glitchedgitz/grroxy-db/utils"
	"github.com/glitchedgitz/grroxy-db/templates/actions"
	"github.com/glitchedgitz/grroxy-db/types"
	"github.com/projectdiscovery/dsl"
	"gopkg.in/yaml.v2"
)

// MatchReplaceRequest strings or regex
func (p *Proxy) MatchReplaceRequest(req string) string {
	// lazy mode - ninja level - elaborate
	m := make(map[string]interface{})
	m["req"] = req
	if v, err := dsl.EvalExpr(p.options.RequestMatchReplaceDSL, m); err != nil {
		return req
	} else {
		return fmt.Sprint(v)
	}
}

// DropReqResp returns a response with status code 502
// i.e. Bad Gateway and Terminate the connection
func DropReqResp(req *http.Request) *http.Response {
	resp := &http.Response{}
	resp.Request = req
	resp.Header = make(http.Header)
	resp.StatusCode = http.StatusBadGateway
	resp.Status = http.StatusText(http.StatusBadGateway)
	buf := bytes.NewBufferString("")
	resp.Body = io.NopCloser(buf)
	return resp
}

type attach = struct {
	Id     string   `db:"id" json:"id"`
	Labels []string `db:"labels" json:"labels"`
	Note   string   `db:"note" json:"note"`
}

func getHeaders(h http.Header) map[string]string {
	headers := map[string]string{}
	for header, value := range h {
		// header = strings.ReplaceAll(header, "-", "_")
		// header = strings.ToLower(header)
		headers[header] = strings.Join(value, " ///// ")
	}
	return headers
}

func (p *Proxy) OnRequest(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	// rateLimit <- ""
	// defer func() { <-rateLimit }()

	if strings.Contains(req.URL.Host, "grroxy") {
		return req, nil
	}

	var userdata types.UserData

	if ctx.UserData != nil {
		userdata = ctx.UserData.(types.UserData)
	}

	// Convert request to string
	// requestInBytes, err := httputils.DumpRequestOut(req, true)
	// utils.CheckErr("Req:Dumping Bytes Error", err)
	// requestInString := string(requestInBytes)

	requestRateLimit <- 0

	// Initiate variables
	var (
		index   = <-generateIndex
		id      = utils.FormatNumericID(index, 15)
		method  = http.MethodGet
		host    = req.URL.Host
		port    = ""
		isHttps = req.URL.Scheme == "https"
	)

	// Set method
	if req.Method != "" {
		method = req.Method
	}

	// Set host and port
	if strings.Contains(host, ":") {
		t := strings.Split(host, ":")
		host = t[0]
		port = t[1]
	}

	host = req.URL.Scheme + "://" + host

	extension := ""

	if req.URL.Path != "" {
		p := strings.Split(req.URL.Path, "/")
		lastfile := p[len(p)-1]

		if strings.Contains(lastfile, ".") {
			l := strings.Split(lastfile, ".")
			extension = "." + l[len(l)-1]
		}

		if len(extension) > 10 {
			extension = ""
		}
	}

	userdata = types.UserData{
		ID:       id,
		Index:    index,
		Raw:      id,
		Attached: id,
		Host:     host,
		Port:     port,
		HasResp:  false,
		Req: types.RequestData{
			Method:     method,
			HasCookies: len(req.Cookies()) > 0,
			HasParams:  len(req.URL.Query()) > 0,
			Length:     req.ContentLength,
			Headers:    getHeaders(req.Header),
			IsHTTPS:    isHttps,
			Url:        req.URL.RequestURI(),
			Path:       req.URL.Path,
			Query:      req.URL.RawQuery,
			Fragment:   req.URL.RawFragment,
			Ext:        extension,
		},
		Resp: types.ResponseData{
			Title:      "",
			Mime:       "",
			Status:     0,
			Length:     0,
			HasCookies: false,
			Date:       "",
			Time:       "",
		},
		IsReqEdited:  false,
		IsRespEdited: false,
	}

	tmpdata := types.UserData{
		Host: userdata.Host,
		Req:  userdata.Req,
	}

	requestJson := utils.StructToMap(&tmpdata, "json")
	results, err := p.templates.Run(requestJson, "proxy:before_request")

	if err != nil {
		log.Println("Error: before_request template: ", err)
	} else {

		log.Println("[OnRequest] before_request Checking template results: ", results)

		for _, action := range results {
			switch action.ActionName {
			case actions.Modify:

				for key, value := range action.Data {
					if key == "replace" {
						for _, replace := range value.([]any) {
							var r actions.ModifierReplace
							intermediate, err := yaml.Marshal(replace)
							if err != nil {
								panic(err)
							}

							err = yaml.Unmarshal(intermediate, &r)
							if err != nil {
								log.Println("Error: Template replace", err)
							}

							extractedValue, err := utils.ExtractValueFromMap(&requestJson, r.Key)
							if err != nil {
								log.Println("Error: Extracting value", err)
							}

							updatedValue, err := utils.FindAndReplaceAll(fmt.Sprint(extractedValue), r.Search, r.Replace, r.Regex)
							if err != nil {
								log.Println(err)
								continue
							}
							userdata.RequestUpdateKey(req, r.Key, updatedValue)

						}
					} else if key == "delete" {
						userdata.RequestDeleteKey(req, key)
					} else if strings.HasPrefix(key, "req.") {
						userdata.RequestUpdateKey(req, key, value)
					}
				}

			default:
				log.Println("[OnRequest] Unknown Action for before_request ")
			}
		}
	}

	requestInBytes, _ := httputil.DumpRequestOut(req, true)
	requestInString := string(requestInBytes)

	// Add to database
	p.DBCreate("_attached", map[string]any{
		"id":     userdata.ID,
		"labels": []string{},
		"note":   "",
	})

	p.DBCreate("_raw", map[string]string{
		"id":       userdata.ID,
		"req":      requestInString,
		"attached": userdata.ID,
	})

	var requestNew *http.Request

	// Intercept
	if p.options.Intercept && p.checkFilters(requestJson) {

		updatedString, edited := p.interceptWait(&userdata, "req", req.ContentLength)

		if userdata.Action == "drop" {
			ctx.UserData = userdata

			log.Printf("[Request][Intercept][%s]: Dropping Request \n", userdata.Host+"/"+userdata.Req.Path)
			return req, DropReqResp(req)
		}

		if edited {
			userdata.IsReqEdited = true
		}

		p.grroxydb.Create("_data", userdata)

		// Convert string to request
		req.Body.Close()
		requestNew, err = http.ReadRequest(bufio.NewReader(strings.NewReader(fmt.Sprint(updatedString))))
		utils.CheckErr("Error in reading updated request", err)

		req.Method = requestNew.Method
		req.Header = requestNew.Header
		req.Body = requestNew.Body
		req.Host = requestNew.Host
		req.ContentLength = requestNew.ContentLength

		newURL := requestNew.URL
		newURL.Host = req.URL.Host
		newURL.Scheme = req.URL.Scheme
		req.URL = newURL

		// log.Println(http.DumpRequestOut())

		// Todo: Set Host, Port and Scheme
		// req.URL.Host // this can include the port also
		// req.URL.Scheme
	}

	p._requestAddToDB(&userdata)
	ctx.UserData = userdata
	// ctx.Req = req

	// defer req.Body.Close()
	return req, nil
}

func (p *Proxy) _requestAddToDB(userdata *types.UserData) {
	typ := "folder"
	if userdata.Req.Ext != "" {
		typ = "file"
	}

	// log.Println("[_requestAddToDB]userdata: ", userdata)
	p.grroxydb.Create("_data", userdata)

	s_data := types.SitemapGet{
		Host:     userdata.Host,
		Path:     userdata.Req.Path,
		Query:    userdata.Req.Query,
		Fragment: userdata.Req.Fragment,
		Ext:      userdata.Req.Ext,
		Type:     typ,
		Data:     userdata.ID,
	}

	p.DBNewSitemap(s_data)

	log.Println("[_requestAddToDB] Checking template")

	// this seems like an extra step, one data struct should be used everywhere
	go p.runReqTemplates(userdata)
}

func (p *Proxy) runReqTemplates(userdata *types.UserData) {
	tmpdata := types.UserData{
		Req: userdata.Req,
	}

	d := utils.StructToMap(&tmpdata, "json")
	results, _ := p.templates.Run(d, "proxy:request")

	log.Println("[_requestAddToDB] Checking template results: ", results)

	for _, y := range results {

		name := y.Data["name"].(string)

		if len(name) == 0 {
			continue
		}

		var l_data = types.Label{
			Name:  name,
			Color: y.Data["color"].(string),
			Type:  y.Data["type"].(string),
			Icon:  y.Data["icon"].(string),
			ID:    userdata.ID,
		}

		switch y.ActionName {
		case actions.CreateLabel:
			p.DBAttachLabel(l_data)
		default:
			log.Println("[_requestAddToDB] Unknown Action")
		}
	}
}

func (p *Proxy) SearchAndReplaceRequest() {

}
