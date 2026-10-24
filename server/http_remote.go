package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

func (app *App) remoteDisplaysHandler(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		writeJSON(writer, http.StatusOK, app.remote.List())
	case http.MethodPost:
		var registration struct {
			ID string `json:"id"`
		}
		if err := decodeRemoteRequest(writer, request, &registration); err != nil || !validRemoteID(registration.ID) {
			writeJSONError(writer, http.StatusBadRequest, "Invalid display ID")
			return
		}
		writeJSON(writer, http.StatusCreated, app.remote.Register(registration.ID))
	default:
		writer.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (app *App) remoteDisplayHandler(writer http.ResponseWriter, request *http.Request) {
	parts := strings.Split(strings.Trim(request.URL.Path, "/"), "/")
	if len(parts) < 4 || len(parts) > 5 || parts[0] != "api" || parts[1] != "remote" || parts[2] != "displays" || !validRemoteID(parts[3]) {
		http.NotFound(writer, request)
		return
	}
	id := parts[3]
	if len(parts) == 4 {
		if request.Method != http.MethodGet {
			writer.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		display, exists := app.remote.Get(id)
		if !exists {
			writeJSONError(writer, http.StatusNotFound, "Display is unavailable")
			return
		}
		writeJSON(writer, http.StatusOK, display)
		return
	}
	if parts[4] != "sync" && parts[4] != "commands" {
		http.NotFound(writer, request)
		return
	}
	if request.Method != http.MethodPost {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if parts[4] == "sync" {
		app.remoteSyncHandler(writer, request, id)
		return
	}
	app.remoteCommandHandler(writer, request, id)
}

func (app *App) remoteSyncHandler(writer http.ResponseWriter, request *http.Request, id string) {
	var syncRequest struct {
		State           json.RawMessage `json:"state"`
		AcknowledgedIDs []string        `json:"acknowledgedIds"`
	}
	if err := decodeRemoteRequest(writer, request, &syncRequest); err != nil {
		writeJSONError(writer, http.StatusBadRequest, "Invalid display state")
		return
	}
	state, err := decodeRemoteState(syncRequest.State)
	if err != nil {
		writeJSONError(writer, http.StatusBadRequest, "Invalid display state")
		return
	}
	for _, commandID := range syncRequest.AcknowledgedIDs {
		if commandID == "" {
			writeJSONError(writer, http.StatusBadRequest, "Invalid command acknowledgement")
			return
		}
	}
	commands, err := app.remote.Sync(id, state, syncRequest.AcknowledgedIDs)
	if errors.Is(err, errRemoteDisplayNotFound) {
		writeJSONError(writer, http.StatusNotFound, "Display is unavailable")
		return
	}
	if err != nil {
		writeJSONError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, struct {
		Commands []RemoteCommand `json:"commands"`
	}{Commands: commands})
}

func decodeRemoteState(data json.RawMessage) (RemotePlaybackState, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil || len(fields) != 7 {
		return RemotePlaybackState{}, errors.New("invalid display state")
	}
	for _, name := range []string{"episodeId", "playing", "position", "duration", "volume", "muted", "error"} {
		if _, exists := fields[name]; !exists {
			return RemotePlaybackState{}, errors.New("invalid display state")
		}
		if name != "episodeId" && name != "error" && string(fields[name]) == "null" {
			return RemotePlaybackState{}, errors.New("invalid display state")
		}
	}
	var state RemotePlaybackState
	if err := json.Unmarshal(data, &state); err != nil {
		return RemotePlaybackState{}, err
	}
	return state, nil
}

func (app *App) remoteCommandHandler(writer http.ResponseWriter, request *http.Request, id string) {
	var body json.RawMessage
	if err := decodeRemoteRequest(writer, request, &body); err != nil {
		writeJSONError(writer, http.StatusBadRequest, "Invalid remote command")
		return
	}
	payload, err := decodeRemoteCommand(body)
	if err != nil {
		writeJSONError(writer, http.StatusBadRequest, "Invalid remote command")
		return
	}
	if _, exists := app.remote.Get(id); !exists {
		writeJSONError(writer, http.StatusNotFound, "Display is unavailable")
		return
	}
	if payload.Type == "playEpisode" {
		catalog := app.catalog.Load()
		if catalog == nil || catalog.byID[*payload.EpisodeID] == nil || catalog.byID[*payload.EpisodeID].Status != "ready" {
			writeJSONError(writer, http.StatusConflict, "Episode is unavailable")
			return
		}
	}
	if err := app.remote.Enqueue(id, payload); errors.Is(err, errRemoteDisplayNotFound) {
		writeJSONError(writer, http.StatusNotFound, "Display is unavailable")
		return
	} else if err != nil {
		writeJSONError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(writer, http.StatusAccepted, map[string]bool{"queued": true})
}

func decodeRemoteCommand(data json.RawMessage) (RemoteCommandPayload, error) {
	invalid := errors.New("invalid remote command")
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return RemoteCommandPayload{}, err
	}
	var commandType string
	if err := json.Unmarshal(fields["type"], &commandType); err != nil {
		return RemoteCommandPayload{}, invalid
	}
	required := ""
	switch commandType {
	case "playEpisode":
		required = "episodeId"
	case "setPlaying":
		required = "playing"
	case "seek":
		required = "position"
	case "setMuted":
		required = "muted"
	case "setVolume":
		required = "volume"
	case "stop":
	default:
		return RemoteCommandPayload{}, invalid
	}
	expectedFields := 1
	if required != "" {
		expectedFields++
	}
	if len(fields) != expectedFields {
		return RemoteCommandPayload{}, invalid
	}
	if required != "" {
		value, exists := fields[required]
		if !exists || string(value) == "null" {
			return RemoteCommandPayload{}, invalid
		}
	}
	var payload RemoteCommandPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return RemoteCommandPayload{}, err
	}
	if err := payload.validate(); err != nil {
		return RemoteCommandPayload{}, err
	}
	return payload, nil
}

func decodeRemoteRequest(writer http.ResponseWriter, request *http.Request, value any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return errors.New("unexpected request content")
	}
	return nil
}

func validRemoteID(id string) bool {
	if len(id) == 0 || len(id) > 128 {
		return false
	}
	for _, char := range id {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-' || char == '_' {
			continue
		}
		return false
	}
	return true
}
