package types

type SitemapGet struct {
	Host     string
	Path     string
	Query    string
	Fragment string
	MainID   string
	Type     string
}

type SitemapFetch struct {
	// ID     string `db:"id" json:"id"`
	Host string `db:"host" json:"host"`
	Path string `db:"path" json:"path"`
	// Type   string `db:"type" json:"type"`
	// MainID string `db:"mainID" json:"mainID"`
}
type SitemapFetchResponse struct {
	Path   string `db:"path" json:"path"`
	Type   string `db:"type" json:"type"`
	MainID string `db:"mainID" json:"mainID"`
}

type SitemapRows struct {
	Host    string `db:"host" json:"host"`
	Path    string `db:"path" json:"path"`
	Page    int64  `db:"page" json:"page"`
	PerPage int64  `db:"perPage" json:"perPage"`
}

type SitemapRowsResponse struct {
	TotalItems int `json:"totalItems"`
	TotalPages int `json:"totalPages"`
}
