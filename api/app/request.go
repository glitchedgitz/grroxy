package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"log"

	"github.com/glitchedgitz/grroxy-db/grrhttp"
	"github.com/glitchedgitz/grroxy-db/types"
	"github.com/glitchedgitz/grroxy-db/utils"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
)

func generateUserData(data types.AddRequestBodyType) (types.UserData, error) {

	log.Printf("[generateUserData] Called with index: %v", data.Index)
	var userdata types.UserData

	// Convert request to string
	// requestInBytes, err := httputils.DumpRequestOut(req, true)
	// utils.CheckErr("Req:Dumping Bytes Error", err)
	// requestInString := string(requestInBytes)

	log.Printf("[generateUserData] Reading Request: %s", data.Request)
	req, err := http.ReadRequest(bufio.NewReader(strings.NewReader(fmt.Sprint(data.Request + "\n\n"))))
	if err != nil {
		return userdata, err
	}

	log.Printf("[generateUserData] Request: %+v", req)

	log.Printf("[generateUserData] Initiating variables")

	// Initiate variables
	var (
		index   = data.Index
		id      = utils.FormatStringID(fmt.Sprintf("%v.%v", index, data.IndexMinor), 15)
		method  = http.MethodGet
		host    = ""
		port    = ""
		isHttps = false
	)

	if data.Url != "" {
		host = data.Url
		isHttps = strings.HasPrefix(host, "https://")
		host = strings.Replace(host, "http://", "", 1)
		host = strings.Replace(host, "https://", "", 1)
	} else {
		host = req.URL.Host
		isHttps = req.URL.Scheme == "https"
	}

	log.Printf("[generateUserData] Setting method")
	// Set method
	if req.Method != "" {
		method = req.Method
	}

	log.Printf("[generateUserData] Setting host and port")
	// Set host and port
	if strings.Contains(host, ":") {
		t := strings.Split(host, ":")
		host = t[0]
		port = t[1]
	}

	log.Printf("[generateUserData] Setting extension")
	if isHttps {
		host = "https://" + host
	} else {
		host = "http://" + host
	}

	log.Printf("[generateUserData] Setting extension")
	extension := ""

	log.Printf("[generateUserData] Setting path")

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

	log.Printf("[generateUserData] Extension: %s", extension)
	log.Printf("[generateUserData] Setting userdata")
	userdata = types.UserData{
		ID:         id,
		Index:      index,
		IndexMinor: data.IndexMinor,
		IsHTTPS:    isHttps,
		Attached:   id,
		Req:        id,
		Resp:       id,
		Host:       host,
		Port:       port,
		HasResp:    false,
		ReqJson: types.RequestData{
			Method:     method,
			HasCookies: len(req.Cookies()) > 0,
			HasParams:  len(req.URL.Query()) > 0,
			Length:     req.ContentLength,
			Headers:    grrhttp.GetHeaders(req.Header),
			Url:        req.URL.RequestURI(),
			Path:       req.URL.Path,
			Query:      req.URL.RawQuery,
			Fragment:   req.URL.RawFragment,
			Ext:        extension,
		},
		RespJson: types.ResponseData{
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

	log.Printf("[generateUserData] Returning userdata: %+v", userdata)
	return userdata, nil
}

func generateResponseForUserData(userdata *types.UserData, response string) {
	log.Printf("[generateResponseForUserData] Called for user ID: %s", userdata.ID)
	resp, err := http.ReadResponse(bufio.NewReader(strings.NewReader(fmt.Sprint(response)+"\n\n")), nil)
	if err != nil {
		log.Printf("[generateResponseForUserData] Error reading response: %v", err)
		panic(err)
	}

	userdata.RespJson = types.ResponseData{
		HasCookies: len(resp.Cookies()) > 0,
		Title:      "",
		Mime:       resp.Header.Get("content-type"),
		Headers:    grrhttp.GetHeaders(resp.Header),
		Status:     resp.StatusCode,
		Length:     resp.ContentLength,
		Date:       resp.Header.Get("Date"),
		Time:       time.Now().Format(time.RFC3339),
	}

	log.Printf("[generateResponseForUserData] Response: %+v", resp)
}

func (backend *Backend) AddRequest(e *core.ServeEvent) error {
	log.Println("[AddRequest] Registering /api/request/add route")
	e.Router.AddRoute(echo.Route{
		Method: "POST",
		Path:   "/api/request/add",
		Handler: func(c echo.Context) error {
			log.Println("[AddRequest] Handler called")
			var body types.AddRequestBodyType
			if err := c.Bind(&body); err != nil {
				log.Printf("[AddRequest] Error binding body: %v", err)
				return err
			}
			log.Printf("[AddRequest] Request body: %+v", body)
			userdata, err := generateUserData(body)
			if err != nil {
				log.Printf("[AddRequest] Error generating user data: %v", err)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to generate user data"})
			}

			if body.Response != "" {
				log.Println("[AddRequest] Response provided, generating response data")
				generateResponseForUserData(&userdata, body.Response)
			}

			log.Printf("[AddRequest] Saving _attached record")
			collection, err := backend.App.Dao().FindCollectionByNameOrId("_attached")
			if err != nil {
				log.Printf("[AddRequest] Error finding _attached collection: %v", err)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to find collection"})
			}

			record := models.NewRecord(collection)
			record.Set("id", userdata.ID)
			record.Set("labels", []string{})
			record.Set("note", "")

			log.Printf("[AddRequest] Saving _attached record")
			err = backend.App.Dao().SaveRecord(record)
			if err != nil {
				log.Printf("[AddRequest] Error saving _attached record: %v", err)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to save record in _attached"})
			}

			log.Printf("[AddRequest] Saving _req record")
			reqCollection, err := backend.App.Dao().FindCollectionByNameOrId("_req")
			if err != nil {
				log.Printf("[AddRequest] Error finding _req collection: %v", err)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to find _req collection"})
			}
			reqRecord := models.NewRecord(reqCollection)
			reqRecord.Set("id", userdata.ID)
			reqRecord.Set("method", userdata.ReqJson.Method)
			reqRecord.Set("url", userdata.ReqJson.Url)
			reqRecord.Set("path", userdata.ReqJson.Path)
			reqRecord.Set("query", userdata.ReqJson.Query)
			reqRecord.Set("fragment", userdata.ReqJson.Fragment)
			reqRecord.Set("ext", userdata.ReqJson.Ext)
			reqRecord.Set("has_cookies", userdata.ReqJson.HasCookies)
			reqRecord.Set("length", userdata.ReqJson.Length)
			reqRecord.Set("headers", userdata.ReqJson.Headers)
			reqRecord.Set("raw", body.Request)

			err = backend.App.Dao().SaveRecord(reqRecord)
			if err != nil {
				log.Printf("[AddRequest] Error saving _req record: %v", err)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to save record in _req"})
			}

			// If response exists, save it to _resp collection
			if body.Response != "" {
				log.Printf("[AddRequest] Saving _resp record")
				respCollection, err := backend.App.Dao().FindCollectionByNameOrId("_resp")
				if err != nil {
					log.Printf("[AddRequest] Error finding _resp collection: %v", err)
					return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to find _resp collection"})
				}
				respRecord := models.NewRecord(respCollection)
				respRecord.Set("id", userdata.ID)
				respRecord.Set("title", userdata.RespJson.Title)
				respRecord.Set("mime", userdata.RespJson.Mime)
				respRecord.Set("status", userdata.RespJson.Status)
				respRecord.Set("length", userdata.RespJson.Length)
				respRecord.Set("has_cookies", userdata.RespJson.HasCookies)
				respRecord.Set("headers", userdata.RespJson.Headers)
				respRecord.Set("raw", body.Response)

				err = backend.App.Dao().SaveRecord(respRecord)
				if err != nil {
					log.Printf("[AddRequest] Error saving _resp record: %v", err)
					return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to save record in _resp"})
				}
			}

			log.Printf("[AddRequest] _data record")
			collection3, err := backend.App.Dao().FindCollectionByNameOrId("_data")
			if err != nil {
				log.Printf("[AddRequest] Error finding _data collection: %v", err)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to find collection"})
			}

			record3 := models.NewRecord(collection3)
			log.Printf("[AddRequest] Loading userdata into _data record")

			b, err := json.Marshal(userdata)
			if err != nil {
				// handle error
			}
			var m map[string]any
			if err := json.Unmarshal(b, &m); err != nil {
				// handle error
			}

			record3.Load(m)

			log.Printf("[AddRequest] Saving _data record")
			err = backend.App.Dao().SaveRecord(record3)
			if err != nil {
				log.Printf("[AddRequest] Error saving _data record: %v", err)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to save record in _data"})
			}

			log.Printf("[AddRequest] Successfully processed request for user ID: %s", userdata.ID)

			typ := "folder"

			if userdata.ReqJson.Ext != "" {
				typ = "file"
			}

			s_data := types.SitemapGet{
				Host:     userdata.Host,
				Path:     userdata.ReqJson.Path,
				Query:    userdata.ReqJson.Query,
				Fragment: userdata.ReqJson.Fragment,
				Ext:      userdata.ReqJson.Ext,
				Type:     typ,
				Data:     userdata.ID,
			}

			go func() {
				err = backend.handleSitemapNew(&s_data)
				if err != nil {
					log.Printf("[AddRequest] Error handling sitemap new for user ID: %s, err: %v", userdata.ID, err)
					// return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to handle sitemap new"})
				}
			}()

			return c.JSON(http.StatusOK, userdata)
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})

	return nil
}
