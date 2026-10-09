package server

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

func cachePath(cacheDir, id, fingerprint string) string {
	return filepath.Join(cacheDir, id+"-"+fingerprint+".mp4")
}

func prepareEpisode(episode *Episode, cacheDir string) error {
	return prepareEpisodeWithStatus(episode, cacheDir, nil)
}

func prepareEpisodeWithStatus(episode *Episode, cacheDir string, update func(string, int)) error {
	report := func(stage string, progress int) {
		if update != nil {
			update(stage, progress)
		}
	}
	if episode.UnavailableReason != nil {
		return fmt.Errorf("%s", *episode.UnavailableReason)
	}
	info, err := os.Lstat(episode.SourcePath)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("Source file is unavailable")
	}
	duration, compatible, hasAudio, err := probe(episode.SourcePath)
	if err != nil {
		return err
	}
	fastStartNeeded := true
	if compatible {
		fastStartNeeded, err = needsFastStart(episode.SourcePath)
	}
	if err != nil {
		return err
	}
	if !fastStartNeeded {
		episode.DurationSeconds = &duration
		episode.PlaybackPath = episode.SourcePath
		episode.Status = "ready"
		return nil
	}
	report("Checking prepared cache", 0)
	finger, err := fingerprint(episode.SourcePath)
	if err != nil {
		return fmt.Errorf("Source file is unavailable")
	}
	finalPath := cachePath(cacheDir, episode.ID, finger)
	if _, err := os.Stat(finalPath); err == nil {
		cachedDuration, cacheCompatible, _, probeErr := probe(finalPath)
		cacheNeedsFastStart, atomErr := needsFastStart(finalPath)
		if probeErr == nil && cacheCompatible && atomErr == nil && !cacheNeedsFastStart {
			episode.DurationSeconds = &cachedDuration
			episode.PlaybackPath = finalPath
			episode.Status = "ready"
			return nil
		}
		if err := os.Remove(finalPath); err != nil {
			return fmt.Errorf("Unable to replace invalid cache entry")
		}
	}
	temporary, err := os.CreateTemp(cacheDir, episode.ID+"-*.tmp")
	if err != nil {
		return fmt.Errorf("Cache is unavailable")
	}
	temporaryPath := temporary.Name()
	if err := temporary.Close(); err != nil {
		os.Remove(temporaryPath)
		return fmt.Errorf("Cache is unavailable")
	}
	defer os.Remove(temporaryPath)
	args := []string{"-v", "error", "-y", "-i", episode.SourcePath}
	if !hasAudio {
		args = append(args, "-f", "lavfi", "-i", "anullsrc=channel_layout=stereo:sample_rate=48000")
	}
	args = append(args, "-map", "0:v:0")
	if hasAudio {
		args = append(args, "-map", "0:a:0")
	} else {
		args = append(args, "-map", "1:a:0", "-shortest")
	}
	if compatible {
		args = append(args, "-c", "copy")
	} else {
		args = append(args, "-c:v", "libx264", "-c:a", "aac")
	}
	args = append(args, "-movflags", "+faststart", "-f", "mp4", temporaryPath)
	args = append([]string{"-nostats", "-progress", "pipe:1"}, args...)
	command := exec.Command("ffmpeg", args...)
	output, err := command.StdoutPipe()
	if err != nil {
		return fmt.Errorf("Unable to prepare MP4 for streaming")
	}
	var errorOutput bytes.Buffer
	command.Stderr = &errorOutput
	if err := command.Start(); err != nil {
		return fmt.Errorf("Unable to prepare MP4 for streaming")
	}
	stage := "Converting video"
	if compatible {
		stage = "Remuxing MP4"
	}
	report(stage, 0)
	scanner := bufio.NewScanner(output)
	lastProgress := 0
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "out_time_us=") || duration <= 0 {
			continue
		}
		microseconds, err := strconv.ParseInt(strings.TrimPrefix(line, "out_time_us="), 10, 64)
		if err != nil {
			continue
		}
		progress := int(microseconds * 100 / (duration * 1000000))
		if progress > 99 {
			progress = 99
		}
		if progress >= lastProgress+2 {
			lastProgress = progress
			report(stage, progress)
		}
	}
	if err := command.Wait(); err != nil || scanner.Err() != nil {
		if errorOutput.Len() > 0 {
			log.Printf("media preparation failed for episode %s", episode.ID)
		}
		return fmt.Errorf("Unable to prepare MP4 for streaming")
	}
	report("Validating playback", 99)
	cachedDuration, cacheCompatible, _, err := probe(temporaryPath)
	preparedNeedsFastStart, atomErr := needsFastStart(temporaryPath)
	if err != nil || !cacheCompatible || atomErr != nil || preparedNeedsFastStart {
		return fmt.Errorf("Prepared MP4 failed validation")
	}
	if err := os.Rename(temporaryPath, finalPath); err != nil {
		return fmt.Errorf("Unable to save prepared MP4")
	}
	episode.DurationSeconds = &cachedDuration
	episode.PlaybackPath = finalPath
	episode.Status = "ready"
	return nil
}

func removeStaleCache(cacheDir string, episodes []episodeFile) error {
	ids := make(map[string]struct{}, len(episodes))
	for _, episode := range episodes {
		ids[episode.id] = struct{}{}
	}
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return err
	}
	current := make(map[string]string, len(episodes))
	for _, episode := range episodes {
		if value, err := fingerprint(episode.path); err == nil {
			current[episode.id] = value
		}
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		for id := range ids {
			if !strings.HasPrefix(name, id+"-") {
				continue
			}
			middle, sheet := previewPaths(cacheDir, id, current[id])
			keep := current[id] != "" && (name == filepath.Base(cachePath(cacheDir, id, current[id])) || name == filepath.Base(middle) || name == filepath.Base(sheet))
			if !keep && (strings.HasSuffix(name, ".mp4") || strings.HasSuffix(name, ".tmp") || strings.HasSuffix(name, ".jpg")) {
				if err := os.Remove(filepath.Join(cacheDir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
					return err
				}
			}
		}
	}
	return nil
}

func prepareCatalog(catalog *Catalog, files []episodeFile, cacheDir string) {
	if err := removeStaleCache(cacheDir, files); err != nil {
		log.Printf("unable to inspect cache directory")
	}
	jobs := make(chan *Episode)
	var workers sync.WaitGroup
	var failures sync.Mutex
	for i := 0; i < 2; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for episode := range jobs {
				if episode.UnavailableReason != nil {
					failures.Lock()
					catalog.failures++
					failures.Unlock()
					continue
				}
				if err := preparePreview(episode, cacheDir); err != nil {
					log.Printf("episode %s preview unavailable: %s", episode.ID, err.Error())
				}
				if episode.PreviewPath != "" {
					url := "/api/episodes/" + episode.ID + "/preview"
					episode.PreviewURL = &url
				}
				if episode.PreviewSpritePath != "" {
					url := "/api/episodes/" + episode.ID + "/preview-sheet"
					episode.PreviewSpriteURL = &url
				}
				if err := prepareEpisode(episode, cacheDir); err != nil {
					reason := err.Error()
					episode.Status = "unavailable"
					episode.UnavailableReason = &reason
					failures.Lock()
					catalog.failures++
					failures.Unlock()
					log.Printf("episode %s unavailable: %s", episode.ID, reason)
				}
				if episode.Status == "ready" {
					url := "/api/episodes/" + episode.ID + "/stream"
					episode.StreamURL = &url
				}
			}
		}()
	}
	for i := range catalog.Series {
		for j := range catalog.Series[i].Seasons {
			for k := range catalog.Series[i].Seasons[j].Episodes {
				jobs <- &catalog.Series[i].Seasons[j].Episodes[k]
			}
		}
	}
	close(jobs)
	workers.Wait()
}

func Prepare(catalog *Catalog, files []episodeFile, cacheDir string) {
	prepareCatalog(catalog, files, cacheDir)
}
