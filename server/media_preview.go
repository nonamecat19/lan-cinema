package server

import (
	"fmt"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

func previewPaths(cacheDir, id, fingerprint string) (string, string) {
	base := id + "-" + fingerprint
	return filepath.Join(cacheDir, base+"-middle.jpg"), filepath.Join(cacheDir, base+"-sheet.jpg")
}

func validJPEG(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	config, err := jpeg.DecodeConfig(file)
	return err == nil && config.Width > 0 && config.Height > 0
}

func createJPEG(source, output string, args ...string) error {
	commandArgs := []string{"-v", "error", "-y", "-i", source}
	commandArgs = append(commandArgs, args...)
	commandArgs = append(commandArgs, "-frames:v", "1", "-q:v", "4", "-update", "1", output)
	command := exec.Command("ffmpeg", commandArgs...)
	if output, err := command.CombinedOutput(); err != nil {
		if len(output) > 0 {
			return fmt.Errorf("FFmpeg preview extraction failed")
		}
		return fmt.Errorf("FFmpeg preview extraction failed")
	}
	return nil
}

func preparePreview(episode *Episode, cacheDir string) error {
	info, err := os.Lstat(episode.SourcePath)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("Source file is unavailable")
	}
	duration, _, _, err := probe(episode.SourcePath)
	if err != nil {
		return err
	}
	finger, err := fingerprint(episode.SourcePath)
	if err != nil {
		return fmt.Errorf("Source file is unavailable")
	}
	middlePath, sheetPath := previewPaths(cacheDir, episode.ID, finger)
	if validJPEG(middlePath) && validJPEG(sheetPath) {
		episode.PreviewPath = middlePath
		episode.PreviewSpritePath = sheetPath
	} else {
		middleTemp, err := os.CreateTemp(cacheDir, episode.ID+"-middle-*.jpg")
		if err != nil {
			return fmt.Errorf("Cache is unavailable")
		}
		middleTempPath := middleTemp.Name()
		if err := middleTemp.Close(); err != nil {
			os.Remove(middleTempPath)
			return fmt.Errorf("Cache is unavailable")
		}
		defer os.Remove(middleTempPath)
		seek := strconv.FormatFloat(float64(duration)/2, 'f', 3, 64)
		middleFilter := "scale=480:270:force_original_aspect_ratio=decrease,pad=480:270:(ow-iw)/2:(oh-ih)/2"
		if err := createJPEG(episode.SourcePath, middleTempPath, "-ss", seek, "-map", "0:v:0", "-vf", middleFilter); err != nil {
			return err
		}
		if !validJPEG(middleTempPath) {
			return fmt.Errorf("Generated middle preview is invalid")
		}
		if err := os.Rename(middleTempPath, middlePath); err != nil {
			return fmt.Errorf("Unable to save middle preview")
		}
		episode.PreviewPath = middlePath
	}
	interval := int64(10)
	if duration > 3600 {
		interval = (duration + 359) / 360
	}
	frameCount := int((duration + interval - 1) / interval)
	columns := 10
	rows := (frameCount + columns - 1) / columns
	if rows < 1 {
		rows = 1
	}
	if episode.PreviewSpritePath == "" {
		sheetTemp, err := os.CreateTemp(cacheDir, episode.ID+"-sheet-*.jpg")
		if err != nil {
			return fmt.Errorf("Cache is unavailable")
		}
		sheetTempPath := sheetTemp.Name()
		if err := sheetTemp.Close(); err != nil {
			os.Remove(sheetTempPath)
			return fmt.Errorf("Cache is unavailable")
		}
		defer os.Remove(sheetTempPath)
		filter := fmt.Sprintf("fps=1/%d,scale=160:90:force_original_aspect_ratio=decrease,pad=160:90:(ow-iw)/2:(oh-ih)/2,tile=%dx%d:nb_frames=%d:padding=0:margin=0", interval, columns, rows, frameCount)
		if err := createJPEG(episode.SourcePath, sheetTempPath, "-map", "0:v:0", "-vf", filter); err != nil {
			return err
		}
		if !validJPEG(sheetTempPath) {
			return fmt.Errorf("Generated preview sheet is invalid")
		}
		if err := os.Rename(sheetTempPath, sheetPath); err != nil {
			return fmt.Errorf("Unable to save preview sheet")
		}
		episode.PreviewSpritePath = sheetPath
	}
	episode.PreviewInterval = interval
	episode.PreviewColumns = columns
	episode.PreviewRows = rows
	return nil
}
