import type { Series } from '../domain/media'

function storageKey(id: string): string {
  return `frame:season:${id}`
}

export function readSelectedSeason(series: Series): number | undefined {
  try {
    const saved = Number(sessionStorage.getItem(storageKey(series.id)))
    return series.seasons.find((season) => season.number === saved)?.number ?? series.seasons[0]?.number
  } catch {
    return series.seasons[0]?.number
  }
}

export function writeSelectedSeason(id: string, season: number): void {
  try {
    sessionStorage.setItem(storageKey(id), String(season))
  } catch {
    return
  }
}
