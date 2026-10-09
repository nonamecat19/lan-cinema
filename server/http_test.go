package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestStreamSupportsRangesAndHead(t *testing.T) {
	media := filepath.Join(t.TempDir(), "episode.mp4")
	if err := os.WriteFile(media, []byte("0123456789"), 0644); err != nil {
		t.Fatal(err)
	}
	episode := Episode{ID: "episode", Status: "ready", SourcePath: media, PlaybackPath: media}
	reason := "Duplicate episode number"
	unavailable := Episode{ID: "unavailable", Status: "unavailable", UnavailableReason: &reason}
	catalog := &Catalog{byID: map[string]*Episode{episode.ID: &episode, unavailable.ID: &unavailable}}
	app := NewApp(t.TempDir())
	app.SetCatalog(catalog)
	tests := []struct {
		method      string
		path        string
		rangeHeader string
		status      int
		body        string
	}{
		{http.MethodGet, "/api/episodes/episode/stream", "bytes=2-5", http.StatusPartialContent, "2345"},
		{http.MethodGet, "/api/episodes/episode/stream", "bytes=20-30", http.StatusRequestedRangeNotSatisfiable, "invalid range: failed to overlap\n"},
		{http.MethodHead, "/api/episodes/episode/stream", "", http.StatusOK, ""},
		{http.MethodGet, "/api/episodes/missing/stream", "", http.StatusNotFound, "404 page not found\n"},
		{http.MethodGet, "/api/episodes/unavailable/stream", "", http.StatusConflict, "{\"error\":\"Duplicate episode number\"}\n"},
	}
	for _, test := range tests {
		request := httptest.NewRequest(test.method, test.path, nil)
		if test.rangeHeader != "" {
			request.Header.Set("Range", test.rangeHeader)
		}
		response := httptest.NewRecorder()
		app.Handler().ServeHTTP(response, request)
		if response.Code != test.status || response.Body.String() != test.body {
			t.Errorf("%s %s: status %d body %q", test.method, test.path, response.Code, response.Body.String())
		}
	}
}

func TestCatalogReportsPreparation(t *testing.T) {
	app := NewApp(t.TempDir())
	request := httptest.NewRequest(http.MethodGet, "/api/catalog", nil)
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable || response.Header().Get("Retry-After") != "3" || response.Body.String() != "{\"error\":\"Library is preparing\"}\n" {
		t.Fatalf("unexpected preparation response: status %d headers %v body %q", response.Code, response.Header(), response.Body.String())
	}
}
