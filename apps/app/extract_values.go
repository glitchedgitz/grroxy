package app

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/glitchedgitz/grroxy/internal/schemas"
	"github.com/glitchedgitz/grroxy/internal/utils"
	"github.com/glitchedgitz/pocketbase/apis"
	"github.com/glitchedgitz/pocketbase/core"
	"github.com/glitchedgitz/pocketbase/models"
	"github.com/labstack/echo/v5"
)

// defaultExtractFields are searched when the caller does not name any. The raw
// request and response are what the quick searches were written against.
var defaultExtractFields = []string{"req.raw", "resp.raw"}

// ExtractValuesRequest is the POST /api/extract/values body.
//
// A target is one or more rows to search, named by record id. A pattern is
// either supplied inline via search or named via name, which pulls it from the
// _searches collection.
type ExtractValuesRequest struct {
	RowTargets

	// Name of a saved quick search in _searches, used when no inline pattern
	// is given. Matched case-insensitively.
	Name string `json:"name"`

	// Inline pattern. Pattern is an alias for Search so callers can send
	// whichever name reads better.
	Search        string `json:"search"`
	Pattern       string `json:"pattern"`
	Regexp        *bool  `json:"regexp"`
	CaseSensitive bool   `json:"caseSensitive"`
	WholeWord     bool   `json:"wholeWord"`

	// Fields to search on each row, eg. "req.raw", "resp.raw", "req.url",
	// "resp_edited.raw". Defaults to defaultExtractFields.
	Fields []string `json:"fields"`

	// Group is the capture group to return instead of the whole match.
	Group int `json:"group"`
	// Unique drops repeated values from the flat "values" list. Defaults to true.
	Unique *bool `json:"unique"`
	// Limit caps the matches taken per field, 0 means all of them.
	Limit int `json:"limit"`
}

// ExtractValuesResponse is the POST /api/extract/values response.
//
// Just the matched strings. Per-row breakdowns with match offsets were more
// bulk than signal — what a caller wants from an extraction is the list.
type ExtractValuesResponse struct {
	Success bool                  `json:"success"`
	Name    string                `json:"name,omitempty"`
	Pattern schemas.SearchPattern `json:"pattern"`
	Regex   string                `json:"regex"`
	Values  []string              `json:"values"`
	Count   int                   `json:"count"`
	Skipped map[string]string     `json:"skipped,omitempty"`
}

// ExtractValuesEndpoint extracts values out of stored requests/responses by
// running a quick search pattern over them.
func (backend *Backend) ExtractValuesEndpoint(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/extract/values",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			authRecord, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			if admin == nil && authRecord == nil {
				return c.String(http.StatusForbidden, "")
			}

			var body ExtractValuesRequest
			if err := c.Bind(&body); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]any{
					"error": "Invalid request body",
				})
			}

			resp, skipped, err := backend.extractValuesLogic(body)
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]any{
					"error":   err.Error(),
					"skipped": skipped,
				})
			}

			return c.JSON(http.StatusOK, resp)
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

// extractValuesLogic is the work behind /api/extract/values and the
// extractValues MCP tool. On error it also returns whatever it managed to
// resolve, so a caller can report which targets were bad.
func (backend *Backend) extractValuesLogic(body ExtractValuesRequest) (ExtractValuesResponse, map[string]string, error) {
	pattern, name, err := backend.resolveSearchPattern(body)
	if err != nil {
		return ExtractValuesResponse{}, nil, err
	}

	re, err := pattern.Compile()
	if err != nil {
		return ExtractValuesResponse{}, nil, err
	}

	ids, skipped, err := backend.resolveExtractTargets(body.RowTargets)
	if err != nil {
		return ExtractValuesResponse{}, skipped, err
	}

	fields := body.Fields
	if len(fields) == 0 {
		fields = defaultExtractFields
	}

	resp := ExtractValuesResponse{
		Success: true,
		Name:    name,
		Pattern: pattern,
		Regex:   re.String(),
		Values:  make([]string, 0),
	}
	if len(skipped) > 0 {
		resp.Skipped = skipped
	}

	unique := body.Unique == nil || *body.Unique
	seen := make(map[string]bool)

	for _, id := range ids {
		for _, value := range backend.extractFromRow(id, fields, re, body.Group, body.Limit) {
			if unique {
				if seen[value] {
					continue
				}
				seen[value] = true
			}
			resp.Values = append(resp.Values, value)
		}
	}

	resp.Count = len(resp.Values)

	log.Printf("[ExtractValues] pattern=%q rows=%d values=%d", re.String(), len(ids), resp.Count)

	return resp, skipped, nil
}

// resolveSearchPattern picks the pattern to run: an inline one from the body,
// otherwise the saved search named by the caller. Returns the pattern and the
// name it came from (empty for inline patterns).
func (backend *Backend) resolveSearchPattern(body ExtractValuesRequest) (schemas.SearchPattern, string, error) {
	search := body.Search
	if search == "" {
		search = body.Pattern
	}

	if search != "" {
		// an inline search reads as a pattern unless the caller says otherwise
		isRegexp := true
		if body.Regexp != nil {
			isRegexp = *body.Regexp
		}

		return schemas.SearchPattern{
			Search:        search,
			Regexp:        isRegexp,
			CaseSensitive: body.CaseSensitive,
			WholeWord:     body.WholeWord,
		}, "", nil
	}

	if body.Name == "" {
		return schemas.SearchPattern{}, "", fmt.Errorf("either search (a pattern) or name (a saved search) is required")
	}

	return backend.findSavedSearch(body.Name)
}

// findSavedSearch looks a quick search up by name. They are global settings and
// live on the launcher, the project db copy is only a fallback for when the
// launcher is not reachable.
func (backend *Backend) findSavedSearch(name string) (schemas.SearchPattern, string, error) {
	if pattern, found, err := backend.findSavedSearchOnLauncher(name); err != nil {
		log.Printf("[ExtractValues] launcher lookup for %q failed, falling back to the project db: %v", name, err)
	} else if found {
		return pattern, name, nil
	}

	records, err := backend.App.Dao().FindRecordsByExpr("_searches")
	if err != nil {
		return schemas.SearchPattern{}, "", fmt.Errorf("failed to read saved searches: %w", err)
	}

	for _, record := range records {
		if !strings.EqualFold(record.GetString("name"), name) {
			continue
		}

		var pattern schemas.SearchPattern
		if err := json.Unmarshal([]byte(record.GetString("data")), &pattern); err != nil {
			return schemas.SearchPattern{}, "", fmt.Errorf("saved search %q is not a valid pattern: %w", name, err)
		}

		return pattern, record.GetString("name"), nil
	}

	return schemas.SearchPattern{}, "", fmt.Errorf("no saved search named %q", name)
}

// findSavedSearchOnLauncher reads _searches off the launcher over HTTP, the same
// way templates are fetched. found is false when the launcher answered but has
// no search under that name.
func (backend *Backend) findSavedSearchOnLauncher(name string) (schemas.SearchPattern, bool, error) {
	if backend.Config.LauncherAddr == "" {
		return schemas.SearchPattern{}, false, nil
	}

	url := fmt.Sprintf("http://%s/api/collections/_searches/records?perPage=500", backend.Config.LauncherAddr)

	resp, err := http.Get(url)
	if err != nil {
		return schemas.SearchPattern{}, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return schemas.SearchPattern{}, false, fmt.Errorf("launcher returned %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return schemas.SearchPattern{}, false, err
	}

	var result struct {
		Items []struct {
			Name string                `json:"name"`
			Data schemas.SearchPattern `json:"data"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return schemas.SearchPattern{}, false, err
	}

	for _, item := range result.Items {
		if strings.EqualFold(item.Name, name) {
			return item.Data, true, nil
		}
	}

	return schemas.SearchPattern{}, false, nil
}

// RowTargets is how the extract endpoints name the rows to work on: by record
// id, as strings.
//
// Ids rather than indexes because an index does not identify a row. index_minor
// splits one index across several rows, so "5" and "5.11" are different
// requests; a lookup by index = 5 returns both, which means a caller can neither
// ask for just 5 nor for just 5.11.
type RowTargets struct {
	IDs []string `json:"ids"`
}

// resolveExtractTargets normalises the requested ids to the padded form they
// are stored under. skipped maps a rejected id to the reason, so a bad entry in
// a batch does not fail the whole call.
func (backend *Backend) resolveExtractTargets(body RowTargets) ([]string, map[string]string, error) {
	ids := make([]string, 0, len(body.IDs))
	skipped := make(map[string]string)
	seen := make(map[string]bool)

	for _, raw := range body.IDs {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}

		// downloadRequest turns an id into a file name, so a separator in one
		// would write outside the directory it is supposed to stay in
		if strings.ContainsAny(id, `/\`) {
			skipped[raw] = "not a valid record id"
			continue
		}

		// stored ids are left padded to 15 with underscores. An id that already
		// carries them is taken as is, a bare "5" or "5.11" gets padded here so
		// callers can pass whichever form they have.
		if !strings.Contains(id, "_") {
			if len(id) > 15 {
				skipped[raw] = "too long to be a record id"
				continue
			}
			id = utils.FormatStringID(id, 15)
		}

		if len(id) != 15 {
			skipped[raw] = "not a valid record id"
			continue
		}

		if seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}

	if len(ids) == 0 {
		if len(skipped) > 0 {
			return nil, skipped, fmt.Errorf("none of the requested ids were usable")
		}
		return nil, nil, fmt.Errorf("ids is required")
	}

	return ids, skipped, nil
}

// extractFromRow runs the regex over each requested field of a single row and
// returns everything it matched, in the order the fields were asked for.
func (backend *Backend) extractFromRow(id string, fields []string, re *regexp.Regexp, group, limit int) []string {
	dao := backend.App.Dao()

	// _req, _resp and their edited variants all share the _data record id
	reqRecord, _ := dao.FindRecordById("_req", id)
	respRecord, _ := dao.FindRecordById("_resp", id)
	reqEditedRecord, _ := dao.FindRecordById("_req_edited", id)
	respEditedRecord, _ := dao.FindRecordById("_resp_edited", id)

	values := make([]string, 0)

	for _, field := range fields {
		value := extractFieldValue(reqRecord, respRecord, reqEditedRecord, respEditedRecord, field)
		if value == nil {
			continue
		}

		text := convertValueToString(value)
		if text == "" {
			continue
		}

		values = append(values, matchAll(re, text, group, limit)...)
	}

	return values
}

// matchAll collects the matches of re in text. group selects a capture group,
// 0 (or an out of range group) meaning the whole match. limit caps the number
// of matches, 0 means all of them.
func matchAll(re *regexp.Regexp, text string, group, limit int) []string {
	n := -1
	if limit > 0 {
		n = limit
	}

	found := re.FindAllStringSubmatchIndex(text, n)
	matches := make([]string, 0, len(found))

	for _, loc := range found {
		start, end := loc[0], loc[1]

		// 2 ints per group, group 0 is the whole match
		if group > 0 && group*2+1 < len(loc) {
			// an optional group that did not participate is reported as -1
			if loc[group*2] < 0 {
				continue
			}
			start, end = loc[group*2], loc[group*2+1]
		}

		matches = append(matches, text[start:end])
	}

	return matches
}
