package server

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	MediaRoot           string
	CacheDir            string
	TorrentRoot         string
	TorrentExtraRoot    string
	TorrentURL          string
	TorrentDownloadRoot string
	Port                string
}

func LoadConfig() (Config, error) {
	config := Config{MediaRoot: os.Getenv("MEDIA_ROOT"), CacheDir: os.Getenv("CACHE_DIR"), TorrentRoot: os.Getenv("TORRENT_ROOT"), TorrentExtraRoot: os.Getenv("TORRENT_EXTRA_ROOT"), TorrentURL: os.Getenv("TORRENT_URL"), TorrentDownloadRoot: os.Getenv("TORRENT_DOWNLOAD_ROOT"), Port: os.Getenv("PORT")}
	if config.MediaRoot == "" || config.CacheDir == "" {
		return Config{}, fmt.Errorf("MEDIA_ROOT and CACHE_DIR are required")
	}
	if !filepath.IsAbs(config.MediaRoot) || !filepath.IsAbs(config.CacheDir) {
		return Config{}, fmt.Errorf("MEDIA_ROOT and CACHE_DIR must be absolute paths")
	}
	root, err := filepath.Abs(filepath.Clean(config.MediaRoot))
	if err != nil {
		return Config{}, fmt.Errorf("invalid MEDIA_ROOT")
	}
	cache, err := filepath.Abs(filepath.Clean(config.CacheDir))
	if err != nil {
		return Config{}, fmt.Errorf("invalid CACHE_DIR")
	}
	if pathsOverlap(root, cache) {
		return Config{}, fmt.Errorf("MEDIA_ROOT and CACHE_DIR must not overlap")
	}
	if err := ensureReadableRoot(root); err != nil {
		return Config{}, fmt.Errorf("MEDIA_ROOT is missing or unreadable")
	}
	if err := os.MkdirAll(cache, 0755); err != nil {
		return Config{}, fmt.Errorf("CACHE_DIR is unavailable")
	}
	if info, err := os.Stat(cache); err != nil || !info.IsDir() {
		return Config{}, fmt.Errorf("CACHE_DIR must be a directory")
	}
	if config.Port == "" {
		config.Port = "8080"
	}
	config.MediaRoot = root
	config.CacheDir = cache
	if config.TorrentRoot == "" {
		config.TorrentRoot = "/torrent-files"
	}
	if !filepath.IsAbs(config.TorrentRoot) {
		return Config{}, fmt.Errorf("TORRENT_ROOT must be an absolute path")
	}
	config.TorrentRoot = filepath.Clean(config.TorrentRoot)
	if config.TorrentExtraRoot != "" {
		if !filepath.IsAbs(config.TorrentExtraRoot) {
			return Config{}, fmt.Errorf("TORRENT_EXTRA_ROOT must be absolute")
		}
		config.TorrentExtraRoot = filepath.Clean(config.TorrentExtraRoot)
	}
	if config.TorrentURL == "" {
		config.TorrentURL = "http://qbittorrent:8080"
	}
	if config.TorrentDownloadRoot == "" {
		config.TorrentDownloadRoot = "/downloads"
	}
	if !filepath.IsAbs(config.TorrentDownloadRoot) {
		return Config{}, fmt.Errorf("TORRENT_DOWNLOAD_ROOT must be an absolute path")
	}
	config.TorrentDownloadRoot = filepath.Clean(config.TorrentDownloadRoot)
	if pathsOverlap(root, config.TorrentDownloadRoot) || pathsOverlap(cache, config.TorrentDownloadRoot) {
		return Config{}, fmt.Errorf("TORRENT_DOWNLOAD_ROOT must not overlap MEDIA_ROOT or CACHE_DIR")
	}
	if err := os.MkdirAll(config.TorrentDownloadRoot, 0755); err != nil {
		return Config{}, fmt.Errorf("TORRENT_DOWNLOAD_ROOT is unavailable")
	}
	config.Port = strings.TrimPrefix(config.Port, ":")
	return config, nil
}
