package server

import (
	"bufio"
	"encoding/hex"
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
	return filepath.Join(cacheDir, id+"-v3-"+fingerprint+".mp4")
}

func prepareEpisode(episode *Episode, cacheDir string) error {
	return prepareEpisodeWithStatus(episode, cacheDir, nil)
}

type preparationPlan struct {
	media     mediaInfo
	finalPath string
}

func inspectEpisode(episode *Episode, cacheDir string) (preparationPlan, bool, error) {
	if episode.UnavailableReason != nil {
		return preparationPlan{}, false, fmt.Errorf("%s", *episode.UnavailableReason)
	}
	info, err := os.Lstat(episode.SourcePath)
	if err != nil || !info.Mode().IsRegular() {
		return preparationPlan{}, false, fmt.Errorf("Source file is unavailable")
	}
	media, err := probe(episode.SourcePath)
	if err != nil {
		return preparationPlan{}, false, err
	}
	if media.H264 && media.AAC && media.MP4 {
		fastStartNeeded, err := needsFastStart(episode.SourcePath)
		if err != nil {
			return preparationPlan{}, false, err
		}
		if !fastStartNeeded {
			episode.DurationSeconds = &media.Duration
			episode.PlaybackPath = episode.SourcePath
			episode.Status = "ready"
			return preparationPlan{}, true, nil
		}
	}
	finger, err := fingerprint(episode.SourcePath)
	if err != nil {
		return preparationPlan{}, false, fmt.Errorf("Source file is unavailable")
	}
	finalPath := cachePath(cacheDir, episode.ID, finger)
	if _, err := os.Stat(finalPath); err == nil {
		cached, probeErr := probe(finalPath)
		if probeErr == nil && cached.H264 && cached.AAC && cached.MP4 {
			cacheNeedsFastStart, atomErr := needsFastStart(finalPath)
			if atomErr == nil && !cacheNeedsFastStart {
				episode.DurationSeconds = &cached.Duration
				episode.PlaybackPath = finalPath
				episode.Status = "ready"
				return preparationPlan{}, true, nil
			}
		}
		if err := os.Remove(finalPath); err != nil {
			return preparationPlan{}, false, fmt.Errorf("Unable to replace invalid cache entry")
		}
	}
	return preparationPlan{media: media, finalPath: finalPath}, false, nil
}

func prepareEpisodeWithStatus(episode *Episode, cacheDir string, update func(string, int)) error {
	plan, ready, err := inspectEpisode(episode, cacheDir)
	if err != nil || ready {
		return err
	}
	return convertEpisode(episode, cacheDir, plan, update)
}

func convertEpisode(episode *Episode, cacheDir string, plan preparationPlan, update func(string, int)) error {
	report := func(stage string, progress int) {
		if update != nil {
			update(stage, progress)
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
	device := ""
	if !plan.media.H264 {
		device = vaapiDevice()
	}
	for {
		stage := "Converting video"
		if plan.media.H264 {
			stage = "Remuxing MP4"
		}
		mode := "software"
		if device != "" {
			mode = "vaapi"
		}
		stderr, conversionErr := runConversion(conversionArgs(episode, plan.media, temporaryPath, device), plan.media.Duration, stage, report)
		if conversionErr == nil {
			report("Validating playback", 99)
			cached, probeErr := probe(temporaryPath)
			fastStartNeeded, atomErr := needsFastStart(temporaryPath)
			if probeErr == nil && cached.H264 && cached.AAC && atomErr == nil && !fastStartNeeded {
				if err := os.Rename(temporaryPath, plan.finalPath); err != nil {
					return fmt.Errorf("Unable to save prepared MP4")
				}
				episode.DurationSeconds = &cached.Duration
				episode.PlaybackPath = plan.finalPath
				episode.Status = "ready"
				return nil
			}
			validationDetail := "output validation failed"
			switch {
			case probeErr != nil:
				validationDetail = "output probe failed: " + probeErr.Error()
			case !cached.H264:
				validationDetail = "output video codec is not H.264"
			case !cached.AAC:
				validationDetail = "output audio codec is not AAC"
			case atomErr != nil:
				validationDetail = "fast-start validation failed: " + atomErr.Error()
			case fastStartNeeded:
				validationDetail = "output is missing fast-start metadata"
			}
			log.Printf("ffmpeg output validation failed episode=%s mode=%s detail=%q", episode.ID, mode, ffmpegDiagnostic(validationDetail, episode.SourcePath, temporaryPath))
		} else {
			diagnostic := ffmpegDiagnostic(stderr, episode.SourcePath, temporaryPath)
			if diagnostic == "" {
				diagnostic = ffmpegDiagnostic(conversionErr.Error(), episode.SourcePath, temporaryPath)
			}
			log.Printf("ffmpeg conversion failed episode=%s mode=%s detail=%q", episode.ID, mode, diagnostic)
		}
		if device == "" {
			return fmt.Errorf("Unable to prepare browser-compatible MP4")
		}
		log.Printf("hardware encoding failed for episode %s; retrying in software", episode.ID)
		device = ""
	}
}

func vaapiDevice() string {
	if os.Getenv("VIDEO_ACCELERATION") == "software" {
		return ""
	}
	devices, _ := filepath.Glob("/dev/dri/renderD*")
	for _, device := range devices {
		info, err := os.Stat(device)
		if err == nil && info.Mode()&os.ModeDevice != 0 {
			return device
		}
	}
	return ""
}

func conversionArgs(episode *Episode, media mediaInfo, temporaryPath, device string) []string {
	args := []string{"-nostats", "-progress", "pipe:1", "-v", "error", "-y"}
	if device != "" {
		args = append(args, "-vaapi_device", device)
	}
	args = append(args, "-i", episode.SourcePath)
	if !media.HasAudio {
		args = append(args, "-f", "lavfi", "-i", "anullsrc=channel_layout=stereo:sample_rate=48000")
	}
	args = append(args, "-map", "0:v:0")
	if media.HasAudio {
		args = append(args, "-map", "0:a:0")
	} else {
		args = append(args, "-map", "1:a:0", "-shortest")
	}
	if media.H264 {
		args = append(args, "-c:v", "copy")
	} else if device != "" {
		args = append(args, "-vf", "format=nv12,hwupload", "-c:v", "h264_vaapi", "-qp", "23")
	} else {
		args = append(args, "-pix_fmt", "yuv420p", "-c:v", "libx264", "-preset", "veryfast")
	}
	audioCodec := "aac"
	if media.AAC {
		audioCodec = "copy"
	}
	args = append(args, "-c:a", audioCodec)
	args = append(args, "-movflags", "+faststart", "-f", "mp4", temporaryPath)
	return args
}

func runConversion(args []string, duration int64, stage string, report func(string, int)) (string, error) {
	command := exec.Command("ffmpeg", args...)
	output, err := command.StdoutPipe()
	if err != nil {
		return "", err
	}
	errorOutput := &ffmpegErrorBuffer{}
	command.Stderr = errorOutput
	if err := command.Start(); err != nil {
		return errorOutput.String(), err
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
	if err := command.Wait(); err != nil {
		return errorOutput.String(), err
	}
	if err := scanner.Err(); err != nil {
		return errorOutput.String(), err
	}
	return errorOutput.String(), nil
}

type ffmpegErrorBuffer struct {
	data []byte
}

func (buffer *ffmpegErrorBuffer) Write(data []byte) (int, error) {
	const limit = 2048
	written := len(data)
	if len(data) >= limit {
		buffer.data = append(buffer.data[:0], data[len(data)-limit:]...)
		return written, nil
	}
	overflow := len(buffer.data) + len(data) - limit
	if overflow > 0 {
		copy(buffer.data, buffer.data[overflow:])
		buffer.data = buffer.data[:len(buffer.data)-overflow]
	}
	buffer.data = append(buffer.data, data...)
	return written, nil
}

func (buffer *ffmpegErrorBuffer) String() string {
	return string(buffer.data)
}

func ffmpegDiagnostic(output string, paths ...string) string {
	for _, path := range paths {
		if path != "" {
			output = strings.ReplaceAll(output, path, "<file>")
		}
	}
	output = strings.Join(strings.Fields(output), " ")
	if len(output) > 2048 {
		output = output[len(output)-2048:]
	}
	return output
}

func removeStaleCache(cacheDir string, episodes []episodeFile) error {
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
		id := ""
		for currentID := range current {
			if strings.HasPrefix(name, currentID+"-") {
				id = currentID
				break
			}
		}
		if id == "" && episodeArtifact(name) {
			id = name[:64]
		}
		if id == "" || !artifactExtension(name) {
			continue
		}
		middle, sheet := previewPaths(cacheDir, id, current[id])
		keep := current[id] != "" && (name == filepath.Base(cachePath(cacheDir, id, current[id])) || name == filepath.Base(middle) || name == filepath.Base(sheet))
		if !keep {
			if err := os.Remove(filepath.Join(cacheDir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	return nil
}

func episodeArtifact(name string) bool {
	if len(name) < 66 || name[64] != '-' {
		return false
	}
	if _, err := hex.DecodeString(name[:64]); err != nil {
		return false
	}
	return artifactExtension(name)
}

func artifactExtension(name string) bool {
	return strings.HasSuffix(name, ".mp4") || strings.HasSuffix(name, ".tmp") || strings.HasSuffix(name, ".jpg")
}

func removeEpisodeArtifacts(cacheDir, id string) error {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, id+"-") || !episodeArtifact(name) {
			continue
		}
		if err := os.Remove(filepath.Join(cacheDir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
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
	previewJobs := make(chan *Episode)
	for i := 0; i < 2; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for episode := range previewJobs {
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
			}
		}()
	}
	for i := range catalog.Series {
		for j := range catalog.Series[i].Seasons {
			for k := range catalog.Series[i].Seasons[j].Episodes {
				episode := &catalog.Series[i].Seasons[j].Episodes[k]
				if episode.Status == "ready" {
					previewJobs <- episode
				}
			}
		}
	}
	close(previewJobs)
	workers.Wait()
}

func Prepare(catalog *Catalog, files []episodeFile, cacheDir string) {
	prepareCatalog(catalog, files, cacheDir)
}
