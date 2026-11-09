import type { TorrentDownload, TorrentDownloadFile } from '../domain/downloads'
import type { Episode, Series, TorrentSeasonOption, TorrentSeriesSource } from '../domain/media'

export type StatusKey = 'available' | 'waiting' | 'downloading' | 'paused' | 'importing' | 'preparing' | 'ready' | 'attention' | 'unavailable'

export interface DisplayStatus {
  key: StatusKey
  label: string
  icon: string
}

export interface TorrentEpisodeStatus {
  sourceId: string
  sourceName: string
  status: DisplayStatus
  detail: string
}

export interface SeriesEpisodeRow {
  number: number
  episode?: Episode
  title: string
  torrentStatuses: TorrentEpisodeStatus[]
}

const statuses: Record<StatusKey, DisplayStatus> = {
  available: { key: 'available', label: 'Available', icon: '○' },
  waiting: { key: 'waiting', label: 'Waiting', icon: '…' },
  downloading: { key: 'downloading', label: 'Downloading', icon: '↓' },
  paused: { key: 'paused', label: 'Paused', icon: 'Ⅱ' },
  importing: { key: 'importing', label: 'Importing', icon: '⇢' },
  preparing: { key: 'preparing', label: 'Preparing', icon: '◌' },
  ready: { key: 'ready', label: 'Ready to watch', icon: '✓' },
  attention: { key: 'attention', label: 'Needs attention', icon: '!' },
  unavailable: { key: 'unavailable', label: 'Transfer status unavailable', icon: '?' },
}

const statusPriority: Record<StatusKey, number> = {
  attention: 0,
  unavailable: 0,
  downloading: 1,
  importing: 2,
  preparing: 3,
  paused: 4,
  waiting: 5,
  ready: 6,
  available: 7,
}

function statusForFile(file: TorrentDownloadFile): DisplayStatus {
  if (file.status === 'waiting') return statuses.waiting
  if (file.status === 'queued') return statuses.waiting
  if (file.status === 'downloading') return statuses.downloading
  if (file.status === 'paused') return statuses.paused
  if (file.status === 'importing') return statuses.importing
  if (file.status === 'preparing') return statuses.preparing
  if (file.status === 'ready') return statuses.ready
  return statuses.attention
}

function higherPriority(left: DisplayStatus, right: DisplayStatus): DisplayStatus {
  return statusPriority[left.key] <= statusPriority[right.key] ? left : right
}

function sourceSeasonStatus(option: TorrentSeasonOption, download: TorrentDownload | undefined): DisplayStatus {
  if (option.deleting) return statuses.attention
  if (!option.selected) return statuses.available
  if (!download) return option.paused ? statuses.paused : statuses.unavailable
  const files = download.files.filter((file) => file.season === option.number)
  if (!files.length) return option.paused || download.paused ? statuses.paused : statuses.waiting
  if (files.every((file) => statusForFile(file).key === 'ready')) return statuses.ready
  return files.map(statusForFile).reduce(higherPriority)
}

function sourceDownload(source: TorrentSeriesSource, downloads: TorrentDownload[]): TorrentDownload | undefined {
  return downloads.find((download) => download.hash === source.hash)
}

function episodeStatusForSource(source: TorrentSeriesSource, seasonNumber: number, episodeNumber: number, downloads: TorrentDownload[]): TorrentEpisodeStatus {
  const option = source.seasons.find((season) => season.number === seasonNumber)
  const download = sourceDownload(source, downloads)
  if (!option?.selected) return { sourceId: source.id, sourceName: source.name, status: statuses.available, detail: 'Season is not selected for download' }
  if (option.deleting) return { sourceId: source.id, sourceName: source.name, status: statuses.attention, detail: 'Season delete needs retry' }
  const files = download?.files.filter((file) => file.season === seasonNumber && file.episode === episodeNumber) || []
  if (!files.length) {
    const status = option.paused || download?.paused ? statuses.paused : statuses.unavailable
    return { sourceId: source.id, sourceName: source.name, status, detail: status.label }
  }
  const status = files.map(statusForFile).reduce(higherPriority)
  const details = [...new Set(files.map((file) => file.detail).filter(Boolean))]
  const progress = files.reduce((value, file) => Math.max(value, file.progress), 0)
  const transferProgress = status.key === 'downloading' || status.key === 'paused' ? ` · ${(progress * 100).toFixed(1)}%` : ''
  return { sourceId: source.id, sourceName: source.name, status, detail: `${details.join(' · ')}${transferProgress}` }
}

export function torrentTitle(fileName: string): string {
  const basename = fileName.split(/[\\/]/).pop() || fileName
  const withoutExtension = basename.replace(/\.[^.]+$/, '')
  const title = withoutExtension.replace(/^S\d{1,3}E\d{1,3}\s*[-._ ]*/i, '').trim()
  return title || basename
}

export function episodeRowsForSeason(series: Series, seasonNumber: number, downloads: TorrentDownload[]): SeriesEpisodeRow[] {
  const librarySeason = series.seasons.find((season) => season.number === seasonNumber)
  const rows = new Map<number, SeriesEpisodeRow>()
  for (const episode of librarySeason?.episodes || []) {
    rows.set(episode.number, { number: episode.number, episode, title: episode.title, torrentStatuses: [] })
  }
  for (const source of series.torrentSources || []) {
    const inventory = (source.episodes || []).filter((episode) => episode.season === seasonNumber)
    const episodeNumbers = new Set<number>()
    for (const episode of inventory) {
      episodeNumbers.add(episode.number)
      if (!rows.has(episode.number)) rows.set(episode.number, { number: episode.number, title: torrentTitle(episode.name), torrentStatuses: [] })
    }
    for (const episodeNumber of episodeNumbers) {
      const row = rows.get(episodeNumber)
      if (row) row.torrentStatuses.push(episodeStatusForSource(source, seasonNumber, episodeNumber, downloads))
    }
  }
  return [...rows.values()].sort((left, right) => left.number - right.number)
}

export function seasonStatusFor(series: Series, seasonNumber: number, downloads: TorrentDownload[]): DisplayStatus {
  const sources = (series.torrentSources || []).filter((source) => source.seasons.some((season) => season.number === seasonNumber))
  if (sources.length) {
    return sources.map((source) => {
      const option = source.seasons.find((season) => season.number === seasonNumber)
      return option ? sourceSeasonStatus(option, sourceDownload(source, downloads)) : statuses.unavailable
    }).reduce(higherPriority)
  }
  const episodes = series.seasons.find((season) => season.number === seasonNumber)?.episodes || []
  if (episodes.some((episode) => episode.status === 'unavailable')) return statuses.attention
  if (episodes.some((episode) => episode.status === 'preparing' || episode.status === 'queued')) return statuses.preparing
  return statuses.ready
}

export function torrentSeasonStatus(source: TorrentSeriesSource, seasonNumber: number, downloads: TorrentDownload[]): DisplayStatus {
  const option = source.seasons.find((season) => season.number === seasonNumber)
  return option ? sourceSeasonStatus(option, sourceDownload(source, downloads)) : statuses.unavailable
}
