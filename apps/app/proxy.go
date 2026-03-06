package app

import (
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/glitchedgitz/grroxy/grx/browser"
	"github.com/glitchedgitz/grroxy/internal/utils"
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
	Chrome     *browser.ChromeRemote
}

// ProxyManager manages multiple proxy instances
type ProxyManager struct {
	instances  map[string]*ProxyInstance
	mu         sync.RWMutex
	index      atomic.Uint64 // Shared atomic counter for unique indices across all proxies (for requests)
	proxyIndex atomic.Uint64 // Counter for proxy IDs
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

// GetNextProxyID returns the next unique proxy ID (thread-safe)
func (pm *ProxyManager) GetNextProxyID() string {
	idx := pm.proxyIndex.Add(1)
	return utils.FormatNumericID(float64(idx), 15)
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

// initializeProxyIndexFromDB queries the database to get the current max proxy count
func (pm *ProxyManager) initializeProxyIndexFromDB(backend *Backend) error {
	dao := backend.App.Dao()

	// Query for the total number of proxies in _proxies collection
	var result struct {
		TotalProxies int `db:"total_proxies" json:"total_proxies"`
	}

	err := dao.DB().
		NewQuery("SELECT COUNT(*) as total_proxies FROM _proxies").
		One(&result)

	if err != nil {
		return fmt.Errorf("failed to query total proxies: %w", err)
	}

	// Set the proxy index counter
	totalProxies := uint64(result.TotalProxies)
	pm.proxyIndex.Store(totalProxies)

	log.Printf("[ProxyManager] Proxy Index Initialization:")
	log.Printf("[ProxyManager]   - Total proxies in database: %d", totalProxies)
	log.Printf("[ProxyManager]   - Next proxy ID will use index: %d", totalProxies+1)

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

// GetChromeRemote returns a ChromeRemote instance for a proxy, initializing it if necessary
func (pm *ProxyManager) GetChromeRemote(proxyID string) (*browser.ChromeRemote, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	inst := pm.instances[proxyID]
	if inst == nil {
		return nil, fmt.Errorf("proxy %s not found", proxyID)
	}

	if inst.Browser != "chrome" {
		return nil, fmt.Errorf("proxy %s does not have a Chrome browser attached", proxyID)
	}

	if inst.Chrome != nil {
		return inst.Chrome, nil
	}

	if inst.BrowserCmd == nil || inst.BrowserCmd.Process == nil {
		return nil, fmt.Errorf("Chrome browser process not running for proxy %s", proxyID)
	}

	// Get profile directory
	var profileDir string
	for _, arg := range inst.BrowserCmd.Args {
		if strings.HasPrefix(arg, "--user-data-dir=") {
			profileDir = strings.TrimPrefix(arg, "--user-data-dir=")
			break
		}
	}

	if profileDir == "" {
		return nil, fmt.Errorf("could not determine Chrome profile directory for proxy %s", proxyID)
	}

	// Get debug URL
	debugURL, err := browser.GetChromeDebugURL(profileDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get Chrome debug URL: %v", err)
	}

	// Connect to Chrome
	cr, err := browser.NewChromeRemote(debugURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Chrome: %v", err)
	}
	inst.Chrome = cr
	return cr, nil
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
	// close ChromeRemote if any
	if inst.Chrome != nil {
		log.Printf("[ProxyManager] Closing ChromeRemote for proxy %s", id)
		inst.Chrome.Close()
		inst.Chrome = nil
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

// TakeScreenshot captures a screenshot using the Chrome browser attached to a proxy instance
// Returns: screenshot bytes, file path (if saved), error
func (pm *ProxyManager) TakeScreenshot(proxyID string, fullPage bool, savePath string) ([]byte, string, error) {
	pm.mu.Lock()
	inst := pm.instances[proxyID]
	if inst == nil {
		pm.mu.Unlock()
		return nil, "", fmt.Errorf("proxy %s not found", proxyID)
	}

	if inst.Browser != "chrome" {
		pm.mu.Unlock()
		return nil, "", fmt.Errorf("proxy %s does not have a Chrome browser attached (browser: %s)", proxyID, inst.Browser)
	}

	// Initialize ChromeRemote if not already present
	if inst.Chrome == nil {
		if inst.BrowserCmd == nil || inst.BrowserCmd.Process == nil {
			pm.mu.Unlock()
			return nil, "", fmt.Errorf("Chrome browser process not running for proxy %s", proxyID)
		}

		// Get the profile directory from the browser command
		var profileDir string
		for _, arg := range inst.BrowserCmd.Args {
			if strings.HasPrefix(arg, "--user-data-dir=") {
				profileDir = strings.TrimPrefix(arg, "--user-data-dir=")
				break
			}
		}

		if profileDir == "" {
			pm.mu.Unlock()
			return nil, "", fmt.Errorf("could not determine Chrome profile directory for proxy %s", proxyID)
		}

		// Get the Chrome debug URL
		debugURL, err := browser.GetChromeDebugURL(profileDir)
		if err != nil {
			pm.mu.Unlock()
			return nil, "", fmt.Errorf("failed to get Chrome debug URL: %v", err)
		}

		// Connect to Chrome
		cr, err := browser.NewChromeRemote(debugURL)
		if err != nil {
			pm.mu.Unlock()
			return nil, "", fmt.Errorf("failed to connect to Chrome: %v", err)
		}
		inst.Chrome = cr
	}
	chrome := inst.Chrome
	pm.mu.Unlock()

	// Capture the screenshot
	screenshotBytes, err := chrome.TakeScreenshot("", fullPage)
	if err != nil {
		return nil, "", fmt.Errorf("failed to capture screenshot: %v", err)
	}

	// Save to file if path is provided
	var filePath string
	if savePath != "" {
		if err := os.WriteFile(savePath, screenshotBytes, 0644); err != nil {
			return screenshotBytes, "", fmt.Errorf("failed to save screenshot to %s: %v", savePath, err)
		}
		filePath = savePath
		log.Printf("[TakeScreenshot] Screenshot saved to: %s", filePath)
	}

	return screenshotBytes, filePath, nil
}

// ClickElement clicks an element on the page using the Chrome browser attached to a proxy instance
func (pm *ProxyManager) ClickElement(proxyID string, url string, selector string, waitForNavigation bool) error {
	pm.mu.Lock()
	inst := pm.instances[proxyID]
	if inst == nil {
		pm.mu.Unlock()
		return fmt.Errorf("proxy %s not found", proxyID)
	}

	if inst.Browser != "chrome" {
		pm.mu.Unlock()
		return fmt.Errorf("proxy %s does not have a Chrome browser attached (browser: %s)", proxyID, inst.Browser)
	}

	// Initialize ChromeRemote if not already present
	if inst.Chrome == nil {
		if inst.BrowserCmd == nil || inst.BrowserCmd.Process == nil {
			pm.mu.Unlock()
			return fmt.Errorf("Chrome browser process not running for proxy %s", proxyID)
		}

		var profileDir string
		for _, arg := range inst.BrowserCmd.Args {
			if strings.HasPrefix(arg, "--user-data-dir=") {
				profileDir = strings.TrimPrefix(arg, "--user-data-dir=")
				break
			}
		}

		if profileDir == "" {
			pm.mu.Unlock()
			return fmt.Errorf("could not determine Chrome profile directory for proxy %s", proxyID)
		}

		debugURL, err := browser.GetChromeDebugURL(profileDir)
		if err != nil {
			pm.mu.Unlock()
			return fmt.Errorf("failed to get Chrome debug URL: %v", err)
		}

		cr, err := browser.NewChromeRemote(debugURL)
		if err != nil {
			pm.mu.Unlock()
			return fmt.Errorf("failed to connect to Chrome: %v", err)
		}
		inst.Chrome = cr
	}
	chrome := inst.Chrome
	pm.mu.Unlock()

	// Click the element
	if err := chrome.ClickElement("", url, selector, waitForNavigation); err != nil {
		return fmt.Errorf("failed to click element: %v", err)
	}

	return nil
}

// GetElements retrieves information about clickable elements on the page
func (pm *ProxyManager) GetElements(proxyID string, url string) ([]browser.ElementInfo, error) {
	pm.mu.Lock()
	inst := pm.instances[proxyID]
	if inst == nil {
		pm.mu.Unlock()
		return nil, fmt.Errorf("proxy %s not found", proxyID)
	}

	if inst.Browser != "chrome" {
		pm.mu.Unlock()
		return nil, fmt.Errorf("proxy %s does not have a Chrome browser attached (browser: %s)", proxyID, inst.Browser)
	}

	// Initialize ChromeRemote if not already present
	if inst.Chrome == nil {
		if inst.BrowserCmd == nil || inst.BrowserCmd.Process == nil {
			pm.mu.Unlock()
			return nil, fmt.Errorf("Chrome browser process not running for proxy %s", proxyID)
		}

		var profileDir string
		for _, arg := range inst.BrowserCmd.Args {
			if strings.HasPrefix(arg, "--user-data-dir=") {
				profileDir = strings.TrimPrefix(arg, "--user-data-dir=")
				break
			}
		}

		if profileDir == "" {
			pm.mu.Unlock()
			return nil, fmt.Errorf("could not determine Chrome profile directory for proxy %s", proxyID)
		}

		debugURL, err := browser.GetChromeDebugURL(profileDir)
		if err != nil {
			pm.mu.Unlock()
			return nil, fmt.Errorf("failed to get Chrome debug URL: %v", err)
		}

		cr, err := browser.NewChromeRemote(debugURL)
		if err != nil {
			pm.mu.Unlock()
			return nil, fmt.Errorf("failed to connect to Chrome: %v", err)
		}
		inst.Chrome = cr
	}
	chrome := inst.Chrome
	pm.mu.Unlock()

	// Get elements from the page
	elements, err := chrome.GetElements("", url)
	if err != nil {
		return nil, fmt.Errorf("failed to get elements: %v", err)
	}

	return elements, nil
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

// loadProxySettings loads intercept and filter settings for a proxy
func (backend *Backend) loadProxySettings(proxy *RawProxyWrapper, proxyRecord *models.Record) error {
	log.Printf("[ProxySettings] Loading settings for proxy ID: %s", proxyRecord.Id)

	// Load intercept setting from _proxies record
	intercept := proxyRecord.GetBool("intercept")
	proxy.Intercept = intercept
	log.Printf("[ProxySettings] Intercept: %v", intercept)

	// Load filters from _ui collection (format: proxy/{proxyID})
	filterstring, err := backend.loadProxyFilters(proxyRecord.Id)
	if err != nil {
		log.Printf("[ProxySettings] Error loading filters: %v, using empty filters", err)
		filterstring = ""
	}

	proxy.Filters = filterstring
	log.Printf("[ProxySettings] Filters: %s", filterstring)

	return nil
}

type ProxyBody struct {
	HTTP    string `json:"http,omitempty"`
	Browser string `json:"browser,omitempty"`
	Name    string `json:"name,omitempty"` // Optional name for the proxy instance
}

func (backend *Backend) InitializeProxy() error {
	log.Println("[InitializeProxy] Initializing proxy index from database...")
	if ProxyMgr.proxyIndex.Load() == 0 {
		if err := ProxyMgr.initializeIndexFromDB(backend); err != nil {
			log.Printf("[StartProxy] Warning: Failed to initialize proxy index from database: %v", err)
			return err
		}
	}
	log.Println("[InitializeProxy] Proxy index initialized from database:", ProxyMgr.index.Load())
	return nil
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

			// Initialize proxy index from database if not already initialized
			if ProxyMgr.proxyIndex.Load() == 0 {
				if err := ProxyMgr.initializeProxyIndexFromDB(backend); err != nil {
					log.Printf("[StartProxy] Warning: Failed to initialize proxy index from database: %v", err)
				}
			}

			// Generate unique proxy ID (this will be the primary ID, not the listen address)
			proxyID := ProxyMgr.GetNextProxyID()
			log.Printf("[StartProxy] Generated proxy ID: %s for address: %s", proxyID, body.HTTP)

			// Create new rawproxy wrapper
			configDir := path.Join(backend.Config.ConfigDirectory)

			// Disable file captures by passing empty string (we save to database instead)
			// To enable file captures for testing, uncomment the line below:
			// outputDir := path.Join(backend.Config.ConfigDirectory, "captures")
			outputDir := "" // Empty = disabled

			newProxy, err := NewRawProxyWrapper(body.HTTP, configDir, outputDir, backend, proxyID)
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

			// Add complete instance to manager using the formatted ID as key
			ProxyMgr.AddProxyInstance(proxyID, proxyInstance)

			// Update PROXY for backward compatibility
			updateProxyVar()

			// // Load initial intercept and filter settings from proxy record
			// if err := backend.loadProxySettings(newProxy, proxyRecord); err != nil {
			// 	log.Printf("[StartProxy] Warning: Failed to load proxy settings: %v", err)
			// }

			// Start the proxy
			if err := newProxy.RunProxy(); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			if body.Browser != "" {
				// Use the certificate path from the rawproxy
				certPath := newProxy.GetCertPath()

				// Generate browser profile directory: [projectid]+[proxyid]
				// This ensures each browser instance has its own isolated profile
				profileID := backend.Config.ProjectID + proxyID
				profileDir := path.Join(backend.Config.ConfigDirectory, "profiles", profileID)
				log.Printf("[StartProxy] Browser profile directory: %s", profileDir)

				go func(proxyID, browserType, listenAddr, cert, profDir string) {
					cmd, err := browser.LaunchBrowser(browserType, listenAddr, cert, profDir)
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
				}(proxyID, body.Browser, body.HTTP, certPath, profileDir)
			}

			// Create proxy record in database
			dao := backend.App.Dao()
			proxiesCollection, err := dao.FindCollectionByNameOrId("_proxies")
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": fmt.Sprintf("Failed to find _proxies collection: %v", err)})
			}

			proxyRecord := models.NewRecord(proxiesCollection)
			proxyRecord.Set("id", proxyID)
			proxyRecord.Set("label", label)
			proxyRecord.Set("addr", body.HTTP)
			proxyRecord.Set("browser", body.Browser)
			proxyRecord.Set("intercept", false) // Default to false
			proxyRecord.Set("state", "running")
			proxyRecord.Set("color", "")
			proxyRecord.Set("profile", "")

			// Initialize data column (filters are now stored separately in _ui collection)
			proxyData := map[string]interface{}{}
			proxyRecord.Set("data", proxyData)

			if err := dao.SaveRecord(proxyRecord); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": fmt.Sprintf("Failed to save proxy record: %v", err)})
			}

			log.Printf("[StartProxy] Created proxy record in database with ID: %s", proxyID)

			return c.JSON(http.StatusOK, map[string]any{
				"id":         proxyID,
				"listenAddr": body.HTTP,
				"label":      label,
				"browser":    body.Browser,
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

// updateProxyState updates the state field of a proxy record
func (backend *Backend) updateProxyState(proxyID string, state string) {
	dao := backend.App.Dao()
	proxyRecord, err := dao.FindRecordById("_proxies", proxyID)
	if err != nil {
		log.Printf("[ProxyState][WARN] Failed to find proxy record %s: %v", proxyID, err)
		return
	}

	proxyRecord.Set("state", state)
	if err := dao.SaveRecord(proxyRecord); err != nil {
		log.Printf("[ProxyState][WARN] Failed to update proxy state for %s: %v", proxyID, err)
	} else {
		log.Printf("[ProxyState] Updated proxy %s state to: %s", proxyID, state)
	}
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

			type StopProxyBody struct {
				ID string `json:"id,omitempty"` // Formatted ID like "______________1"
			}

			var body StopProxyBody
			if err := c.Bind(&body); err != nil {
				// If no body provided and field is optional, stop all proxies
				log.Println("[StopProxy] No body or empty body provided, stopping all proxies")
				proxyIDs := ProxyMgr.GetAllProxies()
				for _, proxyID := range proxyIDs {
					if err := ProxyMgr.StopProxy(proxyID); err != nil {
						log.Printf("[WARN] Error stopping proxy %s: %v", proxyID, err)
					}
					backend.updateProxyState(proxyID, "")
					ProxyMgr.RemoveProxy(proxyID)
				}
			} else if body.ID != "" {
				// Stop specific proxy by ID
				proxyID := body.ID
				log.Printf("[StopProxy] Stopping specific proxy: %s", proxyID)

				// Check if proxy exists
				if proxy := ProxyMgr.GetProxy(proxyID); proxy == nil {
					log.Printf("[StopProxy][WARN] Proxy %s not found in manager", proxyID)
					return c.JSON(http.StatusNotFound, map[string]interface{}{"error": fmt.Sprintf("Proxy %s not found", proxyID)})
				}

				if err := ProxyMgr.StopProxy(proxyID); err != nil {
					log.Printf("[StopProxy][ERROR] Failed to stop proxy %s: %v", proxyID, err)
					return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
				}

				backend.updateProxyState(proxyID, "")
				log.Printf("[StopProxy] Removing proxy %s from manager", proxyID)
				ProxyMgr.RemoveProxy(proxyID)
			} else {
				// No ID field, stop all proxies
				log.Println("[StopProxy] ID field not specified, stopping all proxies")
				proxyIDs := ProxyMgr.GetAllProxies()
				for _, proxyID := range proxyIDs {
					if err := ProxyMgr.StopProxy(proxyID); err != nil {
						log.Printf("[WARN] Error stopping proxy %s: %v", proxyID, err)
					}
					backend.updateProxyState(proxyID, "")
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

func (backend *Backend) RestartProxy(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/proxy/restart",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil
			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			type RestartProxyBody struct {
				ID string `json:"id"` // Formatted ID like "______________1"
			}

			var body RestartProxyBody
			if err := c.Bind(&body); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Invalid request body"})
			}

			if body.ID == "" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Proxy ID is required"})
			}

			proxyID := body.ID
			log.Printf("[RestartProxy] Restarting proxy: %s", proxyID)

			// Check if proxy is already running
			if ProxyMgr.GetProxy(proxyID) != nil {
				return c.JSON(http.StatusConflict, map[string]interface{}{"error": "Proxy is already running"})
			}

			// Get the proxy record from database
			dao := backend.App.Dao()
			proxyRecord, err := dao.FindRecordById("_proxies", proxyID)
			if err != nil {
				log.Printf("[RestartProxy] Proxy record not found: %s", proxyID)
				return c.JSON(http.StatusNotFound, map[string]interface{}{"error": "Proxy record not found"})
			}

			// Read proxy configuration from record
			listenAddr := proxyRecord.GetString("addr")
			browserType := proxyRecord.GetString("browser")
			label := proxyRecord.GetString("label")

			log.Printf("[RestartProxy] Found proxy config - addr: %s, browser: %s, label: %s", listenAddr, browserType, label)

			// Check if port is available
			availableHost, err := utils.CheckAndFindAvailablePort(listenAddr)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			if availableHost != listenAddr {
				if browserType == "" {
					return c.JSON(http.StatusConflict, map[string]interface{}{
						"error":         "port not available",
						"availableHost": availableHost,
					})
				}
			}

			// Initialize global index from database if not already initialized
			if ProxyMgr.index.Load() == 0 {
				if err := ProxyMgr.initializeIndexFromDB(backend); err != nil {
					log.Printf("[RestartProxy] Warning: Failed to initialize global index from database: %v", err)
				}
			}

			// Create new rawproxy wrapper with existing ID
			configDir := path.Join(backend.Config.ConfigDirectory)
			outputDir := "" // Disabled

			listenAddr = availableHost

			newProxy, err := NewRawProxyWrapper(listenAddr, configDir, outputDir, backend, proxyID)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			// Update proxy record state to running
			proxyRecord.Set("state", "running")
			proxyRecord.Set("addr", listenAddr)
			if err := dao.SaveRecord(proxyRecord); err != nil {
				log.Printf("[RestartProxy][WARN] Failed to update proxy state: %v", err)
			}

			// Create proxy instance
			proxyInstance := &ProxyInstance{
				Proxy:      newProxy,
				Browser:    browserType,
				BrowserCmd: nil,
				Label:      label,
			}

			// Add to manager with the same ID
			ProxyMgr.AddProxyInstance(proxyID, proxyInstance)

			// Update PROXY for backward compatibility
			updateProxyVar()

			// Load intercept and filter settings from proxy record
			if err := backend.loadProxySettings(newProxy, proxyRecord); err != nil {
				log.Printf("[RestartProxy] Warning: Failed to load proxy settings: %v", err)
			}

			// Start the proxy
			if err := newProxy.RunProxy(); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			// Launch browser if configured
			if browserType != "" {
				certPath := newProxy.GetCertPath()

				// Generate browser profile directory: [projectid]+[proxyid]
				profileID := backend.Config.ProjectID + proxyID
				profileDir := path.Join(backend.Config.ConfigDirectory, "profiles", profileID)
				log.Printf("[RestartProxy] Browser profile directory: %s", profileDir)

				go func(proxyID, browserType, listenAddr, cert, profDir string) {
					cmd, err := browser.LaunchBrowser(browserType, listenAddr, cert, profDir)
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
				}(proxyID, browserType, listenAddr, certPath, profileDir)
			}

			log.Printf("[RestartProxy] Successfully restarted proxy %s", proxyID)

			return c.JSON(http.StatusOK, map[string]any{
				"id":         proxyID,
				"listenAddr": listenAddr,
				"label":      label,
				"browser":    browserType,
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
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
						"id":         id,                    // Formatted ID like "______________1"
						"listenAddr": inst.Proxy.listenAddr, // Listen address like "127.0.0.1:8080"
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

func (backend *Backend) ScreenshotProxy(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/proxy/screenshot",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil

			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			type ScreenshotBody struct {
				ID       string `json:"id"`                 // Proxy ID (required)
				URL      string `json:"url,omitempty"`      // URL to navigate to (optional, empty = current tab)
				FullPage bool   `json:"fullPage,omitempty"` // Capture full page or viewport (default: false)
				SaveFile bool   `json:"saveFile,omitempty"` // Save to disk in cache directory (default: false)
			}

			var body ScreenshotBody
			if err := c.Bind(&body); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Invalid request body"})
			}

			if body.ID == "" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Proxy ID is required"})
			}

			log.Printf("[ScreenshotProxy] Taking screenshot for proxy %s (url=%s, fullPage=%v, saveFile=%v)",
				body.ID, body.URL, body.FullPage, body.SaveFile)

			// Generate file path if saveFile is requested
			var savePath string
			if body.SaveFile {
				timestamp := time.Now().Format("20060102-150405")
				filename := fmt.Sprintf("screenshot-%s.png", timestamp)
				savePath = path.Join(backend.Config.CacheDirectory, filename)
				log.Printf("[ScreenshotProxy] Will save screenshot to: %s", savePath)
			}

			// Capture the screenshot using ProxyManager
			screenshotBytes, filePath, err := ProxyMgr.TakeScreenshot(body.ID, body.FullPage, savePath)
			if err != nil {
				log.Printf("[ScreenshotProxy] Error taking screenshot: %v", err)
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			// Encode screenshot as base64 for JSON response
			screenshotBase64 := base64.StdEncoding.EncodeToString(screenshotBytes)

			response := map[string]interface{}{
				"screenshot": screenshotBase64,
				"size":       len(screenshotBytes),
				"timestamp":  time.Now().Format(time.RFC3339),
			}

			if filePath != "" {
				response["filePath"] = filePath
			}

			log.Printf("[ScreenshotProxy] Screenshot captured successfully (%d bytes)", len(screenshotBytes))
			return c.JSON(http.StatusOK, response)
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

func (backend *Backend) ClickProxy(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/proxy/click",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil

			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			type ClickBody struct {
				ID                string `json:"id"`                          // Proxy ID (required)
				URL               string `json:"url,omitempty"`               // URL to navigate to (optional, empty = current page)
				Selector          string `json:"selector"`                    // CSS selector for element to click (required)
				WaitForNavigation bool   `json:"waitForNavigation,omitempty"` // Wait for navigation after click (default: false)
			}

			var body ClickBody
			if err := c.Bind(&body); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Invalid request body"})
			}

			if body.ID == "" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Proxy ID is required"})
			}

			if body.Selector == "" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Selector is required"})
			}

			log.Printf("[ClickProxy] Clicking element for proxy %s (url=%s, selector=%s, waitNav=%v)",
				body.ID, body.URL, body.Selector, body.WaitForNavigation)

			// Click the element using ProxyManager
			err := ProxyMgr.ClickElement(body.ID, body.URL, body.Selector, body.WaitForNavigation)
			if err != nil {
				log.Printf("[ClickProxy] Error clicking element: %v", err)
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			log.Printf("[ClickProxy] Element clicked successfully")
			return c.JSON(http.StatusOK, map[string]interface{}{
				"success":   true,
				"message":   "Element clicked successfully",
				"selector":  body.Selector,
				"timestamp": time.Now().Format(time.RFC3339),
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

func (backend *Backend) GetElementsProxy(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/proxy/elements",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil

			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			type ElementsBody struct {
				ID  string `json:"id"`            // Proxy ID (required)
				URL string `json:"url,omitempty"` // URL to navigate to (optional, empty = current page)
			}

			var body ElementsBody
			if err := c.Bind(&body); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Invalid request body"})
			}

			if body.ID == "" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Proxy ID is required"})
			}

			log.Printf("[GetElementsProxy] Getting elements for proxy %s (url=%s)", body.ID, body.URL)

			// Get elements using ProxyManager
			elements, err := ProxyMgr.GetElements(body.ID, body.URL)
			if err != nil {
				log.Printf("[GetElementsProxy] Error getting elements: %v", err)
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			log.Printf("[GetElementsProxy] Found %d clickable elements", len(elements))
			return c.JSON(http.StatusOK, map[string]interface{}{
				"elements":  elements,
				"count":     len(elements),
				"timestamp": time.Now().Format(time.RFC3339),
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

// ListChromeTabs endpoint - lists all open tabs in Chrome
func (backend *Backend) ListChromeTabs(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/proxy/chrome/tabs",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil
			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			type ListTabsBody struct {
				ProxyID string `json:"proxyId"`
			}

			var body ListTabsBody
			if err := c.Bind(&body); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Invalid request body"})
			}

			if body.ProxyID == "" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "proxyId is required"})
			}

			// Get Chrome remote
			chrome, err := ProxyMgr.GetChromeRemote(body.ProxyID)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			// List tabs
			tabs, err := chrome.ListTabs()
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": fmt.Sprintf("Failed to list tabs: %v", err)})
			}

			return c.JSON(http.StatusOK, map[string]interface{}{
				"tabs":      tabs,
				"count":     len(tabs),
				"timestamp": time.Now().Format(time.RFC3339),
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

// OpenChromeTab endpoint - opens a new tab in Chrome
func (backend *Backend) OpenChromeTab(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/proxy/chrome/tab/open",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil
			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			type OpenTabBody struct {
				ProxyID string `json:"proxyId"`
				URL     string `json:"url"` // Optional, defaults to about:blank
			}

			var body OpenTabBody
			if err := c.Bind(&body); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Invalid request body"})
			}

			if body.ProxyID == "" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "proxyId is required"})
			}

			// Get Chrome remote
			chrome, err := ProxyMgr.GetChromeRemote(body.ProxyID)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			// Open new tab
			targetID, err := chrome.OpenTab(body.URL)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": fmt.Sprintf("Failed to open tab: %v", err)})
			}

			return c.JSON(http.StatusOK, map[string]interface{}{
				"targetId":  targetID,
				"url":       body.URL,
				"timestamp": time.Now().Format(time.RFC3339),
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

// NavigateChromeTab endpoint - navigates a tab to a URL
func (backend *Backend) NavigateChromeTab(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/proxy/chrome/tab/navigate",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil
			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

			type NavigateTabBody struct {
				ProxyID   string `json:"proxyId"`
				TargetID  string `json:"targetId"` // Optional, empty = active tab
				URL       string `json:"url"`
				WaitUntil string `json:"waitUntil"` // Optional: domcontentloaded, load, networkidle
				TimeoutMs int    `json:"timeoutMs"` // Optional: timeout in milliseconds
			}

			var body NavigateTabBody
			if err := c.Bind(&body); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Invalid request body"})
			}

			if body.ProxyID == "" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "proxyId is required"})
			}

			if body.URL == "" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "url is required"})
			}

			// Get Chrome remote
			chrome, err := ProxyMgr.GetChromeRemote(body.ProxyID)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			}

			// Navigate tab
			result, err := chrome.Navigate(body.TargetID, body.URL, body.WaitUntil, body.TimeoutMs)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": fmt.Sprintf("Failed to navigate tab: %v", err)})
			}

			return c.JSON(http.StatusOK, map[string]interface{}{
				"targetId":     body.TargetID,
				"url":          result.FinalURL,
				"status":       result.Status,
				"navigationId": result.NavigationID,
				"timestamp":    time.Now().Format(time.RFC3339),
			})
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}
