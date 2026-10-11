package server

import (
	"log"
)

var previewSlots = make(chan struct{}, 2)

func cloneCatalog(source *Catalog) *Catalog {
	copyCatalog := &Catalog{Series: make([]Series, len(source.Series)), byID: make(map[string]*Episode), count: source.count, failures: source.failures}
	for i, series := range source.Series {
		copyCatalog.Series[i] = series
		copyCatalog.Series[i].Seasons = make([]Season, len(series.Seasons))
		for j, season := range series.Seasons {
			copyCatalog.Series[i].Seasons[j] = season
			copyCatalog.Series[i].Seasons[j].Episodes = append([]Episode(nil), season.Episodes...)
			for k := range copyCatalog.Series[i].Seasons[j].Episodes {
				episode := &copyCatalog.Series[i].Seasons[j].Episodes[k]
				copyCatalog.byID[episode.ID] = episode
			}
		}
	}
	return copyCatalog
}

func (app *App) RefreshCatalog() error {
	app.refreshMu.Lock()
	defer app.refreshMu.Unlock()
	files, err := Scan(app.mediaRoot)
	if err != nil {
		return err
	}
	catalog := NewCatalog(files)
	app.catalogMu.Lock()
	previous := app.catalog.Load()
	if len(app.active) == 0 {
		if err := removeStaleCache(app.cacheDir, files); err != nil {
			log.Printf("unable to inspect cache directory")
		}
	}
	jobs := make([]Episode, 0)
	for i := range catalog.Series {
		for j := range catalog.Series[i].Seasons {
			for k := range catalog.Series[i].Seasons[j].Episodes {
				episode := &catalog.Series[i].Seasons[j].Episodes[k]
				if episode.UnavailableReason != nil {
					catalog.failures++
					continue
				}
				finger, err := fingerprint(episode.SourcePath)
				if err != nil {
					reason := "Source file is unavailable"
					episode.Status = "unavailable"
					episode.PreparationStage = "Cannot prepare"
					episode.UnavailableReason = &reason
					catalog.failures++
					continue
				}
				episode.Fingerprint = finger
				if previous != nil {
					if old := previous.byID[episode.ID]; old != nil && old.Fingerprint == finger {
						*episode = *old
					}
				}
				if episode.Status == "unavailable" {
					catalog.failures++
				}
				if episode.Status == "ready" || episode.Status == "unavailable" || app.active[episode.ID] == finger {
					continue
				}
				episode.Status = "queued"
				episode.PreparationStage = "Waiting to prepare"
				episode.PreparationProgress = 0
				app.active[episode.ID] = finger
				jobs = append(jobs, *episode)
			}
		}
	}
	app.catalog.Store(catalog)
	app.ready.Store(true)
	app.catalogMu.Unlock()
	for _, episode := range jobs {
		go app.runPreparation(episode)
	}
	return nil
}

func (app *App) publishEpisode(episode Episode) {
	app.catalogMu.Lock()
	defer app.catalogMu.Unlock()
	current := app.catalog.Load()
	if current == nil {
		return
	}
	old := current.byID[episode.ID]
	if old == nil || old.Fingerprint != episode.Fingerprint {
		return
	}
	updated := cloneCatalog(current)
	if old.Status == "unavailable" && episode.Status != "unavailable" && updated.failures > 0 {
		updated.failures--
	}
	if old.Status != "unavailable" && episode.Status == "unavailable" {
		updated.failures++
	}
	*updated.byID[episode.ID] = episode
	app.catalog.Store(updated)
}

func (app *App) runPreparation(episode Episode) {
	defer func() {
		app.catalogMu.Lock()
		if app.active[episode.ID] == episode.Fingerprint {
			delete(app.active, episode.ID)
		}
		app.catalogMu.Unlock()
	}()
	update := func(stage string, progress int) {
		episode.PreparationStage = stage
		episode.PreparationProgress = progress
		app.publishEpisode(episode)
	}
	episode.Status = "preparing"
	update("Probing media", 0)
	plan, ready, err := inspectEpisode(&episode, app.cacheDir)
	if err == nil && !ready {
		episode.Status = "queued"
		update("Waiting to convert", 0)
		app.prepareSlots <- struct{}{}
		episode.Status = "preparing"
		update("Converting video", 0)
		err = convertEpisode(&episode, app.cacheDir, plan, update)
		<-app.prepareSlots
	}
	if err != nil {
		reason := err.Error()
		episode.Status = "unavailable"
		episode.PreparationStage = "Preparation failed"
		episode.UnavailableReason = &reason
		app.publishEpisode(episode)
		log.Printf("episode %s unavailable: %s", episode.ID, reason)
		return
	}
	episode.PreparationStage = "Ready to watch"
	episode.PreparationProgress = 100
	streamURL := "/api/episodes/" + episode.ID + "/stream"
	episode.StreamURL = &streamURL
	app.publishEpisode(episode)
	previewSlots <- struct{}{}
	defer func() { <-previewSlots }()
	if err := preparePreview(&episode, app.cacheDir); err != nil {
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
	app.publishEpisode(episode)
}
