package api

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/glitchedgitz/grroxy-db/rawhttp"
	"github.com/glitchedgitz/grroxy-db/utils"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
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

			host := data["host"].(string)
			host = strings.TrimPrefix(host, "http://")
			host = strings.TrimPrefix(host, "https://")

			request := data["req"].(string)
			// replace \n with \r\n

			// request = strings.ReplaceAll(request, "\n", "\r\n") + "\r\n\r\n"

			log.Println("RawRequest TLS: ", data["tls"].(bool))
			log.Println("RawRequest Hostname: ", data["host"].(string))
			log.Println("RawRequest Port: ", data["port"].(string))
			log.Println("RawRequest Timeout: ", time.Duration(data["timeout"].(float64))*time.Second)
			log.Println("RawRequest Request: ", request)

			mappedData := RawRequest{
				TLS:      data["tls"].(bool),
				Hostname: host,
				Port:     data["port"].(string),
				Request:  request,
				Timeout:  time.Duration(data["timeout"].(float64)) * time.Second,
			}

			// respString, err := sendRawRequest2(mappedData)
			var respString = ""
			var err error

			log.Println("httpversion: ", data["httpversion"])

			// Create rawhttp client with timeout
			client := rawhttp.NewClient(rawhttp.Config{
				Timeout:            mappedData.Timeout,
				InsecureSkipVerify: true, // For security testing, skip cert verification
			})

			// Get the current time before sending
			timeBefore := time.Now()

			// Send the raw request
			resp, err := client.SendString(
				mappedData.Request,
				mappedData.Hostname,
				mappedData.Port,
				mappedData.TLS,
			)

			// Get the time after sending
			timeAfter := time.Now()

			// Calculate the time difference
			timeTaken := utils.CalculateTime(timeBefore, timeAfter)

			if err != nil {
				log.Printf("Error sending raw request: %v", err)
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{
					"error": err.Error(),
					"time":  timeTaken,
				})
			}

			// Get response string from raw bytes
			if resp != nil {
				respString = string(resp.RawBytes)
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
