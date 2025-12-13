package tools_api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/glitchedgitz/grroxy-db/fuzzer"
	"github.com/glitchedgitz/grroxy-db/rawhttp"
	"github.com/glitchedgitz/grroxy-db/schemas"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
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
	Collection  string            `json:"collection"`
	Request     string            `json:"request"`
	Host        string            `json:"host"`
	Port        string            `json:"port"`
	UseTLS      bool              `json:"useTLS"`
	UseHTTP2    bool              `json:"useHTTP2"` // Enable HTTP/2 support
	Markers     map[string]string `json:"markers"`
	Mode        string            `json:"mode"`
	Concurrency int               `json:"concurrency"`
	Timeout     float64           `json:"timeout"` // in seconds
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
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
			}

			// log.Println("[StartFuzzer] Request:", body)

			// Validate required fields
			if body.Request == "" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "request is required"})
			}
			if body.Host == "" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "host is required"})
			}
			if body.Markers == nil {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "markers is required"})
			}
			if len(body.Markers) == 0 {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "markers cannot be blank"})
			}
			for key, value := range body.Markers {
				if value == "" {
					return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": fmt.Sprintf("marker '%s' must have a value", key)})
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

			// Register in database
			id := backend.RegisterProcessInDB(config, body, "Fuzzer", "fuzzer", schemas.ProcessState.Inqueue)

			// Create collection for this fuzzer
			err := backend.CreateCollection(body.Collection, schemas.Fuzzer)
			if err != nil && !strings.Contains(err.Error(), "UNIQUE constraint failed") {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": fmt.Sprintf("Failed to create collection: %v", err)})
			}

			collection, err := backend.App.Dao().FindCollectionByNameOrId(body.Collection)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": fmt.Sprintf("Failed to find collection: %v", err)})
			}

			// Create fuzzer instance
			f := fuzzer.NewFuzzer(config)

			// Store fuzzer instance
			FuzzerMgr.mu.Lock()
			FuzzerMgr.instances[id] = f
			FuzzerMgr.mu.Unlock()

			// Update state to running
			backend.SetProcess(id, schemas.ProcessState.Running)

			// Start result processing in a goroutine
			go func() {
				for result := range f.Results {
					fuzzerResult, ok := result.(fuzzer.FuzzerResult)
					if !ok {
						log.Printf("[StartFuzzer] Invalid result type: %T", result)
						continue
					}

					log.Println("[StartFuzzer] markers: ", fuzzerResult.Markers)

					// Parse request and response
					data := parseAndSaveResult(fuzzerResult.Request, fuzzerResult.Response)

					// Add common fields
					data["fuzzer_id"] = id
					data["time"] = fuzzerResult.Time.Nanoseconds()
					data["markers"] = fuzzerResult.Markers

					// Create and save record
					record := models.NewRecord(collection)
					for key, value := range data {
						record.Set(key, value)
					}
					err := backend.App.Dao().SaveRecord(record)
					if err != nil {
						log.Printf("[StartFuzzer] Failed to save record: %v", err)
					}
				}

				log.Println("[StartFuzzer] results processing completed for ", id)

				backend.SetProcess(id, schemas.ProcessState.Completed)

				// Clean up after all results are processed
				FuzzerMgr.mu.Lock()
				delete(FuzzerMgr.instances, id)
				FuzzerMgr.mu.Unlock()
			}()

			// Start fuzzing in a separate goroutine (non-blocking)
			err = f.Fuzz()
			if err != nil {
				log.Printf("[StartFuzzer] Error: %v", err)
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			// Return immediately with the fuzzer ID
			return c.JSON(http.StatusOK, map[string]interface{}{
				"id": id,
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
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
			}

			id := body["id"]
			if id == "" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "id is required"})
			}

			FuzzerMgr.mu.RLock()
			f, exists := FuzzerMgr.instances[id]
			FuzzerMgr.mu.RUnlock()

			if !exists {
				return c.JSON(http.StatusNotFound, map[string]interface{}{"error": "fuzzer not found"})
			}

			f.Stop()
			backend.SetProcess(id, schemas.ProcessState.Killed)

			return c.JSON(http.StatusOK, map[string]interface{}{
				"status": "stopped",
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}
