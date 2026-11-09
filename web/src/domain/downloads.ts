export interface TorrentSource {
  id: string
  name: string
  displayName: string
  infoHash: string
}

export interface TorrentFile {
  index: number
  name: string
  size: number
  progress: number
  priority: number
  season: number
}

export interface TorrentSelection {
  seriesTitle: string
  seasons: number[]
  forceOneByOne?: boolean
  files: TorrentFile[]
}

export interface TorrentDownloadFile {
  name: string
  season: number
  episode: number
  size: number
  progress: number
  status: string
  detail: string
  preparationProgress?: number
}

export interface TorrentDownload {
  hash: string
  name: string
  progress: number
  state: string
  downloadSpeed: number
  uploadSpeed: number
  eta: number
  size: number
  completed: number
  paused: boolean
  files: TorrentDownloadFile[]
  detail?: string
}
