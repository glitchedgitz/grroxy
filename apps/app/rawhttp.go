package app

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/glitchedgitz/grroxy/grx/rawhttp"
	"github.com/glitchedgitz/grroxy/internal/utils"
	"github.com/glitchedgitz/pocketbase/apis"
	"github.com/glitchedgitz/pocketbase/core"
	"github.com/glitchedgitz/pocketbase/models"
	"github.com/labstack/echo/v5"
)

func (backend *Backend) SendHttpRaw(e *core.ServeEvent) error {

	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/http/raw",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil

			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			var data map[string]interface{}
			if err := c.Bind(&data); err != nil {
				return err
			}

			// Parse request data
			host := data["host"].(string)
			port := data["port"].(string)
			tls := data["tls"].(bool)
			request := data["req"].(string)
			timeout := time.Duration(data["timeout"].(float64)) * time.Second

			// Check if HTTP/2 is requested
			useHTTP2 := false
			if http2Val, ok := data["http2"].(bool); ok {
				useHTTP2 = http2Val
			}

			log.Println("RawRequest TLS: ", tls)
			log.Println("RawRequest Hostname: ", host)
			log.Println("RawRequest Port: ", port)
			log.Println("RawRequest Timeout: ", timeout)
			log.Println("RawRequest HTTP/2: ", useHTTP2)

			// Use SendRawHTTPRequest function
			respString, timeTaken, err := SendRawHTTPRequest(host, port, tls, request, timeout, useHTTP2)

			if err != nil {
				log.Printf("Error sending raw request: %v", err)
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{
					"error": err.Error(),
					"time":  timeTaken,
				})
			}

			response := map[string]any{
				"resp": respString,
				"time": timeTaken,
			}

			return c.JSON(http.StatusOK, response)
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

// SendRawHTTPRequest sends a raw HTTP request using the rawhttp client and returns response, time taken, and error
func SendRawHTTPRequest(host, port string, tls bool, request string, timeout time.Duration, http2 bool) (string, string, error) {
	// Clean up host
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimPrefix(host, "https://")

	log.Println("[SendRawHTTPRequest] TLS:", tls)
	log.Println("[SendRawHTTPRequest] Hostname:", host)
	log.Println("[SendRawHTTPRequest] Port:", port)
	log.Println("[SendRawHTTPRequest] Timeout:", timeout)
	log.Println("[SendRawHTTPRequest] HTTP/2:", http2)

	// Create rawhttp client with timeout
	client := rawhttp.NewClient(rawhttp.Config{
		Timeout:            timeout,
		InsecureSkipVerify: true,
	})

	// Get the current time before sending
	timeBefore := time.Now()

	// Send the raw request
	req := rawhttp.Request{
		RawBytes: []byte(request),
		Host:     host,
		Port:     port,
		UseTLS:   tls,
		UseHTTP2: http2,
		Timeout:  timeout,
	}

	resp, err := client.Send(req)

	// Get the time after sending
	timeAfter := time.Now()

	// Calculate the time difference
	timeTaken := utils.CalculateTime(timeBefore, timeAfter)

	if err != nil {
		log.Printf("[SendRawHTTPRequest] Error sending raw request: %v", err)
		return "", timeTaken, err
	}

	// Get response string from raw bytes
	respString := ""
	if resp != nil {
		respString = string(resp.RawBytes)
	}

	return respString, timeTaken, nil
}
