package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"time"

	"github.com/glitchedgitz/grroxy/grx/version"
	"github.com/glitchedgitz/grroxy/internal/types"
	"github.com/glitchedgitz/grroxy/internal/utils"
	"github.com/glitchedgitz/pocketbase/models"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/pocketbase/dbx"
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
	TLS                     bool     `json:"tls" jsonschema:"required" jsonschema_description:"use https or http"`
	Host                    string   `json:"host" jsonschema:"required" jsonschema_description:"the host to send the request to"`
	Port                    int      `json:"port" jsonschema:"required" jsonschema_description:"the port to send the request to"`
	HttpVersion             int      `json:"httpVersion" jsonschema:"required" jsonschema_description:"1 or 2"`
	AttachToIndex           float64  `json:"attachToIndex" jsonschema:"required" jsonschema_description:"origin index of request you are modifying"`
	Request                 string   `json:"request" jsonschema:"required" jsonschema_description:"raw request"`
	Note                    string   `json:"note" jsonschema:"required" jsonschema_description:"the note to attach to the request"`
	Labels                  []string `json:"labels,omitempty" jsonschema_description:"the labels to attach to the request"`
	AutoUpdateContentLength bool     `json:"autoUpdateContentLength" jsonschema:"required" jsonschema_description:"auto update content length, default: true"`
}

type ListHostsArgs struct {
	Search string `json:"search,omitempty" jsonschema_description:"the search to get the table for, use empty string to get all results"`
	Page   int    `json:"page" jsonschema:"required" jsonschema_description:"the page to get the data from, start from 1"`
}

// --- Host arg structs ---

type GetHostInfoArgs struct {
	Host string `json:"host" jsonschema:"required" jsonschema_description:"the host ID to get the info for"`
}

type GetNoteForHostArgs struct {
	Host string `json:"host" jsonschema:"required" jsonschema_description:"the host to get the note for"`
}

type SetNoteForHostArgs struct {
	Host string           `json:"host" jsonschema:"required" jsonschema_description:"the host to set the note for"`
	Edit []NoteEditAction `json:"edit,omitempty" jsonschema_description:"lines to be updated"`
}

type NoteEditAction struct {
	Index int    `json:"index" jsonschema:"required" jsonschema_description:"the index of the line to edit"`
	Line  string `json:"line,omitempty" jsonschema_description:"the content to edit the line with, to delete write [delete]"`
}

type ModifyHostLabelsArgs struct {
	Host   string            `json:"host" jsonschema:"required" jsonschema_description:"the host to update the label, include the protocol eg: http://example.com"`
	Labels []HostLabelAction `json:"labels" jsonschema:"required" jsonschema_description:"the labels to update for the host"`
}

type HostLabelAction struct {
	Action string `json:"action" jsonschema:"required" jsonschema_description:"add, remove, or toggle"`
	Name   string `json:"name" jsonschema:"required" jsonschema_description:"the name of the label"`
	Color  string `json:"color,omitempty" jsonschema_description:"the color of the label (only for add/toggle)"`
	Type   string `json:"type,omitempty" jsonschema_description:"the type of the label (only for add/toggle)"`
}

type ModifyHostNotesArgs struct {
	Host  string           `json:"host" jsonschema:"required" jsonschema_description:"the host to update the note, include the protocol eg: http://example.com"`
	Notes []HostNoteAction `json:"notes" jsonschema:"required" jsonschema_description:"the notes to update for the host"`
}

type HostNoteAction struct {
	Action string `json:"action" jsonschema:"required" jsonschema_description:"add, update, or remove"`
	Index  int    `json:"index,omitempty" jsonschema_description:"the index of the note to update/remove (not needed for add)"`
	Text   string `json:"text,omitempty" jsonschema_description:"the text of the note (for add/update actions)"`
	Author string `json:"author,omitempty" jsonschema_description:"the author of the note (for add action, defaults to you)"`
}

// --- Proxy arg structs ---

type ProxyStartArgs struct {
	Name string `json:"name,omitempty" jsonschema_description:"Optional label for the proxy instance"`
}

type ProxyStopArgs struct {
	ID string `json:"id,omitempty" jsonschema_description:"The proxy ID to stop. If not provided, stops all running proxies"`
}

type ProxyScreenshotArgs struct {
	ID string `json:"id" jsonschema:"required" jsonschema_description:"The proxy ID with Chrome browser attached"`
}

type ProxyClickArgs struct {
	ID                string `json:"id" jsonschema:"required" jsonschema_description:"The proxy ID with Chrome browser attached"`
	URL               string `json:"url,omitempty" jsonschema_description:"URL to navigate to before clicking. If empty, operates on the current active page"`
	Selector          string `json:"selector" jsonschema:"required" jsonschema_description:"CSS selector for the element to click"`
	WaitForNavigation bool   `json:"waitForNavigation,omitempty" jsonschema_description:"If true, waits for page navigation after click (default: false)"`
}

type ProxyElementsArgs struct {
	ID  string `json:"id" jsonschema:"required" jsonschema_description:"The proxy ID with Chrome browser attached"`
	URL string `json:"url,omitempty" jsonschema_description:"URL to navigate to before extracting elements. If empty, analyzes the current active page"`
}

type ProxyListTabsArgs struct {
	ProxyID string `json:"proxyId" jsonschema:"required" jsonschema_description:"The proxy ID with Chrome browser attached"`
}

type ProxyOpenTabArgs struct {
	ProxyID string `json:"proxyId" jsonschema:"required" jsonschema_description:"The proxy ID with Chrome browser attached"`
	URL     string `json:"url,omitempty" jsonschema_description:"URL to open in the new tab. Defaults to about:blank if not provided"`
}

type ProxyNavigateTabArgs struct {
	ProxyID   string `json:"proxyId" jsonschema:"required" jsonschema_description:"The proxy ID with Chrome browser attached"`
	TargetID  string `json:"targetId,omitempty" jsonschema_description:"Chrome target ID of the tab to navigate. If empty, navigates the active tab"`
	URL       string `json:"url" jsonschema:"required" jsonschema_description:"URL to navigate to"`
	WaitUntil string `json:"waitUntil,omitempty" jsonschema_description:"Load state to wait for: domcontentloaded, load (default), or networkidle"`
	TimeoutMs int    `json:"timeoutMs,omitempty" jsonschema_description:"Timeout in milliseconds. Default: 30000"`
}

type ProxyActivateTabArgs struct {
	ProxyID  string `json:"proxyId" jsonschema:"required" jsonschema_description:"The proxy ID with Chrome browser attached"`
	TargetID string `json:"targetId" jsonschema:"required" jsonschema_description:"Chrome target ID of the tab to activate"`
}

type ProxyCloseTabArgs struct {
	ProxyID  string `json:"proxyId" jsonschema:"required" jsonschema_description:"The proxy ID with Chrome browser attached"`
	TargetID string `json:"targetId" jsonschema:"required" jsonschema_description:"Chrome target ID of the tab to close"`
}

type ProxyReloadTabArgs struct {
	ProxyID     string `json:"proxyId" jsonschema:"required" jsonschema_description:"The proxy ID with Chrome browser attached"`
	TargetID    string `json:"targetId,omitempty" jsonschema_description:"Chrome target ID of the tab to reload. If empty, reloads the active tab"`
	BypassCache bool   `json:"bypassCache,omitempty" jsonschema_description:"If true, reloads ignoring cache (hard refresh). Default: false"`
}

type ProxyGoBackArgs struct {
	ProxyID  string `json:"proxyId" jsonschema:"required" jsonschema_description:"The proxy ID with Chrome browser attached"`
	TargetID string `json:"targetId,omitempty" jsonschema_description:"Chrome target ID of the tab. If empty, operates on the active tab"`
}

type ProxyGoForwardArgs struct {
	ProxyID  string `json:"proxyId" jsonschema:"required" jsonschema_description:"The proxy ID with Chrome browser attached"`
	TargetID string `json:"targetId,omitempty" jsonschema_description:"Chrome target ID of the tab. If empty, operates on the active tab"`
}

// --- Intercept arg structs ---

type InterceptToggleArgs struct {
	ID     string `json:"id" jsonschema:"required" jsonschema_description:"The proxy ID to enable/disable interception on"`
	Enable bool   `json:"enable" jsonschema:"required" jsonschema_description:"true to enable interception, false to disable (forwards all pending)"`
}

type InterceptActionArgs struct {
	ID           string `json:"id" jsonschema:"required" jsonschema_description:"The intercept record ID (from interceptList)"`
	Action       string `json:"action" jsonschema:"required" jsonschema_description:"Action to take: forward (pass through) or drop (block)"`
	IsReqEdited  bool   `json:"isReqEdited,omitempty" jsonschema_description:"If true, the request has been edited"`
	IsRespEdited bool   `json:"isRespEdited,omitempty" jsonschema_description:"If true, the response has been edited"`
	ReqEdited    string `json:"reqEdited,omitempty" jsonschema_description:"Raw edited HTTP request string (only if isReqEdited is true)"`
	RespEdited   string `json:"respEdited,omitempty" jsonschema_description:"Raw edited HTTP response string (only if isRespEdited is true)"`
}

type InterceptReadArgs struct {
	ProxyID string `json:"proxyId" jsonschema:"required" jsonschema_description:"The proxy ID to read intercepted rows from"`
}

type InterceptGetRawArgs struct {
	ID string `json:"id" jsonschema:"required" jsonschema_description:"The intercept record ID (from interceptPrintRowsInDetails)"`
}

// --- Browser tool arg structs ---

type ProxyTypeArgs struct {
	ID         string `json:"id" jsonschema:"required" jsonschema_description:"The proxy ID with Chrome browser attached"`
	Selector   string `json:"selector" jsonschema:"required" jsonschema_description:"CSS selector for the input element to type into"`
	Text       string `json:"text" jsonschema:"required" jsonschema_description:"The text to type into the element"`
	ClearFirst bool   `json:"clearFirst,omitempty" jsonschema_description:"If true, clears the existing value before typing (default: false)"`
	TimeoutMs  int    `json:"timeoutMs,omitempty" jsonschema_description:"Timeout in milliseconds. Default: 15000"`
}

type ProxyEvalArgs struct {
	ID        string `json:"id" jsonschema:"required" jsonschema_description:"The proxy ID with Chrome browser attached"`
	Js        string `json:"js" jsonschema:"required" jsonschema_description:"JavaScript expression to evaluate in the page context. The result is returned as JSON."`
	TimeoutMs int    `json:"timeoutMs,omitempty" jsonschema_description:"Timeout in milliseconds. Default: 15000"`
}

type ProxyWaitForSelectorArgs struct {
	ID        string `json:"id" jsonschema:"required" jsonschema_description:"The proxy ID with Chrome browser attached"`
	Selector  string `json:"selector" jsonschema:"required" jsonschema_description:"CSS selector to wait for"`
	TimeoutMs int    `json:"timeoutMs,omitempty" jsonschema_description:"Timeout in milliseconds. Default: 30000"`
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
		records, err = dao.FindRecordsByExpr(collection.Id)
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

func (backend *Backend) listHostsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args ListHostsArgs
	if err := request.BindArguments(&args); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	dao := backend.App.Dao()

	perPage := 50
	offset := 0
	if args.Page > 1 {
		offset = (args.Page - 1) * perPage
	}

	filter := ""
	if args.Search != "" {
		filter = fmt.Sprintf("host ~ '%s' || title ~ '%s' || domain ~ '%s'", args.Search, args.Search, args.Search)
	}

	var records []*models.Record
	var err error

	if filter == "" {
		records, err = dao.FindRecordsByExpr("_hosts")
	} else {
		records, err = dao.FindRecordsByFilter("_hosts", filter, "-created", perPage, offset)
	}

	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to fetch hosts: %v", err)), nil
	}

	// Expand tech and labels relations
	for _, record := range records {
		dao.ExpandRecord(record, []string{"tech", "labels"}, nil)
	}

	items := make([]map[string]any, 0, len(records))
	for _, record := range records {
		// Resolve tech names
		techNames := []string{}
		for _, t := range record.ExpandedAll("tech") {
			techNames = append(techNames, t.GetString("name"))
		}

		// Resolve label names
		labelNames := []string{}
		for _, l := range record.ExpandedAll("labels") {
			labelNames = append(labelNames, l.GetString("name"))
		}

		items = append(items, map[string]any{
			"id":     record.GetString("id"),
			"host":   record.GetString("host"),
			"title":  record.GetString("title"),
			"tech":   techNames,
			"labels": labelNames,
			"notes":  record.Get("notes"),
		})
	}

	// Get total count
	var total []*models.Record
	if filter == "" {
		total, _ = dao.FindRecordsByExpr("_hosts")
	} else {
		total, _ = dao.FindRecordsByFilter("_hosts", filter, "", 0, 0)
	}

	result := map[string]any{
		"page":       args.Page,
		"perPage":    perPage,
		"totalItems": len(total),
		"totalPages": (len(total) + perPage - 1) / perPage,
		"items":      items,
	}

	return mcpJSONResult(result)
}

// ---------------------------------------------------------------------------
// Host tool handlers
// ---------------------------------------------------------------------------

func (backend *Backend) getHostInfoHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args GetHostInfoArgs
	if err := request.BindArguments(&args); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	dao := backend.App.Dao()
	host := trimHost(args.Host)

	record, err := dao.FindFirstRecordByFilter("_hosts", "host = {:host}", dbx.Params{"host": host})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("host not found: %s", host)), nil
	}

	dao.ExpandRecord(record, []string{"tech", "labels"}, nil)

	techNames := []string{}
	for _, t := range record.ExpandedAll("tech") {
		techNames = append(techNames, t.GetString("name"))
	}

	labelNames := []string{}
	for _, l := range record.ExpandedAll("labels") {
		labelNames = append(labelNames, l.GetString("name"))
	}

	return mcpJSONResult(map[string]any{
		"id":     record.GetString("id"),
		"host":   record.GetString("host"),
		"title":  record.GetString("title"),
		"domain": record.GetString("domain"),
		"status": record.GetString("status"),
		"tech":   techNames,
		"labels": labelNames,
		"notes":  record.Get("notes"),
	})
}

func (backend *Backend) getNoteForHostHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args GetNoteForHostArgs
	if err := request.BindArguments(&args); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	dao := backend.App.Dao()
	host := trimHost(args.Host)

	record, err := dao.FindFirstRecordByFilter("_hosts", "host = {:host}", dbx.Params{"host": host})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("host not found: %s", host)), nil
	}

	return mcpJSONResult(map[string]any{
		"host":  host,
		"notes": record.Get("notes"),
	})
}

func (backend *Backend) setNoteForHostHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args SetNoteForHostArgs
	if err := request.BindArguments(&args); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	dao := backend.App.Dao()
	host := trimHost(args.Host)

	record, err := dao.FindFirstRecordByFilter("_hosts", "host = {:host}", dbx.Params{"host": host})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("host not found: %s", host)), nil
	}

	// Get existing notes as string lines
	existingNotes, _ := record.Get("notes").([]any)
	noteLines := make([]string, len(existingNotes))
	for i, n := range existingNotes {
		if s, ok := n.(string); ok {
			noteLines[i] = s
		}
	}

	// Apply edits
	for _, edit := range args.Edit {
		if edit.Line == "[delete]" {
			if edit.Index >= 0 && edit.Index < len(noteLines) {
				noteLines = append(noteLines[:edit.Index], noteLines[edit.Index+1:]...)
			}
		} else if edit.Index >= len(noteLines) {
			noteLines = append(noteLines, edit.Line)
		} else if edit.Index >= 0 {
			noteLines[edit.Index] = edit.Line
		}
	}

	record.Set("notes", noteLines)
	if err := dao.SaveRecord(record); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to save notes: %v", err)), nil
	}

	return mcpJSONResult(map[string]any{
		"success": true,
		"host":    host,
		"notes":   noteLines,
	})
}

func (backend *Backend) modifyHostLabelsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args ModifyHostLabelsArgs
	if err := request.BindArguments(&args); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	dao := backend.App.Dao()
	host := trimHost(args.Host)

	hostRecord, err := dao.FindFirstRecordByFilter("_hosts", "host = {:host}", dbx.Params{"host": host})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("host not found: %s", host)), nil
	}

	currentLabelIDs := hostRecord.GetStringSlice("labels")

	for _, labelAction := range args.Labels {
		// Find or create the label
		labelRecord, err := dao.FindFirstRecordByFilter("_labels", "name = {:name}", dbx.Params{"name": labelAction.Name})

		if labelAction.Action == "add" || labelAction.Action == "toggle" {
			if err != nil {
				// Label doesn't exist, create it
				labelsCollection, cerr := dao.FindCollectionByNameOrId("_labels")
				if cerr != nil {
					return mcp.NewToolResultError(fmt.Sprintf("failed to find _labels collection: %v", cerr)), nil
				}
				labelRecord = models.NewRecord(labelsCollection)
				labelRecord.Set("name", labelAction.Name)
				labelRecord.Set("color", labelAction.Color)
				labelRecord.Set("type", labelAction.Type)
				if serr := dao.SaveRecord(labelRecord); serr != nil {
					return mcp.NewToolResultError(fmt.Sprintf("failed to create label: %v", serr)), nil
				}
			}

			labelID := labelRecord.Id
			found := false
			for _, id := range currentLabelIDs {
				if id == labelID {
					found = true
					break
				}
			}

			if labelAction.Action == "toggle" && found {
				// Remove
				newIDs := make([]string, 0, len(currentLabelIDs))
				for _, id := range currentLabelIDs {
					if id != labelID {
						newIDs = append(newIDs, id)
					}
				}
				currentLabelIDs = newIDs
			} else if !found {
				currentLabelIDs = append(currentLabelIDs, labelID)
			}

		} else if labelAction.Action == "remove" {
			if err == nil {
				labelID := labelRecord.Id
				newIDs := make([]string, 0, len(currentLabelIDs))
				for _, id := range currentLabelIDs {
					if id != labelID {
						newIDs = append(newIDs, id)
					}
				}
				currentLabelIDs = newIDs
			}
		}
	}

	hostRecord.Set("labels", currentLabelIDs)
	if err := dao.SaveRecord(hostRecord); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to save labels: %v", err)), nil
	}

	return mcpJSONResult(map[string]any{
		"success": true,
		"host":    host,
		"labels":  currentLabelIDs,
	})
}

func (backend *Backend) modifyHostNotesHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args ModifyHostNotesArgs
	if err := request.BindArguments(&args); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	dao := backend.App.Dao()
	host := trimHost(args.Host)

	record, err := dao.FindFirstRecordByFilter("_hosts", "host = {:host}", dbx.Params{"host": host})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("host not found: %s", host)), nil
	}

	// Get existing notes as array of maps
	existingRaw, _ := record.Get("notes").([]any)
	notes := make([]map[string]any, 0, len(existingRaw))
	for _, n := range existingRaw {
		if m, ok := n.(map[string]any); ok {
			notes = append(notes, m)
		}
	}

	// Apply actions in order
	for _, action := range args.Notes {
		switch action.Action {
		case "add":
			author := action.Author
			if author == "" {
				author = "you"
			}
			notes = append(notes, map[string]any{"text": action.Text, "author": author})
		case "update":
			if action.Index >= 0 && action.Index < len(notes) {
				notes[action.Index]["text"] = action.Text
			}
		case "remove":
			if action.Index >= 0 && action.Index < len(notes) {
				notes = append(notes[:action.Index], notes[action.Index+1:]...)
			}
		}
	}

	record.Set("notes", notes)
	if err := dao.SaveRecord(record); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to save notes: %v", err)), nil
	}

	return mcpJSONResult(map[string]any{
		"success": true,
		"host":    host,
		"notes":   notes,
	})
}

// ---------------------------------------------------------------------------
// Proxy tool handlers
// ---------------------------------------------------------------------------

func (backend *Backend) proxyListHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ProxyMgr.mu.RLock()
	instances := make([]map[string]any, 0, len(ProxyMgr.instances))
	for id, inst := range ProxyMgr.instances {
		if inst != nil && inst.Proxy != nil {
			var browserPid int
			if inst.BrowserCmd != nil && inst.BrowserCmd.Process != nil {
				browserPid = inst.BrowserCmd.Process.Pid
			}
			instances = append(instances, map[string]any{
				"id":         id,
				"listenAddr": inst.Proxy.listenAddr,
				"label":      inst.Label,
				"browser":    inst.Browser,
				"browserPid": browserPid,
				"intercept":  inst.Proxy.Intercept,
			})
		}
	}
	ProxyMgr.mu.RUnlock()

	return mcpJSONResult(map[string]any{
		"proxies": instances,
		"count":   len(instances),
	})
}

func (backend *Backend) proxyStartHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args ProxyStartArgs
	if err := request.BindArguments(&args); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	body := &ProxyBody{
		Browser: "chrome",
		Name:    args.Name,
	}

	result, err := backend.startProxyLogic(body)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcpJSONResult(result)
}

func (backend *Backend) proxyStopHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args ProxyStopArgs
	if err := request.BindArguments(&args); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if args.ID == "" {
		ProxyMgr.StopAllProxies()
		return mcpJSONResult(map[string]any{"success": true, "message": "All proxies stopped"})
	}

	if err := ProxyMgr.StopProxy(args.ID); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	backend.updateProxyState(args.ID, "")
	ProxyMgr.RemoveProxy(args.ID)

	return mcpJSONResult(map[string]any{"success": true, "message": fmt.Sprintf("Proxy %s stopped", args.ID)})
}

func (backend *Backend) proxyScreenshotHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args ProxyScreenshotArgs
	if err := request.BindArguments(&args); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Save screenshot to projectsDirectory/projectID/screenshots/
	screenshotDir := path.Join(backend.Config.ProjectsDirectory, backend.Config.ProjectID, "screenshots")
	if err := os.MkdirAll(screenshotDir, 0755); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create screenshot directory: %v", err)), nil
	}

	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("screenshot-%s.png", timestamp)
	savePath := path.Join(screenshotDir, filename)

	_, filePath, err := ProxyMgr.TakeScreenshot(args.ID, false, savePath)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// screenshotBase64 := base64.StdEncoding.EncodeToString(screenshotBytes)
	return mcpJSONResult(map[string]any{
		// "screenshot": screenshotBase64,
		// "size":     len(screenshotBytes),
		"filePath": filePath,
	})
}

func (backend *Backend) proxyClickHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args ProxyClickArgs
	if err := request.BindArguments(&args); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if args.Selector == "" {
		return mcp.NewToolResultError("selector is required"), nil
	}

	if err := ProxyMgr.ClickElement(args.ID, args.URL, args.Selector, args.WaitForNavigation); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcpJSONResult(map[string]any{
		"success":  true,
		"message":  "Element clicked successfully",
		"selector": args.Selector,
	})
}

func (backend *Backend) proxyElementsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args ProxyElementsArgs
	if err := request.BindArguments(&args); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	elements, err := ProxyMgr.GetElements(args.ID, args.URL)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcpJSONResult(map[string]any{
		"elements": elements,
		"count":    len(elements),
	})
}

func (backend *Backend) proxyListTabsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args ProxyListTabsArgs
	if err := request.BindArguments(&args); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	chrome, err := ProxyMgr.GetChromeRemote(args.ProxyID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	tabs, err := chrome.ListTabs()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list tabs: %v", err)), nil
	}

	return mcpJSONResult(map[string]any{
		"tabs":  tabs,
		"count": len(tabs),
	})
}

func (backend *Backend) proxyOpenTabHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args ProxyOpenTabArgs
	if err := request.BindArguments(&args); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	chrome, err := ProxyMgr.GetChromeRemote(args.ProxyID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	targetID, err := chrome.OpenTab(args.URL)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to open tab: %v", err)), nil
	}

	return mcpJSONResult(map[string]any{
		"targetId": targetID,
		"url":      args.URL,
	})
}

func (backend *Backend) proxyNavigateTabHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args ProxyNavigateTabArgs
	if err := request.BindArguments(&args); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if args.URL == "" {
		return mcp.NewToolResultError("url is required"), nil
	}

	chrome, err := ProxyMgr.GetChromeRemote(args.ProxyID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result, err := chrome.Navigate(args.TargetID, args.URL, args.WaitUntil, args.TimeoutMs)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to navigate tab: %v", err)), nil
	}

	return mcpJSONResult(map[string]any{
		"targetId":     args.TargetID,
		"url":          result.FinalURL,
		"status":       result.Status,
		"navigationId": result.NavigationID,
	})
}

func (backend *Backend) proxyActivateTabHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args ProxyActivateTabArgs
	if err := request.BindArguments(&args); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	chrome, err := ProxyMgr.GetChromeRemote(args.ProxyID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := chrome.ActivateTab(args.TargetID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to activate tab: %v", err)), nil
	}

	return mcpJSONResult(map[string]any{
		"ok":       true,
		"targetId": args.TargetID,
	})
}

func (backend *Backend) proxyCloseTabHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args ProxyCloseTabArgs
	if err := request.BindArguments(&args); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	chrome, err := ProxyMgr.GetChromeRemote(args.ProxyID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := chrome.CloseTab(args.TargetID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to close tab: %v", err)), nil
	}

	return mcpJSONResult(map[string]any{
		"ok":       true,
		"targetId": args.TargetID,
	})
}

func (backend *Backend) proxyReloadTabHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args ProxyReloadTabArgs
	if err := request.BindArguments(&args); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	chrome, err := ProxyMgr.GetChromeRemote(args.ProxyID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := chrome.ReloadTab(args.TargetID, args.BypassCache); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to reload tab: %v", err)), nil
	}

	return mcpJSONResult(map[string]any{
		"ok":       true,
		"targetId": args.TargetID,
	})
}

func (backend *Backend) proxyGoBackHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args ProxyGoBackArgs
	if err := request.BindArguments(&args); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	chrome, err := ProxyMgr.GetChromeRemote(args.ProxyID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := chrome.GoBack(args.TargetID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to go back: %v", err)), nil
	}

	return mcpJSONResult(map[string]any{
		"ok":       true,
		"targetId": args.TargetID,
	})
}

func (backend *Backend) proxyGoForwardHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args ProxyGoForwardArgs
	if err := request.BindArguments(&args); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	chrome, err := ProxyMgr.GetChromeRemote(args.ProxyID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := chrome.GoForward(args.TargetID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to go forward: %v", err)), nil
	}

	return mcpJSONResult(map[string]any{
		"ok":       true,
		"targetId": args.TargetID,
	})
}

// ---------------------------------------------------------------------------
// Intercept tool handlers
// ---------------------------------------------------------------------------

func (backend *Backend) interceptToggleHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args InterceptToggleArgs
	if err := request.BindArguments(&args); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	dao := backend.App.Dao()
	proxyRecord, err := dao.FindRecordById("_proxies", args.ID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("proxy not found: %s", args.ID)), nil
	}

	proxyRecord.Set("intercept", args.Enable)
	if err := dao.SaveRecord(proxyRecord); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to update intercept setting: %v", err)), nil
	}

	// dao.SaveRecord doesn't trigger OnRecordAfterUpdateRequest hooks,
	// so update the in-memory proxy state directly
	ProxyMgr.mu.RLock()
	inst := ProxyMgr.instances[args.ID]
	ProxyMgr.mu.RUnlock()

	if inst != nil && inst.Proxy != nil {
		inst.Proxy.Intercept = args.Enable
		if !args.Enable {
			go backend.forwardProxyIntercepts(args.ID)
		}
	}

	action := "enabled"
	if !args.Enable {
		action = "disabled"
	}

	return mcpJSONResult(map[string]any{
		"success": true,
		"message": fmt.Sprintf("Interception %s for proxy %s", action, args.ID),
		"proxyId": args.ID,
		"enabled": args.Enable,
	})
}

func (backend *Backend) interceptActionHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args InterceptActionArgs
	if err := request.BindArguments(&args); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if args.Action != "forward" && args.Action != "drop" {
		return mcp.NewToolResultError("action must be 'forward' or 'drop'"), nil
	}
	if args.ID == "" {
		return mcp.NewToolResultError("intercept ID is required"), nil
	}

	update := InterceptUpdate{
		Action:        args.Action,
		IsReqEdited:   args.IsReqEdited,
		IsRespEdited:  args.IsRespEdited,
		ReqEditedRaw:  args.ReqEdited,
		RespEditedRaw: args.RespEdited,
	}
	NotifyInterceptUpdate(args.ID, update)

	return mcpJSONResult(map[string]any{
		"success": true,
		"message": fmt.Sprintf("Intercept %s: %s", args.ID, args.Action),
		"id":      args.ID,
		"action":  args.Action,
	})
}

func (backend *Backend) interceptReadHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args InterceptReadArgs
	if err := request.BindArguments(&args); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	dao := backend.App.Dao()

	generatedBy := fmt.Sprintf("proxy/%s", args.ProxyID)
	records, err := dao.FindRecordsByFilter("_intercept", "generated_by ~ {:gb}", "-created", 100, 0, dbx.Params{"gb": generatedBy})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to read intercepted rows: %v", err)), nil
	}

	rows := make([]map[string]any, 0, len(records))
	for _, rec := range records {
		rows = append(rows, map[string]any{
			"id":       rec.GetString("id"),
			"host":     rec.GetString("host"),
			"port":     rec.GetString("port"),
			"index":    rec.GetFloat("index"),
			"has_resp": rec.GetBool("has_resp"),
			"http":     rec.GetString("http"),
			"req":      rec.Get("req_json"),
			"resp":     rec.Get("resp_json"),
		})
	}

	return mcpJSONResult(map[string]any{
		"proxyId": args.ProxyID,
		"rows":    rows,
		"count":   len(rows),
	})
}

func (backend *Backend) interceptGetRawHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args InterceptGetRawArgs
	if err := request.BindArguments(&args); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	dao := backend.App.Dao()

	result := map[string]any{"id": args.ID}

	reqRecord, _ := dao.FindRecordById("_req", args.ID)
	if reqRecord != nil {
		result["raw_request"] = reqRecord.GetString("raw")
	}

	respRecord, _ := dao.FindRecordById("_resp", args.ID)
	if respRecord != nil {
		result["raw_response"] = respRecord.GetString("raw")
	}

	if reqRecord == nil && respRecord == nil {
		return mcp.NewToolResultError(fmt.Sprintf("no request/response found for ID: %s", args.ID)), nil
	}

	return mcpJSONResult(result)
}

// ---------------------------------------------------------------------------
// Browser tool handlers
// ---------------------------------------------------------------------------

func (backend *Backend) proxyTypeHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args ProxyTypeArgs
	if err := request.BindArguments(&args); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if args.Selector == "" {
		return mcp.NewToolResultError("selector is required"), nil
	}
	if args.Text == "" {
		return mcp.NewToolResultError("text is required"), nil
	}

	if err := ProxyMgr.TypeText(args.ID, args.Selector, args.Text, args.ClearFirst, args.TimeoutMs); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcpJSONResult(map[string]any{
		"success":  true,
		"message":  "Text typed successfully",
		"selector": args.Selector,
	})
}

func (backend *Backend) proxyEvalHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args ProxyEvalArgs
	if err := request.BindArguments(&args); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if args.Js == "" {
		return mcp.NewToolResultError("js expression is required"), nil
	}

	result, err := ProxyMgr.Evaluate(args.ID, args.Js, args.TimeoutMs)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcpJSONResult(map[string]any{
		"success": true,
		"result":  result,
	})
}

func (backend *Backend) proxyWaitForSelectorHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args ProxyWaitForSelectorArgs
	if err := request.BindArguments(&args); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if args.Selector == "" {
		return mcp.NewToolResultError("selector is required"), nil
	}

	if err := ProxyMgr.WaitForSelector(args.ID, args.Selector, args.TimeoutMs); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcpJSONResult(map[string]any{
		"success":  true,
		"message":  fmt.Sprintf("Selector %s found", args.Selector),
		"selector": args.Selector,
	})
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
