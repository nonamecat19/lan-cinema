export type EpisodeStatus = 'ready' | 'preparing' | 'queued' | 'unavailable'

export interface Episode {
  id: string
  number: number
  title: string
  durationSeconds: number | null
  status: EpisodeStatus
  preparationStage?: string
  preparationProgress?: number
  unavailableReason: string | null
  streamUrl: string | null
  previewUrl: string | null
  previewSpriteUrl: string | null
  previewIntervalSeconds: number
  previewColumns: number
  previewRows: number
}

export interface Season {
  number: number
  episodes: Episode[]
}

export interface TorrentSeasonOption {
  number: number
  episodes: number
  selected: boolean
  paused: boolean
  deleting: boolean
}

export interface TorrentSeriesEpisode {
  season: number
  number: number
  name: string
}

export interface TorrentSeriesSource {
  id: string
  hash: string
  name: string
  forceOneByOne: boolean
  seasons: TorrentSeasonOption[]
  availableSeasons: TorrentSeasonOption[]
  episodes?: TorrentSeriesEpisode[]
}

export interface Series {
  id: string
  title: string
  seasons: Season[]
  torrentSources?: TorrentSeriesSource[]
}

export interface Catalog {
  series: Series[]
}

export interface EpisodeWithSeason extends Episode {
  seasonNumber: number
}

export interface EpisodeLocation {
  episode: Episode
  season: Season
  series: Series
}
