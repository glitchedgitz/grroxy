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

type Label struct {
	ID    string `db:"id" json:"id"`
	Name  string `db:"name" json:"name"`
	Color string `db:"color" json:"color"`
	Type  string `db:"type" json:"type"`
	Icon  string `db:"icon" json:"icon"`
}

// type SitemapFetchResponse struct {
// 	Path   string `db:"path" json:"path"`
// 	Type   string `db:"type" json:"type"`
// 	Title  string `db:"title" json:"title"` // The first folder  OR file OR anything after ?/#
// 	Data string `db:"data" json:"data"`
// }

type SitemapFetch struct {
	Host  string `db:"host" json:"host"`
	Path  string `db:"path" json:"path"`
	Depth int    `db:"depth" json:"depth"` // Depth limit: 0 or not set = default 1 level, -1 = unlimited, positive number = specific depth
	// ID     string `db:"id" json:"id"`
	// Type   string `db:"type" json:"type"`
	// Data string `db:"data" json:"data"`
}

type SitemapNode struct {
	Host          string         `json:"host"`
	Path          string         `json:"path"`
	Type          interface{}    `json:"type"`
	Title         string         `json:"title"`
	Ext           interface{}    `json:"ext"`
	Query         interface{}    `json:"query"`
	Children      []*SitemapNode `json:"children,omitempty"`
	ChildrenCount int            `json:"children_count"`
	IsFolder      bool           `json:"is_folder"`
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
