package app

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/glitchedgitz/pocketbase/apis"
	"github.com/glitchedgitz/pocketbase/core"
	"github.com/glitchedgitz/pocketbase/models"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
)

// labelRequestsPerPage is how many rows one page of /api/label/requests holds.
// Rows carry the req/resp json, so a page has to stay small enough to be worth
// handing to a model in one go.
const labelRequestsPerPage = 20

// ListLabelsRequest is the POST /api/label/list body. Both fields are optional,
// an empty body lists every label.
type ListLabelsRequest struct {
	// Search is a substring of the label name, matched case-insensitively.
	Search string `json:"search"`
	// Type narrows to one label type, eg. "custom", "severity", "tech".
	Type string `json:"type"`
}

// LabelInfo is a label as the read endpoints report it.
type LabelInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
	Icon  string `json:"icon,omitempty"`
	Type  string `json:"type,omitempty"`
	// Count is how many rows the label has been attached to, read off the same
	// "label:{id}" counter the sidebar counts with.
	Count int64 `json:"count"`
}

type ListLabelsResponse struct {
	Success bool        `json:"success"`
	Count   int         `json:"count"`
	Labels  []LabelInfo `json:"labels"`
}

// LabelRequestsRequest is the POST /api/label/requests body.
//
// The label is named, not id'd — a name is what the user sees on the row and
// what they say when they ask for "the requests tagged sqli".
type LabelRequestsRequest struct {
	Label string `json:"label"`
	// Page starts at 1.
	Page int `json:"page"`
	// Filter is an extra dadql filter over the _data fields, ANDed with the
	// label, eg. "resp.status = 200".
	Filter string `json:"filter"`
	// Host restricts the rows to one host. With a scheme it has to match the
	// stored host exactly, without one it is matched as a substring.
	Host string `json:"host"`
}

type LabelRequestsResponse struct {
	Success bool             `json:"success"`
	Label   LabelInfo        `json:"label"`
	Page    int              `json:"page"`
	PerPage int              `json:"perPage"`
	Count   int              `json:"count"`
	HasMore bool             `json:"hasMore"`
	Rows    []map[string]any `json:"rows"`
}

// ListLabelsEndpoint lists the labels that exist, with how many rows each one
// is on.
func (backend *Backend) ListLabelsEndpoint(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/label/list",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			authRecord, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			if admin == nil && authRecord == nil {
				return c.String(http.StatusForbidden, "")
			}

			var body ListLabelsRequest
			if err := c.Bind(&body); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]any{"error": "Invalid request body"})
			}

			resp, err := backend.listLabelsLogic(body)
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

// LabelRequestsEndpoint returns the requests carrying a label, by label name.
func (backend *Backend) LabelRequestsEndpoint(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/label/requests",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			authRecord, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			if admin == nil && authRecord == nil {
				return c.String(http.StatusForbidden, "")
			}

			var body LabelRequestsRequest
			if err := c.Bind(&body); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]any{"error": "Invalid request body"})
			}

			resp, err := backend.labelRequestsLogic(body)
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

// labelListFilter narrows a label listing by name and/or type. An empty filter
// means "every label" — the caller has to read them with FindRecordsByExpr,
// FindRecordsByFilter refuses an empty expression.
//
// Values are bound as params rather than interpolated, a label name is user
// input.
func labelListFilter(search, labelType string) (string, dbx.Params) {
	conditions := []string{}
	params := dbx.Params{}

	if search = strings.TrimSpace(search); search != "" {
		conditions = append(conditions, "name ~ {:search}")
		params["search"] = search
	}

	if labelType = strings.TrimSpace(labelType); labelType != "" {
		conditions = append(conditions, "type = {:type}")
		params["type"] = labelType
	}

	return strings.Join(conditions, " AND "), params
}

// labelRowsFilter builds the "_data" filter for the rows carrying a label.
//
// The label is matched by id, not by name: the filter compares text
// case-sensitively, and a name match would also have to decide what "auth"
// means when "authz" exists. "?=" is "any of" — a row carries several labels
// and one of them being this one is what makes it a hit.
//
// host and extra are optional. extra is parenthesised so that an OR inside it
// cannot widen the label condition.
func labelRowsFilter(labelID, host, extra string) (string, dbx.Params) {
	conditions := []string{"attached.labels.id ?= {:labelID}"}
	params := dbx.Params{"labelID": labelID}

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

// listLabelsLogic is the work behind /api/label/list and the listLabels MCP
// tool.
func (backend *Backend) listLabelsLogic(body ListLabelsRequest) (ListLabelsResponse, error) {
	dao := backend.App.Dao()

	filter, params := labelListFilter(body.Search, body.Type)

	var records []*models.Record
	var err error

	// an empty filter is not a filter — FindRecordsByFilter refuses it, so the
	// unnarrowed listing has to go through FindRecordsByExpr
	if filter == "" {
		records, err = dao.FindRecordsByExpr("_labels")
	} else {
		records, err = dao.FindRecordsByFilter("_labels", filter, "name", 0, 0, params)
	}
	if err != nil {
		return ListLabelsResponse{}, fmt.Errorf("failed to read labels: %w", err)
	}

	labels := make([]LabelInfo, 0, len(records))
	for _, record := range records {
		labels = append(labels, backend.labelInfo(record))
	}

	return ListLabelsResponse{
		Success: true,
		Count:   len(labels),
		Labels:  labels,
	}, nil
}

// labelRequestsLogic is the work behind /api/label/requests and the
// getRequestsByLabel MCP tool.
func (backend *Backend) labelRequestsLogic(body LabelRequestsRequest) (LabelRequestsResponse, error) {
	dao := backend.App.Dao()

	label, err := backend.resolveLabelByName(body.Label)
	if err != nil {
		return LabelRequestsResponse{}, err
	}

	filter, params := labelRowsFilter(label.Id, body.Host, body.Filter)

	page := body.Page
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * labelRequestsPerPage

	// one extra row is read to tell whether a next page exists
	records, err := dao.FindRecordsByFilter(
		"_data",
		filter,
		"-index,-index_minor",
		labelRequestsPerPage+1,
		offset,
		params,
	)
	if err != nil {
		return LabelRequestsResponse{}, fmt.Errorf("failed to fetch rows for label %q: %w", label.GetString("name"), err)
	}

	hasMore := len(records) > labelRequestsPerPage
	if hasMore {
		records = records[:labelRequestsPerPage]
	}

	// a row usually carries more than the label that was asked for, and the
	// other ones are what says why it is worth looking at. Expanded for the
	// whole page at once rather than per row — ExpandRecord would be a query
	// each
	expandErrs := dao.ExpandRecords(records, []string{"attached.labels"}, nil)

	rows := make([]map[string]any, 0, len(records))
	for _, record := range records {
		row := map[string]any{
			"id":           record.GetString("id"),
			"index":        record.GetFloat("index"),
			"index_minor":  record.GetFloat("index_minor"),
			"host":         record.GetString("host"),
			"port":         record.GetString("port"),
			"generated_by": record.GetString("generated_by"),
			"has_params":   record.GetBool("has_params"),
			"has_resp":     record.GetBool("has_resp"),
			"http":         record.GetString("http"),
			// headers are dropped to keep a page of rows compact, they are on
			// the raw request/response if they are wanted
			"req":  withoutHeaders(record.Get("req_json")),
			"resp": withoutHeaders(record.Get("resp_json")),
		}

		if attached := record.ExpandedOne("attached"); len(expandErrs) == 0 && attached != nil {
			names := []string{}
			for _, l := range attached.ExpandedAll("labels") {
				names = append(names, l.GetString("name"))
			}
			row["labels"] = names

			if note := strings.TrimSpace(attached.GetString("note")); note != "" {
				row["note"] = note
			}
		}

		rows = append(rows, row)
	}

	return LabelRequestsResponse{
		Success: true,
		Label:   backend.labelInfo(label),
		Page:    page,
		PerPage: labelRequestsPerPage,
		Count:   len(rows),
		HasMore: hasMore,
		Rows:    rows,
	}, nil
}

// resolveLabelByName finds the label a name refers to. An exact name wins; a
// name that only partly matches is taken when it leaves no doubt which label
// was meant, so "sql" finds "sqli" as long as it is the only candidate.
func (backend *Backend) resolveLabelByName(name string) (*models.Record, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("label is required")
	}

	records, err := backend.labelsMatchingName(name)
	if err != nil {
		return nil, err
	}

	partial := []string{}
	for _, record := range records {
		if strings.EqualFold(record.GetString("name"), name) {
			return record, nil
		}
		partial = append(partial, record.GetString("name"))
	}

	switch len(records) {
	case 0:
		return nil, fmt.Errorf("no label named %q, call listLabels to see the ones that exist", name)
	case 1:
		return records[0], nil
	default:
		return nil, fmt.Errorf("%q matches several labels (%s), name one of them exactly", name, strings.Join(partial, ", "))
	}
}

// labelInfo turns a _labels record into the shape the endpoints report, count
// included.
func (backend *Backend) labelInfo(record *models.Record) LabelInfo {
	info := LabelInfo{
		ID:    record.Id,
		Name:  record.GetString("name"),
		Color: record.GetString("color"),
		Icon:  record.GetString("icon"),
		Type:  record.GetString("type"),
	}

	if backend.CounterManager != nil {
		info.Count = backend.CounterManager.Get("label:"+record.Id, "_labels", "")
	}

	return info
}
