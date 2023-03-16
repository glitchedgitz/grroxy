package api

import "net/http"

type Endpoint struct {
	Method   string
	Endpoint string
}

var V1 = struct {
	Data         Endpoint
	SitemapNew   Endpoint
	SitemapFetch Endpoint
}{
	Data: Endpoint{
		Method:   http.MethodPost,
		Endpoint: "/api/data",
	},
	SitemapNew: Endpoint{
		Method:   http.MethodPost,
		Endpoint: "/api/sitemap/new",
	},
	SitemapFetch: Endpoint{
		Method:   http.MethodPost,
		Endpoint: "/api/sitemap/fetch",
	},
}
