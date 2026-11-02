package server

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

var errRemoteDisplayNotFound = errors.New("display is unavailable")

type RemotePlaybackState struct {
	EpisodeID *string `json:"episodeId"`
	Playing   bool    `json:"playing"`
	Position  float64 `json:"position"`
	Duration  float64 `json:"duration"`
	Volume    float64 `json:"volume"`
	Muted     bool    `json:"muted"`
	Error     *string `json:"error"`
}

type RemoteCommandPayload struct {
	Type      string   `json:"type"`
	EpisodeID *string  `json:"episodeId,omitempty"`
	Playing   *bool    `json:"playing,omitempty"`
	Position  *float64 `json:"position,omitempty"`
	Muted     *bool    `json:"muted,omitempty"`
	Volume    *float64 `json:"volume,omitempty"`
}

type RemoteCommand struct {
	ID string `json:"id"`
	RemoteCommandPayload
}

type RemoteDisplay struct {
	ID    string              `json:"id"`
	Label string              `json:"label"`
	State RemotePlaybackState `json:"state"`
}

type remoteSession struct {
	display  RemoteDisplay
	lastSeen time.Time
	commands []RemoteCommand
}

type RemoteManager struct {
	mu               sync.Mutex
	displays         map[string]*remoteSession
	nextLabel        uint64
	nextCommand      uint64
	commandNamespace string
}

func NewRemoteManager() *RemoteManager {
	var namespace [16]byte
	if _, err := rand.Read(namespace[:]); err != nil {
		panic(fmt.Errorf("generate remote command namespace: %w", err))
	}
	return &RemoteManager{displays: make(map[string]*remoteSession), commandNamespace: hex.EncodeToString(namespace[:])}
}

func (manager *RemoteManager) prune(now time.Time) {
	for id, session := range manager.displays {
		if now.Sub(session.lastSeen) >= 15*time.Second {
			delete(manager.displays, id)
		}
	}
}

func (manager *RemoteManager) Register(id string) RemoteDisplay {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	now := time.Now()
	manager.prune(now)
	if session, exists := manager.displays[id]; exists {
		session.lastSeen = now
		return session.display
	}
	manager.nextLabel++
	display := RemoteDisplay{
		ID:    id,
		Label: fmt.Sprintf("Screen %d", manager.nextLabel),
		State: RemotePlaybackState{Volume: 1},
	}
	manager.displays[id] = &remoteSession{display: display, lastSeen: now}
	return display
}

func (manager *RemoteManager) List() []RemoteDisplay {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.prune(time.Now())
	displays := make([]RemoteDisplay, 0, len(manager.displays))
	for _, session := range manager.displays {
		displays = append(displays, session.display)
	}
	sort.Slice(displays, func(i, j int) bool { return displays[i].Label < displays[j].Label })
	return displays
}

func (manager *RemoteManager) Get(id string) (RemoteDisplay, bool) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.prune(time.Now())
	session, exists := manager.displays[id]
	if !exists {
		return RemoteDisplay{}, false
	}
	return session.display, true
}

func (manager *RemoteManager) Sync(id string, state RemotePlaybackState, acknowledgedIDs []string) ([]RemoteCommand, error) {
	if err := state.validate(); err != nil {
		return nil, err
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	now := time.Now()
	manager.prune(now)
	session, exists := manager.displays[id]
	if !exists {
		return nil, errRemoteDisplayNotFound
	}
	if len(acknowledgedIDs) > 0 {
		acknowledged := make(map[string]struct{}, len(acknowledgedIDs))
		for _, commandID := range acknowledgedIDs {
			acknowledged[commandID] = struct{}{}
		}
		pending := session.commands[:0]
		for _, command := range session.commands {
			if _, done := acknowledged[command.ID]; !done {
				pending = append(pending, command)
			}
		}
		session.commands = pending
	}
	session.display.State = state
	session.lastSeen = now
	return append([]RemoteCommand{}, session.commands...), nil
}

func (manager *RemoteManager) Enqueue(id string, payload RemoteCommandPayload) error {
	if err := payload.validate(); err != nil {
		return err
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.prune(time.Now())
	session, exists := manager.displays[id]
	if !exists {
		return errRemoteDisplayNotFound
	}
	manager.nextCommand++
	session.commands = append(session.commands, RemoteCommand{
		ID:                   fmt.Sprintf("command-%s-%d", manager.commandNamespace, manager.nextCommand),
		RemoteCommandPayload: payload,
	})
	return nil
}

func (state RemotePlaybackState) validate() error {
	if !finiteRemoteNumber(state.Position) || state.Position < 0 || !finiteRemoteNumber(state.Duration) || state.Duration < 0 || !finiteRemoteNumber(state.Volume) || state.Volume < 0 || state.Volume > 1 {
		return errors.New("invalid playback state")
	}
	return nil
}

func (payload RemoteCommandPayload) validate() error {
	valid := false
	switch payload.Type {
	case "playEpisode":
		valid = payload.EpisodeID != nil && *payload.EpisodeID != "" && payload.Playing == nil && payload.Position == nil && payload.Muted == nil && payload.Volume == nil
	case "setPlaying":
		valid = payload.EpisodeID == nil && payload.Playing != nil && payload.Position == nil && payload.Muted == nil && payload.Volume == nil
	case "seek":
		valid = payload.EpisodeID == nil && payload.Playing == nil && payload.Position != nil && finiteRemoteNumber(*payload.Position) && *payload.Position >= 0 && payload.Muted == nil && payload.Volume == nil
	case "setMuted":
		valid = payload.EpisodeID == nil && payload.Playing == nil && payload.Position == nil && payload.Muted != nil && payload.Volume == nil
	case "setVolume":
		valid = payload.EpisodeID == nil && payload.Playing == nil && payload.Position == nil && payload.Muted == nil && payload.Volume != nil && finiteRemoteNumber(*payload.Volume) && *payload.Volume >= 0 && *payload.Volume <= 1
	case "stop":
		valid = payload.EpisodeID == nil && payload.Playing == nil && payload.Position == nil && payload.Muted == nil && payload.Volume == nil
	}
	if !valid {
		return errors.New("invalid remote command")
	}
	return nil
}

func finiteRemoteNumber(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
