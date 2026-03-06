package tools

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/glitchedgitz/grroxy/grx/fuzzer"
	"github.com/glitchedgitz/grroxy/grx/rawhttp"
	"github.com/glitchedgitz/grroxy/internal/schemas"
	"github.com/glitchedgitz/grroxy/internal/sdk"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/daos"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/models/schema"
	pbTypes "github.com/pocketbase/pocketbase/tools/types"
)

type FuzzerManager struct {
	instances map[string]*fuzzer.Fuzzer
	mu        sync.RWMutex
}

var FuzzerMgr = &FuzzerManager{
	instances: make(map[string]*fuzzer.Fuzzer),
}

type FuzzerStartRequest struct {
	Collection  string         `json:"collection"`
	Request     string         `json:"request"`
	Host        string         `json:"host"`
	Port        string         `json:"port"`
	UseTLS      bool           `json:"useTLS"`
	UseHTTP2    bool           `json:"http2"`    // Enable HTTP/2 support
	Markers     map[string]any `json:"markers"` // marker -> string (file path) or []string (inline payloads)
	Mode        string         `json:"mode"`
	Concurrency int            `json:"concurrency"`
	Timeout     float64        `json:"timeout"` // in seconds
	ProcessData any            `json:"process_data"`
	GeneratedBy string         `json:"generated_by"`
}

// CreateCollection creates a collection with the specified schema
func (backend *Tools) CreateCollection(collectionName string, dbSchema schema.Schema) error {
	collection := &models.Collection{
		Name:       collectionName,
		Type:       models.CollectionTypeBase,
		ListRule:   nil,
		ViewRule:   pbTypes.Pointer(""),
		CreateRule: pbTypes.Pointer(""),
		UpdateRule: pbTypes.Pointer(""),
		DeleteRule: nil,
		Schema:     dbSchema,
	}

	if err := backend.App.Dao().SaveCollection(collection); err != nil {
		return err
	}

	return nil
}

// parseAndSaveResult parses the request and response using rawhttp and returns data for saving
func parseAndSaveResult(rawRequest, rawResponse string) map[string]any {
	data := make(map[string]any)

	// Save raw request and response
	data["raw_request"] = rawRequest
	data["raw_response"] = rawResponse

	// Parse request
	parsedReq := rawhttp.ParseRequest([]byte(rawRequest))
	data["req_method"] = parsedReq.Method
	data["req_url"] = parsedReq.URL
	data["req_version"] = parsedReq.HTTPVersion

	// Convert headers to JSON
	if len(parsedReq.Headers) > 0 {
		reqHeadersJSON, err := json.Marshal(parsedReq.Headers)
		if err == nil {
			var headers interface{}
			if err := json.Unmarshal(reqHeadersJSON, &headers); err == nil {
				data["req_headers"] = headers
			}
		}
	}

	// Parse response
	parsedResp := rawhttp.ParseResponse([]byte(rawResponse))
	data["resp_version"] = parsedResp.Version
	data["resp_status"] = parsedResp.Status
	data["resp_status_full"] = parsedResp.StatusFull
	data["resp_length"] = len(rawResponse)

	// Convert headers to JSON
	if len(parsedResp.Headers) > 0 {
		respHeadersJSON, err := json.Marshal(parsedResp.Headers)
		if err == nil {
			var headers interface{}
			if err := json.Unmarshal(respHeadersJSON, &headers); err == nil {
				data["resp_headers"] = headers
			}
		}
	}

	return data
}

func (backend *Tools) StartFuzzer(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/fuzzer/start",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil

			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			var body FuzzerStartRequest
			if err := c.Bind(&body); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{
					"status":     "error",
					"process_id": "",
					"fuzzer_id":  "",
					"error":      err.Error(),
				})
			}

			// log.Println("[StartFuzzer] Request:", body)

			// Validate required fields
			if body.Request == "" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{
					"status":     "error",
					"process_id": "",
					"fuzzer_id":  "",
					"error":      "request is required",
				})
			}
			if body.Host == "" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{
					"status":     "error",
					"process_id": "",
					"fuzzer_id":  "",
					"error":      "host is required",
				})
			}
			if len(body.Markers) == 0 {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{
					"status":     "error",
					"process_id": "",
					"fuzzer_id":  "",
					"error":      "markers is required",
				})
			}
			for key, value := range body.Markers {
				switch v := value.(type) {
				case string:
					if v == "" {
						return c.JSON(http.StatusBadRequest, map[string]interface{}{
							"status":     "error",
							"process_id": "",
							"fuzzer_id":  "",
							"error":      fmt.Sprintf("marker '%s' must have a file path", key),
						})
					}
				case []interface{}:
					if len(v) == 0 {
						return c.JSON(http.StatusBadRequest, map[string]interface{}{
							"status":     "error",
							"process_id": "",
							"fuzzer_id":  "",
							"error":      fmt.Sprintf("marker '%s' must have at least one payload", key),
						})
					}
				default:
					return c.JSON(http.StatusBadRequest, map[string]interface{}{
						"status":     "error",
						"process_id": "",
						"fuzzer_id":  "",
						"error":      fmt.Sprintf("marker '%s' must be a string (file path) or array (payloads)", key),
					})
				}
			}

			// Clean host (remove http:// or https://)
			host := strings.TrimPrefix(body.Host, "http://")
			host = strings.TrimPrefix(host, "https://")

			// Convert timeout from seconds to duration
			timeout := time.Duration(body.Timeout) * time.Second
			if timeout == 0 {
				timeout = 10 * time.Second
			}

			// Create process in main app's database using SDK
			if backend.AppSDK == nil {
				return c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
					"status":     "error",
					"process_id": "",
					"fuzzer_id":  "",
					"error":      "Not connected to main app. Please initialize SDK using tools.LoginSDK(url, email, password)",
				})
			}

			// Create fuzzer config
			config := fuzzer.FuzzerConfig{
				Request:     body.Request,
				Host:        host,
				Port:        body.Port,
				UseTLS:      body.UseTLS,
				UseHTTP2:    body.UseHTTP2,
				Markers:     body.Markers,
				Mode:        body.Mode,
				Concurrency: body.Concurrency,
				Timeout:     timeout,
			}

			id, err := backend.AppSDK.CreateProcess(sdk.CreateProcessRequest{
				Name:        "Fuzzer",
				Description: fmt.Sprintf("Fuzzing %s", body.Host),
				Type:        "fuzzer",
				State:       "In Queue",
				Data: map[string]any{
					"request_body": body,
				},
				GeneratedBy: body.GeneratedBy,
			})
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{
					"status":     "error",
					"process_id": "",
					"fuzzer_id":  "",
					"error":      fmt.Sprintf("Failed to create process: %v", err),
				})
			}

			// backend.AppSDK.AddRequest(types.AddRequestBodyType{
			// 	Url:         "",
			// 	Index:       body.,
			// 	Request:     body.Request,
			// 	Response:    `json:"response"`,
			// 	GeneratedBy: `json:"generated_by"`,
			// 	Note:        `json:"note,omitempty"`,
			// })

			// Create collection for this fuzzer
			err = backend.CreateCollection(body.Collection, schemas.Fuzzer)
			if err != nil && !strings.Contains(err.Error(), "UNIQUE constraint failed") {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{
					"status":     "error",
					"process_id": "",
					"fuzzer_id":  "",
					"error":      fmt.Sprintf("Failed to create collection: %v", err),
				})
			}

			collection, err := backend.App.Dao().FindCollectionByNameOrId(body.Collection)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{
					"status":     "error",
					"process_id": "",
					"fuzzer_id":  "",
					"error":      fmt.Sprintf("Failed to find collection: %v", err),
				})
			}

			// Create fuzzer instance
			f := fuzzer.NewFuzzer(config)

			// Store fuzzer instance
			FuzzerMgr.mu.Lock()
			FuzzerMgr.instances[id] = f
			FuzzerMgr.mu.Unlock()

			// Update state to running via SDK
			err = backend.AppSDK.UpdateProcess(id, sdk.ProgressUpdate{
				Completed: 0,
				Total:     100,
				Message:   "Starting fuzzer...",
				State:     "Running",
			})
			if err != nil {
				log.Printf("[StartFuzzer] Failed to update process state: %v", err)
			}

			// Start result processing in a goroutine
			go func() {
				const batchSize = 100
				const flushInterval = 2 * time.Second
				const progressUpdateInterval = 1 * time.Second
				batch := make([]*models.Record, 0, batchSize)
				ticker := time.NewTicker(flushInterval)
				progressTicker := time.NewTicker(progressUpdateInterval)
				defer ticker.Stop()
				defer progressTicker.Stop()

				flush := func() {
					if len(batch) == 0 {
						return
					}
					err := backend.App.Dao().RunInTransaction(func(txDao *daos.Dao) error {
						for _, record := range batch {
							if err := txDao.SaveRecord(record); err != nil {
								return err
							}
						}
						return nil
					})
					if err != nil {
						log.Printf("[StartFuzzer] Failed to save batch for %s: %v", id, err)
					}
					batch = batch[:0]
				}

				for {
					select {
					case result, ok := <-f.Results:
						if !ok {
							flush()
							goto resultsDone
						}
						fuzzerResult, ok := result.(fuzzer.FuzzerResult)
						if !ok {
							log.Printf("[StartFuzzer] Invalid result type: %T", result)
							continue
						}

						// log.Println("[StartFuzzer] markers: ", fuzzerResult.Markers)

						// Parse request and response
						data := parseAndSaveResult(fuzzerResult.Request, fuzzerResult.Response)

						// Add common fields
						data["fuzzer_id"] = id
						data["time"] = fuzzerResult.Time.Nanoseconds()
						data["markers"] = fuzzerResult.Markers

						// Create record
						record := models.NewRecord(collection)
						for key, value := range data {
							record.Set(key, value)
						}
						batch = append(batch, record)

						if len(batch) >= batchSize {
							flush()
						}
					case <-ticker.C:
						flush()
					case <-progressTicker.C:
						// Update progress via SDK
						completed, total := f.GetProgress()
						if total > 0 {
							err := backend.AppSDK.UpdateProcess(id, sdk.ProgressUpdate{
								Completed: completed,
								Total:     total,
								Message:   fmt.Sprintf("Processing: %d/%d requests", completed, total),
								State:     "Running",
							})
							if err != nil {
								log.Printf("[StartFuzzer] Failed to update progress: %v", err)
							}
						}
					}
				}
			resultsDone:

				log.Println("[StartFuzzer] results processing completed for ", id)

				// Final progress update via SDK
				completed, total := f.GetProgress()
				err := backend.AppSDK.CompleteProcess(id, fmt.Sprintf("Completed: %d/%d requests", completed, total))
				if err != nil {
					log.Printf("[StartFuzzer] Failed to complete process: %v", err)
				}

				// Clean up after all results are processed
				FuzzerMgr.mu.Lock()
				delete(FuzzerMgr.instances, id)
				FuzzerMgr.mu.Unlock()
			}()

			// Start fuzzing in a separate goroutine (non-blocking)
			go func() {
				err := f.Fuzz()
				if err != nil {
					log.Printf("[StartFuzzer] Fuzzer error for %s: %v", id, err)

					// Update process as failed via SDK
					sdkErr := backend.AppSDK.FailProcess(id, fmt.Sprintf("Fuzzer error: %v", err))
					if sdkErr != nil {
						log.Printf("[StartFuzzer] Failed to update process as failed: %v", sdkErr)
					}

					// Clean up
					FuzzerMgr.mu.Lock()
					delete(FuzzerMgr.instances, id)
					FuzzerMgr.mu.Unlock()
				}
			}()

			// Return immediately with the fuzzer ID
			return c.JSON(http.StatusOK, map[string]interface{}{
				"status":     "started",
				"process_id": id,
				"fuzzer_id":  id,
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

func (backend *Tools) StopFuzzer(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/fuzzer/stop",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil

			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			var body map[string]string
			if err := c.Bind(&body); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{
					"status":     "error",
					"process_id": "",
					"fuzzer_id":  "",
					"error":      err.Error(),
				})
			}

			id := body["id"]
			if id == "" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{
					"status":     "error",
					"process_id": "",
					"fuzzer_id":  "",
					"error":      "id is required",
				})
			}

			FuzzerMgr.mu.RLock()
			f, exists := FuzzerMgr.instances[id]
			FuzzerMgr.mu.RUnlock()

			if !exists {
				return c.JSON(http.StatusNotFound, map[string]interface{}{
					"status":     "error",
					"process_id": id,
					"fuzzer_id":  id,
					"error":      "fuzzer not found",
				})
			}

			// Get current progress before stopping
			completed, total := f.GetProgress()

			// Stop the fuzzer
			f.Stop()

			// Update process with final progress via SDK
			err := backend.AppSDK.UpdateProcess(id, sdk.ProgressUpdate{
				Completed: completed,
				Total:     total,
				Message:   fmt.Sprintf("Stopped by user at %d/%d requests", completed, total),
				State:     "Killed",
			})
			if err != nil {
				log.Printf("[StopFuzzer] Failed to update process: %v", err)
			}

			return c.JSON(http.StatusOK, map[string]interface{}{
				"status":     "stopped",
				"process_id": id,
				"fuzzer_id":  id,
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}
