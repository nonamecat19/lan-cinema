package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

func (app *App) seriesHandler(writer http.ResponseWriter, request *http.Request) {
	parts := strings.Split(strings.Trim(request.URL.Path, "/"), "/")
	if len(parts) < 4 || parts[0] != "api" || parts[1] != "series" || (parts[3] != "seasons" && parts[3] != "ordering") {
		http.NotFound(writer, request)
		return
	}
	if request.Method != http.MethodPost {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if app.torrents == nil {
		writeJSONError(writer, http.StatusServiceUnavailable, "Torrent downloads are not configured")
		return
	}
	catalog := app.catalog.Load()
	if catalog == nil {
		writeJSONError(writer, http.StatusServiceUnavailable, "Library is preparing")
		return
	}
	var series *Series
	for i := range catalog.Series {
		if catalog.Series[i].ID == parts[2] {
			series = &catalog.Series[i]
			break
		}
	}
	if series == nil {
		http.NotFound(writer, request)
		return
	}
	if len(parts) == 4 && parts[3] == "seasons" {
		var selection struct {
			SourceID string `json:"sourceId"`
			Seasons  []int  `json:"seasons"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 1<<20)).Decode(&selection); err != nil || selection.SourceID == "" {
			writeJSONError(writer, http.StatusBadRequest, "Invalid season selection")
			return
		}
		if err := app.torrents.AddSeasons(selection.SourceID, series.Title, selection.Seasons); err != nil {
			writeJSONError(writer, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(writer, http.StatusAccepted, map[string]bool{"started": true})
		return
	}
	if len(parts) == 4 && parts[3] == "ordering" {
		var selection struct {
			SourceID      string `json:"sourceId"`
			ForceOneByOne bool   `json:"forceOneByOne"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 1<<20)).Decode(&selection); err != nil || selection.SourceID == "" {
			writeJSONError(writer, http.StatusBadRequest, "Invalid ordering selection")
			return
		}
		if err := app.torrents.SetForceOneByOne(selection.SourceID, series.Title, selection.ForceOneByOne); err != nil {
			writeJSONError(writer, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(writer, http.StatusOK, map[string]bool{"updated": true})
		return
	}
	if len(parts) != 6 || parts[3] != "seasons" || (parts[5] != "pause" && parts[5] != "resume" && parts[5] != "delete") {
		http.NotFound(writer, request)
		return
	}
	number, err := strconv.Atoi(parts[4])
	if err != nil || number < 1 || number > 999 {
		writeJSONError(writer, http.StatusBadRequest, "Invalid season number")
		return
	}
	var selection struct {
		SourceID string `json:"sourceId"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 1<<20)).Decode(&selection); err != nil || selection.SourceID == "" {
		writeJSONError(writer, http.StatusBadRequest, "Invalid torrent source")
		return
	}
	if parts[5] == "delete" {
		err = app.torrents.DeleteSeason(selection.SourceID, series.Title, number, app.cacheDir, app.RefreshCatalog)
	} else {
		err = app.torrents.UpdateSeason(selection.SourceID, series.Title, number, parts[5] == "pause")
	}
	if err != nil {
		writeJSONError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, map[string]bool{"updated": true})
}
