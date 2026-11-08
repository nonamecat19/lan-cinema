package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (manager *TorrentManager) add(source TorrentSource) error {
	if _, err := manager.getTorrent(source.InfoHash); err == nil {
		return nil
	}
	data, err := os.ReadFile(source.path)
	if err != nil {
		return err
	}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("savepath", filepath.Join(manager.downloads, source.InfoHash))
	_ = writer.WriteField("paused", "true")
	_ = writer.WriteField("root_folder", "false")
	fileWriter, err := writer.CreateFormFile("torrents", filepath.Base(source.path))
	if err != nil {
		return err
	}
	if _, err := fileWriter.Write(data); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	request, err := http.NewRequest(http.MethodPost, manager.baseURL+"/api/v2/torrents/add", body)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := manager.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	accepted := strings.Contains(strings.ToLower(string(responseBody)), "ok")
	if !accepted {
		var result struct {
			SuccessCount int `json:"success_count"`
			FailureCount int `json:"failure_count"`
		}
		if json.Unmarshal(responseBody, &result) == nil {
			accepted = result.SuccessCount > 0 && result.FailureCount == 0
		}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || !accepted {
		return fmt.Errorf("qBittorrent rejected the torrent (HTTP %d): %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	if _, err := manager.files(source.InfoHash); err != nil {
		return fmt.Errorf("torrent was added but its files did not become available: %w", err)
	}
	if _, err := manager.postForm("/api/v2/torrents/stop", url.Values{"hashes": {source.InfoHash}}); err != nil {
		return fmt.Errorf("torrent was added but could not be stopped for inspection: %w", err)
	}
	return nil
}

func (manager *TorrentManager) getTorrent(hash string) (qbitInfo, error) {
	if !validInfoHash(hash) {
		return qbitInfo{}, os.ErrNotExist
	}
	var infos []qbitInfo
	if err := manager.getJSON("/api/v2/torrents/info?hashes="+url.QueryEscape(hash), &infos); err != nil {
		return qbitInfo{}, err
	}
	if len(infos) == 0 {
		return qbitInfo{}, os.ErrNotExist
	}
	return infos[0], nil
}

func (manager *TorrentManager) files(hash string) ([]TorrentFile, error) {
	if !validInfoHash(hash) {
		return nil, os.ErrNotExist
	}
	var qfiles []qbitFile
	for attempt := 0; ; attempt++ {
		err := manager.getJSON("/api/v2/torrents/files?hash="+url.QueryEscape(hash), &qfiles)
		if err == nil {
			break
		}
		var statusErr qbitHTTPError
		if !errors.As(err, &statusErr) || statusErr.status != http.StatusNotFound || attempt >= 19 {
			return nil, err
		}
		time.Sleep(250 * time.Millisecond)
	}
	files := make([]TorrentFile, 0, len(qfiles))
	for index, file := range qfiles {
		season := seasonFromPath(file.Name)
		files = append(files, TorrentFile{Index: index, Name: filepath.ToSlash(file.Name), Size: file.Size, Progress: file.Progress, Priority: file.Priority, Season: season})
	}
	return files, nil
}

func (manager *TorrentManager) filePriority(hash string, files []TorrentFile, selected []int) error {
	ids := make([]string, 0)
	if selected == nil {
		for _, file := range files {
			ids = append(ids, strconv.Itoa(file.Index))
		}
	} else {
		for _, index := range selected {
			ids = append(ids, strconv.Itoa(index))
		}
	}
	priority := 0
	if selected != nil {
		priority = 1
	}
	return manager.setFilePriority(hash, ids, priority)
}

func (manager *TorrentManager) setFilePriority(hash string, ids []string, priority int) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := manager.postForm("/api/v2/torrents/filePrio", url.Values{"hash": {hash}, "id": {strings.Join(ids, "|")}, "priority": {strconv.Itoa(priority)}})
	return err
}

func (manager *TorrentManager) getJSON(endpoint string, target any) error {
	request, err := http.NewRequest(http.MethodGet, manager.baseURL+endpoint, nil)
	if err != nil {
		return err
	}
	response, err := manager.client.Do(request)
	if err != nil {
		return fmt.Errorf("torrent client is unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return qbitHTTPError{status: response.StatusCode}
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(target); err != nil {
		return fmt.Errorf("torrent client returned invalid data")
	}
	return nil
}

func (manager *TorrentManager) postForm(endpoint string, values url.Values) (string, error) {
	request, err := http.NewRequest(http.MethodPost, manager.baseURL+endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := manager.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("torrent client is unavailable")
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("qBittorrent request failed (HTTP %d): %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	return string(body), nil
}
