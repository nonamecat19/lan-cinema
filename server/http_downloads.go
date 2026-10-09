package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

func (app *App) torrentsHandler(writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodPost {
		if app.torrents == nil {
			writeJSONError(writer, http.StatusServiceUnavailable, "Torrent uploads are not configured")
			return
		}
		request.Body = http.MaxBytesReader(writer, request.Body, 9<<20)
		if err := request.ParseMultipartForm(1 << 20); err != nil {
			writeJSONError(writer, http.StatusBadRequest, "Choose a .torrent file smaller than 8 MB")
			return
		}
		if request.MultipartForm != nil {
			defer request.MultipartForm.RemoveAll()
		}
		file, header, err := request.FormFile("torrent")
		if err != nil {
			writeJSONError(writer, http.StatusBadRequest, "Choose a .torrent file")
			return
		}
		defer file.Close()
		if header.Size > 8<<20 {
			writeJSONError(writer, http.StatusRequestEntityTooLarge, "Torrent file must be smaller than 8 MB")
			return
		}
		data, err := io.ReadAll(io.LimitReader(file, (8<<20)+1))
		if err != nil || len(data) > 8<<20 {
			writeJSONError(writer, http.StatusBadRequest, "Unable to read torrent file")
			return
		}
		source, err := app.torrents.Upload(header.Filename, data)
		if err != nil {
			writeJSONError(writer, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(writer, http.StatusCreated, source)
		return
	}
	if request.Method != http.MethodGet {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if app.torrents == nil {
		writeJSONError(writer, http.StatusServiceUnavailable, "Torrent downloads are not configured")
		return
	}
	sources, err := app.torrents.Sources()
	if err != nil {
		writeJSONError(writer, http.StatusServiceUnavailable, "Torrent folder is unavailable")
		return
	}
	writeJSON(writer, http.StatusOK, sources)
}

func (app *App) torrentHandler(writer http.ResponseWriter, request *http.Request) {
	if app.torrents == nil {
		writeJSONError(writer, http.StatusServiceUnavailable, "Torrent downloads are not configured")
		return
	}
	parts := strings.Split(strings.Trim(request.URL.Path, "/"), "/")
	if len(parts) == 3 && parts[0] == "api" && parts[1] == "torrents" {
		if request.Method != http.MethodGet {
			writer.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		selection, err := app.torrents.Inspect(parts[2])
		if err != nil {
			writeJSONError(writer, http.StatusBadGateway, "Unable to inspect torrent: "+err.Error())
			return
		}
		writeJSON(writer, http.StatusOK, selection)
		return
	}
	if len(parts) != 4 || parts[0] != "api" || parts[1] != "torrents" || parts[3] != "start" || request.Method != http.MethodPost {
		http.NotFound(writer, request)
		return
	}
	var selection TorrentSelection
	if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 1<<20)).Decode(&selection); err != nil {
		writeJSONError(writer, http.StatusBadRequest, "Invalid download selection")
		return
	}
	if err := app.torrents.Start(parts[2], selection); err != nil {
		writeJSONError(writer, http.StatusBadRequest, "Unable to start the selected download")
		return
	}
	writeJSON(writer, http.StatusAccepted, map[string]bool{"started": true})
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func (app *App) downloadsHandler(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if app.torrents == nil {
		writeJSONError(writer, http.StatusServiceUnavailable, "Torrent downloads are not configured")
		return
	}
	downloads, err := app.torrents.Downloads(app.catalog.Load())
	if err != nil {
		writeJSONError(writer, http.StatusBadGateway, "Torrent client is unavailable")
		return
	}
	writeJSON(writer, http.StatusOK, downloads)
}

func (app *App) downloadHandler(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost || app.torrents == nil {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	parts := strings.Split(strings.Trim(request.URL.Path, "/"), "/")
	if len(parts) != 4 || parts[0] != "api" || parts[1] != "downloads" || (parts[3] != "pause" && parts[3] != "resume" && parts[3] != "delete") {
		http.NotFound(writer, request)
		return
	}
	if parts[3] == "delete" {
		if err := app.torrents.Delete(parts[2]); err != nil {
			writeJSONError(writer, http.StatusBadGateway, "Unable to remove torrent and downloaded files")
			return
		}
		writeJSON(writer, http.StatusOK, map[string]bool{"deleted": true})
		return
	}
	if err := app.torrents.Pause(parts[2], parts[3] == "pause"); err != nil {
		writeJSONError(writer, http.StatusBadGateway, "Unable to update torrent")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]bool{"updated": true})
}
