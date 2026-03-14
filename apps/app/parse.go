package app

import (
	"log"
	"net/http"
	"net/url"

	"github.com/glitchedgitz/grroxy/grx/rawhttp"
	"github.com/glitchedgitz/pocketbase/apis"
	"github.com/glitchedgitz/pocketbase/core"
	"github.com/glitchedgitz/pocketbase/models"
	"github.com/labstack/echo/v5"
)

type ParseRawRequest struct {
	Request  string `json:"request"`
	Response string `json:"response"`
}

func (backend *Backend) ParseRaw(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/request/parse",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			record, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)
			if admin == nil && record == nil {
				return c.String(http.StatusForbidden, "")
			}

			var reqData ParseRawRequest
			if err := c.Bind(&reqData); err != nil {
				log.Printf("[Parse Raw] Error binding body: %v", err)
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
			}

			result := map[string]any{}

			if reqData.Request != "" {
				parsed := rawhttp.ParseRequest([]byte(reqData.Request))
				parsedURL, _ := url.Parse(parsed.URL)

				query := ""
				path := parsed.URL
				if parsedURL != nil {
					query = parsedURL.RawQuery
					path = parsedURL.Path
				}

				result["request"] = map[string]any{
					"method":  parsed.Method,
					"url":     parsed.URL,
					"path":    path,
					"query":   query,
					"version": parsed.HTTPVersion,
					"headers": parsed.Headers,
					"body":    parsed.Body,
				}
			}

			if reqData.Response != "" {
				parsed := rawhttp.ParseResponse([]byte(reqData.Response))
				result["response"] = map[string]any{
					"status":     parsed.Status,
					"statusFull": parsed.StatusFull,
					"version":    parsed.Version,
					"headers":    parsed.Headers,
					"body":       parsed.Body,
				}
			}

			return c.JSON(http.StatusOK, result)
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}
