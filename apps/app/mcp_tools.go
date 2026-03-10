package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/glitchedgitz/grroxy/grx/version"
	"github.com/glitchedgitz/grroxy/internal/types"
	"github.com/glitchedgitz/grroxy/internal/utils"
	"github.com/glitchedgitz/pocketbase/models"
	"github.com/mark3labs/mcp-go/mcp"
)

// trimHost extracts scheme+host from a full URL (e.g. "https://example.com/path" → "https://example.com")
func trimHost(host string) string {
	u, err := url.Parse(host)
	if err != nil || u.Scheme == "" {
		return host
	}
	return u.Scheme + "://" + u.Host
}

// ---------------------------------------------------------------------------
// Input schemas (struct-based, type-safe)
// ---------------------------------------------------------------------------

type GetRequestResponseArgs struct {
	ID string `json:"id" jsonschema:"required" jsonschema_description:"The record ID to fetch request/response for"`
}

type HostPrintSitemapArgs struct {
	Host  string `json:"host" jsonschema:"required" jsonschema_description:"The host to print the sitemap for (e.g. https://example.com)"`
	Path  string `json:"path" jsonschema:"required" jsonschema_description:"Base path to fetch sitemap from (empty string for root)"`
	Depth int    `json:"depth" jsonschema:"required" jsonschema_description:"Depth of the sitemap tree (0 or 1 = 1 level, -1 = unlimited, positive = specific depth)"`
}

type HostPrintRowsArgs struct {
	Host   string `json:"host" jsonschema:"required" jsonschema_description:"The host to fetch rows for (e.g. https://example.com)"`
	Filter string `json:"filter" jsonschema:"required" jsonschema_description:"PocketBase filter expression (e.g. 'data.req_json.method = \"POST\"'), empty string for no filter"`
	Limit  int    `json:"limit" jsonschema:"required,minimum=1,maximum=500" jsonschema_description:"Maximum number of rows to return"`
	Offset int    `json:"offset" jsonschema:"required,minimum=0" jsonschema_description:"Number of rows to skip"`
}

type SendRequestArgs struct {
	Host    string  `json:"host" jsonschema:"required" jsonschema_description:"Target host (e.g. example.com)"`
	Port    string  `json:"port" jsonschema:"required" jsonschema_description:"Target port (e.g. 443)"`
	TLS     bool    `json:"tls" jsonschema:"required" jsonschema_description:"Use TLS"`
	Request string  `json:"request" jsonschema:"required" jsonschema_description:"Raw HTTP request string"`
	Timeout float64 `json:"timeout" jsonschema:"required,minimum=1,maximum=120" jsonschema_description:"Request timeout in seconds"`
	HTTP2   bool    `json:"http2" jsonschema:"required" jsonschema_description:"Use HTTP/2"`
	Index   float64 `json:"index" jsonschema:"required" jsonschema_description:"Request index number"`
	Url     string  `json:"url" jsonschema:"required" jsonschema_description:"Full URL of the request (e.g. https://example.com/path)"`
	Note    string  `json:"note" jsonschema:"required" jsonschema_description:"Note for this request"`
}

// ---------------------------------------------------------------------------
// Tool handlers
// ---------------------------------------------------------------------------

func (backend *Backend) versionHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	result := map[string]any{
		"release":  version.RELEASED_APP_VERSION,
		"backend":  version.RELEASED_BACKEND_VERSION,
		"frontend": version.RELEASED_FRONTEND_VERSION,
	}
	return mcpJSONResult(result)
}

func (backend *Backend) getRequestResponseFromIDHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args GetRequestResponseArgs
	if err := request.BindArguments(&args); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	dao := backend.App.Dao()

	// Pad ID to 15 chars with leading underscores
	id := utils.FormatStringID(args.ID, 15)

	reqRecord, _ := dao.FindRecordById("_req", id)
	respRecord, _ := dao.FindRecordById("_resp", id)

	if reqRecord == nil && respRecord == nil {
		return mcp.NewToolResultError(fmt.Sprintf("no record found for ID: %s", id)), nil
	}

	result := map[string]any{"id": id}

	if reqRecord != nil {
		result["request"] = reqRecord.GetString("raw")
	}

	if respRecord != nil {
		result["response"] = respRecord.GetString("raw")
	}

	return mcpJSONResult(result)
}

func (backend *Backend) hostPrintSitemapHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args HostPrintSitemapArgs
	if err := request.BindArguments(&args); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	data := &types.SitemapFetch{
		Host:  trimHost(args.Host),
		Path:  args.Path,
		Depth: args.Depth,
	}

	nodes, err := backend.sitemapFetchLogic(data)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcpJSONResult(nodes)
}

func (backend *Backend) hostPrintRowsInDetailsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args HostPrintRowsArgs
	if err := request.BindArguments(&args); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	host := trimHost(args.Host)
	dao := backend.App.Dao()
	siteDB := utils.ParseDatabaseName(host)

	collection, err := dao.FindCollectionByNameOrId(siteDB)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("host not found: %s", host)), nil
	}

	var records []*models.Record
	if args.Filter == "" {
		records, err = dao.FindRecordsByExpr(collection.Id)
	} else {
		records, err = dao.FindRecordsByFilter(collection.Id, args.Filter, "-created", args.Limit, args.Offset)
	}
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to fetch records: %v", err)), nil
	}

	// Expand the "data" relation to get _data records
	for _, record := range records {
		dao.ExpandRecord(record, []string{"data"}, nil)
	}

	rows := make([]map[string]any, 0, len(records))
	for _, record := range records {
		expanded := record.ExpandedAll("data")
		for _, dataRecord := range expanded {
			reqJSON := dataRecord.Get("req_json")
			respJSON := dataRecord.Get("resp_json")

			// Remove headers from req/resp to keep response compact
			if req, ok := reqJSON.(map[string]any); ok {
				delete(req, "headers")
			}
			if resp, ok := respJSON.(map[string]any); ok {
				delete(resp, "headers")
			}

			rows = append(rows, map[string]any{
				"id":           dataRecord.GetString("id"),
				"index":        dataRecord.GetFloat("index"),
				"index_minor":  dataRecord.GetFloat("index_minor"),
				"host":         dataRecord.GetString("host"),
				"port":         dataRecord.GetString("port"),
				"generated_by": dataRecord.GetString("generated_by"),
				"has_params":   dataRecord.GetBool("has_params"),
				"has_resp":     dataRecord.GetBool("has_resp"),
				"http":         dataRecord.GetString("http"),
				"req":          reqJSON,
				"resp":         respJSON,
			})
		}
	}

	result := map[string]any{
		"host":      host,
		"totalRows": len(rows),
		"rows":      rows,
	}

	return mcpJSONResult(result)
}

func (backend *Backend) sendRequestHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args SendRequestArgs
	if err := request.BindArguments(&args); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Extract just the hostname (e.g. "https://example.com/path" → "example.com")
	host := args.Host
	if u, err := url.Parse(host); err == nil && u.Host != "" {
		host = u.Hostname()
	}

	resp, err := backend.sendRepeaterLogic(&RepeaterSendRequest{
		Host:        host,
		Port:        args.Port,
		TLS:         args.TLS,
		Request:     args.Request,
		Timeout:     args.Timeout,
		HTTP2:       args.HTTP2,
		Index:       args.Index,
		Url:         args.Url,
		Note:        args.Note,
		GeneratedBy: "ai/mcp/claudecode",
	})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcpJSONResult(resp)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mcpJSONResult(v any) (*mcp.CallToolResult, error) {
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(jsonBytes)), nil
}
