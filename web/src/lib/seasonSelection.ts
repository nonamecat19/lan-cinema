import type { Series } from '../domain/media'

function storageKey(id: string): string {
  return `frame:season:${id}`
}

export function readSelectedSeason(series: Series): number | undefined {
  const seasons = availableSeasonNumbers(series)
  try {
    const stored = sessionStorage.getItem(storageKey(series.id))
    if (stored !== null) {
      const saved = Number(stored)
      if (seasons.includes(saved)) return saved
    }
    return seasons[0]
  } catch {
    return seasons[0]
  }
}

export function availableSeasonNumbers(series: Series): number[] {
  return Array.from(new Set([
    ...series.seasons.map((season) => season.number),
    ...(series.torrentSources || []).flatMap((source) => source.seasons.map((season) => season.number)),
  ])).sort((a, b) => a - b)
}

export function writeSelectedSeason(id: string, season: number): void {
  try {
    sessionStorage.setItem(storageKey(id), String(season))
  } catch {
    return
  }
}
