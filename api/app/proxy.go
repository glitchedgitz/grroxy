package api

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"path"
	"sync"
	"sync/atomic"

	"github.com/glitchedgitz/grroxy-db/browser"
	"github.com/glitchedgitz/grroxy-db/utils"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
)

// ProxyInstance holds a proxy and its optional runtime attachments (browser, label, etc.)
type ProxyInstance struct {
	Proxy      *RawProxyWrapper
	Browser    string // `json:"browser"`
	BrowserCmd *exec.Cmd
	Label      string // `json:"label"`
}

// ProxyManager manages multiple proxy instances
type ProxyManager struct {
	instances map[string]*ProxyInstance
	mu        sync.RWMutex
	index     atomic.Uint64 // Shared atomic counter for unique indices across all proxies
}

// Global proxy manager instance
var ProxyMgr = &ProxyManager{
	instances: make(map[string]*ProxyInstance),
}

// init is intentionally empty - initialization happens on first proxy start
func init() {
}

// SetGlobalIndex sets the global index from the database
func (pm *ProxyManager) SetGlobalIndex(value uint64) {
	pm.index.Store(value)
	log.Printf("[ProxyManager] Global index set to: %d", value)
}

// GetNextIndex returns the next unique index (thread-safe)
func (pm *ProxyManager) GetNextIndex() uint64 {
	return pm.index.Add(1)
}

// initializeIndexFromDB queries the database to get the current max index
func (pm *ProxyManager) initializeIndexFromDB(backend *Backend) error {
	dao := backend.App.Dao()

	// Query for the total number of rows in _data collection
	var result struct {
		TotalRows int `db:"total_rows" json:"total_rows"`
	}

	err := dao.DB().
		NewQuery("SELECT COUNT(*) as total_rows FROM _data").
		One(&result)

	if err != nil {
		return fmt.Errorf("failed to query total rows: %w", err)
	}

	// Set the atomic counter to the total rows count
	totalRows := uint64(result.TotalRows)
	pm.index.Store(totalRows)

	log.Printf("[ProxyManager] ========================================")
	log.Printf("[ProxyManager] Global Index Initialization:")
	log.Printf("[ProxyManager]   - Total rows in database: %d", totalRows)
	log.Printf("[ProxyManager]   - Next index will be: %d", totalRows+1)
	log.Printf("[ProxyManager]   - Counter starting at: %d", totalRows)
	log.Printf("[ProxyManager] ========================================")

	return nil
}

// GetProxy returns a proxy by ID (listen address)
func (pm *ProxyManager) GetProxy(id string) *RawProxyWrapper {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	if inst := pm.instances[id]; inst != nil {
		return inst.Proxy
	}
	return nil
}

// GetInstance returns a proxy instance by ID
func (pm *ProxyManager) GetInstance(id string) *ProxyInstance {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.instances[id]
}

// AddProxy adds a proxy to the manager
func (pm *ProxyManager) AddProxy(id string, proxy *RawProxyWrapper) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if inst := pm.instances[id]; inst != nil {
		inst.Proxy = proxy
	} else {
		pm.instances[id] = &ProxyInstance{Proxy: proxy}
	}
}

// AddProxyInstance adds a complete proxy instance to the manager
func (pm *ProxyManager) AddProxyInstance(id string, instance *ProxyInstance) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.instances[id] = instance
}

// RemoveProxy removes a proxy from the manager
func (pm *ProxyManager) RemoveProxy(id string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.instances, id)
}

// GetAllProxies returns all proxy IDs
func (pm *ProxyManager) GetAllProxies() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	ids := make([]string, 0, len(pm.instances))
	for id := range pm.instances {
		ids = append(ids, id)
	}
	return ids
}

// StopProxy stops a specific proxy
func (pm *ProxyManager) StopProxy(id string) error {
	log.Printf("[ProxyManager] StopProxy called for ID: %s", id)

	pm.mu.RLock()
	inst := pm.instances[id]
	pm.mu.RUnlock()

	if inst == nil || inst.Proxy == nil {
		log.Printf("[ProxyManager] Proxy with ID '%s' not found", id)
		return fmt.Errorf("proxy %s not found", id)
	}

	log.Printf("[ProxyManager] Proxy found, calling Stop()...")
	err := inst.Proxy.Stop()
	// attempt to close tied browser/terminal if any
	pm.mu.Lock()
	if inst.BrowserCmd != nil && inst.BrowserCmd.Process != nil {
		clientType := "browser"
		isTerminal := inst.Browser == "terminal"
		if isTerminal {
			clientType = "terminal"
		}
		log.Printf("[ProxyManager] Attempting to terminate %s for proxy %s (pid=%d)", clientType, id, inst.BrowserCmd.Process.Pid)

		var killErr error
		if isTerminal {
			// Use special terminal cleanup for better window closing
			killErr = browser.CloseTerminalWindow(inst.BrowserCmd)
		} else {
			// Standard browser process kill
			killErr = inst.BrowserCmd.Process.Kill()
		}

		if killErr != nil {
			log.Printf("[ProxyManager] Failed to kill %s process for %s: %v", clientType, id, killErr)
		} else {
			log.Printf("[ProxyManager] %s process for %s terminated", clientType, id)
		}
	}
	pm.mu.Unlock()
	return err
}

// StopAllProxies stops all running proxies
func (pm *ProxyManager) StopAllProxies() {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	for id, inst := range pm.instances {
		if inst != nil && inst.Proxy != nil {
			if err := inst.Proxy.Stop(); err != nil {
				log.Printf("[ProxyManager] Error stopping proxy %s: %v", id, err)
			}
		}
	}
}

// ApplyToAllProxies applies a function to all running proxies
func (pm *ProxyManager) ApplyToAllProxies(fn func(proxy *RawProxyWrapper, proxyID string)) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	for id, inst := range pm.instances {
		if inst != nil && inst.Proxy != nil {
			fn(inst.Proxy, id)
		}
	}
}

// DEPRECATED: Backward compatibility - returns first proxy or nil
var PROXY *RawProxyWrapper

func updateProxyVar() {
	ProxyMgr.mu.RLock()
	defer ProxyMgr.mu.RUnlock()

	// Set PROXY to first proxy for backward compatibility
	for _, inst := range ProxyMgr.instances {
		if inst != nil && inst.Proxy != nil {
			PROXY = inst.Proxy
			return
		}
	}
	PROXY = nil
}

type ProxyBody struct {
	HTTP    string `json:"http,omitempty"`
	Browser string `json:"browser,omitempty"`
	Name    string `json:"name,omitempty"` // Optional name for the proxy instance
}

func (backend *Backend) StartProxy(e *core.ServeEvent) error {

	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/proxy/start",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil

			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			log.Println("/api/proxy/start begins")

			var body ProxyBody
			if err := c.Bind(&body); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			log.Println("/api/proxy/start body", body)

			if body.HTTP == "" && body.Browser != "" {
				body.HTTP = "127.0.0.1:9797"
			}

			availableHost, err := utils.CheckAndFindAvailablePort(body.HTTP)

			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			if body.Browser == "" && availableHost != body.HTTP {
				return c.JSON(http.StatusOK, map[string]interface{}{"error": "port not available", "availableHost": availableHost})
			} else {
				body.HTTP = availableHost
			}

			// Proxy ID is the listen address
			proxyID := body.HTTP

			// Initialize global index from database if not already initialized
			// This ensures all proxies use the same unique index counter
			if ProxyMgr.index.Load() == 0 {
				if err := ProxyMgr.initializeIndexFromDB(backend); err != nil {
					log.Printf("[StartProxy] Warning: Failed to initialize global index from database: %v", err)
				}
			}

			// Create new rawproxy wrapper
			configDir := path.Join(backend.Config.HomeDirectory, ".config", "grroxy")

			// Disable file captures by passing empty string (we save to database instead)
			// To enable file captures for testing, uncomment the line below:
			// outputDir := path.Join(backend.Config.HomeDirectory, ".config", "grroxy", "captures")
			outputDir := "" // Empty = disabled

			newProxy, err := NewRawProxyWrapper(body.HTTP, configDir, outputDir, backend)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			// Generate label if not provided
			label := body.Name
			browserType := body.Browser
			if browserType == "" {
				label = body.HTTP
			} else if label == "" {
				// Generate label in format: {browser} {instance_number}

				// Count existing instances of this browser type
				ProxyMgr.mu.RLock()
				count := 0
				for _, inst := range ProxyMgr.instances {
					if inst != nil && (inst.Browser == browserType || (browserType == "proxy" && inst.Browser == "")) {
						count++
					}
				}
				ProxyMgr.mu.RUnlock()
				count++

				if count > 1 {
					label = fmt.Sprintf("%s %d", browserType, count)
				} else {
					label = browserType
				}
			}

			// Create complete proxy instance with all fields
			proxyInstance := &ProxyInstance{
				Proxy:      newProxy,
				Browser:    body.Browser,
				BrowserCmd: nil, // Will be set later if browser is launched
				Label:      label,
			}

			// Add complete instance to manager
			ProxyMgr.AddProxyInstance(proxyID, proxyInstance)

			// Update PROXY for backward compatibility
			updateProxyVar()

			// Load initial intercept filters
			if err := backend.loadInterceptFilters(); err != nil {
				log.Printf("[StartProxy] Warning: Failed to load intercept filters: %v", err)
			}

			// Start the proxy
			if err := newProxy.RunProxy(); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			if body.Browser != "" {
				// Use the certificate path from the rawproxy
				certPath := newProxy.GetCertPath()
				go func(proxyID, browserType, listenAddr, cert string) {
					cmd, err := browser.LaunchBrowser(browserType, listenAddr, cert)
					if err != nil {
						log.Println("Error launching browser:", err)
						return
					}
					ProxyMgr.mu.Lock()
					if inst := ProxyMgr.instances[proxyID]; inst != nil {
						inst.Browser = browserType
						inst.BrowserCmd = cmd
					}
					ProxyMgr.mu.Unlock()
				}(proxyID, body.Browser, body.HTTP, certPath)
			}

			return c.JSON(http.StatusOK, map[string]any{"id": proxyID, "listenAddr": proxyID, "label": label, "browser": body.Browser})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

func (backend *Backend) StopProxy(e *core.ServeEvent) error {

	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/proxy/stop",
		Handler: func(c echo.Context) error {

			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil

			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			var body ProxyBody
			if err := c.Bind(&body); err != nil {
				// If no body provided and field is optional, stop all proxies
				log.Println("[StopProxy] No body or empty body provided, stopping all proxies")
				proxyIDs := ProxyMgr.GetAllProxies()
				for _, proxyID := range proxyIDs {
					if err := ProxyMgr.StopProxy(proxyID); err != nil {
						log.Printf("[WARN] Error stopping proxy %s: %v", proxyID, err)
					}
					ProxyMgr.RemoveProxy(proxyID)
				}
			} else if body.HTTP != "" {
				// Stop specific proxy
				proxyID := body.HTTP
				log.Printf("[StopProxy] Stopping specific proxy: %s", proxyID)

				// Check if proxy exists
				if proxy := ProxyMgr.GetProxy(proxyID); proxy == nil {
					log.Printf("[StopProxy][WARN] Proxy %s not found in manager", proxyID)
				}

				if err := ProxyMgr.StopProxy(proxyID); err != nil {
					log.Printf("[StopProxy][ERROR] Failed to stop proxy %s: %v", proxyID, err)
					return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
				}
				log.Printf("[StopProxy] Removing proxy %s from manager", proxyID)
				ProxyMgr.RemoveProxy(proxyID)
			} else {
				// No HTTP field, stop all proxies
				log.Println("[StopProxy] HTTP field not specified, stopping all proxies")
				proxyIDs := ProxyMgr.GetAllProxies()
				for _, proxyID := range proxyIDs {
					if err := ProxyMgr.StopProxy(proxyID); err != nil {
						log.Printf("[WARN] Error stopping proxy %s: %v", proxyID, err)
					}
					ProxyMgr.RemoveProxy(proxyID)
				}
			}

			// Update PROXY for backward compatibility
			updateProxyVar()

			return c.JSON(http.StatusOK, map[string]any{"message": "Proxy stopped"})
		},
	})
	return nil
}

func (backend *Backend) ListProxies(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodGet,
		Path:   "/api/proxy/list",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil

			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			ProxyMgr.mu.RLock()
			instances := make([]map[string]interface{}, 0, len(ProxyMgr.instances))
			for id, inst := range ProxyMgr.instances {
				if inst != nil && inst.Proxy != nil {
					var browserPid int
					if inst.BrowserCmd != nil && inst.BrowserCmd.Process != nil {
						browserPid = inst.BrowserCmd.Process.Pid
					}
					instances = append(instances, map[string]interface{}{
						"id":         id,
						"listenAddr": id,
						"label":      inst.Label,
						"browser":    inst.Browser,
						"browserPid": browserPid,
					})
				}
			}
			ProxyMgr.mu.RUnlock()

			return c.JSON(http.StatusOK, map[string]interface{}{
				"proxies": instances,
				"count":   len(instances),
			})
		},
	})
	return nil
}
