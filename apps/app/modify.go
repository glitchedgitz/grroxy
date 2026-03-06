package app

import (
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/glitchedgitz/grroxy/grx/rawhttp"
	"github.com/glitchedgitz/grroxy/grx/templates"
	"github.com/glitchedgitz/grroxy/grx/templates/actions"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
)

type ModifyRequestRequest struct {
	Request string                      `json:"request"`
	Url     string                      `json:"url"`
	Tasks   []map[string]map[string]any `json:"tasks"`
}

func (backend *Backend) ModifyRequest(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/request/modify",
		Handler: func(c echo.Context) error {
			log.Println("[Modify Request] Handler called")

			// Check authentication
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil

			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			// Bind request body
			var reqData ModifyRequestRequest
			if err := c.Bind(&reqData); err != nil {
				log.Printf("[Modify request] Error binding body: %v", err)
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
			}

			parsedRequest := rawhttp.ParseRequest([]byte(reqData.Request))

			parsedURL, err := url.Parse(parsedRequest.URL)
			if err != nil {
				log.Printf("[Modify request] Error parsing URL: %v", err)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Error parsing URL", "message": err.Error()})
			}

			hasCookies := false
			if len(parsedRequest.Headers) > 0 {
				for _, header := range parsedRequest.Headers {
					if header[0] == "Cookie" {
						hasCookies = true
					}
				}
			}

			reqRecord := make(map[string]any)
			reqRecord["method"] = parsedRequest.Method
			reqRecord["url"] = parsedRequest.URL
			reqRecord["path"] = parsedURL.Path
			reqRecord["query"] = parsedURL.RawQuery
			reqRecord["fragment"] = parsedURL.RawFragment
			reqRecord["ext"] = strings.TrimPrefix(parsedURL.Path, ".")
			reqRecord["has_cookies"] = hasCookies
			reqRecord["length"] = len(reqData.Request)
			reqRecord["headers"] = parsedRequest.Headers
			reqRecord["raw"] = reqData.Request

			actions, err := templates.ParseTemplateActions([]templates.Actions{{
				Id:        "",
				Condition: "",
				Todo:      reqData.Tasks,
			},
			}, map[string]any{
				"req": reqRecord,
			}, "all")

			if err != nil {
				log.Printf("[Modify request] Error parsing template: %v", err)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Error parsing template"})
			}

			var modifiedRequest = ""

			log.Printf("[Modify request] Actions: %+v", actions)
			modifiedRequest, err = runActions(actions, reqRecord)
			if err != nil {
				log.Printf("[Modify request] Error running actions: %v", err)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Error running actions", "message": err.Error()})
			}

			log.Printf("[Modify request] Request data: %+v", reqData)

			return c.JSON(http.StatusOK, map[string]string{"success": "true",
				"request": modifiedRequest,
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})

	return nil
}

func runActions(tasks []templates.Action, requestData map[string]any) (string, error) {
	log.Println("Run Actions: Tasks", tasks)
	for _, task := range tasks {

		switch task.ActionName {
		case actions.Set:
			for key, value := range task.Data {
				log.Println("Run Actions: Set", key, value)
				RequestUpdateKey(requestData, key, value)
			}
		case actions.Delete:
			for key := range task.Data {
				log.Println("Run Actions: Delete", key)
				RequestDeleteKey(requestData, key)
			}
		case actions.Replace:
			log.Println("Run Actions: Replace")
			search := task.Data["search"].(string)
			value := task.Data["value"].(string)
			isRegex := false
			if regex, ok := task.Data["regex"].(bool); ok {
				isRegex = regex
			}
			RequestReplace(requestData, search, value, isRegex)
			log.Println("Run Actions: Replace", requestData)
		default:
			log.Println("[runActions] Unknown action: ", task.ActionName)
		}
	}

	// Rebuild the request from requestData
	modifiedRequest := buildRawRequest(requestData)
	return modifiedRequest, nil
}

func buildRawRequest(requestData map[string]any) string {
	var builder strings.Builder

	// Build request line
	method := "GET"
	if m, ok := requestData["method"].(string); ok {
		method = m
	}

	urlStr := "/"
	if u, ok := requestData["url"].(string); ok {
		if parsedURL, err := url.Parse(u); err == nil {
			urlStr = parsedURL.RequestURI()
		}
	}

	builder.WriteString(method)
	builder.WriteString(" ")
	builder.WriteString(urlStr)
	builder.WriteString(" HTTP/1.1\r\n")

	// Build headers
	if headers, ok := requestData["headers"].([][]string); ok {
		for _, header := range headers {
			if len(header) >= 2 {
				builder.WriteString(header[0])
				builder.WriteString(header[1])
				builder.WriteString("\r\n")
			}
		}
	}

	builder.WriteString("\r\n")

	// Add body if present
	if rawReq, ok := requestData["raw"].(string); ok {
		parts := strings.SplitN(rawReq, "\r\n\r\n", 2)
		if len(parts) == 2 {
			builder.WriteString(parts[1])
		}
	}

	return builder.String()
}

func RequestUpdateKey(requestData map[string]any, key string, value any) {
	log.Println("[RequestUpdateKey] key: '", key, "' value: '", value, "'")
	if key == "req.method" {
		requestData["method"] = value.(string)
	} else if key == "req.url" {
		parsedURL, err := url.Parse(value.(string))
		if err != nil {
			return
		}
		requestData["url"] = value.(string)
		requestData["path"] = parsedURL.Path
		requestData["query"] = parsedURL.RawQuery
		requestData["fragment"] = parsedURL.RawFragment

	} else if key == "req.path" {
		requestData["path"] = value.(string)
		// Update URL to reflect new path
		if urlStr, ok := requestData["url"].(string); ok {
			parsedURL, err := url.Parse(urlStr)
			if err == nil {
				parsedURL.Path = value.(string)
				requestData["url"] = parsedURL.String()
			}
		}

	} else if strings.HasPrefix(key, "req.query") {
		queryParam := strings.TrimPrefix(key, "req.query")[1:]
		if urlStr, ok := requestData["url"].(string); ok {
			parsedURL, err := url.Parse(urlStr)
			if err == nil {
				params := parsedURL.Query()
				params.Set(queryParam, value.(string))
				parsedURL.RawQuery = params.Encode()
				requestData["url"] = parsedURL.String()
				requestData["query"] = parsedURL.RawQuery
			}
		}

	} else if strings.HasPrefix(key, "req.headers") {
		log.Println("[RequestUpdateKey] req.headers: ", key, value)

		header := strings.TrimPrefix(key, "req.headers")[1:]
		if headers, ok := requestData["headers"].([][]string); ok {
			found := false
			for i, h := range headers {
				if h[0] == header+":" {
					headers[i][1] = value.(string)
					found = true
					break
				}
			}
			if !found {
				requestData["headers"] = append(headers, []string{header + ": ", value.(string)})
			}
		}

	} else if key == "req.body" {
		newBody := value.(string)
		requestData["length"] = len(newBody)
		// Update the raw request body part
		if rawReq, ok := requestData["raw"].(string); ok {
			parts := strings.SplitN(rawReq, "\r\n\r\n", 2)
			if len(parts) == 2 {
				requestData["raw"] = parts[0] + "\r\n\r\n" + newBody
			}
		}
	}
}

func RequestDeleteKey(requestData map[string]any, key string) {
	if key == "req.method" {
		requestData["method"] = "GET"

	} else if key == "req.url" {
		requestData["url"] = ""
		requestData["path"] = ""
		requestData["query"] = ""
		requestData["fragment"] = ""

	} else if key == "req.path" {
		requestData["path"] = ""
		if urlStr, ok := requestData["url"].(string); ok {
			parsedURL, err := url.Parse(urlStr)
			if err == nil {
				parsedURL.Path = ""
				requestData["url"] = parsedURL.String()
			}
		}

	} else if strings.HasPrefix(key, "req.query") {
		queryParam := strings.TrimPrefix(key, "req.query")[1:]
		if urlStr, ok := requestData["url"].(string); ok {
			parsedURL, err := url.Parse(urlStr)
			if err == nil {
				params := parsedURL.Query()
				params.Del(queryParam)
				parsedURL.RawQuery = params.Encode()
				requestData["url"] = parsedURL.String()
				requestData["query"] = parsedURL.RawQuery
			}
		}

	} else if strings.HasPrefix(key, "req.headers") {
		header := strings.TrimPrefix(key, "req.headers")[1:]

		if headers, ok := requestData["headers"].([][]string); ok {
			newHeaders := [][]string{}

			// Check if header ends with * for wildcard deletion
			if strings.HasSuffix(header, "*") {
				prefix := strings.TrimSuffix(header, "*")
				// Delete all headers starting with the prefix
				for _, h := range headers {
					// h[0] is the header name with colon (e.g., "Sec-Fetch-Dest:")
					// Check if it starts with the prefix followed by colon or dash
					headerName := strings.TrimSuffix(h[0], ":")
					if !strings.HasPrefix(headerName, prefix) {
						newHeaders = append(newHeaders, h)
					}
				}
			} else {
				// Exact match deletion (existing behavior)
				for _, h := range headers {
					if h[0] != header+":" {
						newHeaders = append(newHeaders, h)
					}
				}
			}
			requestData["headers"] = newHeaders
		}

	} else if key == "req.body" {
		requestData["length"] = 0
		if rawReq, ok := requestData["raw"].(string); ok {
			parts := strings.SplitN(rawReq, "\r\n\r\n", 2)
			if len(parts) == 2 {
				requestData["raw"] = parts[0] + "\r\n\r\n"
			}
		}
	}
}

func RequestReplace(requestData map[string]any, search string, value string, isRegex bool) {
	log.Println("[RequestReplace] search: '", search, "' value: '", value, "' regex:", isRegex)

	// Rebuild the raw request from current state to preserve previous modifications
	rawReq := buildRawRequest(requestData)

	var newRaw string
	if isRegex {
		// Use regex replacement
		re, err := regexp.Compile(search)
		if err != nil {
			log.Printf("[RequestReplace] Error compiling regex: %v", err)
			return
		}
		newRaw = re.ReplaceAllString(rawReq, value)
	} else {
		// Simple string replacement
		newRaw = strings.ReplaceAll(rawReq, search, value)
	}

	// Update the raw request
	requestData["raw"] = newRaw

	// Re-parse the modified request to update other fields
	parsedRequest := rawhttp.ParseRequest([]byte(newRaw))

	parsedURL, err := url.Parse(parsedRequest.URL)
	if err != nil {
		log.Printf("[RequestReplace] Error parsing URL after replace: %v", err)
		return
	}

	// Update all relevant fields
	requestData["method"] = parsedRequest.Method
	requestData["url"] = parsedRequest.URL
	requestData["path"] = parsedURL.Path
	requestData["query"] = parsedURL.RawQuery
	requestData["fragment"] = parsedURL.RawFragment
	requestData["headers"] = parsedRequest.Headers
	requestData["length"] = len(newRaw)

	// Check for cookies
	hasCookies := false
	if len(parsedRequest.Headers) > 0 {
		for _, header := range parsedRequest.Headers {
			if header[0] == "Cookie" {
				hasCookies = true
				break
			}
		}
	}
	requestData["has_cookies"] = hasCookies
}
