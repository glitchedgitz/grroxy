package app

import (
	"context"
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

type MCP struct {
	server    *mcpserver.MCPServer
	sseServer *mcpserver.SSEServer
	active    bool
	conns     atomic.Int64
}

func (backend *Backend) mcpInit() {
	s := mcpserver.NewMCPServer(
		"grroxy",
		version.CURRENT_BACKEND_VERSION,
		mcpserver.WithToolCapabilities(false),
	)

	helloTool := mcp.NewTool("hello",
		mcp.WithDescription("Say hello"),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Name to greet"),
		),
	)
	s.AddTool(helloTool, backend.helloHandler)

	versionTool := mcp.NewTool("version",
		mcp.WithDescription("Get grroxy version information"),
	)
	s.AddTool(versionTool, backend.versionHandler)

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
				return c.JSON(http.StatusOK, map[string]any{
					"active": false,
				})
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

			// 1. Write .claude/.mcp.json
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

			// // 2. Build tools section for CLAUDE.md
			// toolsSection := "\n<toolsList>\n\n"
			// tools := backend.MCP.server.ListTools()
			// for name, t := range tools {
			// 	toolsSection += fmt.Sprintf(" `%s` — %s\n", name, t.Tool.Description)
			// }
			// toolsSection = "</toolsList>"

			// 3. Write CLAUDE.md (user-provided content + tools)
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

func (backend *Backend) helloHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Hello, %s!", name)), nil
}

func (backend *Backend) versionHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	info := fmt.Sprintf("Grroxy Version Info:\n  Backend:  v%s\n  Frontend: v%s\n  Release:  v%s",
		version.CURRENT_BACKEND_VERSION,
		version.CURRENT_FRONTEND_VERSION,
		version.RELEASED_APP_VERSION,
	)
	return mcp.NewToolResultText(info), nil
}
