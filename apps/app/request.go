package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"log"

	"github.com/glitchedgitz/grroxy/grx/rawhttp"
	"github.com/glitchedgitz/grroxy/internal/types"
	"github.com/glitchedgitz/grroxy/internal/utils"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
)

func generateUserData(data types.AddRequestBodyType, indexMinor float64) (types.UserData, error) {

	log.Printf("[generateUserData] Called with index: %v, indexMinor: %v", data.Index, indexMinor)
	var userdata types.UserData

	// Convert request to string
	// requestInBytes, err := httputils.DumpRequestOut(req, true)
	// utils.CheckErr("Req:Dumping Bytes Error", err)
	// requestInString := string(requestInBytes)

	log.Printf("[generateUserData] Reading Request: %s", data.Request)
	// Use tolerant raw parser (no dependency on net/http parsing)
	parsed := rawhttp.ParseRequest([]byte(data.Request))
	log.Printf("[generateUserData] Parsed Request: %+v", parsed)

	log.Printf("[generateUserData] Initiating variables")

	// Initiate variables
	var (
		index   = data.Index
		id      = ""
		method  = http.MethodGet
		host    = ""
		port    = ""
		isHttps = false
	)

	if indexMinor != -1 {
		id = utils.FormatStringID(fmt.Sprintf("%v.%v", index, indexMinor), 15)
	} else {
		id = utils.FormatStringID(fmt.Sprintf("%v", index), 15)
	}

	if data.Url != "" {
		host = data.Url
		isHttps = strings.HasPrefix(host, "https://")
		host = strings.Replace(host, "http://", "", 1)
		host = strings.Replace(host, "https://", "", 1)
	} else {
		// Fallback to Host header if provided
		if h, ok := rawhttp.GetHeaderValue(parsed.Headers, "host"); ok {
			host = h
		}
		// No scheme in request line; cannot infer reliably. Leave isHttps false unless URL hints
		isHttps = false
	}

	log.Printf("[generateUserData] Setting method")
	// Set method
	if parsed.Method != "" {
		method = parsed.Method
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

	// Parse URL path/query/fragment from request line URL
	var reqURL *url.URL
	if parsed.URL != "" {
		// url.ParseRequestURI handles absolute path or absolute-form URI
		if u, err := url.ParseRequestURI(parsed.URL); err == nil {
			reqURL = u
		}
	}
	if reqURL != nil && reqURL.Path != "" {
		p := strings.Split(reqURL.Path, "/")
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
	// Build http.Header from parsed headers for existing helpers
	httpHdr := http.Header{}
	for _, header := range parsed.Headers {
		if len(header) >= 2 {
			httpHdr.Set(header[0], header[1])
		}
	}

	// Determine content length
	var contentLen int64 = 0
	if clStr, ok := rawhttp.GetHeaderValue(parsed.Headers, "content-length"); ok {
		if n, err := strconv.ParseInt(strings.TrimSpace(clStr), 10, 64); err == nil {
			contentLen = n
		}
	}

	// Determine cookies and params
	hasCookies := false
	if c, ok := rawhttp.GetHeaderValue(parsed.Headers, "cookie"); ok && strings.TrimSpace(c) != "" {
		hasCookies = true
	}
	hasParams := false
	urlPath := ""
	urlQuery := ""
	urlFragment := ""
	if reqURL != nil {
		hasParams = reqURL.RawQuery != ""
		urlPath = reqURL.Path
		urlQuery = reqURL.RawQuery
		urlFragment = reqURL.RawFragment
	}

	userdata = types.UserData{
		ID:         id,
		Index:      index,
		IndexMinor: indexMinor,
		IsHTTPS:    isHttps,
		Http:       parsed.HTTPVersion,
		Attached:   id,
		Req:        id,
		Resp:       id,
		Host:       host,
		Port:       port,
		HasResp:    false,
		ReqJson: types.RequestData{
			Method:     method,
			HasCookies: hasCookies,
			HasParams:  hasParams,
			Length:     contentLen,
			Headers:    rawhttp.GetHeaders(httpHdr),
			Url:        parsed.URL,
			Path:       urlPath,
			Query:      urlQuery,
			Fragment:   urlFragment,
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
	parsed := rawhttp.ParseResponse([]byte(response))

	// Build http.Header from parsed headers for existing helpers
	httpHdr := http.Header{}
	for _, header := range parsed.Headers {
		if len(header) >= 2 {
			httpHdr.Set(header[0], header[1])
		}
	}

	// Determine content length
	var contentLen int64 = 0
	if clStr, ok := rawhttp.GetHeaderValue(parsed.Headers, "content-length:"); ok {
		if n, err := strconv.ParseInt(strings.TrimSpace(clStr), 10, 64); err == nil {
			contentLen = n
		}
	}

	var contentType string
	if ct, ok := rawhttp.GetHeaderValue(parsed.Headers, "content-type:"); ok {
		contentType = ct
	}

	var date string
	if d, ok := rawhttp.GetHeaderValue(parsed.Headers, "date:"); ok {
		date = d
	}

	extractTitle, _ := utils.ExtractTitle([]byte(response))

	// Cookies via Set-Cookie
	hasCookies := false
	if sc, ok := rawhttp.GetHeaderValue(parsed.Headers, "set-cookie"); ok && strings.TrimSpace(sc) != "" {
		hasCookies = true
	}

	userdata.RespJson = types.ResponseData{
		HasCookies: hasCookies,
		Title:      extractTitle,
		Mime:       contentType,
		Headers:    rawhttp.GetHeaders(httpHdr),
		Status:     parsed.Status,
		Length:     contentLen,
		Date:       date,
		Time:       time.Now().Format(time.RFC3339),
	}

	log.Printf("[generateResponseForUserData] Parsed Response: %+v", userdata.RespJson)
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

			// Validate generated_by is provided
			if body.GeneratedBy == "" {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "Generated by is required"})
			}

			// Use SaveRequestToBackend function
			userdata, err := backend.SaveRequestToBackend(body)
			if err != nil {
				log.Printf("[AddRequest] Error saving to backend: %v", err)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to save request to backend"})
			}

			return c.JSON(http.StatusOK, userdata)
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})

	return nil
}

// SaveRequestToBackend saves the request and response to the backend database
func (backend *Backend) SaveRequestToBackend(reqBody types.AddRequestBodyType) (types.UserData, error) {
	log.Println("[SaveRequestToBackend] Called with index:", reqBody.Index)

	var newindex = float64(reqBody.Index)
	var indexMinor float64 = -1

	if reqBody.Index == 0 {
		newindex = float64(ProxyMgr.GetNextIndex())
		reqBody.Index = newindex
	} else {
		// Calculate index_minor using counter system automatically
		// Each index has its own counter for minor indexes
		counterKey := fmt.Sprintf("row:%.0f", reqBody.Index)
		indexMinor = float64(backend.CounterManager.Increment(counterKey, "", ""))
		log.Printf("[SaveRequestToBackend] Auto-calculated index_minor: %.0f for index: %.0f", indexMinor, reqBody.Index)
	}

	// Generate user data from request
	userdata, err := generateUserData(reqBody, indexMinor)
	if err != nil {
		log.Printf("[SaveRequestToBackend] Error generating user data: %v", err)
		return types.UserData{}, err
	}

	// If response exists, generate response data
	if reqBody.Response != "" {
		log.Println("[SaveRequestToBackend] Response provided, generating response data")
		generateResponseForUserData(&userdata, reqBody.Response)
	}

	// Set generated by field
	if reqBody.GeneratedBy != "" {
		userdata.GeneratedBy = reqBody.GeneratedBy
	}

	// Save _attached record
	log.Printf("[SaveRequestToBackend] Saving _attached record")
	collection, err := backend.App.Dao().FindCollectionByNameOrId("_attached")
	if err != nil {
		log.Printf("[SaveRequestToBackend] Error finding _attached collection: %v", err)
		return types.UserData{}, err
	}

	record := models.NewRecord(collection)
	record.Set("id", userdata.ID)
	record.Set("labels", []string{})
	record.Set("note", reqBody.Note)

	err = backend.App.Dao().SaveRecord(record)
	if err != nil {
		log.Printf("[SaveRequestToBackend] Error saving _attached record: %v", err)
		return types.UserData{}, err
	}

	// Save _req record
	log.Printf("[SaveRequestToBackend] Saving _req record")
	reqCollection, err := backend.App.Dao().FindCollectionByNameOrId("_req")
	if err != nil {
		log.Printf("[SaveRequestToBackend] Error finding _req collection: %v", err)
		return types.UserData{}, err
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
	reqRecord.Set("raw", reqBody.Request)

	err = backend.App.Dao().SaveRecord(reqRecord)
	if err != nil {
		log.Printf("[SaveRequestToBackend] Error saving _req record: %v", err)
		return types.UserData{}, err
	}

	// If response exists, save it to _resp collection
	if reqBody.Response != "" {
		log.Printf("[SaveRequestToBackend] Saving _resp record")
		respCollection, err := backend.App.Dao().FindCollectionByNameOrId("_resp")
		if err != nil {
			log.Printf("[SaveRequestToBackend] Error finding _resp collection: %v", err)
			return types.UserData{}, err
		}

		respRecord := models.NewRecord(respCollection)
		respRecord.Set("id", userdata.ID)
		respRecord.Set("title", userdata.RespJson.Title)
		respRecord.Set("mime", userdata.RespJson.Mime)
		respRecord.Set("status", userdata.RespJson.Status)
		respRecord.Set("length", userdata.RespJson.Length)
		respRecord.Set("has_cookies", userdata.RespJson.HasCookies)
		respRecord.Set("headers", userdata.RespJson.Headers)
		respRecord.Set("raw", reqBody.Response)

		err = backend.App.Dao().SaveRecord(respRecord)
		if err != nil {
			log.Printf("[SaveRequestToBackend] Error saving _resp record: %v", err)
			return types.UserData{}, err
		}
	}

	// Save _data record
	log.Printf("[SaveRequestToBackend] Saving _data record")
	dataCollection, err := backend.App.Dao().FindCollectionByNameOrId("_data")
	if err != nil {
		log.Printf("[SaveRequestToBackend] Error finding _data collection: %v", err)
		return types.UserData{}, err
	}

	dataRecord := models.NewRecord(dataCollection)
	log.Printf("[SaveRequestToBackend] Loading userdata into _data record")

	b, err := json.Marshal(userdata)
	if err != nil {
		log.Printf("[SaveRequestToBackend] Error marshaling userdata: %v", err)
		return types.UserData{}, err
	}

	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		log.Printf("[SaveRequestToBackend] Error unmarshaling userdata: %v", err)
		return types.UserData{}, err
	}

	dataRecord.Load(m)

	err = backend.App.Dao().SaveRecord(dataRecord)
	if err != nil {
		log.Printf("[SaveRequestToBackend] Error saving _data record: %v", err)
		return types.UserData{}, err
	}

	log.Printf("[SaveRequestToBackend] Successfully processed request for user ID: %s", userdata.ID)

	// Handle sitemap
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
			log.Printf("[SaveRequestToBackend] Error handling sitemap new for user ID: %s, err: %v", userdata.ID, err)
		}
	}()

	return userdata, nil
}
