package tools

import (
	"fmt"
	"log"
	"net/http"

	"github.com/glitchedgitz/grroxy/internal/sdk"
	"github.com/glitchedgitz/pocketbase/apis"
	"github.com/glitchedgitz/pocketbase/core"
	"github.com/glitchedgitz/pocketbase/models"
	"github.com/labstack/echo/v5"
)

// LoginSDK initializes and authenticates the SDK client for connecting to the main app
func (t *Tools) LoginSDK(url, email, password string) error {
	if url == "" {
		return fmt.Errorf("app URL cannot be empty")
	}
	if email == "" {
		return fmt.Errorf("admin email cannot be empty")
	}
	if password == "" {
		return fmt.Errorf("admin password cannot be empty")
	}

	// Store the URL
	t.AppURL = url

	// Create SDK client
	t.AppSDK = sdk.NewClient(
		url,
		sdk.WithAdminEmailPassword(email, password),
	)

	// Test the connection
	if err := t.AppSDK.Authorize(); err != nil {
		return fmt.Errorf("failed to authenticate with main app: %w", err)
	}

	log.Printf("[LoginSDK] Successfully connected to main app at %s", url)
	return nil
}

type LoginSDKRequest struct {
	URL      string `json:"url"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (backend *Tools) SDKStatus(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodGet,
		Path:   "/api/sdk/status",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil

			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			return c.JSON(http.StatusOK, map[string]interface{}{
				"status":    "success",
				"connected": backend.AppSDK != nil,
				"url":       backend.AppURL,
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

func (backend *Tools) LoginSDKEndpoint(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/sdk/login",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil

			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			var body LoginSDKRequest
			if err := c.Bind(&body); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{
					"status":    "error",
					"connected": false,
					"error":     err.Error(),
				})
			}

			// Validate required fields
			if body.URL == "" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{
					"status":    "error",
					"connected": false,
					"error":     "url is required",
				})
			}
			if body.Email == "" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{
					"status":    "error",
					"connected": false,
					"error":     "email is required",
				})
			}
			if body.Password == "" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{
					"status":    "error",
					"connected": false,
					"error":     "password is required",
				})
			}

			// Attempt to login
			err := backend.LoginSDK(body.URL, body.Email, body.Password)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"status":    "error",
					"connected": false,
					"error":     err.Error(),
				})
			}

			return c.JSON(http.StatusOK, map[string]interface{}{
				"status":    "success",
				"connected": true,
				"url":       backend.AppURL,
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}
