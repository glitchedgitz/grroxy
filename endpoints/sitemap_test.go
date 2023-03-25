package endpoints

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo"
	"github.com/stretchr/testify/assert"
)

func TestSitemapRows(t *testing.T) {
	dbAPI := &DatabaseAPI{}

	// Test successful request
	body := &bytes.Buffer{}
	json.NewEncoder(body).Encode(map[string]interface{}{
		"host": "example.com",
		"path": "/path/to/sitemap",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sitemap_rows", body)
	rec := httptest.NewRecorder()

	c := dbAPI.echo.NewContext(req, rec)

	err := dbAPI.SitemapRows(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Test malformed request
	body = &bytes.Buffer{}
	json.NewEncoder(body).Encode(map[string]interface{}{
		"not_a_valid_field": "example.com",
	})

	req = httptest.NewRequest(http.MethodPost, "/api/v1/sitemap_rows", body)
	rec = httptest.NewRecorder()

	c = dbAPI.echo.NewContext(req, rec)

	err = dbAPI.SitemapRows(c)
	assert.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSitemapFetch(t *testing.T) {
	// create a fake HTTP request to test the SitemapFetch function
	req, err := http.NewRequest(http.MethodGet, "/v1/sitemap/fetch?host=example.com&path=/news", nil)
	if err != nil {
		t.Fatal(err)
	}

	// create a fake echo context using httptest
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)

	// create a new DatabaseAPI instance to test
	db := NewDatabaseAPI(nil)

	// test the SitemapFetch function
	if assert.NoError(t, db.SitemapFetch(c)) {
		// check the HTTP response code
		assert.Equal(t, http.StatusOK, rec.Code)

		// check the response body
		var resp []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		}
		err = json.Unmarshal(rec.Body.Bytes(), &resp)
		if assert.NoError(t, err) {
			assert.Equal(t, []struct {
				Path string `json:"path"`
				Type string `json:"type"`
			}{
				{Path: "/news", Type: "news"},
				{Path: "/news/sports", Type: "sports"},
				{Path: "/news/politics", Type: "politics"},
			}, resp)
		}
	}
}

func TestSitemapNew(t *testing.T) {
	// create a fake HTTP request to test the SitemapNew function
	payload := `{"host": "example.com", "path": "/about", "type": "about", "mainID": "123"}`
	req, err := http.NewRequest(http.MethodPost, "/v1/sitemap/new", bytes.NewBufferString(payload))
	if err != nil {
		t.Fatal(err)
	}

	// set the Content-Type header to "application/json"
	req.Header.Set("Content-Type", "application/json")

	// create a fake echo context using httptest
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)

	// create a new DatabaseAPI instance to test
	db := NewDatabaseAPI(nil)

	// test the SitemapNew function
	if assert.NoError(t, db.SitemapNew(c)) {
		// check the HTTP response code
		assert.Equal(t, http.StatusOK, rec.Code)

		// check the response body
		assert.Equal(t, `{"status":"ok"}`, strings.TrimSpace(rec.Body.String()))
	}
}
