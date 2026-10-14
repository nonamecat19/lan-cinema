import { describe, expect, it } from 'vitest'
import type { Episode, Series } from '../domain/media'
import { nextReadyEpisode } from './catalog'

function episode(id: string, number: number, status: Episode['status'] = 'ready'): Episode {
  return { id, number, title: id, durationSeconds: 120, status, unavailableReason: null, streamUrl: `/stream/${id}`, previewUrl: null, previewSpriteUrl: null, previewIntervalSeconds: 10, previewColumns: 10, previewRows: 1 }
}

describe('nextReadyEpisode', () => {
  it('advances by episode and season number, skipping unavailable episodes', () => {
    const series: Series = { id: 'show', title: 'Show', seasons: [
      { number: 2, episodes: [episode('s2e2', 2), episode('s2e1', 1, 'unavailable')] },
      { number: 1, episodes: [episode('s1e3', 3, 'preparing'), episode('s1e2', 2), episode('s1e1', 1)] },
    ] }
    expect(nextReadyEpisode(series, 's1e1')?.id).toBe('s1e2')
    expect(nextReadyEpisode(series, 's1e2')?.id).toBe('s2e2')
    expect(nextReadyEpisode(series, 's2e2')).toBeNull()
  })
})
