package app

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/glitchedgitz/grroxy/grx/version"
	"github.com/glitchedgitz/grroxy/internal/save"
	"github.com/glitchedgitz/pocketbase/core"
	"github.com/labstack/echo/v5"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

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

	// --- Action tools ---

	s.AddTool(
		mcp.NewTool("sendRequest",
			mcp.WithDescription("Send a request via http. Mind the terminating the request with \\r\\n\\r\\n or \\n\\n if there are no body, mind the content length of the body, it should exactly match"),
			mcp.WithInputSchema[SendRequestArgs](),
		),
		backend.sendRequestHandler,
	)

	sseServer := mcpserver.NewSSEServer(s,
		mcpserver.WithStaticBasePath("/mcp"),
		mcpserver.WithKeepAlive(true),
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
