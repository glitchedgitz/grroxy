package grrhttp

import (
	"net/http"
	"strings"
)

func DumpRequest(req *http.Request) string {
	return ""
}

func GetHeaders(h http.Header) map[string]string {
	headers := map[string]string{}
	for header, value := range h {
		// header = strings.ReplaceAll(header, "-", "_")
		// header = strings.ToLower(header)
		headers[header] = strings.Join(value, " ///// ")
	}
	return headers
}
