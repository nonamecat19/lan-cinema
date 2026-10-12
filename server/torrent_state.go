package server

import (
	"encoding/json"
	"os"
)

func (manager *TorrentManager) loadState() {
	data, err := os.ReadFile(manager.statePath)
	if err != nil {
		return
	}
	var state struct {
		Selections map[string]TorrentSelection `json:"selections"`
		Imported   map[string]bool             `json:"imported"`
		Errors     map[string]string           `json:"importErrors"`
	}
	if json.Unmarshal(data, &state) == nil {
		if state.Selections != nil {
			manager.selections = state.Selections
		}
		if state.Imported != nil {
			manager.imported = state.Imported
		}
		if state.Errors != nil {
			manager.importErrors = state.Errors
		}
	}
}

func (manager *TorrentManager) saveStateLocked() {
	data, err := json.Marshal(struct {
		Selections map[string]TorrentSelection `json:"selections"`
		Imported   map[string]bool             `json:"imported"`
		Errors     map[string]string           `json:"importErrors"`
	}{Selections: manager.selections, Imported: manager.imported, Errors: manager.importErrors})
	if err != nil {
		return
	}
	temporary := manager.statePath + ".tmp"
	if os.WriteFile(temporary, data, 0600) == nil {
		_ = os.Rename(temporary, manager.statePath)
	}
}
