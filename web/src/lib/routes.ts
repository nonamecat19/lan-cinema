export function seriesPath(seriesId: string): string {
  return `/series/${encodeURIComponent(seriesId)}`
}

export function seriesSeasonPath(seriesId: string, seasonNumber: number): string {
  return `${seriesPath(seriesId)}/season/${seasonNumber}`
}

export function episodeWatchPath(seriesId: string, seasonNumber: number, episodeId: string): string {
  return `${seriesSeasonPath(seriesId, seasonNumber)}/watch/${encodeURIComponent(episodeId)}`
}
