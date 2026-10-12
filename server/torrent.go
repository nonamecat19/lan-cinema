package server

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type TorrentSource struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	InfoHash    string `json:"infoHash"`
	path        string
}

type TorrentFile struct {
	Index    int     `json:"index"`
	Name     string  `json:"name"`
	Size     int64   `json:"size"`
	Progress float64 `json:"progress"`
	Priority int     `json:"priority"`
	Season   int     `json:"season"`
}

type TorrentSelection struct {
	SeriesTitle string        `json:"seriesTitle"`
	Seasons     []int         `json:"seasons"`
	Files       []TorrentFile `json:"files"`
}

type TorrentSeasonOption struct {
	Number   int `json:"number"`
	Episodes int `json:"episodes"`
}

type TorrentSeriesSource struct {
	ID               string                `json:"id"`
	Name             string                `json:"name"`
	AvailableSeasons []TorrentSeasonOption `json:"availableSeasons"`
}

type TorrentDownload struct {
	Hash      string                `json:"hash"`
	Name      string                `json:"name"`
	Progress  float64               `json:"progress"`
	State     string                `json:"state"`
	Download  int64                 `json:"downloadSpeed"`
	Upload    int64                 `json:"uploadSpeed"`
	ETA       int64                 `json:"eta"`
	Size      int64                 `json:"size"`
	Completed int64                 `json:"completed"`
	Paused    bool                  `json:"paused"`
	Files     []TorrentDownloadFile `json:"files"`
	Detail    string                `json:"detail,omitempty"`
}

type TorrentDownloadFile struct {
	Name                string  `json:"name"`
	Season              int     `json:"season"`
	Episode             int     `json:"episode"`
	Progress            float64 `json:"progress"`
	Status              string  `json:"status"`
	Detail              string  `json:"detail"`
	PreparationProgress int     `json:"preparationProgress,omitempty"`
}

type TorrentManager struct {
	root         string
	extraRoot    string
	uploadRoot   string
	mediaRoot    string
	downloads    string
	client       *http.Client
	baseURL      string
	statePath    string
	mu           sync.Mutex
	selections   map[string]TorrentSelection
	imported     map[string]bool
	importErrors map[string]string
	importing    map[string]int
	linkSource   func(string, string) error
}

type qbitFile struct {
	Index    int     `json:"index"`
	Name     string  `json:"name"`
	Size     int64   `json:"size"`
	Progress float64 `json:"progress"`
	Priority int     `json:"priority"`
}

type qbitHTTPError struct {
	status int
}

func (err qbitHTTPError) Error() string {
	return fmt.Sprintf("qBittorrent request failed (HTTP %d)", err.status)
}

type qbitInfo struct {
	Hash         string  `json:"hash"`
	Name         string  `json:"name"`
	Progress     float64 `json:"progress"`
	State        string  `json:"state"`
	DL           int64   `json:"dlspeed"`
	UL           int64   `json:"upspeed"`
	ETA          int64   `json:"eta"`
	Size         int64   `json:"size"`
	Downloaded   int64   `json:"downloaded"`
	DownloadPath string  `json:"download_path"`
	SavePath     string  `json:"save_path"`
	ContentPath  string  `json:"content_path"`
}

var torrentEpisodePattern = regexp.MustCompile(`(?i)S([0-9]{1,3})E([0-9]{1,3})(?:[^A-Za-z0-9]+(.*))?$`)
var torrentSeasonPattern = regexp.MustCompile(`(?i)^(?:Season\s*|S)([0-9]{1,3})$`)
var torrentNoisePattern = regexp.MustCompile(`(?i)(?:web[-_. ]?dlrip|bdrip|brrip|webrip|hdtv|1080p|720p|2160p|x264|x265|h264|h265|aac|hevc|proper|repack|eng|ukr|rus|mkv|mp4)`)

func NewTorrentManager(root, extraRoot, mediaRoot, downloads, baseURL, statePath string) *TorrentManager {
	manager := &TorrentManager{root: root, extraRoot: extraRoot, uploadRoot: filepath.Join(filepath.Dir(statePath), "torrent-uploads"), mediaRoot: mediaRoot, downloads: downloads, baseURL: strings.TrimRight(baseURL, "/"), statePath: statePath, client: &http.Client{Timeout: 20 * time.Second}, selections: make(map[string]TorrentSelection), imported: make(map[string]bool), importErrors: make(map[string]string), importing: make(map[string]int)}
	_ = os.MkdirAll(manager.uploadRoot, 0755)
	manager.loadState()
	return manager
}

func (manager *TorrentManager) Sources() ([]TorrentSource, error) {
	sources := make([]TorrentSource, 0)
	seenHashes := make(map[string]struct{})
	directories := []string{manager.root, manager.extraRoot, manager.uploadRoot}
	for _, directory := range directories {
		if directory == "" {
			continue
		}
		entries, err := os.ReadDir(directory)
		if err != nil {
			if directory != manager.root && errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.EqualFold(filepath.Ext(entry.Name()), ".torrent") {
				continue
			}
			path := filepath.Join(directory, entry.Name())
			info, err := entry.Info()
			if err != nil || info.Size() <= 0 || info.Size() > 8<<20 {
				continue
			}
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			name, infoHash, err := torrentInfo(data)
			if err != nil {
				continue
			}
			if _, exists := seenHashes[infoHash]; exists {
				continue
			}
			seenHashes[infoHash] = struct{}{}
			fingerprint := sha256.Sum256([]byte(entry.Name() + "\x00" + infoHash))
			sources = append(sources, TorrentSource{ID: hex.EncodeToString(fingerprint[:]), Name: entry.Name(), DisplayName: name, InfoHash: infoHash, path: path})
		}
	}
	sort.Slice(sources, func(i, j int) bool { return strings.ToLower(sources[i].Name) < strings.ToLower(sources[j].Name) })
	return sources, nil
}

func (manager *TorrentManager) Upload(name string, data []byte) (TorrentSource, error) {
	if !strings.EqualFold(filepath.Ext(name), ".torrent") {
		return TorrentSource{}, fmt.Errorf("select a .torrent file")
	}
	if len(data) == 0 || len(data) > 8<<20 {
		return TorrentSource{}, fmt.Errorf("torrent file must be between 1 byte and 8 MB")
	}
	displayName, infoHash, err := torrentInfo(data)
	if err != nil {
		return TorrentSource{}, fmt.Errorf("invalid torrent file")
	}
	if sources, err := manager.Sources(); err == nil {
		for _, source := range sources {
			if source.InfoHash == infoHash {
				return source, nil
			}
		}
	}
	baseName := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	baseName = regexp.MustCompile(`[^A-Za-z0-9._ -]+`).ReplaceAllString(baseName, "_")
	baseName = strings.Trim(baseName, " ._")
	if len(baseName) > 120 {
		baseName = baseName[:120]
	}
	if baseName == "" {
		baseName = "torrent"
	}
	fileName := baseName + "-" + infoHash[:8] + ".torrent"
	path := filepath.Join(manager.uploadRoot, fileName)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return TorrentSource{}, err
	}
	fingerprint := sha256.Sum256([]byte(fileName + "\x00" + infoHash))
	return TorrentSource{ID: hex.EncodeToString(fingerprint[:]), Name: fileName, DisplayName: displayName, InfoHash: infoHash, path: path}, nil
}

func (manager *TorrentManager) Source(id string) (TorrentSource, error) {
	sources, err := manager.Sources()
	if err != nil {
		return TorrentSource{}, err
	}
	for _, source := range sources {
		if source.ID == id {
			return source, nil
		}
	}
	return TorrentSource{}, os.ErrNotExist
}

func (manager *TorrentManager) Inspect(id string) (TorrentSelection, error) {
	source, err := manager.Source(id)
	if err != nil {
		return TorrentSelection{}, err
	}
	if err := manager.add(source); err != nil {
		return TorrentSelection{}, fmt.Errorf("add torrent to qBittorrent: %w", err)
	}
	files, err := manager.files(source.InfoHash)
	if err != nil {
		return TorrentSelection{}, fmt.Errorf("read torrent file list from qBittorrent: %w", err)
	}
	selection := TorrentSelection{SeriesTitle: inferSeriesTitle(files, source.DisplayName), Files: files}
	seasonSet := make(map[int]struct{})
	for _, file := range files {
		if file.Season > 0 {
			seasonSet[file.Season] = struct{}{}
		}
	}
	for season := range seasonSet {
		selection.Seasons = append(selection.Seasons, season)
	}
	sort.Ints(selection.Seasons)
	return selection, nil
}

func (manager *TorrentManager) Start(id string, selection TorrentSelection) error {
	source, err := manager.Source(id)
	if err != nil {
		return err
	}
	if err := manager.add(source); err != nil {
		return err
	}
	files, err := manager.files(source.InfoHash)
	if err != nil {
		return err
	}
	seasons := make(map[int]bool, len(selection.Seasons))
	for _, season := range selection.Seasons {
		if season < 1 || season > 999 {
			return fmt.Errorf("invalid season selection")
		}
		seasons[season] = true
	}
	seriesTitle := cleanSeriesTitle(selection.SeriesTitle)
	if seriesTitle == "" {
		seriesTitle = inferSeriesTitle(files, source.DisplayName)
	}
	if seriesTitle == "" {
		return fmt.Errorf("series title is required")
	}
	selected := make([]int, 0)
	for _, file := range files {
		season, _, _, err := parseTorrentEpisode(file.Name)
		if err == nil && season == file.Season && seasons[season] && supportedTorrentVideo(file.Name) {
			selected = append(selected, file.Index)
		}
	}
	if len(selected) == 0 {
		return fmt.Errorf("select at least one season with video files")
	}
	if _, err := manager.postForm("/api/v2/torrents/stop", url.Values{"hashes": {source.InfoHash}}); err != nil {
		return err
	}
	if err := manager.filePriority(source.InfoHash, files, nil); err != nil {
		return err
	}
	if err := manager.filePriority(source.InfoHash, files, selected); err != nil {
		return err
	}
	if _, err := manager.postForm("/api/v2/torrents/start", url.Values{"hashes": {source.InfoHash}}); err != nil {
		return err
	}
	manager.mu.Lock()
	manager.selections[source.InfoHash] = TorrentSelection{SeriesTitle: seriesTitle, Seasons: selection.Seasons, Files: files}
	manager.saveStateLocked()
	manager.mu.Unlock()
	return nil
}

func (manager *TorrentManager) AvailableForSeries(title string) []TorrentSeriesSource {
	sources, err := manager.Sources()
	if err != nil {
		return nil
	}
	manager.mu.Lock()
	selections := make(map[string]TorrentSelection, len(manager.selections))
	for hash, selection := range manager.selections {
		selections[hash] = selection
	}
	manager.mu.Unlock()
	result := make([]TorrentSeriesSource, 0)
	for _, source := range sources {
		selection, exists := selections[source.InfoHash]
		if !exists || !strings.EqualFold(strings.TrimSpace(selection.SeriesTitle), strings.TrimSpace(title)) {
			continue
		}
		selected := make(map[int]bool)
		for _, season := range selection.Seasons {
			selected[season] = true
		}
		counts := make(map[int]int)
		for _, file := range selection.Files {
			if selected[file.Season] || !supportedTorrentVideo(file.Name) {
				continue
			}
			if season, _, _, err := parseTorrentEpisode(file.Name); err == nil && season == file.Season {
				counts[season]++
			}
		}
		item := TorrentSeriesSource{ID: source.ID, Name: source.DisplayName, AvailableSeasons: make([]TorrentSeasonOption, 0, len(counts))}
		for season, count := range counts {
			item.AvailableSeasons = append(item.AvailableSeasons, TorrentSeasonOption{Number: season, Episodes: count})
		}
		sort.Slice(item.AvailableSeasons, func(i, j int) bool { return item.AvailableSeasons[i].Number < item.AvailableSeasons[j].Number })
		result = append(result, item)
	}
	return result
}

func (manager *TorrentManager) AddSeasons(sourceID, seriesTitle string, seasons []int) error {
	source, err := manager.Source(sourceID)
	if err != nil {
		return err
	}
	manager.mu.Lock()
	selection, exists := manager.selections[source.InfoHash]
	manager.mu.Unlock()
	if !exists || !strings.EqualFold(strings.TrimSpace(selection.SeriesTitle), strings.TrimSpace(seriesTitle)) {
		return fmt.Errorf("torrent is not linked to this series")
	}
	requested := make(map[int]bool)
	for _, season := range seasons {
		if season < 1 || season > 999 {
			return fmt.Errorf("invalid season selection")
		}
		requested[season] = true
	}
	if len(requested) == 0 {
		return fmt.Errorf("select at least one season")
	}
	files, err := manager.files(source.InfoHash)
	if err != nil {
		return err
	}
	indices := make([]int, 0)
	available := make(map[int]bool)
	for _, file := range files {
		if !requested[file.Season] || !supportedTorrentVideo(file.Name) {
			continue
		}
		if season, _, _, err := parseTorrentEpisode(file.Name); err == nil && season == file.Season {
			indices = append(indices, file.Index)
			available[season] = true
		}
	}
	for season := range requested {
		if !available[season] {
			return fmt.Errorf("season %d is not available in this torrent", season)
		}
	}
	if err := manager.filePriority(source.InfoHash, files, indices); err != nil {
		return err
	}
	if _, err := manager.postForm("/api/v2/torrents/start", url.Values{"hashes": {source.InfoHash}}); err != nil {
		return err
	}
	manager.mu.Lock()
	for season := range requested {
		found := false
		for _, existing := range selection.Seasons {
			if existing == season {
				found = true
				break
			}
		}
		if !found {
			selection.Seasons = append(selection.Seasons, season)
		}
	}
	sort.Ints(selection.Seasons)
	selection.Files = files
	manager.selections[source.InfoHash] = selection
	manager.saveStateLocked()
	manager.mu.Unlock()
	return nil
}

func (manager *TorrentManager) Downloads(catalog *Catalog) ([]TorrentDownload, error) {
	var infos []qbitInfo
	if err := manager.getJSON("/api/v2/torrents/info", &infos); err != nil {
		return nil, err
	}
	result := make([]TorrentDownload, 0, len(infos))
	for _, info := range infos {
		download := TorrentDownload{Hash: info.Hash, Name: info.Name, Progress: info.Progress, State: info.State, Download: info.DL, Upload: info.UL, ETA: info.ETA, Size: info.Size, Completed: info.Downloaded, Paused: strings.Contains(info.State, "paused") || strings.Contains(info.State, "stopped"), Files: make([]TorrentDownloadFile, 0)}
		files, err := manager.files(info.Hash)
		if err != nil {
			download.Detail = "File status is temporarily unavailable"
			result = append(result, download)
			continue
		}
		manager.mu.Lock()
		selection := manager.selections[info.Hash]
		imported := make(map[string]bool, len(manager.imported))
		for key, value := range manager.imported {
			if strings.HasPrefix(key, info.Hash+":") {
				imported[key] = value
			}
		}
		importErrors := make(map[string]string, len(manager.importErrors))
		for key, value := range manager.importErrors {
			if strings.HasPrefix(key, info.Hash+":") {
				importErrors[key] = value
			}
		}
		importing := make(map[string]int, len(manager.importing))
		for key, value := range manager.importing {
			if strings.HasPrefix(key, info.Hash+":") {
				importing[key] = value
			}
		}
		manager.mu.Unlock()
		for _, file := range files {
			if file.Priority == 0 || !supportedTorrentVideo(file.Name) {
				continue
			}
			season, episodeNumber, _, err := parseTorrentEpisode(file.Name)
			if err != nil || season != file.Season {
				continue
			}
			item := TorrentDownloadFile{Name: filepath.Base(file.Name), Season: season, Episode: episodeNumber, Progress: file.Progress, Status: "downloading", Detail: "Downloading from peers"}
			key := info.Hash + ":" + strconv.Itoa(file.Index)
			if download.Paused && file.Progress < 0.999999 {
				item.Status, item.Detail = "paused", "Download paused"
			} else if file.Progress == 0 {
				item.Status, item.Detail = "queued", "Waiting for peers"
			} else if file.Progress >= 0.999999 {
				item.Status, item.Detail = "queued", "Waiting to import into the library"
				if reason := importErrors[key]; reason != "" {
					item.Status, item.Detail = "import-error", reason
				} else if progress, active := importing[key]; active {
					item.Status, item.Detail = "importing", "Importing into the library"
					if progress > 0 {
						item.Detail = fmt.Sprintf("Copying into the library · %d%%", progress)
					}
				} else if imported[key] {
					item.Status, item.Detail = "queued", "Waiting for media preparation"
					if found := findCatalogEpisode(catalog, selection.SeriesTitle, season, episodeNumber); found != nil {
						item.Status = found.Status
						item.Detail = found.PreparationStage
						item.PreparationProgress = found.PreparationProgress
						if found.Status == "unavailable" {
							item.Detail = dereference(found.UnavailableReason, "Media preparation failed")
						}
					}
				}
			}
			download.Files = append(download.Files, item)
		}
		sort.Slice(download.Files, func(i, j int) bool {
			if download.Files[i].Season != download.Files[j].Season {
				return download.Files[i].Season < download.Files[j].Season
			}
			return download.Files[i].Episode < download.Files[j].Episode
		})
		result = append(result, download)
	}
	sort.Slice(result, func(i, j int) bool { return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name) })
	return result, nil
}

func findCatalogEpisode(catalog *Catalog, title string, season, number int) *Episode {
	if catalog == nil {
		return nil
	}
	for i := range catalog.Series {
		series := &catalog.Series[i]
		if !strings.EqualFold(series.Title, title) {
			continue
		}
		for j := range series.Seasons {
			if series.Seasons[j].Number != season {
				continue
			}
			for k := range series.Seasons[j].Episodes {
				if series.Seasons[j].Episodes[k].Number == number {
					return &series.Seasons[j].Episodes[k]
				}
			}
		}
	}
	return nil
}

func (manager *TorrentManager) Pause(hash string, pause bool) error {
	if !validInfoHash(hash) {
		return os.ErrNotExist
	}
	endpoint := "/api/v2/torrents/start"
	if pause {
		endpoint = "/api/v2/torrents/stop"
	}
	_, err := manager.postForm(endpoint, url.Values{"hashes": {hash}})
	return err
}

func (manager *TorrentManager) Delete(hash string) error {
	if !validInfoHash(hash) {
		return os.ErrNotExist
	}
	sources, _ := manager.Sources()
	if _, err := manager.postForm("/api/v2/torrents/delete", url.Values{"hashes": {hash}, "deleteFiles": {"true"}}); err != nil {
		return err
	}
	for _, source := range sources {
		if source.InfoHash == hash && pathWithin(manager.uploadRoot, source.path) {
			_ = os.Remove(source.path)
		}
	}
	manager.mu.Lock()
	delete(manager.selections, hash)
	for key := range manager.importErrors {
		if strings.HasPrefix(key, hash+":") {
			delete(manager.importErrors, key)
		}
	}
	for key := range manager.importing {
		if strings.HasPrefix(key, hash+":") {
			delete(manager.importing, key)
		}
	}
	for key := range manager.imported {
		if strings.HasPrefix(key, hash+":") {
			delete(manager.imported, key)
		}
	}
	manager.saveStateLocked()
	manager.mu.Unlock()
	return nil
}

func (manager *TorrentManager) StartWatcher(refresh func() error) {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		pendingRefresh := false
		for range ticker.C {
			if manager.importCompleted() {
				pendingRefresh = true
			}
			if pendingRefresh && refresh() == nil {
				pendingRefresh = false
			}
		}
	}()
}

func (manager *TorrentManager) importCompleted() bool {
	manager.mu.Lock()
	selections := make(map[string]TorrentSelection, len(manager.selections))
	for hash, selection := range manager.selections {
		selections[hash] = selection
	}
	manager.mu.Unlock()
	changed := false
	for hash, selection := range selections {
		info, err := manager.getTorrent(hash)
		if err != nil {
			continue
		}
		files, err := manager.files(hash)
		if err != nil {
			continue
		}
		for _, file := range files {
			key := hash + ":" + strconv.Itoa(file.Index)
			manager.mu.Lock()
			wasImported := manager.imported[key]
			manager.mu.Unlock()
			season, _, _, parseErr := parseTorrentEpisode(file.Name)
			if wasImported || file.Progress < 0.999999 || file.Priority == 0 || !supportedTorrentVideo(file.Name) || parseErr != nil || season != file.Season {
				continue
			}
			manager.mu.Lock()
			manager.importing[key] = 0
			delete(manager.importErrors, key)
			manager.saveStateLocked()
			manager.mu.Unlock()
			source, err := manager.torrentFilePath(hash, file, info)
			if err != nil {
				manager.finishImport(key, "Completed file is unavailable in the download directory")
				continue
			}
			if err := manager.importFileFromPathWithProgress(file, selection.SeriesTitle, source, func(progress int) {
				manager.mu.Lock()
				manager.importing[key] = progress
				manager.mu.Unlock()
			}); err != nil {
				manager.finishImport(key, importFailureDetail(err))
				continue
			}
			manager.mu.Lock()
			manager.imported[key] = true
			delete(manager.importing, key)
			delete(manager.importErrors, key)
			manager.saveStateLocked()
			manager.mu.Unlock()
			changed = true
		}
	}
	return changed
}

func (manager *TorrentManager) finishImport(key, detail string) {
	manager.mu.Lock()
	delete(manager.importing, key)
	manager.importErrors[key] = detail
	manager.saveStateLocked()
	manager.mu.Unlock()
}

func importFailureDetail(err error) string {
	switch {
	case errors.Is(err, syscall.ENOSPC):
		return "Not enough disk space to import the completed file"
	case errors.Is(err, os.ErrPermission):
		return "Media library is not writable"
	case errors.Is(err, os.ErrNotExist):
		return "Completed file disappeared during import"
	case strings.Contains(err.Error(), "torrent file path is invalid"):
		return "Torrent file is outside the download directory"
	case strings.Contains(err.Error(), "existing library episode"):
		return "Existing library episode differs from the completed file"
	default:
		return "Could not import the completed file"
	}
}

func (manager *TorrentManager) importFile(hash string, file TorrentFile, seriesTitle string) error {
	source := filepath.Join(manager.downloads, hash, filepath.FromSlash(file.Name))
	return manager.importFileFromPath(file, seriesTitle, source)
}

func (manager *TorrentManager) torrentFilePath(hash string, file TorrentFile, info qbitInfo) (string, error) {
	name := filepath.FromSlash(file.Name)
	candidates := []string{
		filepath.Join(info.DownloadPath, name),
		filepath.Join(info.SavePath, name),
		filepath.Join(manager.downloads, hash, name),
	}
	if info.ContentPath != "" {
		candidates = append(candidates, info.ContentPath, filepath.Join(info.ContentPath, name))
		contentName := filepath.Base(info.ContentPath)
		prefix := contentName + string(filepath.Separator)
		if strings.HasPrefix(name, prefix) {
			candidates = append(candidates, filepath.Join(info.ContentPath, strings.TrimPrefix(name, prefix)))
		}
	}
	resolvedRoot, err := filepath.EvalSymlinks(manager.downloads)
	if err != nil {
		return "", err
	}
	for _, candidate := range candidates {
		if candidate == "" || !pathWithin(manager.downloads, candidate) {
			continue
		}
		resolvedSource, err := filepath.EvalSymlinks(candidate)
		if err != nil || !pathWithin(resolvedRoot, resolvedSource) {
			continue
		}
		fileInfo, err := os.Stat(resolvedSource)
		if err == nil && fileInfo.Mode().IsRegular() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("torrent file is unavailable in its download paths")
}

func (manager *TorrentManager) importFileFromPath(file TorrentFile, seriesTitle, source string) error {
	return manager.importFileFromPathWithProgress(file, seriesTitle, source, nil)
}

func (manager *TorrentManager) importFileFromPathWithProgress(file TorrentFile, seriesTitle, source string, progress func(int)) error {
	season, episode, title, err := parseTorrentEpisode(file.Name)
	if err != nil {
		return err
	}
	relative := filepath.Join(seriesTitle, fmt.Sprintf("Season %02d", season), fmt.Sprintf("S%02dE%02d - %s%s", season, episode, title, strings.ToLower(filepath.Ext(file.Name))))
	target := filepath.Join(manager.mediaRoot, relative)
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	if !pathWithin(manager.downloads, source) {
		return fmt.Errorf("torrent file path is invalid")
	}
	resolvedRoot, err := filepath.EvalSymlinks(manager.downloads)
	if err != nil {
		return err
	}
	resolvedSource, err := filepath.EvalSymlinks(source)
	if err != nil || !pathWithin(resolvedRoot, resolvedSource) {
		return fmt.Errorf("torrent file path is invalid")
	}
	info, err := os.Stat(resolvedSource)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("torrent file is not regular")
	}
	if existing, err := os.Stat(target); err == nil {
		if existing.Mode().IsRegular() && existing.Size() == info.Size() {
			return nil
		}
		return fmt.Errorf("existing library episode differs from completed file")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	linkSource := manager.linkSource
	if linkSource == nil {
		linkSource = os.Link
	}
	if err := linkSource(resolvedSource, target); err == nil {
		return nil
	}
	input, err := os.Open(resolvedSource)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.CreateTemp(filepath.Dir(target), ".import-*")
	if err != nil {
		return err
	}
	defer os.Remove(output.Name())
	if err := output.Chmod(0644); err != nil {
		output.Close()
		return err
	}
	copyTarget := io.Writer(output)
	if progress != nil && info.Size() > 0 {
		copyTarget = &importProgressWriter{writer: output, size: info.Size(), update: progress}
	}
	_, copyErr := io.Copy(copyTarget, input)
	closeErr := output.Close()
	if copyErr != nil || closeErr != nil {
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	}
	if err := os.Link(output.Name(), target); err != nil {
		if errors.Is(err, os.ErrExist) {
			existing, statErr := os.Stat(target)
			if statErr == nil && existing.Mode().IsRegular() && existing.Size() == info.Size() {
				return nil
			}
			return fmt.Errorf("existing library episode differs from completed file")
		}
		return err
	}
	return nil
}

type importProgressWriter struct {
	writer  io.Writer
	size    int64
	written int64
	last    int
	update  func(int)
}

func (writer *importProgressWriter) Write(data []byte) (int, error) {
	n, err := writer.writer.Write(data)
	writer.written += int64(n)
	percent := int(writer.written * 100 / writer.size)
	if percent > 100 {
		percent = 100
	}
	if percent != writer.last {
		writer.last = percent
		writer.update(percent)
	}
	return n, err
}
