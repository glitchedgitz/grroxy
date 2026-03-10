package app

// RawProxy Wrapper - Integration layer between rawproxy and grroxy
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
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/glitchedgitz/grroxy/grx/rawhttp"
	"github.com/glitchedgitz/grroxy/grx/rawproxy"
	"github.com/glitchedgitz/grroxy/internal/types"
	"github.com/glitchedgitz/grroxy/internal/utils"
	"github.com/glitchedgitz/pocketbase/daos"
	"github.com/glitchedgitz/pocketbase/models"
)

// RawProxyWrapper wraps the rawproxy.Proxy to match our interface
type RawProxyWrapper struct {
	proxy      *rawproxy.Proxy
	config     *rawproxy.Config
	backend    *Backend
	listenAddr string // Store the listen address for this proxy instance
	proxyID    string // Database ID for this proxy instance

	// Goroutine tracking
	wg        sync.WaitGroup // Tracks the proxy goroutine
	stopOnce  sync.Once      // Ensures Stop() is only called once
	isRunning atomic.Bool    // Tracks if proxy is running

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
	wsCollection         *models.Collection // WebSocket messages collection

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
	UserData     map[string]any
	RawRequest   string
	RawResponse  string // Set in onResponse
	RequestStart time.Time
	DataRecord   *models.Record // Single record shared across all operations
}

// NewRawProxyWrapper creates a new rawproxy wrapper with the given configuration
// Set outputDir to empty string ("") to disable file captures
func NewRawProxyWrapper(listenAddr, configDir, outputDir string, backend *Backend, proxyID string) (*RawProxyWrapper, error) {
	wrapper := &RawProxyWrapper{
		backend:    backend,
		listenAddr: listenAddr,
		proxyID:    proxyID,
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
		ReadTimeout:  10 * time.Minute,
		WriteTimeout: 10 * time.Minute,
		IdleTimeout:  10 * time.Minute,
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
	proxy.SetWebSocketMessageHandler(wrapper.onWebSocketMessage)

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

	// WebSocket collection is optional - log warning if not found
	rp.wsCollection, err = dao.FindCollectionByNameOrId("_websockets")
	if err != nil {
		return fmt.Errorf("failed to find _websockets collection: %w", err)
	}

	log.Println("[RawProxy] Successfully cached all collection references")
	return nil
}

// initializeIndex is now handled globally by ProxyManager
// Removed per-proxy index counter in favor of shared global index

// RunProxy starts the proxy server in a non-blocking manner
func (rp *RawProxyWrapper) RunProxy() error {
	if rp.isRunning.Load() {
		return fmt.Errorf("proxy is already running")
	}

	rp.isRunning.Store(true)
	rp.wg.Add(1)

	go func() {
		defer rp.wg.Done()
		defer rp.isRunning.Store(false)

		log.Printf("[RawProxy][RunProxy] Goroutine started for %s", rp.listenAddr)

		if err := rp.proxy.Start(); err != nil && err != http.ErrServerClosed {
			log.Printf("[ERROR] RawProxy server error: %v", err)
		}

		log.Printf("[RawProxy][RunProxy] Goroutine exiting for %s", rp.listenAddr)
	}()

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)

	log.Printf("[RawProxy][RunProxy] Proxy on %s started", rp.listenAddr)
	return nil
}

// Stop gracefully stops the proxy server
func (rp *RawProxyWrapper) Stop() error {
	stopped := false
	rp.stopOnce.Do(func() {
		stopped = true
	})

	if !stopped {
		log.Printf("[RawProxy][Stop] Already stopped or stopping proxy on %s", rp.listenAddr)
		return nil
	}

	log.Printf("[RawProxy] Stopping proxy server on %s...", rp.listenAddr)

	// Check if proxy exists
	if rp.proxy == nil {
		log.Printf("[RawProxy][ERROR] proxy is nil")
		rp.isRunning.Store(false)
		return fmt.Errorf("proxy is nil")
	}

	// Check if actually running
	if !rp.isRunning.Load() {
		log.Printf("[RawProxy][Stop] Proxy on %s was not running", rp.listenAddr)
		return nil
	}

	// Print final statistics
	rp.PrintStats()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	log.Printf("[RawProxy] Calling rawproxy.Stop()...")
	if err := rp.proxy.Stop(ctx); err != nil {
		log.Printf("[RawProxy][ERROR] Error stopping rawproxy: %v", err)
		rp.isRunning.Store(false)
		return err
	}

	// Wait for the goroutine to finish with timeout
	done := make(chan struct{})
	go func() {
		rp.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Printf("[RawProxy][INFO] Proxy on %s stopped successfully, goroutine exited", rp.listenAddr)
	case <-time.After(10 * time.Second):
		log.Printf("[RawProxy][WARN] Timeout waiting for goroutine to exit for proxy on %s", rp.listenAddr)
	}

	rp.isRunning.Store(false)
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

func getExtension(path string) string {
	extension := ""
	if path != "" {
		pathParts := strings.Split(path, "/")
		lastFile := pathParts[len(pathParts)-1]
		if strings.Contains(lastFile, ".") {
			extParts := strings.Split(lastFile, ".")
			extension = "." + extParts[len(extParts)-1]
			if len(extension) > 10 {
				extension = ""
			}
		}
	}
	return extension
}

func generateRequestData(req *http.Request) map[string]any {
	// Dev: check with types.RequestData

	return map[string]any{
		"method":      req.Method,
		"has_cookies": len(req.Cookies()) > 0,
		"has_params":  len(req.URL.Query()) > 0,
		"length":      req.ContentLength,
		"headers":     rawhttp.GetHeaders(req.Header),
		"url":         req.URL.RequestURI(),
		"path":        req.URL.Path,
		"query":       req.URL.RawQuery,
		"fragment":    req.URL.RawFragment,
		"ext":         getExtension(req.URL.Path),
	}
}

func generateResponseData(resp *http.Response) map[string]any {
	// Dev: check with types.ResponseData
	contentLength := resp.ContentLength
	if clStr := strings.TrimSpace(resp.Header.Get("Content-Length")); clStr != "" {
		if parsed, err := strconv.ParseInt(clStr, 10, 64); err == nil {
			contentLength = parsed
		}
	}
	if contentLength < 0 {
		contentLength = 0
	}

	return map[string]any{
		"has_cookies": len(resp.Cookies()) > 0,
		"title":       "",
		"mime":        resp.Header.Get("Content-Type"),
		"headers":     rawhttp.GetHeaders(resp.Header),
		"status":      resp.StatusCode,
		"length":      contentLength,
		"date":        resp.Header.Get("Date"),
		"time":        time.Now().Format(time.RFC3339),
		"proto":       resp.Proto,
	}
}

func updateResponseDataFromRaw(responseData map[string]any, responseRaw string) {
	parsed := rawhttp.ParseResponse([]byte(responseRaw))
	contentLength := int64(len(parsed.Body))
	if clStr, ok := rawhttp.GetHeaderValue(parsed.Headers, "content-length:"); ok {
		if parsedLen, err := strconv.ParseInt(strings.TrimSpace(clStr), 10, 64); err == nil {
			contentLength = parsedLen
		}

		if headers, ok := responseData["headers"].(map[string]string); ok {
			headers["Content-Length"] = strings.TrimSpace(clStr)
		}
	}

	responseData["length"] = contentLength
}

// onRequest handles incoming HTTP requests and saves them to the database
func (rp *RawProxyWrapper) onRequest(reqData *rawproxy.RequestData, req *http.Request) (*http.Request, error) {
	// Skip our own grroxy requests to avoid loops
	if strings.Contains(req.URL.Host, "grroxy") {
		return req, nil
	}

	// Track total requests
	rp.stats.RequestsTotal.Add(1)

	// Generate unique ID and index using the shared global index
	index := ProxyMgr.GetNextIndex()
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

	// Infer scheme based on context:
	// - If scheme is already set (e.g., custom scheme), preserve it
	// - Otherwise, detect based on TLS and WebSocket upgrade header
	scheme := req.URL.Scheme
	if scheme == "" {
		isWebSocket := strings.EqualFold(req.Header.Get("Upgrade"), "websocket")
		isTLS := req.TLS != nil

		switch {
		case isTLS && isWebSocket:
			scheme = "wss"
		case isTLS:
			scheme = "https"
		case isWebSocket:
			scheme = "ws"
		default:
			scheme = "http"
		}
	}
	hostWithScheme := scheme + "://" + host

	// Extract file extension

	requestData := generateRequestData(req)

	// Dev: Uncomment to check the update structure
	// userdata := types.UserData{
	// 	ID:         id,
	// 	Index:      float64(index),
	// 	Req:        id,
	// 	Resp:       id,
	// 	ReqEdited:  id,
	// 	RespEdited: id,
	// 	Attached:   id,
	// 	Host:       hostWithScheme,
	// 	Port:       port,
	// 	HasResp:    false,
	// 	IsHTTPS:    scheme == "https",
	// 	Http:       req.Proto,
	// 	ProxyId:    reqData.RequestID,
	// 	ReqJson:    requestData,
	// 	RespJson: types.ResponseData{
	// 		Title:      "",
	// 		Mime:       "",
	// 		Status:     0,
	// 		Length:     0,
	// 		HasCookies: false,
	// 		Headers:    make(map[string]string),
	// 	},
	// 	IsReqEdited:  false,
	// 	IsRespEdited: false,
	// }

	// Build UserData
	userdata := map[string]any{
		"id":           id,
		"index":        float64(index),
		"req":          id,
		"resp":         id,
		"req_edited":   id,
		"resp_edited":  id,
		"attached":     id,
		"host":         hostWithScheme,
		"port":         port,
		"has_resp":     false,
		"has_params":   len(req.URL.Query()) > 0,
		"is_https":     req.TLS != nil,                      // Secure if TLS is present (works with any scheme)
		"http":         req.Proto,                           // HTTP version: HTTP/1.1, HTTP/2.0, etc.
		"proxy_id":     reqData.RequestID,                   // Proxy request ID from rawproxy: req-00000001
		"generated_by": fmt.Sprintf("proxy/%s", rp.proxyID), // Format: proxy/______________1
		"req_json":     requestData,
		"resp_json": map[string]any{
			"title":       "",
			"mime":        "",
			"status":      0,
			"length":      0,
			"has_cookies": false,
			"headers":     make(map[string]string),
		},
		"is_req_edited":  false,
		"is_resp_edited": false,
	}

	// Dump request to raw string
	normalizeHTTP := (scheme == "http")
	requestInString := rawhttp.DumpRequest(req, normalizeHTTP)

	// Track bytes
	rp.stats.BytesRequest.Add(uint64(len(requestInString)))

	// Create the dataRecord once and store it in RequestContext
	// This single record will be reused throughout the request lifecycle
	dataRecord := models.NewRecord(rp.dataCollection)
	dataRecord.Load(userdata)
	dataRecord.Set("attached", userdata["id"].(string))

	// Create RequestContext to store all request-related data
	reqCtx := &RequestContext{
		UserData:     userdata,
		RawRequest:   requestInString,
		RequestStart: time.Now(),
		DataRecord:   dataRecord,
	}

	// Save to database synchronously (not in goroutine) to ensure it completes
	rp.saveRequestToDB(reqCtx, requestData)

	// Create sitemap entry
	go func() {
		typ := "folder"
		if requestData["ext"] != "" {
			typ = "file"
		}

		sitemapData := types.SitemapGet{
			Host:     userdata["host"].(string),
			Path:     requestData["path"].(string),
			Query:    requestData["query"].(string),
			Fragment: requestData["fragment"].(string),
			Ext:      requestData["ext"].(string),
			Type:     typ,
			Data:     userdata["id"].(string),
		}

		if err := rp.backend.handleSitemapNew(&sitemapData); err != nil {
			log.Printf("[RawProxy][Sitemap][ERROR] Failed to create sitemap entry ID=%s: %v", userdata["id"].(string), err)
		} else {
			log.Printf("[RawProxy][Sitemap][SUCCESS] Created sitemap entry ID=%s", userdata["id"].(string))
		}
	}()

	// Store request context in reqData.Data for response correlation (thread-safe!)
	// rawproxy will pass this same reqData to onResponse
	reqData.Data = reqCtx

	// requestJson := utils.StructToMap(&userdata, "json")

	if rp.Intercept && rp.checkFilters(userdata) {
		log.Printf("[RawProxy][Intercept] Request intercepted: ID=%s", id)

		// Track intercept counters (total, per-proxy)
		rp.backend.CounterManager.IncrementWithStartup("_intercept", "_intercept", "", true)

		var proxyInterceptKey string
		if generatedBy, ok := userdata["generated_by"].(string); ok {
			proxyInterceptKey = generatedBy + "/intercept"
			rp.backend.CounterManager.Increment(proxyInterceptKey, "_intercept", "")
		}

		// Ensure intercept counters are decremented after processing
		defer func() {
			rp.backend.CounterManager.Decrement("_intercept", "_intercept", "")
			if proxyInterceptKey != "" {
				rp.backend.CounterManager.Decrement(proxyInterceptKey, "_intercept", "")
			}
		}()

		updatedString, edited := rp.interceptWait(userdata, "req", req.ContentLength, requestInString)

		if userdata["action"] == "drop" {
			log.Printf("[RawProxy][Intercept][%s] Dropping request\n", userdata["host"].(string)+"/"+requestData["path"].(string))

			// Save the drop action to database
			go rp.saveRequestToDB(reqCtx, requestData)

			// Return error to signal the request should not proceed
			return nil, fmt.Errorf("request dropped by intercept")
		}

		if edited {
			userdata["is_req_edited"] = true
			log.Printf("[RawProxy][Intercept][%s] Request was edited\n", id)

			// Update RawRequest in context with edited version
			reqCtx.RawRequest = updatedString

			// Save edited request to database

			// Convert string back to request
			req.Body.Close()
			requestNew, err := http.ReadRequest(bufio.NewReader(strings.NewReader(updatedString)))
			if err != nil {
				log.Printf("[RawProxy][Intercept][%s][ERROR] Failed to parse edited request: %v\n", id, err)
				return req, fmt.Errorf("failed to parse edited request: %w", err)
			}

			editedRequestData := generateRequestData(requestNew)

			go rp.saveEditedRequest(reqCtx, editedRequestData, updatedString)

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

	responseData := generateResponseData(resp)

	// Update userdata with response information
	userdata := reqCtx.UserData
	userdata["has_resp"] = true
	userdata["resp_json"] = responseData

	// Update the HTTP protocol version if the upstream used a different protocol
	// than what was initially recorded (e.g., browser spoke HTTP/2 to proxy,
	// but upstream only supports HTTP/1.1)
	userdata["http"] = reqData.HttpProto

	// Dump response to raw string
	responseInString := rawhttp.DumpResponse(resp)
	reqCtx.RawResponse = responseInString // Store in context for save functions

	// Track bytes
	rp.stats.BytesResponse.Add(uint64(len(responseInString)))

	// Extract title if HTML
	title, _ := utils.ExtractTitle([]byte(responseInString))
	responseData["title"] = title
	updateResponseDataFromRaw(responseData, responseInString)

	// Save response to database synchronously (not in goroutine) to ensure it completes
	rp.saveResponseToDB(reqCtx, responseData)

	// Check if response should be intercepted
	// responseJson := utils.StructToMap(&userdata, "json")

	if rp.Intercept && rp.checkFilters(userdata) {
		log.Printf("[RawProxy][Intercept] Response intercepted: ID=%s", userdata["id"].(string))

		// Track intercept counters (total, per-proxy, per-host)
		rp.backend.CounterManager.IncrementWithStartup("_intercept", "_intercept", "", true)

		var proxyInterceptKey string
		if generatedBy, ok := userdata["generated_by"].(string); ok {
			proxyInterceptKey = generatedBy + "/intercept"
			rp.backend.CounterManager.Increment(proxyInterceptKey, "_intercept", "")
		}

		// Ensure intercept counters are decremented after processing
		defer func() {
			rp.backend.CounterManager.Decrement("_intercept", "_intercept", "")
			if proxyInterceptKey != "" {
				rp.backend.CounterManager.Decrement(proxyInterceptKey, "_intercept", "")
			}
		}()

		updatedString, edited := rp.interceptWait(userdata, "resp", resp.ContentLength, responseInString)

		if userdata["action"] == "drop" {
			// Extract path from req_json since it's not directly in userdata
			reqJson := userdata["req_json"].(map[string]any)
			log.Printf("[RawProxy][Intercept][%s] Dropping response\n", userdata["host"].(string)+"/"+reqJson["path"].(string))

			// Save the drop action to database
			go rp.saveResponseToDB(reqCtx, responseData)

			// Return error to signal the response should not proceed
			return nil, fmt.Errorf("response dropped by intercept")
		}

		if edited {
			userdata["is_resp_edited"] = true
			log.Printf("[RawProxy][Intercept][%s] Response was edited\n", userdata["id"].(string))

			// Update RawResponse in context with edited version
			reqCtx.RawResponse = updatedString

			// Parse the edited response string back to http.Response
			resp.Body.Close()

			// Parse response from string
			responseReader := bufio.NewReader(strings.NewReader(updatedString))
			respNew, err := http.ReadResponse(responseReader, req)
			if err != nil {
				log.Printf("[RawProxy][Intercept][%s][ERROR] Failed to parse edited response: %v\n", userdata["id"].(string), err)
				return resp, fmt.Errorf("failed to parse edited response: %w", err)
			}

			editedResponseData := generateResponseData(respNew)
			updateResponseDataFromRaw(editedResponseData, updatedString)
			// Save edited response to database
			go rp.saveEditedResponse(reqCtx, editedResponseData, updatedString)

			// Update the response
			return respNew, nil
		}
	}

	// No cleanup needed - reqData is automatically garbage collected after this function returns

	log.Printf("[RawProxy][Response] ID=%s Status=%d Host=%s", userdata["id"].(string), resp.StatusCode, userdata["host"].(string))

	return resp, nil
}

// saveRequestToDB saves the request data to the database collections
func (rp *RawProxyWrapper) saveRequestToDB(reqCtx *RequestContext, requestData map[string]any) {
	if rp.backend == nil || rp.backend.App == nil {
		log.Println("[RawProxy][DB][ERROR] Backend or App is nil")
		return
	}

	startTime := time.Now()
	dao := rp.backend.App.Dao()
	userdata := reqCtx.UserData
	rawRequest := reqCtx.RawRequest
	dataRecord := reqCtx.DataRecord

	log.Printf("[RawProxy][DB][REQUEST] Saving ID=%s Index=%d Method=%s Host=%s Path=%s",
		userdata["id"].(string), int(userdata["index"].(float64)), requestData["method"].(string), userdata["host"].(string), requestData["path"].(string))

	// Create _attached record
	attachedRecord := models.NewRecord(rp.attachedCollection)
	attachedRecord.Set("id", userdata["id"].(string))
	attachedRecord.Set("labels", []string{})
	attachedRecord.Set("note", "")

	// Create _req record with raw request data
	reqRecord := models.NewRecord(rp.reqCollection)
	reqRecord.Load(requestData)
	reqRecord.Set("id", userdata["id"].(string))
	reqRecord.Set("raw", rawRequest)

	handleAttachRecordError := func(err error) error {
		log.Printf("[RawProxy][DB][ERROR] Failed to save _attached record ID=%s: %v", userdata["id"].(string), err)
		rp.stats.RequestsFailed.Add(1)
		return err
	}

	handleReqRecordError := func(err error) error {
		log.Printf("[RawProxy][DB][ERROR] Failed to save _req record ID=%s: %v", userdata["id"].(string), err)
		if err := dao.SaveRecord(reqRecord); err != nil {
			log.Printf("[RawProxy][DB][ERROR] ============================================")
			log.Printf("[RawProxy][DB][ERROR] FAILED TO SAVE _req RECORD!")
			log.Printf("[RawProxy][DB][ERROR] ID: %s", userdata["id"].(string))
			log.Printf("[RawProxy][DB][ERROR] Error: %v", err)
			log.Printf("[RawProxy][DB][ERROR] Error Type: %T", err)
			log.Printf("[RawProxy][DB][ERROR] Raw request size: %d bytes", len(rawRequest))
			log.Printf("[RawProxy][DB][ERROR] Method: %s", requestData["method"].(string))
			log.Printf("[RawProxy][DB][ERROR] URL: %s", requestData["url"].(string))
			log.Printf("[RawProxy][DB][ERROR] ============================================")
			rp.stats.RequestsFailed.Add(1)
			return err
		}
		rp.stats.RequestsFailed.Add(1)
		return err
	}

	handleDataRecordError := func(err error) error {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") && strings.Contains(err.Error(), "index") {
			log.Printf("[RawProxy][DB][ERROR] DUPLICATE INDEX! Failed to save _data record ID=%s Index=%d: %v",
				userdata["id"].(string), int(userdata["index"].(float64)), err)
			log.Printf("[RawProxy][DB][ERROR] This indicates the index counter is out of sync with the database!")
		} else {
			log.Printf("[RawProxy][DB][ERROR] Failed to save _data record ID=%s Index=%d: %v",
				userdata["id"].(string), int(userdata["index"].(float64)), err)
		}
		rp.stats.RequestsFailed.Add(1)
		return err
	}

	err := dao.RunInTransaction(func(txDao *daos.Dao) error {
		if err := txDao.SaveRecord(attachedRecord); err != nil {
			return handleAttachRecordError(err)
		}
		if err := txDao.SaveRecord(reqRecord); err != nil {
			return handleReqRecordError(err)
		}
		if err := txDao.SaveRecord(dataRecord); err != nil {
			return handleDataRecordError(err)
		}
		return nil
	})

	if err != nil {
		log.Printf("[RawProxy][DB][ERROR] Failed to save _data record ID=%s Index=%d: %v",
			userdata["id"].(string), int(userdata["index"].(float64)), err)
		rp.stats.RequestsFailed.Add(1)
		return
	} else {
		dataRecord.MarkAsNotNew()
	}

	elapsed := time.Since(startTime)

	// Track success
	rp.stats.RequestsSaved.Add(1)

	// Increment counters (atomic operations)
	// Total requests counter for _data (load_on_startup - recalculated from DB)
	rp.backend.CounterManager.IncrementWithStartup("_data", "_data", "", true)

	// Per-proxy counter (immediate sync for exact counts)
	if generatedBy, ok := userdata["generated_by"].(string); ok {
		rp.backend.CounterManager.Increment(generatedBy, "_data", "")
	}

	// Per-host counter (immediate sync for exact counts)
	if host, ok := userdata["host"].(string); ok {
		sitemapCollectionName := utils.ParseDatabaseName(host)
		rp.backend.CounterManager.Increment("host:"+sitemapCollectionName, "_data", "")
	}

	log.Printf("[RawProxy][DB][COMPLETE] Request ID=%s saved successfully in %v", userdata["id"].(string), elapsed)
}

// saveResponseToDB updates the database with response data
func (rp *RawProxyWrapper) saveResponseToDB(reqCtx *RequestContext, responseData map[string]any) {
	if rp.backend == nil || rp.backend.App == nil {
		log.Println("[RawProxy][DB][ERROR] Backend or App is nil")
		return
	}

	startTime := time.Now()
	dao := rp.backend.App.Dao()
	userdata := reqCtx.UserData
	rawResponse := reqCtx.RawResponse
	dataRecord := reqCtx.DataRecord

	log.Printf("[RawProxy][DB][RESPONSE] Updating ID=%s Status=%d Mime=%s Title=%s Size=%d bytes",
		userdata["id"].(string), responseData["status"].(int), responseData["mime"].(string), responseData["title"].(string), len(rawResponse))

	// Create _resp record with raw response data
	respRecord := models.NewRecord(rp.respCollection)
	respRecord.Load(responseData)
	respRecord.Set("id", userdata["id"].(string))
	respRecord.Set("raw", rawResponse)

	if err := dao.SaveRecord(respRecord); err != nil {
		log.Printf("[RawProxy][DB][ERROR] ============================================")
		log.Printf("[RawProxy][DB][ERROR] FAILED TO SAVE _resp RECORD!")
		log.Printf("[RawProxy][DB][ERROR] ID: %s", userdata["id"].(string))
		log.Printf("[RawProxy][DB][ERROR] Error: %v", err)
		log.Printf("[RawProxy][DB][ERROR] Error Type: %T", err)
		log.Printf("[RawProxy][DB][ERROR] Raw response size: %d bytes", len(rawResponse))
		log.Printf("[RawProxy][DB][ERROR] Status: %d", responseData["status"].(int))
		log.Printf("[RawProxy][DB][ERROR] ============================================")
		rp.stats.ResponsesFailed.Add(1)
		return
	}
	log.Printf("[RawProxy][DB][SUCCESS] Saved _resp record ID=%s (raw size: %d bytes)",
		userdata["id"].(string), len(rawResponse))

	// Normalize MIME type
	originalMime := responseData["mime"].(string)
	responseData["mime"] = strings.ToLower(responseData["mime"].(string))
	responseData["mime"] = strings.ReplaceAll(responseData["mime"].(string), "\"", "")
	responseData["mime"] = strings.ReplaceAll(responseData["mime"].(string), "'", "")
	responseData["mime"] = strings.ReplaceAll(responseData["mime"].(string), " ", "")

	if originalMime != responseData["mime"].(string) {
		log.Printf("[RawProxy][DB][INFO] Normalized MIME: %s -> %s", originalMime, responseData["mime"].(string))
	}

	dataRecord.Set("resp", userdata["resp"].(string))
	dataRecord.Set("http", userdata["http"].(string))
	dataRecord.Set("has_resp", userdata["has_resp"].(bool))
	dataRecord.Set("resp_json", responseData)
	if err := dao.SaveRecord(dataRecord); err != nil {
		log.Printf("[RawProxy][DB][ERROR] Failed to update _data record ID=%s: %v", userdata["id"].(string), err)
	} else {
		log.Printf("[RawProxy][DB][SUCCESS] Updated _data record ID=%s with response metadata", userdata["id"].(string))
	}

	elapsed := time.Since(startTime)

	// Track success
	rp.stats.ResponsesSaved.Add(1)

	log.Printf("[RawProxy][DB][COMPLETE] Response ID=%s updated successfully in %v", userdata["id"].(string), elapsed)
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
func (rp *RawProxyWrapper) saveEditedRequest(reqCtx *RequestContext, requestData map[string]any, editedRequest string) {
	if rp.backend == nil || rp.backend.App == nil {
		log.Println("[RawProxy][DB][ERROR] Backend or App is nil")
		return
	}

	dao := rp.backend.App.Dao()
	userdata := reqCtx.UserData
	dataRecord := reqCtx.DataRecord
	id := userdata["id"].(string)

	log.Printf("[RawProxy][DB][EDIT] Saving edited request for ID=%s", id)

	// Create _req_edited record with edited request data
	reqEditedRecord := models.NewRecord(rp.reqEditedCollection)
	reqEditedRecord.Set("id", id)
	reqEditedRecord.Load(requestData)
	reqEditedRecord.Set("raw", editedRequest)

	if err := dao.SaveRecord(reqEditedRecord); err != nil {
		log.Printf("[RawProxy][DB][ERROR] Failed to save edited request to _req_edited ID=%s: %v", id, err)
		return
	}
	log.Printf("[RawProxy][DB][SUCCESS] Saved edited request to _req_edited ID=%s", id)

	// Use the shared dataRecord and mark as not new for update
	dataRecord.Set("is_req_edited", true)
	dataRecord.Set("req_edited", id)
	dataRecord.Set("req_edited_json", requestData)
	if err := dao.SaveRecord(dataRecord); err != nil {
		log.Printf("[RawProxy][DB][ERROR] Failed to update is_req_edited flag ID=%s: %v", id, err)
		return
	}
	log.Printf("[RawProxy][DB][SUCCESS] Updated is_req_edited flag for ID=%s", id)
}

// saveEditedResponse saves the edited response to the database
func (rp *RawProxyWrapper) saveEditedResponse(reqCtx *RequestContext, responseData map[string]any, editedResponse string) {
	if rp.backend == nil || rp.backend.App == nil {
		log.Println("[RawProxy][DB][ERROR] Backend or App is nil")
		return
	}

	dao := rp.backend.App.Dao()
	userdata := reqCtx.UserData
	dataRecord := reqCtx.DataRecord
	id := userdata["id"].(string)

	log.Printf("[RawProxy][DB][EDIT] Saving edited response for ID=%s", id)

	// Create _resp_edited record with edited response data
	respEditedRecord := models.NewRecord(rp.respEditedCollection)
	respEditedRecord.Set("id", id)
	respEditedRecord.Load(responseData)
	respEditedRecord.Set("raw", editedResponse)

	if err := dao.SaveRecord(respEditedRecord); err != nil {
		log.Printf("[RawProxy][DB][ERROR] Failed to save edited response to _resp_edited ID=%s: %v", id, err)
		return
	}
	log.Printf("[RawProxy][DB][SUCCESS] Saved edited response to _resp_edited ID=%s", id)

	// Use the shared dataRecord and mark as not new for update
	dataRecord.MarkAsNotNew()
	dataRecord.Set("is_resp_edited", true)
	dataRecord.Set("resp_edited", id)
	dataRecord.Set("resp_edited_json", responseData)
	if err := dao.SaveRecord(dataRecord); err != nil {
		log.Printf("[RawProxy][DB][ERROR] Failed to update is_resp_edited flag ID=%s: %v", id, err)
		return
	}
	log.Printf("[RawProxy][DB][SUCCESS] Updated is_resp_edited flag for ID=%s", id)
}

// onWebSocketMessage handles incoming WebSocket messages and saves them to the database
func (rp *RawProxyWrapper) onWebSocketMessage(msg *rawproxy.WebSocketMessage) error {
	// Start a goroutine to save to DB (non-blocking for the message flow)
	go rp.saveWebSocketMessageToDB(msg)
	return nil
}

// saveWebSocketMessageToDB saves a single WebSocket message to the database
func (rp *RawProxyWrapper) saveWebSocketMessageToDB(msg *rawproxy.WebSocketMessage) {
	if rp.backend == nil || rp.backend.App == nil {
		return
	}

	if rp.wsCollection == nil {
		return
	}

	dao := rp.backend.App.Dao()

	// Handle payload - for binary data, encode as base64
	var payloadStr string
	if msg.IsBinary {
		payloadStr = base64.StdEncoding.EncodeToString(msg.Payload)
	} else {
		payloadStr = string(msg.Payload)
	}

	var id = ""
	var dataIndex = ""
	var generatedBy = ""

	// Find the parent _data record to get its user-friendly index
	// The WebSocket RequestID corresponds to the proxy_id in the _data collection
	if dataRecord, err := dao.FindFirstRecordByData(rp.dataCollection.Name, "proxy_id", msg.RequestID); err == nil {
		dataIndex = dataRecord.GetString("index")
		generatedBy = dataRecord.GetString("generated_by")
	}

	// id = fmt.Sprintf("%s.%d", dataIndex, msg.Index)
	id = utils.FormatStringID(fmt.Sprintf("%v.%v", dataIndex, msg.Index), 15)

	record := models.NewRecord(rp.wsCollection)
	record.Set("id", id)
	record.Set("index", msg.Index)
	record.Set("host", msg.Host)
	record.Set("path", msg.Path)
	record.Set("url", msg.URL)
	record.Set("direction", msg.Direction)
	record.Set("type", msg.Type)
	record.Set("is_binary", msg.IsBinary)
	record.Set("payload", payloadStr)
	record.Set("length", len(msg.Payload))
	record.Set("timestamp", msg.Timestamp)
	record.Set("proxy_id", msg.RequestID)
	record.Set("data_index", dataIndex)
	record.Set("generated_by", generatedBy)

	if err := dao.SaveRecord(record); err != nil {
		log.Printf("[RawProxy][DB][ERROR] Failed to save WebSocket message for %s: %v", msg.RequestID, err)
	}
}
