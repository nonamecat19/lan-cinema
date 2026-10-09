import type { Catalog, Episode, EpisodeStatus, Season, Series, TorrentSeriesSource } from './media'
import type { TorrentDownload, TorrentDownloadFile, TorrentFile, TorrentSelection, TorrentSource } from './downloads'

type RecordValue = Record<string, unknown>
const episodeStatuses: ReadonlySet<string> = new Set(['ready', 'preparing', 'queued', 'unavailable'])

function record(value: unknown): value is RecordValue {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function string(value: unknown): value is string {
  return typeof value === 'string'
}

function number(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value)
}

function nullableString(value: unknown): value is string | null {
  return value === null || string(value)
}

function nullableNumber(value: unknown): value is number | null {
  return value === null || number(value)
}

function episodeStatus(value: unknown): value is EpisodeStatus {
  return string(value) && episodeStatuses.has(value)
}

function array<T>(value: unknown, guard: (item: unknown) => item is T): value is T[] {
  return Array.isArray(value) && value.every(guard)
}

function episode(value: unknown): value is Episode {
  return record(value) && string(value.id) && number(value.number) && string(value.title) && nullableNumber(value.durationSeconds) && episodeStatus(value.status) && nullableString(value.unavailableReason) && nullableString(value.streamUrl) && nullableString(value.previewUrl) && nullableString(value.previewSpriteUrl) && number(value.previewIntervalSeconds) && number(value.previewColumns) && number(value.previewRows)
}

function season(value: unknown): value is Season {
  return record(value) && number(value.number) && array(value.episodes, episode)
}

function torrentSeriesSource(value: unknown): value is TorrentSeriesSource {
  return record(value) && string(value.id) && string(value.name) && array(value.availableSeasons, (item): item is TorrentSeriesSource['availableSeasons'][number] => record(item) && number(item.number) && number(item.episodes))
}

function series(value: unknown): value is Series {
  return record(value) && string(value.id) && string(value.title) && array(value.seasons, season) && (value.torrentSources === undefined || array(value.torrentSources, torrentSeriesSource))
}

function torrentSource(value: unknown): value is TorrentSource {
  return record(value) && string(value.id) && string(value.name) && string(value.displayName) && string(value.infoHash)
}

function torrentFile(value: unknown): value is TorrentFile {
  return record(value) && number(value.index) && string(value.name) && number(value.size) && number(value.progress) && number(value.priority) && number(value.season)
}

function torrentSelection(value: unknown): value is TorrentSelection {
  return record(value) && string(value.seriesTitle) && array(value.seasons, number) && array(value.files, torrentFile)
}

function torrentDownloadFile(value: unknown): value is TorrentDownloadFile {
  return record(value) && string(value.name) && number(value.season) && number(value.episode) && number(value.progress) && string(value.status) && string(value.detail)
}

function torrentDownload(value: unknown): value is TorrentDownload {
  return record(value) && string(value.hash) && string(value.name) && number(value.progress) && string(value.state) && number(value.downloadSpeed) && number(value.uploadSpeed) && number(value.eta) && number(value.size) && number(value.completed) && typeof value.paused === 'boolean' && array(value.files, torrentDownloadFile)
}

export function parseResponse<T>(value: unknown, guard: (item: unknown) => item is T): T {
  if (!guard(value)) throw new Error('The server returned an unexpected response')
  return value
}

export const responseGuards = {
  catalog: (value: unknown): value is Catalog => record(value) && array(value.series, series),
  torrentSources: (value: unknown): value is TorrentSource[] => array(value, torrentSource),
  torrentSource,
  torrentSelection,
  downloads: (value: unknown): value is TorrentDownload[] => array(value, torrentDownload),
  accepted: (value: unknown): value is RecordValue => record(value),
}
