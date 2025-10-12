package proxy

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/elazarl/goproxy"
	"github.com/glitchedgitz/grroxy-db/grrhttp"
	"github.com/glitchedgitz/grroxy-db/templates/actions"
	"github.com/glitchedgitz/grroxy-db/types"
	"github.com/glitchedgitz/grroxy-db/utils"
	"github.com/projectdiscovery/dsl"
	"gopkg.in/yaml.v2"
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

	log.Print("[OnResponse] Starting OnResponse")
	userdata := ctx.UserData.(types.UserData)
	if strings.Contains(userdata.Host, "grroxy") {
		return resp
	}
	if userdata.Action == "drop" {
		log.Printf("[Response][Intercept][%s]: Dropping Response because request dropped \n", userdata.ID)
		return DropReqResp(ctx.Req)
	}

	userdata.Action = ""

	id := userdata.ID
	userdata.HasResp = true

	if resp == nil {
		log.Print("[OnResponse]Returning nil response")
		return nil
	}

	userdata.Resp = types.ResponseData{
		HasCookies: len(resp.Cookies()) > 0,
		Title:      "",
		Mime:       resp.Header.Get("content-type"),
		Headers:    grrhttp.GetHeaders(resp.Header),
		Status:     resp.StatusCode,
		Length:     resp.ContentLength,
		Date:       resp.Header.Get("Date"),
		Time:       time.Now().Format(time.RFC3339),
	}

	responseJson := utils.StructToMap(&userdata, "json")
	results, err := p.templates.Run(responseJson, "proxy:before_response")

	if err != nil {
		log.Println("Error: [proxy:before_response] template: ", err)
	} else {

		log.Println("[OnResponse] before_response Checking template results: ", results)

		for _, action := range results {
			switch action.ActionName {
			case actions.Set:
				for key, value := range action.Data {
					if key == "set" {
						for k, v := range value.(map[string]string) {
							userdata.ResponseUpdateKey(resp, k, v)
						}
					}
				}
			case actions.Delete:
				for key, _ := range action.Data {
					if key == "delete" {
						userdata.ResponseDeleteKey(resp, key)
					}
				}
			case actions.Replace:

				for key, replaces := range action.Data {
					for _, replace := range replaces.([]any) {
						var r actions.ModifierReplace
						intermediate, err := yaml.Marshal(replace)
						if err != nil {
							log.Println("Error: ", err)
						}

						err = yaml.Unmarshal(intermediate, &r)
						if err != nil {
							log.Println("Error: Template replace", err)
						}

						extractedValue, err := utils.ExtractValueFromMap(&responseJson, key)
						if err != nil {
							log.Println("Error: Extracting value", err)
						}

						updatedValue, err := utils.FindAndReplaceAll(fmt.Sprint(extractedValue), r.Search, r.Value, r.Regex)
						if err != nil {
							log.Println(err)
							continue
						}
						userdata.ResponseUpdateKey(resp, key, updatedValue)

					}
				}

			default:
				log.Println("[OnRequest] Unknown Action for before_request ")
			}
		}
	}

	responseInBytes, err := utils.ResponseToByte(resp)
	utils.CheckErr("[OnResponse]", err)
	responseInString := string(responseInBytes)

	var title string
	if responseInBytes != nil {
		title, _ = utils.ExtractTitle(responseInBytes)
	}

	userdata.Resp.Title = title

	r_data := Store_Resp{
		Response: responseInString,
	}

	p.grroxydb.Update("_raw", id, r_data)

	// var updatedString string
	var edited bool
	// Intercept
	if p.options.Intercept && p.checkFilters(responseJson) {

		responseInString, edited = p.interceptWait(&userdata, "resp", resp.ContentLength)

		if userdata.Action == "drop" {
			log.Printf("[Response][Intercept][%s]: Dropping Response \n", userdata.Host+"/"+userdata.Req.Path)
			ctx.UserData = userdata
			return DropReqResp(ctx.Req)
		}

		if edited {
			userdata.IsRespEdited = true
		}

		p.grroxydb.Update("_data", userdata.ID, userdata)

		utils.CheckErr("Error in reading updated request", err)

	}

	p._responseAddToDB(&userdata)
	resp, err = http.ReadResponse(bufio.NewReader(strings.NewReader(fmt.Sprint(responseInString))), ctx.Req)
	utils.CheckErr("[onResponse]: ", err)
	ctx.UserData = userdata
	// defer resp.Body.Close()
	return resp
}

func (p *Proxy) _responseAddToDB(userdata *types.UserData) {
	userdata.Resp.Mime = strings.ToLower(userdata.Resp.Mime)
	userdata.Resp.Mime = strings.ReplaceAll(userdata.Resp.Mime, "\"", "")
	userdata.Resp.Mime = strings.ReplaceAll(userdata.Resp.Mime, "'", "")
	userdata.Resp.Mime = strings.ReplaceAll(userdata.Resp.Mime, " ", "")

	p.DBUpdate("_data", userdata.ID, userdata)

	go p.runRespTemplates(userdata)
}

func (p *Proxy) runRespTemplates(userdata *types.UserData) {

	tmpdata := types.UserData{
		Req:  userdata.Req,
		Resp: userdata.Resp,
	}

	d := utils.StructToMap(&tmpdata, "json")
	results, _ := p.templates.Run(d, "proxy:response")

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
