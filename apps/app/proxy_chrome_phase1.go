package app

// This file contains Phase 1 Chrome browser automation endpoints
// These will be integrated into proxy.go

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/glitchedgitz/grroxy/grx/browser"
	"github.com/glitchedgitz/pocketbase/apis"
	"github.com/glitchedgitz/pocketbase/core"
	"github.com/glitchedgitz/pocketbase/models"
	"github.com/labstack/echo/v5"
)

// ActivateTab endpoint - switches focus to a specific tab
func (backend *Backend) ActivateTab(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/proxy/chrome/tab/activate",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil
			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			type ActivateTabBody struct {
				ProxyID  string `json:"proxyId"`
				TargetID string `json:"targetId"`
			}

			var body ActivateTabBody
			if err := c.Bind(&body); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Invalid request body"})
			}

			if body.ProxyID == "" || body.TargetID == "" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "proxyId and targetId are required"})
			}

			// Get proxy instance
			inst := ProxyMgr.GetInstance(body.ProxyID)
			if inst == nil {
				return c.JSON(http.StatusNotFound, map[string]interface{}{"error": "Proxy not found"})
			}

			if inst.Browser != "chrome" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Proxy does not have Chrome browser attached"})
			}

			// Get profile directory
			var profileDir string
			if inst.BrowserCmd != nil && len(inst.BrowserCmd.Args) > 0 {
				for _, arg := range inst.BrowserCmd.Args {
					if strings.HasPrefix(arg, "--user-data-dir=") {
						profileDir = strings.TrimPrefix(arg, "--user-data-dir=")
						break
					}
				}
			}

			if profileDir == "" {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": "Could not determine Chrome profile directory"})
			}

			// Get debug URL
			debugURL, err := browser.GetChromeDebugURL(profileDir)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": fmt.Sprintf("Failed to get Chrome debug URL: %v", err)})
			}

			// Activate tab
			if err := browser.ActivateTab(debugURL, body.TargetID); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": fmt.Sprintf("Failed to activate tab: %v", err)})
			}

			return c.JSON(http.StatusOK, map[string]interface{}{
				"ok":        true,
				"targetId":  body.TargetID,
				"timestamp": time.Now().Format(time.RFC3339),
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

// FocusWindow endpoint - brings a proxy's Chrome window to the OS
// foreground without the caller needing to know any tab IDs. Used by the
// UI's per-proxy "go to browser" button. Implementation: list the proxy's
// tabs and activate one — activating any tab raises the window.
func (backend *Backend) FocusWindow(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/proxy/chrome/window/focus",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil
			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			type FocusBody struct {
				ProxyID string `json:"proxyId"`
			}

			var body FocusBody
			if err := c.Bind(&body); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Invalid request body"})
			}
			if body.ProxyID == "" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "proxyId is required"})
			}

			inst := ProxyMgr.GetInstance(body.ProxyID)
			if inst == nil {
				return c.JSON(http.StatusNotFound, map[string]interface{}{"error": "Proxy not found"})
			}
			if inst.Browser != "chrome" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Proxy does not have Chrome browser attached"})
			}

			var profileDir string
			if inst.BrowserCmd != nil && len(inst.BrowserCmd.Args) > 0 {
				for _, arg := range inst.BrowserCmd.Args {
					if strings.HasPrefix(arg, "--user-data-dir=") {
						profileDir = strings.TrimPrefix(arg, "--user-data-dir=")
						break
					}
				}
			}
			if profileDir == "" {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": "Could not determine Chrome profile directory"})
			}

			debugURL, err := browser.GetChromeDebugURL(profileDir)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": fmt.Sprintf("Failed to get Chrome debug URL: %v", err)})
			}

			// Hit Chrome's HTTP debug API directly — using chromedp here would
			// attach a new browser context and spin up an `about:blank` tab as
			// a side-effect (then immediately close it on Close()).
			httpBase, err := chromeHTTPBase(debugURL)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			tabID, err := pickPageTab(httpBase)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			if err := activateChromeTab(httpBase, tabID); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": fmt.Sprintf("Failed to focus window: %v", err)})
			}

			return c.JSON(http.StatusOK, map[string]interface{}{
				"ok":        true,
				"targetId":  tabID,
				"timestamp": time.Now().Format(time.RFC3339),
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

// CloseTab endpoint - closes a specific tab
func (backend *Backend) CloseTab(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/proxy/chrome/tab/close",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil
			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			type CloseTabBody struct {
				ProxyID  string `json:"proxyId"`
				TargetID string `json:"targetId"`
			}

			var body CloseTabBody
			if err := c.Bind(&body); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Invalid request body"})
			}

			if body.ProxyID == "" || body.TargetID == "" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "proxyId and targetId are required"})
			}

			// Get proxy instance
			inst := ProxyMgr.GetInstance(body.ProxyID)
			if inst == nil {
				return c.JSON(http.StatusNotFound, map[string]interface{}{"error": "Proxy not found"})
			}

			if inst.Browser != "chrome" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Proxy does not have Chrome browser attached"})
			}

			// Get profile directory
			var profileDir string
			if inst.BrowserCmd != nil && len(inst.BrowserCmd.Args) > 0 {
				for _, arg := range inst.BrowserCmd.Args {
					if strings.HasPrefix(arg, "--user-data-dir=") {
						profileDir = strings.TrimPrefix(arg, "--user-data-dir=")
						break
					}
				}
			}

			if profileDir == "" {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": "Could not determine Chrome profile directory"})
			}

			// Get debug URL
			debugURL, err := browser.GetChromeDebugURL(profileDir)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": fmt.Sprintf("Failed to get Chrome debug URL: %v", err)})
			}

			// Close tab
			if err := browser.CloseTab(debugURL, body.TargetID); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": fmt.Sprintf("Failed to close tab: %v", err)})
			}

			return c.JSON(http.StatusOK, map[string]interface{}{
				"ok":        true,
				"targetId":  body.TargetID,
				"timestamp": time.Now().Format(time.RFC3339),
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

// ReloadTab endpoint - reloads a specific tab
func (backend *Backend) ReloadTab(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/proxy/chrome/tab/reload",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil
			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			type ReloadTabBody struct {
				ProxyID     string `json:"proxyId"`
				TargetID    string `json:"targetId"`    // Optional, empty = active tab
				BypassCache bool   `json:"bypassCache"` // Optional
			}

			var body ReloadTabBody
			if err := c.Bind(&body); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Invalid request body"})
			}

			if body.ProxyID == "" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "proxyId is required"})
			}

			// Get proxy instance
			inst := ProxyMgr.GetInstance(body.ProxyID)
			if inst == nil {
				return c.JSON(http.StatusNotFound, map[string]interface{}{"error": "Proxy not found"})
			}

			if inst.Browser != "chrome" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Proxy does not have Chrome browser attached"})
			}

			// Get profile directory
			var profileDir string
			if inst.BrowserCmd != nil && len(inst.BrowserCmd.Args) > 0 {
				for _, arg := range inst.BrowserCmd.Args {
					if strings.HasPrefix(arg, "--user-data-dir=") {
						profileDir = strings.TrimPrefix(arg, "--user-data-dir=")
						break
					}
				}
			}

			if profileDir == "" {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": "Could not determine Chrome profile directory"})
			}

			// Get debug URL
			debugURL, err := browser.GetChromeDebugURL(profileDir)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": fmt.Sprintf("Failed to get Chrome debug URL: %v", err)})
			}

			// Reload tab
			if err := browser.ReloadTab(debugURL, body.TargetID, body.BypassCache); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": fmt.Sprintf("Failed to reload tab: %v", err)})
			}

			return c.JSON(http.StatusOK, map[string]interface{}{
				"ok":        true,
				"targetId":  body.TargetID,
				"timestamp": time.Now().Format(time.RFC3339),
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

// GoBack endpoint - navigates back in browser history
func (backend *Backend) GoBack(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/proxy/chrome/tab/back",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil
			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			type GoBackBody struct {
				ProxyID  string `json:"proxyId"`
				TargetID string `json:"targetId"` // Optional, empty = active tab
			}

			var body GoBackBody
			if err := c.Bind(&body); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Invalid request body"})
			}

			if body.ProxyID == "" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "proxyId is required"})
			}

			// Get proxy instance
			inst := ProxyMgr.GetInstance(body.ProxyID)
			if inst == nil {
				return c.JSON(http.StatusNotFound, map[string]interface{}{"error": "Proxy not found"})
			}

			if inst.Browser != "chrome" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Proxy does not have Chrome browser attached"})
			}

			// Get profile directory
			var profileDir string
			if inst.BrowserCmd != nil && len(inst.BrowserCmd.Args) > 0 {
				for _, arg := range inst.BrowserCmd.Args {
					if strings.HasPrefix(arg, "--user-data-dir=") {
						profileDir = strings.TrimPrefix(arg, "--user-data-dir=")
						break
					}
				}
			}

			if profileDir == "" {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": "Could not determine Chrome profile directory"})
			}

			// Get debug URL
			debugURL, err := browser.GetChromeDebugURL(profileDir)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": fmt.Sprintf("Failed to get Chrome debug URL: %v", err)})
			}

			// Go back
			if err := browser.GoBack(debugURL, body.TargetID); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": fmt.Sprintf("Failed to go back: %v", err)})
			}

			return c.JSON(http.StatusOK, map[string]interface{}{
				"ok":        true,
				"targetId":  body.TargetID,
				"timestamp": time.Now().Format(time.RFC3339),
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

// GoForward endpoint - navigates forward in browser history
func (backend *Backend) GoForward(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/proxy/chrome/tab/forward",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil
			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			type GoForwardBody struct {
				ProxyID  string `json:"proxyId"`
				TargetID string `json:"targetId"` // Optional, empty = active tab
			}

			var body GoForwardBody
			if err := c.Bind(&body); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Invalid request body"})
			}

			if body.ProxyID == "" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "proxyId is required"})
			}

			// Get proxy instance
			inst := ProxyMgr.GetInstance(body.ProxyID)
			if inst == nil {
				return c.JSON(http.StatusNotFound, map[string]interface{}{"error": "Proxy not found"})
			}

			if inst.Browser != "chrome" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Proxy does not have Chrome browser attached"})
			}

			// Get profile directory
			var profileDir string
			if inst.BrowserCmd != nil && len(inst.BrowserCmd.Args) > 0 {
				for _, arg := range inst.BrowserCmd.Args {
					if strings.HasPrefix(arg, "--user-data-dir=") {
						profileDir = strings.TrimPrefix(arg, "--user-data-dir=")
						break
					}
				}
			}

			if profileDir == "" {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": "Could not determine Chrome profile directory"})
			}

			// Get debug URL
			debugURL, err := browser.GetChromeDebugURL(profileDir)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": fmt.Sprintf("Failed to get Chrome debug URL: %v", err)})
			}

			// Go forward
			if err := browser.GoForward(debugURL, body.TargetID); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": fmt.Sprintf("Failed to go forward: %v", err)})
			}

			return c.JSON(http.StatusOK, map[string]interface{}{
				"ok":        true,
				"targetId":  body.TargetID,
				"timestamp": time.Now().Format(time.RFC3339),
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

// chromeHTTPBase converts the ws:// debug URL Chrome writes into
// DevToolsActivePort into the http://host:port base used by Chrome's
// JSON debugging endpoints (/json, /json/activate/...).
func chromeHTTPBase(debugURL string) (string, error) {
	u, err := url.Parse(debugURL)
	if err != nil {
		return "", fmt.Errorf("invalid debug URL: %v", err)
	}
	host := u.Host
	if host == "" {
		return "", fmt.Errorf("debug URL missing host: %s", debugURL)
	}
	return "http://" + host, nil
}

// pickPageTab returns a page-type target ID from Chrome's /json listing.
// Pure HTTP — no chromedp / CDP session, so no extra tab is opened.
func pickPageTab(httpBase string) (string, error) {
	resp, err := http.Get(httpBase + "/json")
	if err != nil {
		return "", fmt.Errorf("failed to list tabs: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var entries []struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(body, &entries); err != nil {
		return "", fmt.Errorf("failed to parse /json: %v", err)
	}
	for _, e := range entries {
		if e.Type == "page" && e.ID != "" {
			return e.ID, nil
		}
	}
	return "", fmt.Errorf("no page-type tab found")
}

// activateChromeTab hits Chrome's /json/activate/{id} endpoint, which
// activates the tab and raises its window — same behaviour as CDP's
// Target.activateTarget but without spinning up a chromedp session.
func activateChromeTab(httpBase, tabID string) error {
	u := strings.TrimRight(httpBase, "/") + "/json/activate/" + tabID
	resp, err := http.Post(u, "", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("activate %s: %s — %s", tabID, resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}
