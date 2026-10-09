import type { EpisodeLocation, EpisodeWithSeason, Series } from '../domain/media'

export function allEpisodes(series: Series): EpisodeWithSeason[] {
  return series.seasons.flatMap((season) => season.episodes.map((episode) => ({ ...episode, seasonNumber: season.number })))
}

export function nextReadyEpisode(series: Series, currentId: string): EpisodeWithSeason | null {
  const episodes = allEpisodes(series)
  const index = episodes.findIndex((episode) => episode.id === currentId)
  return index < 0 ? null : episodes.slice(index + 1).find((episode) => episode.status === 'ready') || null
}

export function findEpisode(series: Series[], id: string): EpisodeLocation | null {
  for (const show of series) {
    for (const season of show.seasons) {
      const episode = season.episodes.find((item) => item.id === id)
      if (episode) return { episode, season, series: show }
    }
  }
  return null
}

export function episodeCount(series: Series): number {
  return series.seasons.reduce((total, season) => total + season.episodes.length, 0)
}

export function artworkFor(series: Series): string | null {
  return allEpisodes(series).find((episode) => episode.previewUrl)?.previewUrl || null
}
