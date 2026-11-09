import type { Episode, EpisodeWithSeason, Series } from '../domain/media'
import { allEpisodes } from './catalog'

export interface PlaybackProgress {
  position: number
  duration: number
  updatedAt: number
}

export interface ContinueWatchingItem {
  show: Series
  episode: EpisodeWithSeason
  progress: PlaybackProgress
}

export function readProgress(id: string): PlaybackProgress | null {
  try {
    const saved: unknown = JSON.parse(localStorage.getItem(id) || 'null')
    if (typeof saved !== 'object' || saved === null || !('position' in saved) || !('duration' in saved) || !Number.isFinite(saved.position) || !Number.isFinite(saved.duration)) return null
    return { position: Number(saved.position), duration: Number(saved.duration), updatedAt: 'updatedAt' in saved && Number.isFinite(saved.updatedAt) ? Number(saved.updatedAt) : 0 }
  } catch {
    return null
  }
}

export function writeProgress(id: string, position: number, duration: number): void {
  if (!Number.isFinite(position) || !Number.isFinite(duration) || duration <= 0) return
  try {
    localStorage.setItem(id, JSON.stringify({ position, duration, updatedAt: Date.now() } satisfies PlaybackProgress))
  } catch {
    return
  }
}

export function clearProgress(id: string): void {
  try {
    localStorage.removeItem(id)
  } catch {
    return
  }
}

export function resumePosition(episode: Episode): number {
  const saved = readProgress(episode.id)
  if (!saved) return 0
  const duration = episode.durationSeconds || saved.duration
  return saved.position >= 30 && duration - saved.position >= 30 ? saved.position : 0
}

export function progressPercent(episode: Episode): number {
  const saved = readProgress(episode.id)
  if (!saved || !resumePosition(episode)) return 0
  return Math.min(100, saved.position / (episode.durationSeconds || saved.duration) * 100)
}

export function continueWatchingItems(series: Series[]): ContinueWatchingItem[] {
  return series.flatMap((show) => {
    const latest = allEpisodes(show).flatMap((episode) => {
      const progress = readProgress(episode.id)
      return episode.status === 'ready' && progress && resumePosition(episode) ? [{ show, episode, progress }] : []
    }).sort((a, b) => b.progress.updatedAt - a.progress.updatedAt || b.episode.seasonNumber - a.episode.seasonNumber || b.episode.number - a.episode.number)[0]
    return latest ? [latest] : []
  }).sort((a, b) => b.progress.updatedAt - a.progress.updatedAt || a.show.title.localeCompare(b.show.title))
}
