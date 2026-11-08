package server

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func seasonSet(seasons []int) map[int]bool {
	result := make(map[int]bool, len(seasons))
	for _, season := range seasons {
		result[season] = true
	}
	return result
}

func (manager *TorrentManager) reconcilePriorities(hash string, selection TorrentSelection, files []TorrentFile) error {
	selected := seasonSet(selection.Seasons)
	paused := seasonSet(selection.PausedSeasons)
	deleting := seasonSet(selection.DeletingSeasons)
	firstSeason, firstEpisode := -1, -1
	for _, file := range files {
		if !selected[file.Season] || paused[file.Season] || deleting[file.Season] || file.Progress >= 0.999999 || !supportedTorrentVideo(file.Name) {
			continue
		}
		season, episode, _, err := parseTorrentEpisode(file.Name)
		if err == nil && season == file.Season && (firstSeason < 0 || season < firstSeason || season == firstSeason && episode < firstEpisode) {
			firstSeason, firstEpisode = season, episode
		}
	}
	groups := map[int][]string{0: {}, 1: {}, 7: {}}
	for _, file := range files {
		priority := 0
		season, episode, _, err := parseTorrentEpisode(file.Name)
		if err == nil && season == file.Season && supportedTorrentVideo(file.Name) && selected[season] {
			switch {
			case deleting[season]:
				priority = 0
			case file.Progress >= 0.999999:
				priority = 1
			case paused[season]:
				priority = 0
			case selection.ForceOneByOne && season == firstSeason && episode == firstEpisode:
				priority = 7
			case selection.ForceOneByOne:
				priority = 0
			case season == firstSeason:
				priority = 7
			default:
				priority = 1
			}
		}
		if file.Priority != priority {
			groups[priority] = append(groups[priority], strconv.Itoa(file.Index))
		}
	}
	for _, priority := range []int{0, 1, 7} {
		if err := manager.setFilePriority(hash, groups[priority], priority); err != nil {
			return err
		}
	}
	return nil
}

func (manager *TorrentManager) reconcileAll() {
	manager.operationMu.Lock()
	defer manager.operationMu.Unlock()
	manager.mu.Lock()
	selections := make(map[string]TorrentSelection, len(manager.selections))
	for hash, selection := range manager.selections {
		selections[hash] = selection
	}
	manager.mu.Unlock()
	for hash, selection := range selections {
		files, err := manager.files(hash)
		if err == nil {
			_ = manager.reconcilePriorities(hash, selection, files)
		}
	}
}

func (manager *TorrentManager) linkedSelection(sourceID, title string) (TorrentSource, TorrentSelection, error) {
	source, err := manager.Source(sourceID)
	if err != nil {
		return TorrentSource{}, TorrentSelection{}, err
	}
	manager.mu.Lock()
	selection, exists := manager.selections[source.InfoHash]
	manager.mu.Unlock()
	if !exists || !strings.EqualFold(strings.TrimSpace(selection.SeriesTitle), strings.TrimSpace(title)) {
		return TorrentSource{}, TorrentSelection{}, fmt.Errorf("torrent is not linked to this series")
	}
	return source, selection, nil
}

func (manager *TorrentManager) UpdateSeason(sourceID, title string, season int, pause bool) error {
	manager.operationMu.Lock()
	defer manager.operationMu.Unlock()
	source, selection, err := manager.linkedSelection(sourceID, title)
	if err != nil {
		return err
	}
	if !seasonSet(selection.Seasons)[season] {
		return fmt.Errorf("season is not selected")
	}
	if seasonSet(selection.DeletingSeasons)[season] {
		return fmt.Errorf("season delete must be retried")
	}
	paused := seasonSet(selection.PausedSeasons)
	if pause {
		paused[season] = true
	} else {
		delete(paused, season)
	}
	selection.PausedSeasons = selection.PausedSeasons[:0]
	for number := range paused {
		selection.PausedSeasons = append(selection.PausedSeasons, number)
	}
	sort.Ints(selection.PausedSeasons)
	files, err := manager.files(source.InfoHash)
	if err != nil {
		return err
	}
	if err := manager.reconcilePriorities(source.InfoHash, selection, files); err != nil {
		return err
	}
	manager.mu.Lock()
	manager.selections[source.InfoHash] = selection
	manager.saveStateLocked()
	manager.mu.Unlock()
	return nil
}

func (manager *TorrentManager) SetForceOneByOne(sourceID, title string, enabled bool) error {
	manager.operationMu.Lock()
	defer manager.operationMu.Unlock()
	source, selection, err := manager.linkedSelection(sourceID, title)
	if err != nil {
		return err
	}
	selection.ForceOneByOne = enabled
	files, err := manager.files(source.InfoHash)
	if err != nil {
		return err
	}
	if err := manager.reconcilePriorities(source.InfoHash, selection, files); err != nil {
		return err
	}
	manager.mu.Lock()
	manager.selections[source.InfoHash] = selection
	manager.saveStateLocked()
	manager.mu.Unlock()
	return nil
}

func importTarget(mediaRoot, seriesTitle string, file TorrentFile) (string, string, error) {
	season, episode, title, err := parseTorrentEpisode(file.Name)
	if err != nil {
		return "", "", err
	}
	relative := filepath.Join(seriesTitle, fmt.Sprintf("Season %02d", season), fmt.Sprintf("S%02dE%02d - %s%s", season, episode, title, strings.ToLower(filepath.Ext(file.Name))))
	return filepath.Join(mediaRoot, relative), relative, nil
}

func removeRegularFile(root, path string) error {
	if !pathWithin(root, path) || path == root {
		return fmt.Errorf("file path is outside its configured directory")
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("file is not a regular file")
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil || !pathWithin(resolvedRoot, resolvedPath) {
		return fmt.Errorf("file path is outside its configured directory")
	}
	return os.Remove(path)
}

func (manager *TorrentManager) DeleteSeason(sourceID, title string, season int, cacheDir string, refresh func() error) error {
	manager.operationMu.Lock()
	defer manager.operationMu.Unlock()
	source, selection, err := manager.linkedSelection(sourceID, title)
	if err != nil {
		return err
	}
	if !seasonSet(selection.Seasons)[season] {
		return fmt.Errorf("season is not selected")
	}
	info, err := manager.getTorrent(source.InfoHash)
	if err != nil {
		return err
	}
	files, err := manager.files(source.InfoHash)
	if err != nil {
		return err
	}
	wasPaused := strings.Contains(info.State, "paused") || strings.Contains(info.State, "stopped")
	if _, err := manager.postForm("/api/v2/torrents/stop", url.Values{"hashes": {source.InfoHash}}); err != nil {
		return err
	}
	restore := !wasPaused && len(selection.Seasons) > 1
	defer func() {
		if restore {
			_, _ = manager.postForm("/api/v2/torrents/start", url.Values{"hashes": {source.InfoHash}})
		}
	}()
	paused := seasonSet(selection.PausedSeasons)
	paused[season] = true
	deleting := seasonSet(selection.DeletingSeasons)
	deleting[season] = true
	selection.DeletingSeasons = selection.DeletingSeasons[:0]
	for number := range deleting {
		selection.DeletingSeasons = append(selection.DeletingSeasons, number)
	}
	sort.Ints(selection.DeletingSeasons)
	selection.PausedSeasons = selection.PausedSeasons[:0]
	for number := range paused {
		selection.PausedSeasons = append(selection.PausedSeasons, number)
	}
	sort.Ints(selection.PausedSeasons)
	manager.mu.Lock()
	manager.selections[source.InfoHash] = selection
	manager.saveStateLocked()
	manager.mu.Unlock()
	seasonIDs := make([]string, 0)
	for _, file := range files {
		if file.Season == season && file.Priority != 0 {
			seasonIDs = append(seasonIDs, strconv.Itoa(file.Index))
		}
	}
	if err := manager.setFilePriority(source.InfoHash, seasonIDs, 0); err != nil {
		return err
	}
	deletedIDs := make(map[string]bool)
	for _, file := range files {
		if file.Season != season || !supportedTorrentVideo(file.Name) {
			continue
		}
		parsedSeason, _, _, parseErr := parseTorrentEpisode(file.Name)
		if parseErr != nil || parsedSeason != season {
			continue
		}
		key := source.InfoHash + ":" + strconv.Itoa(file.Index)
		manager.mu.Lock()
		imported := manager.imported[key]
		manager.mu.Unlock()
		staged, stageErr := manager.torrentFilePath(source.InfoHash, file, info)
		if imported {
			target, relative, err := importTarget(manager.mediaRoot, selection.SeriesTitle, file)
			if err != nil {
				return err
			}
			if stageErr == nil {
				stagedInfo, err := os.Stat(staged)
				if err != nil {
					return err
				}
				if _, err := existingImportMatches(staged, target, stagedInfo); err != nil {
					return err
				}
			}
			if err := removeRegularFile(manager.mediaRoot, target); err != nil {
				return err
			}
			deletedIDs[digest(relative)] = true
		}
		if stageErr == nil {
			if err := removeRegularFile(manager.downloads, staged); err != nil {
				return err
			}
		}
	}
	remaining := make([]int, 0, len(selection.Seasons))
	for _, number := range selection.Seasons {
		if number != season {
			remaining = append(remaining, number)
		}
	}
	selection.Seasons = remaining
	selection.DeletingSeasons = selection.DeletingSeasons[:0]
	for number := range deleting {
		if number != season {
			selection.DeletingSeasons = append(selection.DeletingSeasons, number)
		}
	}
	selection.PausedSeasons = selection.PausedSeasons[:0]
	for number := range paused {
		if number != season {
			selection.PausedSeasons = append(selection.PausedSeasons, number)
		}
	}
	for _, file := range files {
		if file.Season == season {
			key := source.InfoHash + ":" + strconv.Itoa(file.Index)
			manager.mu.Lock()
			delete(manager.imported, key)
			delete(manager.importErrors, key)
			delete(manager.importing, key)
			manager.mu.Unlock()
		}
	}
	manager.mu.Lock()
	if len(selection.Seasons) == 0 {
		delete(manager.selections, source.InfoHash)
	} else {
		manager.selections[source.InfoHash] = selection
	}
	manager.saveStateLocked()
	manager.mu.Unlock()
	if len(selection.Seasons) == 0 {
		if _, err := manager.postForm("/api/v2/torrents/delete", url.Values{"hashes": {source.InfoHash}, "deleteFiles": {"true"}}); err != nil {
			return err
		}
		restore = false
	} else {
		updated, err := manager.files(source.InfoHash)
		if err != nil {
			return err
		}
		if err := manager.reconcilePriorities(source.InfoHash, selection, updated); err != nil {
			return err
		}
		if !wasPaused {
			if _, err := manager.postForm("/api/v2/torrents/start", url.Values{"hashes": {source.InfoHash}}); err != nil {
				return err
			}
			restore = false
		}
	}
	if err := refresh(); err != nil {
		return err
	}
	for id := range deletedIDs {
		if err := removeEpisodeArtifacts(cacheDir, id); err != nil {
			return err
		}
	}
	return nil
}
