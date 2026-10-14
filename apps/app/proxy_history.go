package app

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/glitchedgitz/grroxy/internal/utils"
	"github.com/glitchedgitz/pocketbase/apis"
	"github.com/glitchedgitz/pocketbase/core"
	"github.com/glitchedgitz/pocketbase/models"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
)

// proxyHistoryPerPage is how many rows one page of /api/proxy/history holds.
// Rows carry the req/resp json, so a page has to stay small enough to be worth
// handing to a model in one go — same budget as /api/label/requests.
const proxyHistoryPerPage = 20

// proxyIDWidth is the width a proxy id is padded to, see
// ProxyManager.GetNextProxyID — "1" is stored as "______________1".
const proxyIDWidth = 15

// ProxyHistoryRequest is the POST /api/proxy/history body. Every field is
// optional, an empty body reads the first page of every proxy's traffic.
type ProxyHistoryRequest struct {
	// ID is the proxy to read. Underscore padding is optional, "1" and
	// "______________1" both work. Empty reads every proxy.
	ID string `json:"id"`
	// Page starts at 1.
	Page int `json:"page"`
	// Filter is an extra dadql filter over the _data fields, ANDed with the
	// proxy, eg. "resp.status = 500".
	Filter string `json:"filter"`
	// Host restricts the rows to one host. With a scheme it has to match the
	// stored host exactly, without one it is matched as a substring.
	Host string `json:"host"`
	// Order is "newest" (default) or "oldest" — the order the proxy saw them in.
	Order string `json:"order"`
}

// ProxyInfo is the proxy a history read was scoped to.
type ProxyInfo struct {
	ID      string `json:"id"`
	Label   string `json:"label,omitempty"`
	Addr    string `json:"addr,omitempty"`
	Browser string `json:"browser,omitempty"`
	State   string `json:"state,omitempty"`
}

type ProxyHistoryResponse struct {
	Success bool `json:"success"`
	// Proxy is only set when the read was scoped to one.
	Proxy   *ProxyInfo       `json:"proxy,omitempty"`
	Page    int              `json:"page"`
	PerPage int              `json:"perPage"`
	Order   string           `json:"order"`
	Count   int              `json:"count"`
	HasMore bool             `json:"hasMore"`
	Rows    []map[string]any `json:"rows"`
}

// ProxyHistoryEndpoint returns the requests a proxy captured.
func (backend *Backend) ProxyHistoryEndpoint(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/proxy/history",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			authRecord, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			if admin == nil && authRecord == nil {
				return c.String(http.StatusForbidden, "")
			}

			var body ProxyHistoryRequest
			if err := c.Bind(&body); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]any{"error": "Invalid request body"})
			}

			if body.ID == "" {
				return c.JSON(http.StatusBadRequest, map[string]any{"error": "id is required"})
			}

			resp, err := backend.proxyHistoryLogic(body)
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
			}

			return c.JSON(http.StatusOK, resp)
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

// padProxyID brings a proxy id to the padded form the records are keyed by, so
// that a caller can say "1" instead of "______________1". An id that is already
// long enough is handed back untouched — FormatStringID pads with
// strings.Repeat, which panics on a negative count.
func padProxyID(proxyID string) string {
	proxyID = strings.TrimSpace(proxyID)
	if proxyID == "" || len(proxyID) >= proxyIDWidth {
		return proxyID
	}
	return utils.FormatStringID(proxyID, proxyIDWidth)
}

// proxyHistoryTag is the generated_by substring that marks a row as captured by
// a proxy. A proxy stamps "proxy/<id>", or "ai/<chatID>/proxy/<id>" when it is
// AI-owned (see RawProxyWrapper.formatGeneratedBy), so a contains-match on
// "proxy/<id>" finds both forms and a bare "proxy/" finds every proxy's rows.
func proxyHistoryTag(proxyID string) string {
	if proxyID = padProxyID(proxyID); proxyID == "" {
		return "proxy/"
	}
	return "proxy/" + proxyID
}

// proxyHistoryFilter builds the "_data" filter for a proxy's traffic.
//
// The tag is bound as a param rather than interpolated: a padded id is all
// underscores, and "_" is a single-character wildcard in LIKE — bound values
// are escaped (see wrapLikeParams in pocketbase/tools/search), so
// "proxy/______________1" cannot also match "proxy/_____________11".
//
// host and extra are optional. extra is parenthesised so that an OR inside it
// cannot widen the proxy condition into "this proxy, or anything else matching".
func proxyHistoryFilter(proxyID, host, extra string) (string, dbx.Params) {
	conditions := []string{"generated_by ~ {:proxy}"}
	params := dbx.Params{"proxy": proxyHistoryTag(proxyID)}

	if host = strings.TrimSpace(host); host != "" {
		// stored hosts carry the scheme ("https://example.com"), so a bare
		// "example.com" can only be matched as a substring
		if strings.Contains(host, "://") {
			conditions = append(conditions, "host = {:host}")
			params["host"] = trimHost(host)
		} else {
			conditions = append(conditions, "host ~ {:host}")
			params["host"] = host
		}
	}

	if extra = strings.TrimSpace(extra); extra != "" {
		conditions = append(conditions, "("+extra+")")
	}

	return strings.Join(conditions, " AND "), params
}

// proxyHistorySort maps the requested order onto the index columns. The index
// counts up as requests arrive, so it is the order the proxy saw them in.
func proxyHistorySort(order string) (string, string) {
	if strings.EqualFold(strings.TrimSpace(order), "oldest") {
		return "index,index_minor", "oldest"
	}
	return "-index,-index_minor", "newest"
}

// proxyHistoryLogic is the work behind /api/proxy/history and the proxyHistory
// MCP tool.
func (backend *Backend) proxyHistoryLogic(body ProxyHistoryRequest) (ProxyHistoryResponse, error) {
	dao := backend.App.Dao()

	proxyID := padProxyID(body.ID)

	// A scoped read resolves the proxy first: an id that matches nothing would
	// otherwise come back as an empty page, which reads like "this proxy saw no
	// traffic" rather than "there is no such proxy".
	var info *ProxyInfo
	if proxyID != "" {
		record, err := dao.FindRecordById("_proxies", proxyID)
		if err != nil || record == nil {
			return ProxyHistoryResponse{}, fmt.Errorf("no proxy %q, call proxyList for the ids that exist", body.ID)
		}
		info = &ProxyInfo{
			ID:      record.Id,
			Label:   record.GetString("label"),
			Addr:    record.GetString("addr"),
			Browser: record.GetString("browser"),
			State:   record.GetString("state"),
		}
	}

	filter, params := proxyHistoryFilter(proxyID, body.Host, body.Filter)
	sort, order := proxyHistorySort(body.Order)

	page := body.Page
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * proxyHistoryPerPage

	// one extra row is read to tell whether a next page exists
	records, err := dao.FindRecordsByFilter(
		"_data",
		filter,
		sort,
		proxyHistoryPerPage+1,
		offset,
		params,
	)
	if err != nil {
		return ProxyHistoryResponse{}, fmt.Errorf("failed to fetch proxy history with filter %q: %w", filter, err)
	}

	hasMore := len(records) > proxyHistoryPerPage
	if hasMore {
		records = records[:proxyHistoryPerPage]
	}

	rows := make([]map[string]any, 0, len(records))
	for _, record := range records {
		rows = append(rows, map[string]any{
			"id":          record.GetString("id"),
			"index":       record.GetFloat("index"),
			"index_minor": record.GetFloat("index_minor"),
			"created":     record.GetString("created"),
			"host":        record.GetString("host"),
			"port":        record.GetString("port"),
			// which proxy captured the row — the only thing telling them apart
			// when the read was not scoped to one
			"generated_by": record.GetString("generated_by"),
			"has_params":   record.GetBool("has_params"),
			"has_resp":     record.GetBool("has_resp"),
			"http":         record.GetString("http"),
			// headers are dropped to keep a page of rows compact, they are on
			// the raw request/response if they are wanted
			"req":  withoutHeaders(record.Get("req_json")),
			"resp": withoutHeaders(record.Get("resp_json")),
		})
	}

	return ProxyHistoryResponse{
		Success: true,
		Proxy:   info,
		Page:    page,
		PerPage: proxyHistoryPerPage,
		Order:   order,
		Count:   len(rows),
		HasMore: hasMore,
		Rows:    rows,
	}, nil
}
