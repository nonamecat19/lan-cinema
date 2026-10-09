package server

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

func torrentInfo(data []byte) (string, string, error) {
	index := 0
	if len(data) == 0 || data[index] != 'd' {
		return "", "", fmt.Errorf("invalid torrent metadata")
	}
	index++
	var name string
	var rawInfo []byte
	for index < len(data) && data[index] != 'e' {
		key, err := bencodeString(data, &index)
		if err != nil {
			return "", "", err
		}
		start := index
		if key == "info" {
			if err := bencodeSkip(data, &index); err != nil {
				return "", "", err
			}
			rawInfo = data[start:index]
			infoIndex := start + 1
			for infoIndex < index-1 && data[infoIndex] != 'e' {
				infoKey, err := bencodeString(data, &infoIndex)
				if err != nil {
					return "", "", err
				}
				valueIndex := infoIndex
				if infoKey == "name.utf-8" || infoKey == "name" {
					value, err := bencodeString(data, &valueIndex)
					if err == nil && (name == "" || infoKey == "name.utf-8") {
						name = value
					}
				}
				if err := bencodeSkip(data, &infoIndex); err != nil {
					return "", "", err
				}
			}
		} else if err := bencodeSkip(data, &index); err != nil {
			return "", "", err
		}
	}
	if len(rawInfo) == 0 {
		return "", "", fmt.Errorf("torrent metadata has no info dictionary")
	}
	hash := sha1.Sum(rawInfo)
	return name, hex.EncodeToString(hash[:]), nil
}

func bencodeString(data []byte, index *int) (string, error) {
	colon := bytes.IndexByte(data[*index:], ':')
	if colon < 1 {
		return "", fmt.Errorf("invalid torrent metadata")
	}
	length, err := strconv.Atoi(string(data[*index : *index+colon]))
	if err != nil || length < 0 {
		return "", fmt.Errorf("invalid torrent metadata")
	}
	start := *index + colon + 1
	end := start + length
	if end > len(data) {
		return "", fmt.Errorf("invalid torrent metadata")
	}
	*index = end
	return string(data[start:end]), nil
}

func bencodeSkip(data []byte, index *int) error {
	if *index >= len(data) {
		return fmt.Errorf("invalid torrent metadata")
	}
	switch data[*index] {
	case 'i':
		end := bytes.IndexByte(data[*index:], 'e')
		if end < 0 {
			return fmt.Errorf("invalid torrent metadata")
		}
		*index += end + 1
	case 'l', 'd':
		terminator := data[*index]
		*index++
		for *index < len(data) && data[*index] != 'e' {
			if err := bencodeSkip(data, index); err != nil {
				return err
			}
		}
		if *index >= len(data) || data[*index] != 'e' || terminator == 0 {
			return fmt.Errorf("invalid torrent metadata")
		}
		*index++
	default:
		_, err := bencodeString(data, index)
		return err
	}
	return nil
}

func validInfoHash(hash string) bool {
	if len(hash) != 40 {
		return false
	}
	_, err := hex.DecodeString(hash)
	return err == nil
}

func seasonFromPath(value string) int {
	parts := strings.Split(filepath.ToSlash(value), "/")
	for _, part := range parts {
		if match := torrentSeasonPattern.FindStringSubmatch(part); match != nil {
			season, _ := strconv.Atoi(match[1])
			return season
		}
		if match := torrentEpisodePattern.FindStringSubmatch(strings.TrimSuffix(part, filepath.Ext(part))); match != nil {
			season, _ := strconv.Atoi(match[1])
			return season
		}
	}
	return 0
}

func parseTorrentEpisode(value string) (int, int, string, error) {
	base := strings.TrimSuffix(filepath.Base(value), filepath.Ext(value))
	match := torrentEpisodePattern.FindStringSubmatch(base)
	if match == nil {
		return 0, 0, "", fmt.Errorf("torrent video filename has no season and episode number")
	}
	season, _ := strconv.Atoi(match[1])
	episode, _ := strconv.Atoi(match[2])
	title := strings.Trim(strings.ReplaceAll(match[3], ".", " "), " -_")
	title = strings.TrimSpace(torrentNoisePattern.ReplaceAllString(title, ""))
	if title == "" {
		title = fmt.Sprintf("Episode %02d", episode)
	}
	title = regexp.MustCompile(`\s+`).ReplaceAllString(title, " ")
	title = safePathLabel(title)
	return season, episode, title, nil
}

func cleanSeriesTitle(value string) string {
	value = strings.TrimSuffix(filepath.Base(value), filepath.Ext(value))
	value = strings.TrimSpace(regexp.MustCompile(`^[^]]+\]\s*`).ReplaceAllString(value, ""))
	value = strings.TrimSpace(torrentNoisePattern.ReplaceAllString(value, ""))
	value = regexp.MustCompile(`\s+`).ReplaceAllString(strings.ReplaceAll(strings.ReplaceAll(value, ".", " "), "_", " "), " ")
	value = strings.Trim(value, " -_[]()")
	return safePathLabel(value)
}

func inferSeriesTitle(files []TorrentFile, fallback string) string {
	for _, file := range files {
		if !supportedTorrentVideo(file.Name) {
			continue
		}
		base := strings.TrimSuffix(filepath.Base(file.Name), filepath.Ext(file.Name))
		match := torrentEpisodePattern.FindStringIndex(base)
		if match != nil {
			if title := cleanSeriesTitle(strings.Trim(base[:match[0]], " .-_")); title != "" {
				return title
			}
		}
	}
	return cleanSeriesTitle(fallback)
}

func safePathLabel(value string) string {
	var result strings.Builder
	for _, character := range value {
		if unicode.IsLetter(character) || unicode.IsDigit(character) || character == ' ' || character == '-' || character == '_' || character == '(' || character == ')' {
			result.WriteRune(character)
		}
	}
	return strings.TrimSpace(strings.Trim(result.String(), " -_()"))
}

func supportedTorrentVideo(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp4", ".mkv", ".avi", ".mov", ".m4v", ".webm":
		return true
	default:
		return false
	}
}
