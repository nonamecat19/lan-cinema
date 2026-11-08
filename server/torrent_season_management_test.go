package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestSeasonPriorityModes(t *testing.T) {
	priorities := make(map[string]int)
	client := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v2/torrents/filePrio" {
			http.NotFound(writer, request)
			return
		}
		_ = request.ParseForm()
		value := 0
		if request.Form.Get("priority") == "1" {
			value = 1
		} else if request.Form.Get("priority") == "7" {
			value = 7
		}
		for _, id := range strings.Split(request.Form.Get("id"), "|") {
			priorities[id] = value
		}
	}))
	defer client.Close()
	manager := &TorrentManager{baseURL: client.URL, client: client.Client()}
	files := []TorrentFile{
		{Index: 0, Name: "S01E01 - First.mkv", Season: 1},
		{Index: 1, Name: "S02E01 - Second.mkv", Season: 2},
		{Index: 2, Name: "S03E01 - Third.mkv", Season: 3},
	}
	selection := TorrentSelection{Seasons: []int{1, 2, 3}}
	assertPriorities := func(want map[string]int) {
		t.Helper()
		if err := manager.reconcilePriorities(strings.Repeat("a", 40), selection, files); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(priorities, want) {
			t.Fatalf("priorities = %#v, want %#v", priorities, want)
		}
		for i := range files {
			files[i].Priority = priorities[strconv.Itoa(i)]
		}
	}
	assertPriorities(map[string]int{"0": 7, "1": 1, "2": 1})
	selection.ForceOneByOne = true
	assertPriorities(map[string]int{"0": 7, "1": 0, "2": 0})
	files[0].Progress = 1
	assertPriorities(map[string]int{"0": 1, "1": 7, "2": 0})
	selection.PausedSeasons = []int{2}
	assertPriorities(map[string]int{"0": 1, "1": 0, "2": 7})
	selection.DeletingSeasons = []int{1}
	assertPriorities(map[string]int{"0": 0, "1": 0, "2": 7})
}

func TestDeleteSeasonKeepsOtherSeason(t *testing.T) {
	root := t.TempDir()
	hash := ""
	files := []qbitFile{
		{Index: 0, Name: "Season 01/S01E01 - First.mkv", Size: 4, Progress: 1, Priority: 1},
		{Index: 1, Name: "Season 02/S02E01 - Second.mkv", Size: 4, Progress: 0.5, Priority: 1},
	}
	starts := 0
	client := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/v2/torrents/info":
			_ = json.NewEncoder(writer).Encode([]qbitInfo{{Hash: hash, State: "downloading", SavePath: filepath.Join(root, "downloads", hash)}})
		case "/api/v2/torrents/files":
			_ = json.NewEncoder(writer).Encode(files)
		case "/api/v2/torrents/start":
			starts++
		case "/api/v2/torrents/filePrio", "/api/v2/torrents/stop":
			_, _ = writer.Write([]byte("Ok."))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer client.Close()
	media := filepath.Join(root, "media")
	downloads := filepath.Join(root, "downloads")
	cache := filepath.Join(root, "cache")
	for _, directory := range []string{media, downloads, cache} {
		if err := os.MkdirAll(directory, 0755); err != nil {
			t.Fatal(err)
		}
	}
	manager := NewTorrentManager(root, "", media, downloads, client.URL, filepath.Join(cache, "state.json"))
	source, err := manager.Upload("series.torrent", []byte("d4:infod4:name6:Seriesee"))
	if err != nil {
		t.Fatal(err)
	}
	hash = source.InfoHash
	paths := make([]string, 0)
	for _, file := range files {
		staged := filepath.Join(downloads, hash, filepath.FromSlash(file.Name))
		if err := os.MkdirAll(filepath.Dir(staged), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(staged, []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
		target, _, err := importTarget(media, "Series", TorrentFile{Name: file.Name})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, staged, target)
	}
	manager.selections[hash] = TorrentSelection{SeriesTitle: "Series", Seasons: []int{1, 2}}
	manager.imported[hash+":0"] = true
	manager.imported[hash+":1"] = true
	_, relative, _ := importTarget(media, "Series", TorrentFile{Name: files[0].Name})
	artifact := filepath.Join(cache, digest(relative)+"-v3-old.mp4")
	if err := os.WriteFile(artifact, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	refreshed := false
	if err := manager.DeleteSeason(source.ID, "Series", 1, cache, func() error { refreshed = true; return nil }); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{paths[0], paths[1], artifact} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("file remains: %s: %v", path, err)
		}
	}
	for _, path := range []string{paths[2], paths[3]} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("other season was removed: %s: %v", path, err)
		}
	}
	if !refreshed || starts != 1 || !reflect.DeepEqual(manager.selections[hash].Seasons, []int{2}) {
		t.Fatalf("unexpected final state: refreshed=%v starts=%d selection=%#v", refreshed, starts, manager.selections[hash])
	}
}

func TestDownloadsIncludesWaitingAndPausedSeasons(t *testing.T) {
	hash := strings.Repeat("b", 40)
	client := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/v2/torrents/info":
			_ = json.NewEncoder(writer).Encode([]qbitInfo{{Hash: hash, Name: "Series", State: "downloading"}})
		case "/api/v2/torrents/files":
			_ = json.NewEncoder(writer).Encode([]qbitFile{
				{Index: 0, Name: "S01E01 - First.mkv", Size: 100, Progress: 0.2, Priority: 7},
				{Index: 1, Name: "S02E01 - Second.mkv", Size: 200, Progress: 0.1, Priority: 0},
				{Index: 2, Name: "S03E01 - Third.mkv", Size: 300, Progress: 0.3, Priority: 0},
			})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer client.Close()
	root := t.TempDir()
	manager := NewTorrentManager(root, "", root, root, client.URL, filepath.Join(root, "state.json"))
	manager.selections[hash] = TorrentSelection{SeriesTitle: "Series", Seasons: []int{1, 2, 3}, PausedSeasons: []int{3}, ForceOneByOne: true}
	downloads, err := manager.Downloads(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(downloads) != 1 || len(downloads[0].Files) != 3 {
		t.Fatalf("selected seasons missing: %#v", downloads)
	}
	for index, status := range []string{"downloading", "waiting", "paused"} {
		if downloads[0].Files[index].Status != status {
			t.Fatalf("season %d status = %q, want %q", index+1, downloads[0].Files[index].Status, status)
		}
	}
	if downloads[0].Files[1].Size != 200 {
		t.Fatalf("season file size = %d", downloads[0].Files[1].Size)
	}
}

func TestRemoveRegularFileRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "episode.mkv")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if err := removeRegularFile(root, link); err == nil {
		t.Fatal("symlink was accepted")
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatal(err)
	}
}

func TestSeasonStatePersists(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(root, "state.json")
	manager := NewTorrentManager(root, "", root, root, "http://example.invalid", state)
	hash := strings.Repeat("a", 40)
	manager.selections[hash] = TorrentSelection{SeriesTitle: "Series", Seasons: []int{1, 2}, PausedSeasons: []int{2}, DeletingSeasons: []int{1}, ForceOneByOne: true}
	manager.saveStateLocked()
	reloaded := NewTorrentManager(root, "", root, root, "http://example.invalid", state)
	selection := reloaded.selections[hash]
	if !selection.ForceOneByOne || !reflect.DeepEqual(selection.PausedSeasons, []int{2}) || !reflect.DeepEqual(selection.DeletingSeasons, []int{1}) {
		t.Fatalf("season state was not saved: %#v", selection)
	}
}
