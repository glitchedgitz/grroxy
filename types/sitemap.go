package types

type SitemapGet struct {
	Host     string `db:"host" json:"host"`
	Path     string `db:"path" json:"path"`
	Query    string `db:"query" json:"query"`
	Fragment string `db:"fragment" json:"fragment"`
	Data     string `db:"data" json:"data"`
	Type     string `db:"type" json:"type"`
	Ext      string `db:"ext" json:"ext"`
}

// type SitemapFetchResponse struct {
// 	Path   string `db:"path" json:"path"`
// 	Type   string `db:"type" json:"type"`
// 	Title  string `db:"title" json:"title"` // The first folder  OR file OR anything after ?/#
// 	Data string `db:"data" json:"data"`
// }

type SitemapFetch struct {
	Host string `db:"host" json:"host"`
	Path string `db:"path" json:"path"`
	// ID     string `db:"id" json:"id"`
	// Type   string `db:"type" json:"type"`
	// Data string `db:"data" json:"data"`
}

type SitemapRows struct {
	Host    string `db:"host" json:"host"`
	Path    string `db:"path" json:"path"`
	Page    int64  `db:"page" json:"page"`
	PerPage int64  `db:"perPage" json:"perPage"`
}

type GetData struct {
	Collection string `db:"collection" json:"collection"`
	Path       string `db:"path" json:"path"`
	Page       int64  `db:"page" json:"page"`
	PerPage    int64  `db:"perPage" json:"perPage"`
	Sort       string `db:"sort" json:"sort"`
	Search     string `db:"search" json:"search"`
	ColType    string `db:"col_type" json:"col_type"`
}

type SitemapRowsResponse struct {
	TotalItems int `json:"totalItems"`
	TotalPages int `json:"totalPages"`
}
