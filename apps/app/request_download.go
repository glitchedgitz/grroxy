package app

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/glitchedgitz/pocketbase/apis"
	"github.com/glitchedgitz/pocketbase/core"
	"github.com/glitchedgitz/pocketbase/models"
	"github.com/labstack/echo/v5"
)

// DownloadRequest is the POST /api/request/download body.
//
// Where the file lands and what it is called are not caller choices. Every
// download is requests/{id}_{part}.txt under the project working directory —
// the directory the frontend file explorer browses — so a request the user
// asked for always turns up where they are already looking, under a name
// pointing back at the row.
type DownloadRequest struct {
	RowTargets

	Part   string `json:"part"`   // "req" (default), "resp", or "both"
	Edited bool   `json:"edited"` // prefer the edited copy, fall back to the original
}

type DownloadedFile struct {
	ID    string `json:"id"`
	Path  string `json:"path"`
	Bytes int    `json:"bytes"`
}

type DownloadResponse struct {
	Success bool              `json:"success"`
	Files   []DownloadedFile  `json:"files"`
	Count   int               `json:"count"`
	Skipped map[string]string `json:"skipped,omitempty"`
}

// DownloadRequestEndpoint writes the raw request/response of stored rows out to
// files on disk.
func (backend *Backend) DownloadRequestEndpoint(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/request/download",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			authRecord, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			if admin == nil && authRecord == nil {
				return c.String(http.StatusForbidden, "")
			}

			var body DownloadRequest
			if err := c.Bind(&body); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]any{"error": "Invalid request body"})
			}

			resp, skipped, err := backend.downloadRequestLogic(body)
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error(), "skipped": skipped})
			}

			return c.JSON(http.StatusOK, resp)
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}

// downloadRequestLogic is the work behind /api/request/download and the
// downloadRequest MCP tool.
func (backend *Backend) downloadRequestLogic(body DownloadRequest) (DownloadResponse, map[string]string, error) {
	var parts []string
	switch strings.ToLower(strings.TrimSpace(body.Part)) {
	case "", "req", "request":
		parts = []string{"req"}
	case "resp", "response":
		parts = []string{"resp"}
	case "both", "all":
		parts = []string{"req", "resp"}
	default:
		return DownloadResponse{}, nil, fmt.Errorf("unknown part %q, use \"req\", \"resp\" or \"both\"", body.Part)
	}

	ids, skipped, err := backend.resolveExtractTargets(body.RowTargets)
	if err != nil {
		return DownloadResponse{}, skipped, err
	}

	// downloads get their own folder so they do not litter the top of the CWD
	// the file explorer opens on
	dir := filepath.Join(backend.Config.CWD(), "requests")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return DownloadResponse{}, skipped, fmt.Errorf("failed to create %s: %w", dir, err)
	}

	dao := backend.App.Dao()
	resp := DownloadResponse{Success: true, Files: make([]DownloadedFile, 0, len(ids)*len(parts))}

	for _, id := range ids {
		// ids are stored left padded, the padding is not worth carrying into a
		// file name. resolveExtractTargets has already rejected separators.
		label := strings.TrimLeft(id, "_")

		for _, part := range parts {
			raw := ""

			if body.Edited {
				if record, _ := dao.FindRecordById("_"+part+"_edited", id); record != nil {
					raw = record.GetString("raw")
				}
			}
			if raw == "" {
				if record, _ := dao.FindRecordById("_"+part, id); record != nil {
					raw = record.GetString("raw")
				}
			}
			if raw == "" {
				skipped[label+"/"+part] = "no " + part + " stored for this row"
				continue
			}

			// the id names the file, eg. 476_req.txt — an index alone would
			// collide index_minor rows, 1 and 1.9 are different requests. The
			// part suffix keeps "both" from writing the two sides over each
			// other, and is what tells the two apart now that the response
			// carries only the path.
			path := filepath.Join(dir, label+"_"+part+".txt")

			if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
				return DownloadResponse{}, skipped, fmt.Errorf("failed to write %s: %w", path, err)
			}

			resp.Files = append(resp.Files, DownloadedFile{ID: id, Path: path, Bytes: len(raw)})
		}
	}

	if len(resp.Files) == 0 {
		// parts, not body.Part — that is empty on the default path and would
		// read as "had a stored ."
		return DownloadResponse{}, skipped, fmt.Errorf("nothing to write, none of the requested rows had a stored %s", strings.Join(parts, " or "))
	}

	resp.Count = len(resp.Files)
	if len(skipped) > 0 {
		resp.Skipped = skipped
	}

	log.Printf("[DownloadRequest] wrote %d file(s) to %s", resp.Count, dir)

	return resp, skipped, nil
}
