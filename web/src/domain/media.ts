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
}

export interface TorrentSeriesSource {
  id: string
  name: string
  availableSeasons: TorrentSeasonOption[]
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
