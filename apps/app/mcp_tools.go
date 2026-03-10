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
	ActiveID string `json:"activeID" jsonschema:"required" jsonschema_description:"The active ID"`
}

type HostPrintSitemapArgs struct {
	Host  string `json:"host" jsonschema:"required" jsonschema_description:"the host to get the sitemap for"`
	Path  string `json:"path" jsonschema:"required" jsonschema_description:"the path to get the sitemap for, use empty string to get the root sitemap"`
	Depth int    `json:"depth" jsonschema:"required" jsonschema_description:"the depth to get the sitemap for, default is -1, use -1 to get the full sitemap"`
}

type HostPrintRowsArgs struct {
	Host   string `json:"host" jsonschema:"required" jsonschema_description:"the host to get the table for"`
	Page   int    `json:"page" jsonschema:"required" jsonschema_description:"the page to get the data from, start from 1"`
	Filter string `json:"filter" jsonschema:"required" jsonschema_description:"filter the results for faster search"`
}

type SendRequestArgs struct {
	TLS                    bool     `json:"tls" jsonschema:"required" jsonschema_description:"use https or http"`
	Host                   string   `json:"host" jsonschema:"required" jsonschema_description:"the host to send the request to"`
	Port                   int      `json:"port" jsonschema:"required" jsonschema_description:"the port to send the request to"`
	HttpVersion            int      `json:"httpVersion" jsonschema:"required" jsonschema_description:"1 or 2"`
	AttachToIndex          float64  `json:"attachToIndex" jsonschema:"required" jsonschema_description:"origin index of request you are modifying"`
	Request                string   `json:"request" jsonschema:"required" jsonschema_description:"raw request"`
	Note                   string   `json:"note" jsonschema:"required" jsonschema_description:"the note to attach to the request"`
	Labels                 []string `json:"labels,omitempty" jsonschema_description:"the labels to attach to the request"`
	AutoUpdateContentLength bool    `json:"autoUpdateContentLength" jsonschema:"required" jsonschema_description:"auto update content length, default: true"`
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
	id := utils.FormatStringID(args.ActiveID, 15)

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

	perPage := 50
	offset := 0
	if args.Page > 1 {
		offset = (args.Page - 1) * perPage
	}

	var records []*models.Record
	if args.Filter == "" {
		records, err = dao.FindRecordsByFilter(collection.Id, "", "-created", perPage, offset)
	} else {
		records, err = dao.FindRecordsByFilter(collection.Id, args.Filter, "-created", perPage, offset)
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

	port := fmt.Sprintf("%d", args.Port)
	http2 := args.HttpVersion == 2

	resp, err := backend.sendRepeaterLogic(&RepeaterSendRequest{
		Host:        host,
		Port:        port,
		TLS:         args.TLS,
		Request:     args.Request,
		Timeout:     30,
		HTTP2:       http2,
		Index:       args.AttachToIndex,
		Url:         fmt.Sprintf("%s://%s:%d", map[bool]string{true: "https", false: "http"}[args.TLS], host, args.Port),
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
