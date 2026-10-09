package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanOrdersEpisodesAndPreservesDuplicateReason(t *testing.T) {
	root := t.TempDir()
	season := filepath.Join(root, "Example", "Season 01")
	if err := os.MkdirAll(season, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"S01E10 - Tenth.mp4", "S01E02 - Second.mp4", "S01E02 - Alternate.mkv", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(season, name), nil, 0644); err != nil {
			t.Fatal(err)
		}
	}
	files, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 || files[0].number != 2 || files[1].number != 2 || files[2].number != 10 {
		t.Fatalf("unexpected scan order: %+v", files)
	}
	catalog := NewCatalog(files)
	episodes := catalog.Series[0].Seasons[0].Episodes
	for _, episode := range episodes[:2] {
		if episode.UnavailableReason == nil || *episode.UnavailableReason != "Duplicate episode number" {
			t.Fatalf("duplicate %s has no reason", episode.ID)
		}
	}
	duplicateCatalog := NewCatalog(files[:2])
	Prepare(duplicateCatalog, files[:2], t.TempDir())
	if duplicateCatalog.Failures() != 2 {
		t.Fatalf("expected two unavailable duplicates, got %d", duplicateCatalog.Failures())
	}
	for _, episode := range duplicateCatalog.Series[0].Seasons[0].Episodes {
		if episode.Status != "unavailable" || episode.StreamURL != nil {
			t.Fatalf("duplicate became playable: %+v", episode)
		}
	}
}
