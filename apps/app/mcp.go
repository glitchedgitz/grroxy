package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/glitchedgitz/grroxy/grx/version"
	"github.com/glitchedgitz/grroxy/internal/save"
	"github.com/glitchedgitz/pocketbase/core"
	"github.com/labstack/echo/v5"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// ---------------------------------------------------------------------------
// Chat session correlation
//
// The cybernetic-ui frontend stamps each AI chat with an ID; the bridge
// passes it through as ?chatId=<id> on the MCP SSE URL. We capture it at
// session-creation time and surface it on every tool-handler context so
// resource-creating tools (sendRequest, etc.) can set generated_by="ai/<id>".
// ---------------------------------------------------------------------------

type chatIDCtxKey struct{}

// sessionID -> chatID. Populated when SSE establishes; read on each /mcp/message.
// Bounded by the number of concurrent SSE sessions; not actively swept since
// session lifetimes are short and entries don't accumulate without traffic.
var mcpSessionChatIDs sync.Map

// ChatIDFromContext returns the chat ID associated with an MCP request, or "".
func ChatIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(chatIDCtxKey{}).(string); ok {
		return v
	}
	return ""
}

func mcpGenerateSessionID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// ---------------------------------------------------------------------------
// MCP state
// ---------------------------------------------------------------------------

type MCP struct {
	server    *mcpserver.MCPServer
	sseServer *mcpserver.SSEServer
	active    bool
	conns     atomic.Int64
}

// ---------------------------------------------------------------------------
// Tool registration
// ---------------------------------------------------------------------------

func (backend *Backend) mcpInit() {
	s := mcpserver.NewMCPServer(
		"grroxy",
		version.CURRENT_BACKEND_VERSION,
		mcpserver.WithToolCapabilities(true),
	)

	// --- Utility tools ---

	s.AddTool(
		mcp.NewTool("grroxyStatus",
			mcp.WithDescription("Check if the grroxy is active"),
		),
		backend.versionHandler,
	)

	// --- Data tools ---

	s.AddTool(
		mcp.NewTool("getRequestResponseFromID",
			mcp.WithDescription("Get the active request and response for active ID"),
			mcp.WithInputSchema[GetRequestResponseArgs](),
		),
		backend.getRequestResponseFromIDHandler,
	)

	s.AddTool(
		mcp.NewTool("hostPrintSitemap",
			mcp.WithDescription("Get the sitemap for a host"),
			mcp.WithInputSchema[HostPrintSitemapArgs](),
		),
		backend.hostPrintSitemapHandler,
	)

	s.AddTool(
		mcp.NewTool("hostPrintRowsInDetails",
			mcp.WithDescription("Get the table for a host"),
			mcp.WithInputSchema[HostPrintRowsArgs](),
		),
		backend.hostPrintRowsInDetailsHandler,
	)

	s.AddTool(
		mcp.NewTool("listHosts",
			mcp.WithDescription("List all hosts with their technologies (as names) and labels (as names)"),
			mcp.WithInputSchema[ListHostsArgs](),
		),
		backend.listHostsHandler,
	)

	s.AddTool(
		mcp.NewTool("getHostInfo",
			mcp.WithDescription("Get detailed info for a specific host by ID, including technologies (as names), labels (as names), and notes"),
			mcp.WithInputSchema[GetHostInfoArgs](),
		),
		backend.getHostInfoHandler,
	)

	s.AddTool(
		mcp.NewTool("getNoteForHost",
			mcp.WithDescription("Get the note for a host"),
			mcp.WithInputSchema[GetNoteForHostArgs](),
		),
		backend.getNoteForHostHandler,
	)

	s.AddTool(
		mcp.NewTool("setNoteForHost",
			mcp.WithDescription("Set the note for a host"),
			mcp.WithInputSchema[SetNoteForHostArgs](),
		),
		backend.setNoteForHostHandler,
	)

	s.AddTool(
		mcp.NewTool("modifyHostLabels",
			mcp.WithDescription("Add or remove labels from a host"),
			mcp.WithInputSchema[ModifyHostLabelsArgs](),
		),
		backend.modifyHostLabelsHandler,
	)

	s.AddTool(
		mcp.NewTool("modifyHostNotes",
			mcp.WithDescription("Add, update, or remove notes for a host"),
			mcp.WithInputSchema[ModifyHostNotesArgs](),
		),
		backend.modifyHostNotesHandler,
	)

	// --- Action tools ---

	s.AddTool(
		mcp.NewTool("sendRequest",
			mcp.WithDescription("Send a request via http. Mind the terminating the request with \\r\\n\\r\\n or \\n\\n if there are no body, mind the content length of the body, it should exactly match"),
			mcp.WithInputSchema[SendRequestArgs](),
		),
		backend.sendRequestHandler,
	)

	// --- Intercept tools ---

	s.AddTool(
		mcp.NewTool("interceptToggle",
			mcp.WithDescription("Enable or disable request/response interception on a proxy. When disabled, all pending intercepts are automatically forwarded"),
			mcp.WithInputSchema[InterceptToggleArgs](),
		),
		backend.interceptToggleHandler,
	)

	s.AddTool(
		mcp.NewTool("interceptPrintRowsInDetails",
			mcp.WithDescription("List intercepted requests/responses for a proxy with full metadata (host, port, method, path, status, headers)"),
			mcp.WithInputSchema[InterceptReadArgs](),
		),
		backend.interceptReadHandler,
	)

	s.AddTool(
		mcp.NewTool("interceptGetRawRequestAndResponse",
			mcp.WithDescription("Get the raw HTTP request and response strings for a specific intercepted record, for reading or editing before forwarding"),
			mcp.WithInputSchema[InterceptGetRawArgs](),
		),
		backend.interceptGetRawHandler,
	)

	s.AddTool(
		mcp.NewTool("interceptAction",
			mcp.WithDescription("Take action on a pending intercept: forward (pass through, optionally with edits) or drop (block the request/response)"),
			mcp.WithInputSchema[InterceptActionArgs](),
		),
		backend.interceptActionHandler,
	)

	// --- Proxy tools ---

	s.AddTool(
		mcp.NewTool("proxyList",
			mcp.WithDescription("Get a list of all running proxy instances with their status, browser type, and configuration"),
		),
		backend.proxyListHandler,
	)

	s.AddTool(
		mcp.NewTool("proxyStart",
			mcp.WithDescription("Start a new proxy instance with optional browser attachment (chrome, firefox, or none)"),
			mcp.WithInputSchema[ProxyStartArgs](),
		),
		backend.proxyStartHandler,
	)

	s.AddTool(
		mcp.NewTool("proxyStop",
			mcp.WithDescription("Stop a running proxy instance by ID, or stop all proxies if no ID is provided"),
			mcp.WithInputSchema[ProxyStopArgs](),
		),
		backend.proxyStopHandler,
	)

	s.AddTool(
		mcp.NewTool("proxyScreenshot",
			mcp.WithDescription("Capture a screenshot from Chrome browser attached to a proxy instance via Chrome DevTools Protocol, wait after calling the tool"),
			mcp.WithInputSchema[ProxyScreenshotArgs](),
		),
		backend.proxyScreenshotHandler,
	)

	s.AddTool(
		mcp.NewTool("proxyClick",
			mcp.WithDescription("Click an element on the page using Chrome browser attached to a proxy instance via Chrome DevTools Protocol"),
			mcp.WithInputSchema[ProxyClickArgs](),
		),
		backend.proxyClickHandler,
	)

	s.AddTool(
		mcp.NewTool("proxyElements",
			mcp.WithDescription("Extract information about all interactive elements on the page (buttons, links, inputs, textareas, selects). Returns unique CSS selectors for each element using nth-of-type paths"),
			mcp.WithInputSchema[ProxyElementsArgs](),
		),
		backend.proxyElementsHandler,
	)

	s.AddTool(
		mcp.NewTool("proxyType",
			mcp.WithDescription("Type text into a form field (input, textarea) on the page. Clicks to focus, optionally clears existing value, then dispatches real key events"),
			mcp.WithInputSchema[ProxyTypeArgs](),
		),
		backend.proxyTypeHandler,
	)

	s.AddTool(
		mcp.NewTool("proxyEval",
			mcp.WithDescription("Execute arbitrary JavaScript in the page context and return the result. Useful for setting values, reading DOM state, triggering events, or any operation not covered by other tools"),
			mcp.WithInputSchema[ProxyEvalArgs](),
		),
		backend.proxyEvalHandler,
	)

	s.AddTool(
		mcp.NewTool("proxyWaitForSelector",
			mcp.WithDescription("Wait for a CSS selector to become visible on the page. Useful for SPA transitions where waitForNavigation doesn't work"),
			mcp.WithInputSchema[ProxyWaitForSelectorArgs](),
		),
		backend.proxyWaitForSelectorHandler,
	)

	// --- Template tools ---

	s.AddTool(
		mcp.NewTool("templateList",
			mcp.WithDescription("List all loaded templates with their hooks and task counts"),
		),
		backend.templateListHandler,
	)

	s.AddTool(
		mcp.NewTool("templateRead",
			mcp.WithDescription("Read a specific template's full content — info, hooks, tasks, conditions, and actions"),
			mcp.WithInputSchema[TemplateReadArgs](),
		),
		backend.templateReadHandler,
	)

	s.AddTool(
		mcp.NewTool("templateGetInfo",
			mcp.WithDescription("Get the full template syntax reference — actions, hooks, condition syntax (dadql), variable interpolation, and examples. Call this before creating templates."),
		),
		backend.templateGetInfoHandler,
	)

	s.AddTool(
		mcp.NewTool("templateCreate",
			mcp.WithDescription("Create a new template on the launcher. Call templateGetInfo first to learn the syntax."),
			mcp.WithInputSchema[TemplateCreateArgs](),
		),
		backend.templateCreateHandler,
	)

	s.AddTool(
		mcp.NewTool("templateUpdate",
			mcp.WithDescription("Update an existing template by ID. Only include fields you want to change."),
			mcp.WithInputSchema[TemplateUpdateArgs](),
		),
		backend.templateUpdateHandler,
	)

	// --- Chrome Tab tools ---

	s.AddTool(
		mcp.NewTool("proxyListTabs",
			mcp.WithDescription("Lists all open tabs in the Chrome browser attached to a proxy instance"),
			mcp.WithInputSchema[ProxyListTabsArgs](),
		),
		backend.proxyListTabsHandler,
	)

	s.AddTool(
		mcp.NewTool("proxyOpenTab",
			mcp.WithDescription("Opens a new tab in the Chrome browser attached to a proxy instance"),
			mcp.WithInputSchema[ProxyOpenTabArgs](),
		),
		backend.proxyOpenTabHandler,
	)

	s.AddTool(
		mcp.NewTool("proxyNavigateTab",
			mcp.WithDescription("Navigates a specific tab (or the active tab) to a URL with configurable wait conditions"),
			mcp.WithInputSchema[ProxyNavigateTabArgs](),
		),
		backend.proxyNavigateTabHandler,
	)

	s.AddTool(
		mcp.NewTool("proxyActivateTab",
			mcp.WithDescription("Switches focus to a specific tab, making it the active tab in Chrome"),
			mcp.WithInputSchema[ProxyActivateTabArgs](),
		),
		backend.proxyActivateTabHandler,
	)

	s.AddTool(
		mcp.NewTool("proxyCloseTab",
			mcp.WithDescription("Closes a specific tab in Chrome"),
			mcp.WithInputSchema[ProxyCloseTabArgs](),
		),
		backend.proxyCloseTabHandler,
	)

	s.AddTool(
		mcp.NewTool("proxyReloadTab",
			mcp.WithDescription("Reloads a specific tab or the active tab, optionally bypassing cache"),
			mcp.WithInputSchema[ProxyReloadTabArgs](),
		),
		backend.proxyReloadTabHandler,
	)

	s.AddTool(
		mcp.NewTool("proxyGoBack",
			mcp.WithDescription("Navigates back in the browser history for a specific tab or the active tab"),
			mcp.WithInputSchema[ProxyGoBackArgs](),
		),
		backend.proxyGoBackHandler,
	)

	s.AddTool(
		mcp.NewTool("proxyGoForward",
			mcp.WithDescription("Navigates forward in the browser history for a specific tab or the active tab"),
			mcp.WithInputSchema[ProxyGoForwardArgs](),
		),
		backend.proxyGoForwardHandler,
	)

	sseServer := mcpserver.NewSSEServer(s,
		mcpserver.WithStaticBasePath("/mcp"),
		mcpserver.WithKeepAlive(true),
		// Capture chatId at SSE establishment and key it by the session ID we mint.
		mcpserver.WithSessionIDGenerator(func(ctx context.Context, r *http.Request) (string, error) {
			id, err := mcpGenerateSessionID()
			if err != nil {
				return "", err
			}
			if chatID := r.URL.Query().Get("chatId"); chatID != "" {
				mcpSessionChatIDs.Store(id, chatID)
			}
			return id, nil
		}),
		// Inject chatId into the request context for tool handlers.
		// Called for both /mcp/sse (no sessionId yet) and /mcp/message (sessionId in query).
		mcpserver.WithSSEContextFunc(func(ctx context.Context, r *http.Request) context.Context {
			if sid := r.URL.Query().Get("sessionId"); sid != "" {
				if v, ok := mcpSessionChatIDs.Load(sid); ok {
					if cid, ok := v.(string); ok && cid != "" {
						ctx = context.WithValue(ctx, chatIDCtxKey{}, cid)
					}
				}
			}
			return ctx
		}),
	)

	backend.MCP = &MCP{
		server:    s,
		sseServer: sseServer,
		active:    true,
	}
}

// ---------------------------------------------------------------------------
// HTTP endpoints
// ---------------------------------------------------------------------------

func (backend *Backend) MCPEndpoint(e *core.ServeEvent) error {
	backend.mcpInit()

	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/mcp/start",
		Handler: func(c echo.Context) error {
			if backend.MCP != nil && backend.MCP.active {
				return c.JSON(http.StatusOK, map[string]any{"message": "MCP server already active"})
			}
			backend.mcpInit()
			log.Println("[MCP] Server started")
			return c.JSON(http.StatusOK, map[string]any{"message": "MCP server started"})
		},
	})

	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/mcp/stop",
		Handler: func(c echo.Context) error {
			if backend.MCP == nil || !backend.MCP.active {
				return c.JSON(http.StatusOK, map[string]any{"message": "MCP server already stopped"})
			}
			backend.MCP.active = false
			log.Println("[MCP] Server stopped")
			return c.JSON(http.StatusOK, map[string]any{"message": "MCP server stopped"})
		},
	})

	e.Router.AddRoute(echo.Route{
		Method: http.MethodGet,
		Path:   "/mcp/health",
		Handler: func(c echo.Context) error {
			if backend.MCP == nil || !backend.MCP.active {
				return c.JSON(http.StatusOK, map[string]any{"active": false})
			}

			tools := backend.MCP.server.ListTools()
			toolNames := make([]string, 0, len(tools))
			for name := range tools {
				toolNames = append(toolNames, name)
			}

			return c.JSON(http.StatusOK, map[string]any{
				"active":      true,
				"status":      "ok",
				"server":      "grroxy",
				"version":     version.CURRENT_BACKEND_VERSION,
				"tools":       toolNames,
				"connections": backend.MCP.conns.Load(),
			})
		},
	})

	e.Router.AddRoute(echo.Route{
		Method: http.MethodGet,
		Path:   "/mcp/listtools",
		Handler: func(c echo.Context) error {
			if backend.MCP == nil || !backend.MCP.active {
				return c.JSON(http.StatusServiceUnavailable, map[string]any{"error": "MCP server not active"})
			}

			tools := backend.MCP.server.ListTools()
			type toolInfo struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			}
			result := make([]toolInfo, 0, len(tools))
			for name, t := range tools {
				result = append(result, toolInfo{
					Name:        name,
					Description: t.Tool.Description,
				})
			}

			return c.JSON(http.StatusOK, map[string]any{
				"tools": result,
				"count": len(result),
			})
		},
	})

	e.Router.AddRoute(echo.Route{
		Method: http.MethodGet,
		Path:   "/mcp/sse",
		Handler: func(c echo.Context) error {
			if backend.MCP == nil || !backend.MCP.active {
				return c.JSON(http.StatusServiceUnavailable, map[string]any{"error": "MCP server not active"})
			}
			backend.MCP.conns.Add(1)
			defer backend.MCP.conns.Add(-1)
			backend.MCP.sseServer.SSEHandler().ServeHTTP(c.Response(), c.Request())
			return nil
		},
	})

	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/mcp/message",
		Handler: func(c echo.Context) error {
			if backend.MCP == nil || !backend.MCP.active {
				return c.JSON(http.StatusServiceUnavailable, map[string]any{"error": "MCP server not active"})
			}
			backend.MCP.sseServer.MessageHandler().ServeHTTP(c.Response(), c.Request())
			return nil
		},
	})

	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/mcp/setup/claude",
		Handler: func(c echo.Context) error {
			var body struct {
				ClaudeMD string `json:"claude_md"`
			}
			if err := c.Bind(&body); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]any{"error": "Invalid request body"})
			}

			cwd, _ := os.Getwd()
			mcpSseURL := fmt.Sprintf("http://%s/mcp/sse", backend.Config.HostAddr)

			mcpSettings := map[string]any{
				"mcpServers": map[string]any{
					"grroxy": map[string]any{
						"type": "sse",
						"url":  mcpSseURL,
					},
				},
			}

			mcpJSON, _ := json.MarshalIndent(mcpSettings, "", "  ")
			save.WriteFile(".mcp.json", mcpJSON)

			claudeMDPath := filepath.Join(cwd, "CLAUDE.md")
			claudeContent := body.ClaudeMD
			save.WriteFile(claudeMDPath, []byte(claudeContent))

			return c.JSON(http.StatusOK, map[string]any{
				"success": true,
				"message": "Claude Code integration configured",
			})
		},
	})

	log.Println("[MCP] Endpoints registered at /mcp/")
	return nil
}
