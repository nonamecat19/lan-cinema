package server

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

type probeResult struct {
	Streams []struct {
		CodecType string `json:"codec_type"`
		CodecName string `json:"codec_name"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

func probe(path string) (int64, bool, bool, error) {
	command := exec.Command("ffprobe", "-v", "error", "-show_entries", "stream=codec_type,codec_name:format=duration", "-of", "json", path)
	output, err := command.Output()
	if err != nil {
		return 0, false, false, fmt.Errorf("FFprobe failed")
	}
	var result probeResult
	if err := json.Unmarshal(output, &result); err != nil {
		return 0, false, false, fmt.Errorf("FFprobe returned invalid data")
	}
	video, audio := false, false
	for _, stream := range result.Streams {
		if stream.CodecType == "video" && stream.CodecName == "h264" {
			video = true
		}
		if stream.CodecType == "audio" && stream.CodecName == "aac" {
			audio = true
		}
	}
	duration, err := strconv.ParseFloat(result.Format.Duration, 64)
	if err != nil || duration <= 0 {
		return 0, false, false, fmt.Errorf("Invalid media duration")
	}
	return int64(duration + 0.5), video && audio, audio, nil
}

func needsFastStart(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	var offset int64
	var moov, mdat int64 = -1, -1
	info, err := file.Stat()
	if err != nil {
		return false, err
	}
	for offset+8 <= info.Size() {
		header := make([]byte, 8)
		if _, err := file.ReadAt(header, offset); err != nil {
			return false, err
		}
		size := int64(uint32(header[0])<<24 | uint32(header[1])<<16 | uint32(header[2])<<8 | uint32(header[3]))
		headerSize := int64(8)
		if size == 1 {
			large := make([]byte, 8)
			if _, err := file.ReadAt(large, offset+8); err != nil {
				return false, err
			}
			size = int64(uint64(large[0])<<56 | uint64(large[1])<<48 | uint64(large[2])<<40 | uint64(large[3])<<32 | uint64(large[4])<<24 | uint64(large[5])<<16 | uint64(large[6])<<8 | uint64(large[7]))
			headerSize = 16
		} else if size == 0 {
			size = info.Size() - offset
		}
		if size < headerSize || offset+size > info.Size() {
			return false, fmt.Errorf("Invalid MP4 atom layout")
		}
		atom := string(header[4:8])
		if atom == "moov" && moov == -1 {
			moov = offset
		}
		if atom == "mdat" && mdat == -1 {
			mdat = offset
		}
		offset += size
	}
	if moov == -1 || mdat == -1 {
		return false, fmt.Errorf("Missing required MP4 atoms")
	}
	return moov > mdat, nil
}

func fingerprint(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d-%d", info.Size(), info.ModTime().UnixNano()), nil
}
