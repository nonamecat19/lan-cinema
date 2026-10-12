package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
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

func TestStartSelectsOnlyNumberedEpisodes(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "sources"), 0755); err != nil {
		t.Fatal(err)
	}
	var priorities []url.Values
	client := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/v2/torrents/info":
			_ = json.NewEncoder(writer).Encode([]qbitInfo{{Hash: request.URL.Query().Get("hashes")}})
		case "/api/v2/torrents/files":
			_ = json.NewEncoder(writer).Encode([]qbitFile{
				{Index: 0, Name: "Season 01/S01E01 - Pilot.mkv", Priority: 1},
				{Index: 1, Name: "Season 01/Bonus.mkv", Priority: 1},
			})
		case "/api/v2/torrents/filePrio":
			_ = request.ParseForm()
			priorities = append(priorities, request.Form)
		case "/api/v2/torrents/stop", "/api/v2/torrents/start":
			writer.WriteHeader(http.StatusOK)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer client.Close()
	manager := NewTorrentManager(filepath.Join(root, "sources"), "", filepath.Join(root, "media"), filepath.Join(root, "downloads"), client.URL, filepath.Join(root, "state.json"))
	source, err := manager.Upload("series.torrent", []byte("d4:infod4:name6:Seriesee"))
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Start(source.ID, TorrentSelection{SeriesTitle: "Series", Seasons: []int{1}}); err != nil {
		t.Fatal(err)
	}
	if len(priorities) != 2 || priorities[0].Get("id") != "0|1" || priorities[0].Get("priority") != "0" || priorities[1].Get("id") != "0" || priorities[1].Get("priority") != "1" {
		t.Fatalf("unexpected priorities: %#v", priorities)
	}
}

func TestCompletedTorrentReportsWaitingActiveAndFailedImport(t *testing.T) {
	root := t.TempDir()
	hash := strings.Repeat("a", 40)
	name := "Season 01/S01E01 - Pilot.mkv"
	client := completedTorrentClient(t, hash, name, filepath.Join(root, "downloads", hash))
	defer client.Close()
	manager := NewTorrentManager(root, "", filepath.Join(root, "media"), filepath.Join(root, "downloads"), client.URL, filepath.Join(root, "state.json"))
	manager.selections[hash] = TorrentSelection{SeriesTitle: "Series", Seasons: []int{1}}
	check := func(status, detail string) {
		t.Helper()
		downloads, err := manager.Downloads(nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(downloads) != 1 || len(downloads[0].Files) != 1 {
			t.Fatalf("unexpected downloads: %#v", downloads)
		}
		file := downloads[0].Files[0]
		if file.Status != status || file.Detail != detail {
			t.Fatalf("got %s %q, want %s %q", file.Status, file.Detail, status, detail)
		}
	}
	check("queued", "Waiting to import into the library")
	key := hash + ":0"
	manager.importing[key] = 42
	check("importing", "Copying into the library · 42%")
	delete(manager.importing, key)
	manager.importErrors[key] = "Completed file is unavailable in the download directory"
	check("import-error", "Completed file is unavailable in the download directory")
}

func TestCompletedTorrentImportPersistsFailureAndRecovers(t *testing.T) {
	root := t.TempDir()
	hash := strings.Repeat("b", 40)
	name := "Season 01/S01E01 - Pilot.mkv"
	downloads := filepath.Join(root, "downloads")
	if err := os.MkdirAll(filepath.Join(downloads, hash, "Season 01"), 0755); err != nil {
		t.Fatal(err)
	}
	client := completedTorrentClient(t, hash, name, filepath.Join(downloads, hash))
	defer client.Close()
	state := filepath.Join(root, "state.json")
	media := filepath.Join(root, "media")
	manager := NewTorrentManager(root, "", media, downloads, client.URL, state)
	manager.selections[hash] = TorrentSelection{SeriesTitle: "Series", Seasons: []int{1}}
	if manager.importCompleted() {
		t.Fatal("missing source marked imported")
	}
	key := hash + ":0"
	restarted := NewTorrentManager(root, "", media, downloads, client.URL, state)
	if restarted.importErrors[key] != "Completed file is unavailable in the download directory" {
		t.Fatalf("failure did not survive restart: %#v", restarted.importErrors)
	}
	data := []byte("complete video bytes")
	if err := os.WriteFile(filepath.Join(downloads, hash, filepath.FromSlash(name)), data, 0644); err != nil {
		t.Fatal(err)
	}
	if !restarted.importCompleted() || !restarted.imported[key] || restarted.importErrors[key] != "" {
		t.Fatalf("import did not recover: imported=%#v errors=%#v", restarted.imported, restarted.importErrors)
	}
	path := filepath.Join(media, "Series", "Season 01", "S01E01 - Pilot.mkv")
	got, err := os.ReadFile(path)
	if err != nil || !reflect.DeepEqual(got, data) {
		t.Fatalf("unexpected library file: %q, %v", got, err)
	}
	if restarted.importCompleted() {
		t.Fatal("imported file was imported again")
	}
	afterRestart := NewTorrentManager(root, "", media, downloads, client.URL, state)
	if !afterRestart.imported[key] || afterRestart.importErrors[key] != "" {
		t.Fatalf("success did not survive restart: imported=%#v errors=%#v", afterRestart.imported, afterRestart.importErrors)
	}
}

func TestImportRejectsDifferentExistingLibraryFile(t *testing.T) {
	root := t.TempDir()
	hash := strings.Repeat("c", 40)
	name := "S01E01 - Pilot.mp4"
	sourceDir := filepath.Join(root, "downloads", hash)
	targetDir := filepath.Join(root, "media", "Series", "Season 01")
	for _, directory := range []string{sourceDir, targetDir} {
		if err := os.MkdirAll(directory, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(sourceDir, name), []byte("full content"), 0644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(targetDir, name)
	if err := os.WriteFile(target, []byte("partial"), 0644); err != nil {
		t.Fatal(err)
	}
	manager := &TorrentManager{downloads: filepath.Join(root, "downloads"), mediaRoot: filepath.Join(root, "media")}
	if err := manager.importFile(hash, TorrentFile{Name: name}, "Series"); err == nil || importFailureDetail(err) != "Existing library episode differs from the completed file" {
		t.Fatalf("unexpected collision result: %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "partial" {
		t.Fatalf("existing file changed: %q, %v", data, err)
	}
}

func TestCopyImportReportsProgressAndPublishesOnlyCompleteFile(t *testing.T) {
	root := t.TempDir()
	hash := strings.Repeat("d", 40)
	name := "S01E01 - Pilot.mp4"
	sourceDir := filepath.Join(root, "downloads", hash)
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	data := []byte(strings.Repeat("video", 20000))
	if err := os.WriteFile(filepath.Join(sourceDir, name), data, 0644); err != nil {
		t.Fatal(err)
	}
	media := filepath.Join(root, "media")
	target := filepath.Join(media, "Series", "Season 01", name)
	manager := &TorrentManager{downloads: filepath.Join(root, "downloads"), mediaRoot: media, linkSource: func(string, string) error { return syscall.EXDEV }}
	var progress []int
	err := manager.importFileFromPathWithProgress(TorrentFile{Name: name}, "Series", filepath.Join(sourceDir, name), func(value int) {
		progress = append(progress, value)
		if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
			t.Fatalf("published incomplete target at %d%%: %v", value, statErr)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(progress) == 0 || progress[len(progress)-1] != 100 {
		t.Fatalf("missing copy progress: %#v", progress)
	}
	got, err := os.ReadFile(target)
	if err != nil || !reflect.DeepEqual(got, data) {
		t.Fatalf("unexpected imported file size %d: %v", len(got), err)
	}
	entries, err := os.ReadDir(filepath.Dir(target))
	if err != nil || len(entries) != 1 || entries[0].Name() != name {
		t.Fatalf("temporary copy remains: %#v, %v", entries, err)
	}
}

func completedTorrentClient(t *testing.T, hash, name, downloadPath string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/v2/torrents/info":
			_ = json.NewEncoder(writer).Encode([]qbitInfo{{Hash: hash, Name: "Series", Progress: 1, DownloadPath: downloadPath}})
		case "/api/v2/torrents/files":
			_ = json.NewEncoder(writer).Encode([]qbitFile{{Index: 0, Name: name, Size: 20, Progress: 1, Priority: 1}})
		default:
			http.NotFound(writer, request)
		}
	}))
}
