package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLinkedTorrentOffersAndAddsAnotherSeason(t *testing.T) {
	root := t.TempDir()
	torrentRoot := filepath.Join(root, "sources")
	if err := os.MkdirAll(torrentRoot, 0755); err != nil {
		t.Fatal(err)
	}
	var calls []string
	client := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/v2/torrents/files":
			_ = json.NewEncoder(writer).Encode([]qbitFile{
				{Index: 0, Name: "Bones/S01E01 - First.mkv", Size: 100, Priority: 1},
				{Index: 1, Name: "Bones/S02E01 - Second.mkv", Size: 100, Priority: 0},
			})
		case "/api/v2/torrents/filePrio", "/api/v2/torrents/start":
			_ = request.ParseForm()
			calls = append(calls, request.URL.Path+"?"+request.Form.Encode())
			_, _ = writer.Write([]byte("Ok."))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer client.Close()
	manager := NewTorrentManager(torrentRoot, "", filepath.Join(root, "media"), filepath.Join(root, "downloads"), client.URL, filepath.Join(root, "torrent-state.json"))
	source, err := manager.Upload("bones.torrent", []byte("d4:infod4:name5:Bonesee"))
	if err != nil {
		t.Fatal(err)
	}
	manager.selections[source.InfoHash] = TorrentSelection{SeriesTitle: "Bones", Seasons: []int{1}, Files: []TorrentFile{
		{Index: 0, Name: "Bones/S01E01 - First.mkv", Season: 1},
		{Index: 1, Name: "Bones/S02E01 - Second.mkv", Season: 2},
	}}
	linked := manager.AvailableForSeries("bones")
	if len(linked) != 1 || linked[0].ID != source.ID || !reflect.DeepEqual(linked[0].AvailableSeasons, []TorrentSeasonOption{{Number: 2, Episodes: 1}}) {
		t.Fatalf("unexpected linked torrent: %#v", linked)
	}
	if err := manager.AddSeasons(source.ID, "Bones", []int{2}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(manager.selections[source.InfoHash].Seasons, []int{1, 2}) {
		t.Fatalf("unexpected selected seasons: %#v", manager.selections[source.InfoHash].Seasons)
	}
	if len(manager.AvailableForSeries("Bones")[0].AvailableSeasons) != 0 {
		t.Fatal("selected season is still offered")
	}
	if len(calls) != 2 || !strings.Contains(calls[0], "id=1") || !strings.Contains(calls[0], "priority=1") || !strings.Contains(calls[1], "/start") {
		t.Fatalf("unexpected torrent client calls: %#v", calls)
	}
}
