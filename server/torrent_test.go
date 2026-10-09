package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportRejectsSymlinkOutsideDownloads(t *testing.T) {
	root := t.TempDir()
	downloads := filepath.Join(root, "downloads")
	media := filepath.Join(root, "media")
	hash := strings.Repeat("a", 40)
	if err := os.MkdirAll(filepath.Join(downloads, hash), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(media, 0755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(root, "outside.mp4")
	if err := os.WriteFile(outside, []byte("outside"), 0644); err != nil {
		t.Fatal(err)
	}
	name := "S01E01 - Pilot.mp4"
	if err := os.Symlink(outside, filepath.Join(downloads, hash, name)); err != nil {
		t.Fatal(err)
	}
	manager := &TorrentManager{downloads: downloads, mediaRoot: media}
	if err := manager.importFile(hash, TorrentFile{Name: name}, "Example"); err == nil {
		t.Fatal("imported a file outside the download directory")
	}
	if _, err := os.Stat(filepath.Join(media, "Example", "Season 01", name)); !os.IsNotExist(err) {
		t.Fatalf("unexpected imported file: %v", err)
	}
}
