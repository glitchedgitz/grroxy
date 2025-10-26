package api

// RawProxy Wrapper - Integration layer between rawproxy and grroxy-db
//
// This wrapper provides:
// - Request/response interception and database storage
// - Direct DAO access (no SDK overhead)
// - Request-response correlation using rawproxy's requestID
// - Automatic MITM certificate management
//
// File Captures:
// - By default, uses /tmp/grroxy-captures (redundant, safe to ignore)
// - Primary storage is database (_data, _raw, _attached collections)
// - Can be changed to permanent directory for testing/debugging
// - OS automatically cleans /tmp directory periodically

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync/atomic"
	"time"

	"github.com/glitchedgitz/grroxy-db/grrhttp"
	"github.com/glitchedgitz/grroxy-db/rawproxy"
	"github.com/glitchedgitz/grroxy-db/types"
	"github.com/glitchedgitz/grroxy-db/utils"
	"github.com/pocketbase/pocketbase/models"
)

// RawProxyWrapper wraps the rawproxy.Proxy to match our interface
type RawProxyWrapper struct {
	proxy   *rawproxy.Proxy
	config  *rawproxy.Config
	backend *Backend
	index   atomic.Uint64

	// Statistics
	stats ProxyStats

	// Cached collections for performance
	reqCollection        *models.Collection
	respCollection       *models.Collection
	reqEditedCollection  *models.Collection
	respEditedCollection *models.Collection
	dataCollection       *models.Collection
	attachedCollection   *models.Collection
	interceptCollection  *models.Collection

	Intercept bool
	Filters   string
}

// ProxyStats tracks proxy statistics
type ProxyStats struct {
	RequestsTotal   atomic.Uint64
	ResponsesTotal  atomic.Uint64
	RequestsSaved   atomic.Uint64
	ResponsesSaved  atomic.Uint64
	RequestsFailed  atomic.Uint64
	ResponsesFailed atomic.Uint64
	BytesRequest    atomic.Uint64
	BytesResponse   atomic.Uint64
}

// RequestContext stores request data for correlation with response
// This data is passed from onRequest to onResponse via rawproxy.RequestData
type RequestContext struct {
	UserData     types.UserData
	RawRequest   string
	RequestStart time.Time
}

// NewRawProxyWrapper creates a new rawproxy wrapper with the given configuration
// Set outputDir to empty string ("") to disable file captures
func NewRawProxyWrapper(listenAddr, configDir, outputDir string, backend *Backend) (*RawProxyWrapper, error) {
	wrapper := &RawProxyWrapper{
		backend: backend,
	}

	// Initialize index from database to continue from last saved record
	if err := wrapper.initializeIndex(); err != nil {
		log.Printf("[RawProxy][WARN] Failed to initialize index from database: %v (starting from 0)", err)
	}

	// If outputDir is empty, use a temp directory (rawproxy requires a valid path)
	// Files will be written here but we primarily use database storage
	// You can periodically clean this directory or ignore it
	if outputDir == "" {
		// Use system temp dir with a subdirectory for rawproxy captures
		// These captures are redundant since we save to database
		outputDir = "/tmp/grroxy-captures"
		log.Println("[RawProxy] File captures set to temp dir (redundant) - primary storage is database")
	} else {
		log.Printf("[RawProxy] File captures ENABLED - saving to: %s", outputDir)
	}

	// Create the configuration for rawproxy
	// Note: ConfigFolder is where ca.crt and ca.key will be stored
	config := &rawproxy.Config{
		ListenAddr:   listenAddr,
		ConfigFolder: configDir,
		OutputDir:    outputDir,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Create the proxy instance
	// This will generate ca.crt and ca.key in ConfigFolder if they don't exist
	proxy, err := rawproxy.New(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create rawproxy: %w", err)
	}

	wrapper.proxy = proxy
	wrapper.config = config

	log.Printf("[RawProxy] Using certificates at: %s", config.CertPath)

	// Cache collection references for performance
	if err := wrapper.cacheCollections(); err != nil {
		return nil, fmt.Errorf("failed to cache collections: %w", err)
	}

	// Set up request and response handlers
	proxy.SetRequestHandler(wrapper.onRequest)
	proxy.SetResponseHandler(wrapper.onResponse)

	return wrapper, nil
}

// cacheCollections caches collection references for performance
func (rp *RawProxyWrapper) cacheCollections() error {
	if rp.backend == nil || rp.backend.App == nil {
		return fmt.Errorf("backend not available")
	}

	dao := rp.backend.App.Dao()
	var err error

	rp.reqCollection, err = dao.FindCollectionByNameOrId("_req")
	if err != nil {
		return fmt.Errorf("failed to find _req collection: %w", err)
	}

	rp.respCollection, err = dao.FindCollectionByNameOrId("_resp")
	if err != nil {
		return fmt.Errorf("failed to find _resp collection: %w", err)
	}

	rp.reqEditedCollection, err = dao.FindCollectionByNameOrId("_req_edited")
	if err != nil {
		return fmt.Errorf("failed to find _req_edited collection: %w", err)
	}

	rp.respEditedCollection, err = dao.FindCollectionByNameOrId("_resp_edited")
	if err != nil {
		return fmt.Errorf("failed to find _resp_edited collection: %w", err)
	}

	rp.dataCollection, err = dao.FindCollectionByNameOrId("_data")
	if err != nil {
		return fmt.Errorf("failed to find _data collection: %w", err)
	}

	rp.attachedCollection, err = dao.FindCollectionByNameOrId("_attached")
	if err != nil {
		return fmt.Errorf("failed to find _attached collection: %w", err)
	}

	rp.interceptCollection, err = dao.FindCollectionByNameOrId("_intercept")
	if err != nil {
		return fmt.Errorf("failed to find _intercept collection: %w", err)
	}

	log.Println("[RawProxy] Successfully cached all collection references")
	return nil
}

// initializeIndex gets the maximum index from database and sets the counter
func (rp *RawProxyWrapper) initializeIndex() error {
	if rp.backend == nil || rp.backend.App == nil {
		return fmt.Errorf("backend not available")
	}

	dao := rp.backend.App.Dao()

	// Query for the total number of rows in _data collection
	// This matches the old proxy behavior: total = result.TotalItems
	var result struct {
		TotalRows int `db:"total_rows" json:"total_rows"`
	}

	err := dao.DB().
		NewQuery("SELECT COUNT(*) as total_rows FROM _data").
		One(&result)

	if err != nil {
		return fmt.Errorf("failed to query total rows: %w", err)
	}

	// Set the atomic counter to the total rows count
	// The next Add(1) will increment it to totalRows + 1
	totalRows := uint64(result.TotalRows)
	rp.index.Store(totalRows)

	log.Printf("[RawProxy][INIT] ========================================")
	log.Printf("[RawProxy][INIT] Index Initialization:")
	log.Printf("[RawProxy][INIT]   - Total rows in database: %d", totalRows)
	log.Printf("[RawProxy][INIT]   - Next index will be: %d", totalRows+1)
	log.Printf("[RawProxy][INIT]   - Counter starting at: %d", totalRows)
	log.Printf("[RawProxy][INIT] ========================================")

	return nil
}

// RunProxy starts the proxy server in a non-blocking manner
func (rp *RawProxyWrapper) RunProxy() error {
	go func() {
		if err := rp.proxy.Start(); err != nil && err != http.ErrServerClosed {
			log.Printf("[ERROR] RawProxy server error: %v", err)
		}
	}()

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)
	return nil
}

// Stop gracefully stops the proxy server
func (rp *RawProxyWrapper) Stop() error {
	log.Println("[RawProxy] Stopping proxy server...")

	// Print final statistics
	rp.PrintStats()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rp.proxy.Stop(ctx); err != nil {
		log.Printf("[RawProxy][ERROR] Error stopping rawproxy: %v", err)
		return err
	}

	log.Println("[RawProxy][INFO] Proxy stopped successfully")
	return nil
}

// SetRequestHandler sets a custom request handler
func (rp *RawProxyWrapper) SetRequestHandler(handler rawproxy.OnRequestHandler) {
	rp.proxy.SetRequestHandler(handler)
}

// SetResponseHandler sets a custom response handler
func (rp *RawProxyWrapper) SetResponseHandler(handler rawproxy.OnResponseHandler) {
	rp.proxy.SetResponseHandler(handler)
}

// GetConfig returns the proxy configuration
func (rp *RawProxyWrapper) GetConfig() *rawproxy.Config {
	return rp.config
}

// GetCertPath returns the path to the CA certificate
func (rp *RawProxyWrapper) GetCertPath() string {
	return rp.config.CertPath
}

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

// CleanupTempCaptures removes temporary capture files (if using /tmp)
// Call this periodically or on shutdown to free up space
func (rp *RawProxyWrapper) CleanupTempCaptures() error {
	if rp.config.OutputDir == "/tmp/grroxy-captures" {
		// Only cleanup if we're using the temp directory
		log.Println("[RawProxy] Cleaning up temporary capture files...")
		// Note: We don't delete the directory here to avoid race conditions
		// The OS will clean up /tmp periodically
		return nil
	}
	return nil
}

// onRequest handles incoming HTTP requests and saves them to the database
func (rp *RawProxyWrapper) onRequest(reqData *rawproxy.RequestData, req *http.Request) (*http.Request, error) {
	// Skip our own grroxy requests to avoid loops
	if strings.Contains(req.URL.Host, "grroxy") {
		return req, nil
	}

	// Track total requests
	rp.stats.RequestsTotal.Add(1)

	// Generate unique ID and index
	index := rp.index.Add(1)
	id := utils.FormatNumericID(float64(index), 15)

	// Log first request to verify index is correct
	if index == 1 {
		log.Printf("[RawProxy][REQUEST] FIRST REQUEST - Using index: %d, ID: %s", index, id)
	}

	// Extract host and port
	// For proxied requests, req.Host contains the actual host, not req.URL.Host
	host := req.Host
	if host == "" {
		host = req.URL.Host // Fallback to URL.Host if req.Host is empty
	}

	port := ""
	if strings.Contains(host, ":") {
		parts := strings.Split(host, ":")
		host = parts[0]
		port = parts[1]
	}

	// Add scheme to host
	scheme := req.URL.Scheme
	if scheme == "" {
		if req.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	hostWithScheme := scheme + "://" + host

	// Extract file extension
	extension := ""
	if req.URL.Path != "" {
		pathParts := strings.Split(req.URL.Path, "/")
		lastFile := pathParts[len(pathParts)-1]
		if strings.Contains(lastFile, ".") {
			extParts := strings.Split(lastFile, ".")
			extension = "." + extParts[len(extParts)-1]
			if len(extension) > 10 {
				extension = ""
			}
		}
	}

	requestData := types.RequestData{
		Method:     req.Method,
		HasCookies: len(req.Cookies()) > 0,
		HasParams:  len(req.URL.Query()) > 0,
		Length:     req.ContentLength,
		Headers:    grrhttp.GetHeaders(req.Header),
		Url:        req.URL.RequestURI(),
		Path:       req.URL.Path,
		Query:      req.URL.RawQuery,
		Fragment:   req.URL.RawFragment,
		Ext:        extension,
	}

	// Build UserData
	userdata := types.UserData{
		ID:         id,
		Index:      float64(index),
		Req:        id,
		Resp:       id,
		ReqEdited:  id,
		RespEdited: id,
		Attached:   id,
		Host:       hostWithScheme,
		Port:       port,
		HasResp:    false,
		IsHTTPS:    scheme == "https",
		ReqJson:    requestData,
		RespJson: types.ResponseData{
			Title:      "",
			Mime:       "",
			Status:     0,
			Length:     0,
			HasCookies: false,
			Headers:    make(map[string]string),
		},
		IsReqEdited:  false,
		IsRespEdited: false,
	}

	// Dump request to raw string
	// httputil.DumpRequest with body=true automatically restores the body
	requestInBytes, _ := httputil.DumpRequest(req, true)
	requestInString := string(requestInBytes)

	// Track bytes
	rp.stats.BytesRequest.Add(uint64(len(requestInString)))

	// Save to database synchronously (not in goroutine) to ensure it completes
	rp.saveRequestToDB(&userdata, requestInString)

	// Create sitemap entry
	go func() {
		typ := "folder"
		if userdata.ReqJson.Ext != "" {
			typ = "file"
		}

		sitemapData := types.SitemapGet{
			Host:     userdata.Host,
			Path:     userdata.ReqJson.Path,
			Query:    userdata.ReqJson.Query,
			Fragment: userdata.ReqJson.Fragment,
			Ext:      userdata.ReqJson.Ext,
			Type:     typ,
			Data:     userdata.ID,
		}

		if err := rp.backend.handleSitemapNew(&sitemapData); err != nil {
			log.Printf("[RawProxy][Sitemap][ERROR] Failed to create sitemap entry ID=%s: %v", userdata.ID, err)
		} else {
			log.Printf("[RawProxy][Sitemap][SUCCESS] Created sitemap entry ID=%s", userdata.ID)
		}
	}()

	// Store request context in reqData.Data for response correlation (thread-safe!)
	// rawproxy will pass this same reqData to onResponse
	reqData.Data = &RequestContext{
		UserData:     userdata,
		RawRequest:   requestInString,
		RequestStart: time.Now(),
	}

	// requestJson := utils.StructToMap(&userdata, "json")

	// if rp.Intercept && rp.checkFilters(requestJson) {
	if rp.Intercept {
		log.Printf("[RawProxy][Intercept] Request intercepted: ID=%s", id)

		updatedString, edited := rp.interceptWait(&userdata, "req", req.ContentLength, requestInString)

		if userdata.Action == "drop" {
			log.Printf("[RawProxy][Intercept][%s] Dropping request\n", userdata.Host+"/"+userdata.ReqJson.Path)

			// Save the drop action to database
			go rp.saveRequestToDB(&userdata, requestInString)

			// Return error to signal the request should not proceed
			return nil, fmt.Errorf("request dropped by intercept")
		}

		if edited {
			userdata.IsReqEdited = true
			log.Printf("[RawProxy][Intercept][%s] Request was edited\n", id)

			// Save edited request to database
			go rp.saveEditedRequest(&userdata, updatedString)

			// Convert string back to request
			req.Body.Close()
			requestNew, err := http.ReadRequest(bufio.NewReader(strings.NewReader(updatedString)))
			if err != nil {
				log.Printf("[RawProxy][Intercept][%s][ERROR] Failed to parse edited request: %v\n", id, err)
				return req, fmt.Errorf("failed to parse edited request: %w", err)
			}

			return requestNew, nil
		}
	}

	log.Printf("[RawProxy][Request] ID=%s Host=%s Path=%s", id, hostWithScheme, req.URL.Path)

	return req, nil
}

// onResponse handles HTTP responses and updates the database
func (rp *RawProxyWrapper) onResponse(reqData *rawproxy.RequestData, resp *http.Response, req *http.Request) (*http.Response, error) {
	// Track total responses
	rp.stats.ResponsesTotal.Add(1)

	// Get request context from reqData.Data
	reqCtx, ok := reqData.Data.(*RequestContext)
	if !ok || reqCtx == nil {
		log.Printf("[RawProxy][Response] Warning: No request context found for requestID=%s", reqData.RequestID)
		return resp, nil
	}

	responseData := types.ResponseData{
		HasCookies: len(resp.Cookies()) > 0,
		Title:      "",
		Mime:       resp.Header.Get("Content-Type"),
		Headers:    grrhttp.GetHeaders(resp.Header),
		Status:     resp.StatusCode,
		Length:     resp.ContentLength,
		Date:       resp.Header.Get("Date"),
		Time:       time.Now().Format(time.RFC3339),
	}

	// Update userdata with response information
	userdata := reqCtx.UserData
	userdata.HasResp = true
	userdata.RespJson = responseData

	// Dump response to raw string
	responseInString := grrhttp.DumpResponse(resp)

	// Track bytes
	rp.stats.BytesResponse.Add(uint64(len(responseInString)))

	// Extract title if HTML
	title, _ := utils.ExtractTitle([]byte(responseInString))
	userdata.RespJson.Title = title

	// Save response to database synchronously (not in goroutine) to ensure it completes
	rp.saveResponseToDB(&userdata, responseInString)

	// Check if response should be intercepted
	// responseJson := utils.StructToMap(&userdata, "json")

	// if rp.Intercept && rp.checkFilters(responseJson) {
	if rp.Intercept {
		log.Printf("[RawProxy][Intercept] Response intercepted: ID=%s", userdata.ID)

		updatedString, edited := rp.interceptWait(&userdata, "resp", resp.ContentLength, responseInString)

		if userdata.Action == "drop" {
			log.Printf("[RawProxy][Intercept][%s] Dropping response\n", userdata.Host+"/"+userdata.ReqJson.Path)

			// Save the drop action to database
			go rp.saveResponseToDB(&userdata, responseInString)

			// Return error to signal the response should not proceed
			return nil, fmt.Errorf("response dropped by intercept")
		}

		if edited {
			userdata.IsRespEdited = true
			log.Printf("[RawProxy][Intercept][%s] Response was edited\n", userdata.ID)

			// Save edited response to database
			go rp.saveEditedResponse(&userdata, updatedString)

			// Parse the edited response string back to http.Response
			resp.Body.Close()

			// Parse response from string
			responseReader := bufio.NewReader(strings.NewReader(updatedString))
			respNew, err := http.ReadResponse(responseReader, req)
			if err != nil {
				log.Printf("[RawProxy][Intercept][%s][ERROR] Failed to parse edited response: %v\n", userdata.ID, err)
				return resp, fmt.Errorf("failed to parse edited response: %w", err)
			}

			// Update the response
			return respNew, nil
		}
	}

	// No cleanup needed - reqData is automatically garbage collected after this function returns

	log.Printf("[RawProxy][Response] ID=%s Status=%d Host=%s", userdata.ID, resp.StatusCode, userdata.Host)

	return resp, nil
}

// saveRequestToDB saves the request data to the database collections
func (rp *RawProxyWrapper) saveRequestToDB(userdata *types.UserData, rawRequest string) {
	if rp.backend == nil || rp.backend.App == nil {
		log.Println("[RawProxy][DB][ERROR] Backend or App is nil")
		return
	}

	startTime := time.Now()
	dao := rp.backend.App.Dao()

	log.Printf("[RawProxy][DB][REQUEST] Saving ID=%s Index=%d Method=%s Host=%s Path=%s",
		userdata.ID, int(userdata.Index), userdata.ReqJson.Method, userdata.Host, userdata.ReqJson.Path)

	// Create _attached record
	attachedRecord := models.NewRecord(rp.attachedCollection)
	attachedRecord.Set("id", userdata.ID)
	attachedRecord.Set("labels", []string{})
	attachedRecord.Set("note", "")

	if err := dao.SaveRecord(attachedRecord); err != nil {
		log.Printf("[RawProxy][DB][ERROR] Failed to save _attached record ID=%s: %v", userdata.ID, err)
		rp.stats.RequestsFailed.Add(1)
		return
	}
	log.Printf("[RawProxy][DB][SUCCESS] Saved _attached record ID=%s", userdata.ID)

	// Create _req record with raw request data
	reqRecord := models.NewRecord(rp.reqCollection)
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
	reqRecord.Set("raw", rawRequest)

	if err := dao.SaveRecord(reqRecord); err != nil {
		log.Printf("[RawProxy][DB][ERROR] ============================================")
		log.Printf("[RawProxy][DB][ERROR] FAILED TO SAVE _req RECORD!")
		log.Printf("[RawProxy][DB][ERROR] ID: %s", userdata.ID)
		log.Printf("[RawProxy][DB][ERROR] Error: %v", err)
		log.Printf("[RawProxy][DB][ERROR] Error Type: %T", err)
		log.Printf("[RawProxy][DB][ERROR] Raw request size: %d bytes", len(rawRequest))
		log.Printf("[RawProxy][DB][ERROR] Method: %s", userdata.ReqJson.Method)
		log.Printf("[RawProxy][DB][ERROR] URL: %s", userdata.ReqJson.Url)
		log.Printf("[RawProxy][DB][ERROR] ============================================")
		rp.stats.RequestsFailed.Add(1)
		return
	}
	log.Printf("[RawProxy][DB][SUCCESS] Saved _req record ID=%s (raw size: %d bytes)",
		userdata.ID, len(rawRequest))

	// Create _data record
	dataRecord := models.NewRecord(rp.dataCollection)
	dataRecord.Set("id", userdata.ID)
	dataRecord.Set("index", userdata.Index)
	dataRecord.Set("host", userdata.Host)
	dataRecord.Set("port", userdata.Port)
	dataRecord.Set("req", userdata.Req)
	dataRecord.Set("resp", userdata.Resp)
	dataRecord.Set("has_resp", userdata.HasResp)
	dataRecord.Set("is_req_edited", userdata.IsReqEdited)
	dataRecord.Set("is_resp_edited", userdata.IsRespEdited)
	dataRecord.Set("is_https", userdata.IsHTTPS)
	dataRecord.Set("has_params", userdata.ReqJson.HasParams)
	dataRecord.Set("req_json", userdata.ReqJson)
	dataRecord.Set("resp_json", userdata.RespJson)
	dataRecord.Set("attached", userdata.ID)

	if err := dao.SaveRecord(dataRecord); err != nil {
		// Check if it's a unique constraint violation on index
		if strings.Contains(err.Error(), "UNIQUE constraint failed") && strings.Contains(err.Error(), "index") {
			log.Printf("[RawProxy][DB][ERROR] DUPLICATE INDEX! Failed to save _data record ID=%s Index=%d: %v",
				userdata.ID, int(userdata.Index), err)
			log.Printf("[RawProxy][DB][ERROR] This indicates the index counter is out of sync with the database!")
		} else {
			log.Printf("[RawProxy][DB][ERROR] Failed to save _data record ID=%s Index=%d: %v",
				userdata.ID, int(userdata.Index), err)
		}
		rp.stats.RequestsFailed.Add(1)
		return
	} else {
		log.Printf("[RawProxy][DB][SUCCESS] Saved _data record ID=%s Index=%d", userdata.ID, int(userdata.Index))
	}

	elapsed := time.Since(startTime)

	// Track success
	rp.stats.RequestsSaved.Add(1)

	log.Printf("[RawProxy][DB][COMPLETE] Request ID=%s saved successfully in %v", userdata.ID, elapsed)
}

// saveResponseToDB updates the database with response data
func (rp *RawProxyWrapper) saveResponseToDB(userdata *types.UserData, rawResponse string) {
	if rp.backend == nil || rp.backend.App == nil {
		log.Println("[RawProxy][DB][ERROR] Backend or App is nil")
		return
	}

	startTime := time.Now()
	dao := rp.backend.App.Dao()

	log.Printf("[RawProxy][DB][RESPONSE] Updating ID=%s Status=%d Mime=%s Title=%s Size=%d bytes",
		userdata.ID, userdata.RespJson.Status, userdata.RespJson.Mime, userdata.RespJson.Title, len(rawResponse))

	// Create _resp record with raw response data
	respRecord := models.NewRecord(rp.respCollection)
	respRecord.Set("id", userdata.ID)
	respRecord.Set("title", userdata.RespJson.Title)
	respRecord.Set("mime", userdata.RespJson.Mime)
	respRecord.Set("status", userdata.RespJson.Status)
	respRecord.Set("length", userdata.RespJson.Length)
	respRecord.Set("has_cookies", userdata.RespJson.HasCookies)
	respRecord.Set("headers", userdata.RespJson.Headers)
	respRecord.Set("raw", rawResponse)

	if err := dao.SaveRecord(respRecord); err != nil {
		log.Printf("[RawProxy][DB][ERROR] ============================================")
		log.Printf("[RawProxy][DB][ERROR] FAILED TO SAVE _resp RECORD!")
		log.Printf("[RawProxy][DB][ERROR] ID: %s", userdata.ID)
		log.Printf("[RawProxy][DB][ERROR] Error: %v", err)
		log.Printf("[RawProxy][DB][ERROR] Error Type: %T", err)
		log.Printf("[RawProxy][DB][ERROR] Raw response size: %d bytes", len(rawResponse))
		log.Printf("[RawProxy][DB][ERROR] Status: %d", userdata.RespJson.Status)
		log.Printf("[RawProxy][DB][ERROR] ============================================")
		rp.stats.ResponsesFailed.Add(1)
		return
	}
	log.Printf("[RawProxy][DB][SUCCESS] Saved _resp record ID=%s (raw size: %d bytes)",
		userdata.ID, len(rawResponse))

	// Update _data record with response info
	dataRecord, err := dao.FindRecordById("_data", userdata.ID)
	if err != nil {
		log.Printf("[RawProxy][DB][ERROR] Failed to find _data record ID=%s for update: %v", userdata.ID, err)
		return
	}

	// Normalize MIME type
	originalMime := userdata.RespJson.Mime
	userdata.RespJson.Mime = strings.ToLower(userdata.RespJson.Mime)
	userdata.RespJson.Mime = strings.ReplaceAll(userdata.RespJson.Mime, "\"", "")
	userdata.RespJson.Mime = strings.ReplaceAll(userdata.RespJson.Mime, "'", "")
	userdata.RespJson.Mime = strings.ReplaceAll(userdata.RespJson.Mime, " ", "")

	if originalMime != userdata.RespJson.Mime {
		log.Printf("[RawProxy][DB][INFO] Normalized MIME: %s -> %s", originalMime, userdata.RespJson.Mime)
	}

	dataRecord.Set("resp", userdata.Resp)
	dataRecord.Set("has_resp", userdata.HasResp)
	dataRecord.Set("resp_json", userdata.RespJson)
	if err := dao.SaveRecord(dataRecord); err != nil {
		log.Printf("[RawProxy][DB][ERROR] Failed to update _data record ID=%s: %v", userdata.ID, err)
	} else {
		log.Printf("[RawProxy][DB][SUCCESS] Updated _data record ID=%s with response metadata", userdata.ID)
	}

	elapsed := time.Since(startTime)

	// Track success
	rp.stats.ResponsesSaved.Add(1)

	log.Printf("[RawProxy][DB][COMPLETE] Response ID=%s updated successfully in %v", userdata.ID, elapsed)
}

// PrintStats logs the current proxy statistics
func (rp *RawProxyWrapper) PrintStats() {
	reqTotal := rp.stats.RequestsTotal.Load()
	respTotal := rp.stats.ResponsesTotal.Load()
	reqSaved := rp.stats.RequestsSaved.Load()
	respSaved := rp.stats.ResponsesSaved.Load()
	reqFailed := rp.stats.RequestsFailed.Load()
	respFailed := rp.stats.ResponsesFailed.Load()
	bytesReq := rp.stats.BytesRequest.Load()
	bytesResp := rp.stats.BytesResponse.Load()

	log.Println("=====================================")
	log.Println("[RawProxy][STATS] Proxy Statistics")
	log.Println("=====================================")
	log.Printf("[RawProxy][STATS] Requests:  Total=%d Saved=%d Failed=%d", reqTotal, reqSaved, reqFailed)
	log.Printf("[RawProxy][STATS] Responses: Total=%d Saved=%d Failed=%d", respTotal, respSaved, respFailed)
	log.Printf("[RawProxy][STATS] Data Transfer: Request=%s Response=%s Total=%s",
		formatBytes(bytesReq), formatBytes(bytesResp), formatBytes(bytesReq+bytesResp))

	if reqTotal > 0 {
		saveRate := float64(reqSaved) / float64(reqTotal) * 100
		log.Printf("[RawProxy][STATS] Save Rate: %.2f%%", saveRate)
	}
	log.Println("=====================================")
}

// formatBytes converts bytes to human-readable format
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// saveEditedRequest saves the edited request to the database
func (rp *RawProxyWrapper) saveEditedRequest(userdata *types.UserData, editedRequest string) {
	if rp.backend == nil || rp.backend.App == nil {
		log.Println("[RawProxy][DB][ERROR] Backend or App is nil")
		return
	}

	dao := rp.backend.App.Dao()
	id := userdata.ID

	log.Printf("[RawProxy][DB][EDIT] Saving edited request for ID=%s", id)

	// Create _req_edited record with edited request data
	reqEditedRecord := models.NewRecord(rp.reqEditedCollection)
	reqEditedRecord.Set("id", id)
	reqEditedRecord.Set("method", userdata.ReqJson.Method)
	reqEditedRecord.Set("url", userdata.ReqJson.Url)
	reqEditedRecord.Set("path", userdata.ReqJson.Path)
	reqEditedRecord.Set("query", userdata.ReqJson.Query)
	reqEditedRecord.Set("fragment", userdata.ReqJson.Fragment)
	reqEditedRecord.Set("ext", userdata.ReqJson.Ext)
	reqEditedRecord.Set("has_cookies", userdata.ReqJson.HasCookies)
	reqEditedRecord.Set("length", userdata.ReqJson.Length)
	reqEditedRecord.Set("headers", userdata.ReqJson.Headers)
	reqEditedRecord.Set("raw", editedRequest)

	if err := dao.SaveRecord(reqEditedRecord); err != nil {
		log.Printf("[RawProxy][DB][ERROR] Failed to save edited request to _req_edited ID=%s: %v", id, err)
		return
	}
	log.Printf("[RawProxy][DB][SUCCESS] Saved edited request to _req_edited ID=%s", id)

	// Update _data record with is_req_edited flag and req_edited_json
	dataRecord, err := dao.FindRecordById("_data", id)
	if err != nil {
		log.Printf("[RawProxy][DB][ERROR] Failed to find _data record ID=%s: %v", id, err)
		return
	}

	dataRecord.Set("is_req_edited", true)
	dataRecord.Set("req_edited", id)
	dataRecord.Set("req_edited_json", userdata.ReqJson)
	if err := dao.SaveRecord(dataRecord); err != nil {
		log.Printf("[RawProxy][DB][ERROR] Failed to update is_req_edited flag ID=%s: %v", id, err)
		return
	}
	log.Printf("[RawProxy][DB][SUCCESS] Updated is_req_edited flag for ID=%s", id)
}

// saveEditedResponse saves the edited response to the database
func (rp *RawProxyWrapper) saveEditedResponse(userdata *types.UserData, editedResponse string) {
	if rp.backend == nil || rp.backend.App == nil {
		log.Println("[RawProxy][DB][ERROR] Backend or App is nil")
		return
	}

	dao := rp.backend.App.Dao()
	id := userdata.ID

	log.Printf("[RawProxy][DB][EDIT] Saving edited response for ID=%s", id)

	// Create _resp_edited record with edited response data
	respEditedRecord := models.NewRecord(rp.respEditedCollection)
	respEditedRecord.Set("id", id)
	respEditedRecord.Set("title", userdata.RespJson.Title)
	respEditedRecord.Set("mime", userdata.RespJson.Mime)
	respEditedRecord.Set("status", userdata.RespJson.Status)
	respEditedRecord.Set("length", userdata.RespJson.Length)
	respEditedRecord.Set("has_cookies", userdata.RespJson.HasCookies)
	respEditedRecord.Set("headers", userdata.RespJson.Headers)
	respEditedRecord.Set("raw", editedResponse)

	if err := dao.SaveRecord(respEditedRecord); err != nil {
		log.Printf("[RawProxy][DB][ERROR] Failed to save edited response to _resp_edited ID=%s: %v", id, err)
		return
	}
	log.Printf("[RawProxy][DB][SUCCESS] Saved edited response to _resp_edited ID=%s", id)

	// Update _data record with is_resp_edited flag and resp_edited_json
	dataRecord, err := dao.FindRecordById("_data", id)
	if err != nil {
		log.Printf("[RawProxy][DB][ERROR] Failed to find _data record ID=%s: %v", id, err)
		return
	}

	dataRecord.Set("is_resp_edited", true)
	dataRecord.Set("resp_edited", id)
	dataRecord.Set("resp_edited_json", userdata.RespJson)
	if err := dao.SaveRecord(dataRecord); err != nil {
		log.Printf("[RawProxy][DB][ERROR] Failed to update is_resp_edited flag ID=%s: %v", id, err)
		return
	}
	log.Printf("[RawProxy][DB][SUCCESS] Updated is_resp_edited flag for ID=%s", id)
}
