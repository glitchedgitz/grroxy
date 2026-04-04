package app

import (
	"io"
	"net/http"
	"net/url"
	"strings"
)

// applySetToRequest modifies an http.Request based on a key like "req.headers.User-Agent"
func applySetToRequest(req *http.Request, key string, value string) {
	if strings.HasPrefix(key, "req.headers.") {
		header := strings.TrimPrefix(key, "req.headers.")
		req.Header.Set(header, value)
	} else if key == "req.method" {
		req.Method = value
	} else if key == "req.path" {
		req.URL.Path = value
	} else if key == "req.url" {
		parsedURL, err := url.Parse(value)
		if err == nil {
			req.URL = parsedURL
		}
	} else if key == "req.body" {
		req.Body = io.NopCloser(strings.NewReader(value))
		req.ContentLength = int64(len(value))
	} else if strings.HasPrefix(key, "req.query.") {
		param := strings.TrimPrefix(key, "req.query.")
		q := req.URL.Query()
		q.Set(param, value)
		req.URL.RawQuery = q.Encode()
	}
}

// applyDeleteToRequest removes a field from an http.Request based on a key
func applyDeleteToRequest(req *http.Request, key string) {
	if strings.HasPrefix(key, "req.headers.") {
		header := strings.TrimPrefix(key, "req.headers.")
		if strings.HasSuffix(header, "*") {
			prefix := strings.TrimSuffix(header, "*")
			for h := range req.Header {
				if strings.HasPrefix(h, prefix) {
					req.Header.Del(h)
				}
			}
		} else {
			req.Header.Del(header)
		}
	} else if key == "req.method" {
		req.Method = "GET"
	} else if key == "req.path" {
		req.URL.Path = ""
	} else if key == "req.url" {
		req.URL, _ = url.Parse("")
	} else if key == "req.body" {
		req.Body = io.NopCloser(strings.NewReader(""))
		req.ContentLength = 0
	} else if strings.HasPrefix(key, "req.query.") {
		param := strings.TrimPrefix(key, "req.query.")
		q := req.URL.Query()
		q.Del(param)
		req.URL.RawQuery = q.Encode()
	}
}
