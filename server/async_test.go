package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRefreshCatalogPublishesQueuedEpisodeBeforePreparation(t *testing.T) {
	root := t.TempDir()
	media := filepath.Join(root, "media")
	cache := filepath.Join(root, "cache")
	season := filepath.Join(media, "Example", "Season 01")
	if err := os.MkdirAll(season, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cache, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(season, "S01E01 - Pilot.mp4"), []byte("invalid video"), 0644); err != nil {
		t.Fatal(err)
	}
	app := NewApp(t.TempDir())
	app.ConfigureLibrary(media, cache, nil)
	app.prepareSlots <- struct{}{}
	app.prepareSlots <- struct{}{}
	if err := app.RefreshCatalog(); err != nil {
		t.Fatal(err)
	}
	episode := app.catalog.Load().Series[0].Seasons[0].Episodes[0]
	if episode.Status != "queued" || episode.PreparationStage != "Waiting to prepare" {
		t.Fatalf("unexpected published episode: %#v", episode)
	}
	<-app.prepareSlots
	deadline := time.After(5 * time.Second)
	for {
		episode = app.catalog.Load().Series[0].Seasons[0].Episodes[0]
		if episode.Status == "unavailable" {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("preparation did not finish, last status: %s", episode.Status)
		case <-time.After(20 * time.Millisecond):
		}
	}
	<-app.prepareSlots
}
