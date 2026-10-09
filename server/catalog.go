package server

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Episode struct {
	ID                  string  `json:"id"`
	Number              int     `json:"number"`
	Title               string  `json:"title"`
	DurationSeconds     *int64  `json:"durationSeconds"`
	Status              string  `json:"status"`
	PreparationStage    string  `json:"preparationStage,omitempty"`
	PreparationProgress int     `json:"preparationProgress,omitempty"`
	UnavailableReason   *string `json:"unavailableReason"`
	StreamURL           *string `json:"streamUrl"`
	PreviewURL          *string `json:"previewUrl"`
	PreviewSpriteURL    *string `json:"previewSpriteUrl"`
	PreviewInterval     int64   `json:"previewIntervalSeconds"`
	PreviewColumns      int     `json:"previewColumns"`
	PreviewRows         int     `json:"previewRows"`
	SourcePath          string  `json:"-"`
	PlaybackPath        string  `json:"-"`
	PreviewPath         string  `json:"-"`
	PreviewSpritePath   string  `json:"-"`
	Fingerprint         string  `json:"-"`
}

type Season struct {
	Number   int       `json:"number"`
	Episodes []Episode `json:"episodes"`
}

type Series struct {
	ID             string                `json:"id"`
	Title          string                `json:"title"`
	Seasons        []Season              `json:"seasons"`
	TorrentSources []TorrentSeriesSource `json:"torrentSources,omitempty"`
}

type Catalog struct {
	Series   []Series `json:"series"`
	byID     map[string]*Episode
	count    int
	failures int
}

type episodeFile struct {
	seriesTitle string
	seriesRel   string
	season      int
	number      int
	title       string
	id          string
	path        string
	rel         string
}

var seasonPattern = regexp.MustCompile(`^Season ([0-9]{2,})$`)
var episodePattern = regexp.MustCompile(`(?i)^S([0-9]{2,})E([0-9]{2,}) - (.+)\.(mp4|mkv|avi|mov|m4v|webm)$`)

func digest(value string) string {
	sum := sha256.Sum256([]byte(filepath.ToSlash(value)))
	return hex.EncodeToString(sum[:])
}

func scanMedia(root string) ([]episodeFile, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	files := make([]episodeFile, 0)
	for _, seriesEntry := range entries {
		if !seriesEntry.IsDir() || seriesEntry.Type()&os.ModeSymlink != 0 {
			continue
		}
		seriesName := seriesEntry.Name()
		seriesPath := filepath.Join(root, seriesName)
		seasonEntries, err := os.ReadDir(seriesPath)
		if err != nil {
			log.Printf("unable to read series directory: %s", filepath.ToSlash(seriesName))
			continue
		}
		for _, seasonEntry := range seasonEntries {
			if !seasonEntry.IsDir() || seasonEntry.Type()&os.ModeSymlink != 0 {
				continue
			}
			seasonMatch := seasonPattern.FindStringSubmatch(seasonEntry.Name())
			if seasonMatch == nil {
				continue
			}
			seasonNumber, _ := strconv.Atoi(seasonMatch[1])
			seasonPath := filepath.Join(seriesPath, seasonEntry.Name())
			episodeEntries, err := os.ReadDir(seasonPath)
			if err != nil {
				log.Printf("unable to read season directory: %s", filepath.ToSlash(filepath.Join(seriesName, seasonEntry.Name())))
				continue
			}
			for _, entry := range episodeEntries {
				if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
					continue
				}
				match := episodePattern.FindStringSubmatch(entry.Name())
				if match == nil {
					extension := strings.ToLower(filepath.Ext(entry.Name()))
					if extension == ".mp4" || extension == ".mkv" || extension == ".avi" || extension == ".mov" || extension == ".m4v" || extension == ".webm" {
						log.Printf("ignoring naming mismatch: %s", filepath.ToSlash(filepath.Join(seriesName, seasonEntry.Name(), entry.Name())))
					}
					continue
				}
				fileSeason, err1 := strconv.Atoi(match[1])
				episodeNumber, err2 := strconv.Atoi(match[2])
				if err1 != nil || err2 != nil || fileSeason != seasonNumber || strings.TrimSpace(match[3]) == "" {
					log.Printf("ignoring naming mismatch: %s", filepath.ToSlash(filepath.Join(seriesName, seasonEntry.Name(), entry.Name())))
					continue
				}
				rel := filepath.Join(seriesName, seasonEntry.Name(), entry.Name())
				files = append(files, episodeFile{
					seriesTitle: seriesName,
					seriesRel:   seriesName,
					season:      seasonNumber,
					number:      episodeNumber,
					title:       strings.TrimSpace(match[3]),
					id:          digest(rel),
					path:        filepath.Join(seasonPath, entry.Name()),
					rel:         filepath.ToSlash(rel),
				})
			}
		}
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].seriesTitle != files[j].seriesTitle {
			a, b := strings.ToLower(files[i].seriesTitle), strings.ToLower(files[j].seriesTitle)
			if a != b {
				return a < b
			}
			return files[i].seriesTitle < files[j].seriesTitle
		}
		if files[i].season != files[j].season {
			return files[i].season < files[j].season
		}
		if files[i].number != files[j].number {
			return files[i].number < files[j].number
		}
		return files[i].rel < files[j].rel
	})
	return files, nil
}

func buildCatalog(files []episodeFile) *Catalog {
	catalog := &Catalog{Series: make([]Series, 0), byID: make(map[string]*Episode), count: len(files)}
	seriesIndex := make(map[string]int)
	type seasonKey struct {
		series string
		season int
	}
	type episodeKey struct {
		seasonKey
		number int
	}
	seasonIndex := make(map[seasonKey]int)
	duplicates := make(map[episodeKey][]string)
	for i := range files {
		file := files[i]
		seriesKey := file.seriesRel
		index, exists := seriesIndex[seriesKey]
		if !exists {
			index = len(catalog.Series)
			seriesIndex[seriesKey] = index
			catalog.Series = append(catalog.Series, Series{ID: digest(file.seriesRel), Title: file.seriesTitle, Seasons: make([]Season, 0)})
		}
		key := seasonKey{series: seriesKey, season: file.season}
		sIndex, exists := seasonIndex[key]
		if !exists {
			sIndex = len(catalog.Series[index].Seasons)
			seasonIndex[key] = sIndex
			catalog.Series[index].Seasons = append(catalog.Series[index].Seasons, Season{Number: file.season, Episodes: make([]Episode, 0)})
		}
		episode := Episode{ID: file.id, Number: file.number, Title: file.title, Status: "queued", PreparationStage: "Waiting to prepare", SourcePath: file.path}
		catalog.Series[index].Seasons[sIndex].Episodes = append(catalog.Series[index].Seasons[sIndex].Episodes, episode)
		duplicateKey := episodeKey{seasonKey: key, number: file.number}
		duplicates[duplicateKey] = append(duplicates[duplicateKey], file.id)
	}
	for i := range catalog.Series {
		for j := range catalog.Series[i].Seasons {
			for k := range catalog.Series[i].Seasons[j].Episodes {
				episode := &catalog.Series[i].Seasons[j].Episodes[k]
				catalog.byID[episode.ID] = episode
			}
		}
	}
	for _, ids := range duplicates {
		if len(ids) < 2 {
			continue
		}
		for _, id := range ids {
			episode := catalog.byID[id]
			reason := "Duplicate episode number"
			episode.UnavailableReason = &reason
			episode.Status = "unavailable"
			episode.PreparationStage = "Cannot prepare"
		}
	}
	return catalog
}

func ensureReadableRoot(root string) error {
	info, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("media root is not a directory")
	}
	_, err = os.ReadDir(root)
	return err
}

func Scan(root string) ([]episodeFile, error) {
	return scanMedia(root)
}

func NewCatalog(files []episodeFile) *Catalog {
	return buildCatalog(files)
}

func (catalog *Catalog) Count() int {
	return catalog.count
}

func (catalog *Catalog) Failures() int {
	return catalog.failures
}
