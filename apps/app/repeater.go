package app

import (
	"log"
	"net/http"
	"time"

	"github.com/glitchedgitz/grroxy/internal/types"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
)

type RepeaterSendRequest struct {
	Host        string  `json:"host"`
	Port        string  `json:"port"`
	TLS         bool    `json:"tls"`
	Request     string  `json:"request"`
	Timeout     float64 `json:"timeout"`
	HTTP2       bool    `json:"http2"`
	Index       float64 `json:"index"`
	Url         string  `json:"url"`
	GeneratedBy string  `json:"generated_by"`
	Note        string  `json:"note,omitempty"`
}

type RepeaterSendResponse struct {
	Response string         `json:"response"`
	Time     string         `json:"time"`
	UserData types.UserData `json:"userdata"`
}

// SendRepeater handles the /api/repeater/send endpoint
func (backend *Backend) SendRepeater(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/repeater/send",
		Handler: func(c echo.Context) error {
			log.Println("[SendRepeater] Handler called")

			// Check authentication
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil

			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			// Bind request body
			var reqData RepeaterSendRequest
			if err := c.Bind(&reqData); err != nil {
				log.Printf("[SendRepeater] Error binding body: %v", err)
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
			}

			log.Printf("[SendRepeater] Request data: %+v", reqData)

			// Send the raw HTTP request using function from rawhttp.go
			timeout := time.Duration(reqData.Timeout) * time.Second
			respString, timeTaken, err := SendRawHTTPRequest(
				reqData.Host,
				reqData.Port,
				reqData.TLS,
				reqData.Request,
				timeout,
				reqData.HTTP2,
			)

			if err != nil {
				log.Printf("[SendRepeater] Error sending request: %v", err)
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{
					"error": err.Error(),
					"time":  timeTaken,
				})
			}

			// Save to backend using function from request.go
			// index_minor will be auto-calculated in SaveRequestToBackend
			addReqBody := types.AddRequestBodyType{
				Url:         reqData.Url,
				Index:       reqData.Index,
				Request:     reqData.Request,
				Response:    respString,
				GeneratedBy: "repeater/" + reqData.GeneratedBy,
				Note:        reqData.Note,
			}

			userdata, err := backend.SaveRequestToBackend(addReqBody)
			if err != nil {
				log.Printf("[SendRepeater] Error saving to backend: %v", err)
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{
					"error":    "Failed to save to backend",
					"response": respString,
					"time":     timeTaken,
				})
			}

			// Return response
			response := RepeaterSendResponse{
				Response: respString,
				Time:     timeTaken,
				UserData: userdata,
			}

			log.Printf("[SendRepeater] Successfully processed request")
			return c.JSON(http.StatusOK, response)
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})

	return nil
}
