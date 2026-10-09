package server

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

type App struct {
	catalog      atomic.Pointer[Catalog]
	ready        atomic.Bool
	webDir       string
	mediaRoot    string
	cacheDir     string
	torrents     *TorrentManager
	refreshMu    sync.Mutex
	catalogMu    sync.Mutex
	active       map[string]string
	prepareSlots chan struct{}
}

func NewApp(webDir string) *App {
	return &App{webDir: webDir, active: make(map[string]string), prepareSlots: make(chan struct{}, 2)}
}

func (app *App) SetCatalog(catalog *Catalog) {
	app.catalog.Store(catalog)
	app.ready.Store(true)
}

func (app *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", app.health)
	mux.HandleFunc("/api/catalog", app.catalogHandler)
	mux.HandleFunc("/api/torrents", app.torrentsHandler)
	mux.HandleFunc("/api/torrents/", app.torrentHandler)
	mux.HandleFunc("/api/downloads", app.downloadsHandler)
	mux.HandleFunc("/api/downloads/", app.downloadHandler)
	mux.HandleFunc("/api/series/", app.seriesHandler)
	mux.HandleFunc("/api/episodes/", app.streamHandler)
	mux.HandleFunc("/", app.frontendHandler)
	return mux
}

func (app *App) ConfigureLibrary(mediaRoot, cacheDir string, manager *TorrentManager) {
	app.mediaRoot = mediaRoot
	app.cacheDir = cacheDir
	app.torrents = manager
}

func (app *App) health(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !app.ready.Load() {
		writer.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	writer.WriteHeader(http.StatusOK)
}

func (app *App) catalogHandler(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	catalog := app.catalog.Load()
	if catalog == nil {
		writer.Header().Set("Retry-After", "3")
		writeJSONError(writer, http.StatusServiceUnavailable, "Library is preparing")
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Content-Type", "application/json")
	series := append([]Series(nil), catalog.Series...)
	if app.torrents != nil {
		for i := range series {
			series[i].TorrentSources = app.torrents.AvailableForSeries(series[i].Title)
		}
	}
	if err := json.NewEncoder(writer).Encode(struct {
		Series []Series `json:"series"`
	}{Series: series}); err != nil {
		log.Printf("catalog response encoding failed")
	}
}

func (app *App) streamHandler(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	parts := strings.Split(strings.Trim(request.URL.Path, "/"), "/")
	if len(parts) != 4 || parts[0] != "api" || parts[1] != "episodes" || (parts[3] != "stream" && parts[3] != "preview" && parts[3] != "preview-sheet") {
		http.NotFound(writer, request)
		return
	}
	catalog := app.catalog.Load()
	if catalog == nil {
		writer.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	episode := catalog.byID[parts[2]]
	if episode == nil {
		http.NotFound(writer, request)
		return
	}
	if parts[3] == "preview" || parts[3] == "preview-sheet" {
		previewPath := episode.PreviewPath
		if parts[3] == "preview-sheet" {
			previewPath = episode.PreviewSpritePath
		}
		if previewPath == "" {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "image/jpeg")
		writer.Header().Set("Cache-Control", "public, max-age=3600")
		http.ServeFile(writer, request, previewPath)
		return
	}
	if parts[3] != "stream" {
		http.NotFound(writer, request)
		return
	}
	if episode.Status != "ready" {
		writeJSONError(writer, http.StatusConflict, dereference(episode.UnavailableReason, "Episode is unavailable"))
		return
	}
	info, err := os.Lstat(episode.SourcePath)
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(writer, request)
		return
	}
	file, err := os.Open(episode.PlaybackPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(writer, request)
			return
		}
		log.Printf("stream open failed for episode %s", episode.ID)
		writeJSONError(writer, http.StatusInternalServerError, "Unable to stream episode")
		return
	}
	defer file.Close()
	fileInfo, err := file.Stat()
	if err != nil {
		log.Printf("stream stat failed for episode %s", episode.ID)
		writeJSONError(writer, http.StatusInternalServerError, "Unable to stream episode")
		return
	}
	writer.Header().Set("Content-Type", "video/mp4")
	writer.Header().Set("Accept-Ranges", "bytes")
	http.ServeContent(writer, request, filepath.Base(episode.PlaybackPath), fileInfo.ModTime(), file)
}

func (app *App) frontendHandler(writer http.ResponseWriter, request *http.Request) {
	if strings.HasPrefix(request.URL.Path, "/api/") {
		http.NotFound(writer, request)
		return
	}
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	clean := filepath.Clean(strings.TrimPrefix(request.URL.Path, "/"))
	path := filepath.Join(app.webDir, clean)
	if relative, err := filepath.Rel(app.webDir, path); err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		path = filepath.Join(app.webDir, "index.html")
	}
	if clean == "." {
		path = filepath.Join(app.webDir, "index.html")
	}
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		http.ServeFile(writer, request, path)
		return
	}
	index := filepath.Join(app.webDir, "index.html")
	if _, err := os.Stat(index); err != nil {
		http.NotFound(writer, request)
		return
	}
	http.ServeFile(writer, request, index)
}

func writeJSONError(writer http.ResponseWriter, status int, message string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]string{"error": message})
}

func dereference(value *string, fallback string) string {
	if value == nil {
		return fallback
	}
	return *value
}
