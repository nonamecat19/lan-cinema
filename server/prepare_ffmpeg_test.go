package server

import (
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func requireFFmpeg(t *testing.T) {
	t.Helper()
	for _, name := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Skipf("%s is unavailable", name)
		}
	}
}

func runFFmpeg(t *testing.T, args ...string) {
	t.Helper()
	command := exec.Command("ffmpeg", append([]string{"-v", "error", "-y"}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg failed: %v: %s", err, output)
	}
}

func fixture(t *testing.T, path, audioCodec string, fastStart bool) {
	t.Helper()
	args := []string{"-f", "lavfi", "-i", "testsrc2=size=64x64:rate=10"}
	if audioCodec != "" {
		args = append(args, "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000")
	}
	args = append(args, "-t", "1", "-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p")
	if audioCodec != "" {
		args = append(args, "-c:a", audioCodec)
	}
	if fastStart {
		args = append(args, "-movflags", "+faststart")
	}
	runFFmpeg(t, append(args, path)...)
}

func audioPeak(t *testing.T, path string) int16 {
	t.Helper()
	command := exec.Command("ffmpeg", "-v", "error", "-i", path, "-map", "0:a:0", "-t", "1", "-f", "s16le", "-acodec", "pcm_s16le", "-ac", "1", "-ar", "8000", "pipe:1")
	data, err := command.Output()
	if err != nil {
		t.Fatalf("audio decode failed: %v", err)
	}
	var peak int16
	for i := 0; i+1 < len(data); i += 2 {
		sample := int16(binary.LittleEndian.Uint16(data[i:]))
		if sample < 0 {
			sample = -sample
		}
		if sample > peak {
			peak = sample
		}
	}
	return peak
}

func TestPrepareConvertsExistingAudioAndInvalidatesSilentCache(t *testing.T) {
	requireFFmpeg(t)
	root := t.TempDir()
	source := filepath.Join(root, "source.mkv")
	fixture(t, source, "ac3", false)
	cache := filepath.Join(root, "cache")
	if err := os.Mkdir(cache, 0755); err != nil {
		t.Fatal(err)
	}
	finger, err := fingerprint(source)
	if err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(cache, "episode-"+finger+".mp4")
	runFFmpeg(t, "-f", "lavfi", "-i", "color=size=64x64:rate=10", "-f", "lavfi", "-i", "anullsrc=channel_layout=stereo:sample_rate=48000", "-t", "1", "-c:v", "libx264", "-c:a", "aac", "-movflags", "+faststart", legacy)
	if audioPeak(t, legacy) != 0 {
		t.Fatal("legacy fixture should be silent")
	}
	episode := Episode{ID: "episode", SourcePath: source}
	if err := prepareEpisode(&episode, cache); err != nil {
		t.Fatal(err)
	}
	if episode.Status != "ready" || episode.PlaybackPath == legacy || audioPeak(t, episode.PlaybackPath) < 1000 {
		t.Fatalf("audio was not preserved: %#v", episode)
	}
	prepared, err := probe(episode.PlaybackPath)
	if err != nil || !prepared.H264 || !prepared.AAC {
		t.Fatalf("unexpected prepared codecs: %#v, %v", prepared, err)
	}
	firstInfo, err := os.Stat(episode.PlaybackPath)
	if err != nil {
		t.Fatal(err)
	}
	second := Episode{ID: "episode", SourcePath: source}
	if err := prepareEpisode(&second, cache); err != nil {
		t.Fatal(err)
	}
	secondInfo, err := os.Stat(second.PlaybackPath)
	if err != nil || !os.SameFile(firstInfo, secondInfo) {
		t.Fatalf("prepared cache was not reused: %v", err)
	}
	if err := removeStaleCache(cache, []episodeFile{{id: "episode", path: source}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy cache was not removed: %v", err)
	}
}

func TestPrepareRemuxesMKVAndAddsAudioOnlyWhenMissing(t *testing.T) {
	requireFFmpeg(t)
	root := t.TempDir()
	cache := filepath.Join(root, "cache")
	if err := os.Mkdir(cache, 0755); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name       string
		audioCodec string
		peak       int16
	}{
		{"aac-mkv", "aac", 1000},
		{"silent-mkv", "", 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := filepath.Join(root, test.name+".mkv")
			fixture(t, source, test.audioCodec, false)
			episode := Episode{ID: test.name, SourcePath: source}
			if err := prepareEpisode(&episode, cache); err != nil {
				t.Fatal(err)
			}
			if episode.Status != "ready" || episode.PlaybackPath == source {
				t.Fatalf("source was not remuxed: %#v", episode)
			}
			prepared, err := probe(episode.PlaybackPath)
			if err != nil || !prepared.H264 || !prepared.AAC || !prepared.MP4 {
				t.Fatalf("unexpected prepared media: %#v, %v", prepared, err)
			}
			if peak := audioPeak(t, episode.PlaybackPath); peak < test.peak || (test.peak == 0 && peak != 0) {
				t.Fatalf("unexpected audio peak: %d", peak)
			}
		})
	}
}

func TestRefreshMakesDirectAndCachedEpisodesReadyWithoutPreparationSlots(t *testing.T) {
	requireFFmpeg(t)
	root := t.TempDir()
	media := filepath.Join(root, "media")
	cache := filepath.Join(root, "cache")
	season := filepath.Join(media, "Example", "Season 01")
	if err := os.MkdirAll(season, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(cache, 0755); err != nil {
		t.Fatal(err)
	}
	direct := filepath.Join(season, "S01E01 - Direct.mp4")
	cached := filepath.Join(season, "S01E02 - Cached.mkv")
	conversion := filepath.Join(season, "S01E03 - Conversion.mkv")
	fixture(t, direct, "aac", true)
	fixture(t, cached, "ac3", false)
	fixture(t, conversion, "ac3", false)
	files, err := Scan(media)
	if err != nil {
		t.Fatal(err)
	}
	prepared := Episode{ID: files[1].id, SourcePath: cached}
	if err := prepareEpisode(&prepared, cache); err != nil {
		t.Fatal(err)
	}
	app := NewApp(t.TempDir())
	app.ConfigureLibrary(media, cache, nil)
	inspectionSlots <- struct{}{}
	inspectionSlots <- struct{}{}
	app.prepareSlots <- struct{}{}
	app.prepareSlots <- struct{}{}
	previewSlots <- struct{}{}
	inspectionReleased := false
	released := false
	defer func() {
		if !inspectionReleased {
			<-inspectionSlots
			<-inspectionSlots
		}
		if !released {
			<-app.prepareSlots
			<-app.prepareSlots
			<-previewSlots
		}
	}()
	if err := app.RefreshCatalog(); err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if status := app.catalog.Load().byID[file.id].Status; status != "queued" {
			t.Fatalf("episode started inspection without capacity: %s", status)
		}
	}
	<-inspectionSlots
	<-inspectionSlots
	inspectionReleased = true
	deadline := time.After(5 * time.Second)
	for {
		catalog := app.catalog.Load()
		first := catalog.byID[files[0].id]
		second := catalog.byID[files[1].id]
		third := catalog.byID[files[2].id]
		if first.Status == "ready" && second.Status == "ready" {
			if third.Status != "queued" {
				t.Fatalf("conversion did not wait for slot: %s", third.Status)
			}
			break
		}
		select {
		case <-deadline:
			t.Fatalf("ready episodes waited for slots: %s, %s", first.Status, second.Status)
		case <-time.After(20 * time.Millisecond):
		}
	}
	<-app.prepareSlots
	<-app.prepareSlots
	<-previewSlots
	released = true
	completion := time.After(5 * time.Second)
	for {
		app.catalogMu.Lock()
		active := len(app.active)
		app.catalogMu.Unlock()
		if active == 0 {
			break
		}
		select {
		case <-completion:
			t.Fatal("preparation did not finish")
		case <-time.After(20 * time.Millisecond):
		}
	}
	if len(previewSlots) != 0 || len(inspectionSlots) != 0 {
		t.Fatalf("media capacity was not released: preview=%d inspection=%d", len(previewSlots), len(inspectionSlots))
	}
}
