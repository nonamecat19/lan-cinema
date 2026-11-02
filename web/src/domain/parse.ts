import type { Catalog, Episode, EpisodeStatus, Season, Series, TorrentSeriesSource } from './media'
import type { TorrentDownload, TorrentDownloadFile, TorrentFile, TorrentSelection, TorrentSource } from './downloads'
import type { RemoteCommand, RemoteDisplay, RemotePlaybackState, RemoteSyncResponse } from './remote'

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

function remoteID(value: unknown): value is string {
  return string(value) && /^[A-Za-z0-9_-]{1,128}$/.test(value)
}

function nonemptyString(value: unknown): value is string {
  return string(value) && value.trim().length > 0
}

function nonnegativeNumber(value: unknown): value is number {
  return number(value) && value >= 0
}

function unitNumber(value: unknown): value is number {
  return number(value) && value >= 0 && value <= 1
}

function remotePlaybackState(value: unknown): value is RemotePlaybackState {
  return record(value) && (value.episodeId === null || remoteID(value.episodeId)) && typeof value.playing === 'boolean' && nonnegativeNumber(value.position) && nonnegativeNumber(value.duration) && unitNumber(value.volume) && typeof value.muted === 'boolean' && nullableString(value.error)
}

function remoteCommand(value: unknown): value is RemoteCommand {
  if (!record(value) || !remoteID(value.id)) return false
  switch (value.type) {
    case 'playEpisode': return Object.keys(value).length === 3 && nonemptyString(value.episodeId)
    case 'setPlaying': return Object.keys(value).length === 3 && typeof value.playing === 'boolean'
    case 'seek': return Object.keys(value).length === 3 && nonnegativeNumber(value.position)
    case 'setMuted': return Object.keys(value).length === 3 && typeof value.muted === 'boolean'
    case 'setVolume': return Object.keys(value).length === 3 && unitNumber(value.volume)
    case 'stop': return Object.keys(value).length === 2
    default: return false
  }
}

function remoteDisplay(value: unknown): value is RemoteDisplay {
  return record(value) && remoteID(value.id) && nonemptyString(value.label) && remotePlaybackState(value.state)
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
  remoteDisplay,
  remoteDisplays: (value: unknown): value is RemoteDisplay[] => array(value, remoteDisplay),
  remoteSync: (value: unknown): value is RemoteSyncResponse => record(value) && array(value.commands, remoteCommand),
  remoteCommandQueued: (value: unknown): value is { queued: true } => record(value) && value.queued === true,
}
