import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Episode, Series } from '../domain/media'
import { readProgress, writeProgress } from '../lib/progress'
import { Player } from './Player'

function episode(id: string, number: number, status: Episode['status'] = 'ready'): Episode {
  return { id, number, title: id, durationSeconds: 120, status, unavailableReason: null, streamUrl: `/stream/${id}`, previewUrl: null, previewSpriteUrl: null, previewIntervalSeconds: 10, previewColumns: 10, previewRows: 1 }
}

const series: Series = { id: 'show', title: 'Show', seasons: [
  { number: 1, episodes: [episode('first', 1), episode('unavailable', 2, 'unavailable')] },
  { number: 2, episodes: [episode('next', 1)] },
] }

function renderPlayer(id = 'first') {
  const navigate = vi.fn()
  const back = vi.fn()
  const season = series.seasons.find((item) => item.episodes.some((entry) => entry.id === id))!
  const current = season.episodes.find((entry) => entry.id === id)!
  const view = render(<Player episode={current} season={season} series={series} navigate={navigate} back={back} />)
  const video = view.container.querySelector('video')!
  Object.defineProperty(video, 'duration', { configurable: true, value: 120 })
  return { ...view, video, navigate, back }
}

describe('Player', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.spyOn(HTMLMediaElement.prototype, 'load').mockImplementation(() => {})
    vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
    vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(() => {})
    Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', { configurable: true, value: vi.fn() })
    vi.spyOn(HTMLElement.prototype, 'getClientRects').mockImplementation(() => [new DOMRect(0, 0, 100, 40)] as unknown as DOMRectList)
  })

  afterEach(() => {
    cleanup()
    vi.useRealTimers()
    vi.restoreAllMocks()
    Reflect.deleteProperty(HTMLElement.prototype, 'scrollIntoView')
    localStorage.clear()
  })

  it('resumes unfinished progress before autoplay and saves on pause', () => {
    writeProgress('first', 55, 120)
    const { video } = renderPlayer()
    fireEvent.loadedMetadata(video)
    expect(video.currentTime).toBe(55)
    expect(HTMLMediaElement.prototype.play).toHaveBeenCalledOnce()
    video.currentTime = 64
    fireEvent.pause(video)
    expect(readProgress('first')?.position).toBe(64)
  })

  it('starts at zero for nearly completed progress and works without storage', () => {
    writeProgress('first', 100, 120)
    const { video } = renderPlayer()
    fireEvent.loadedMetadata(video)
    expect(video.currentTime).toBe(0)
    cleanup()
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('storage denied') })
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('storage denied') })
    const second = renderPlayer()
    expect(() => fireEvent.loadedMetadata(second.video)).not.toThrow()
    expect(HTMLMediaElement.prototype.play).toHaveBeenCalledTimes(2)
  })

  it('toggles playback and clamps ten-second remote seeks', () => {
    const { video } = renderPlayer()
    fireEvent.loadedMetadata(video)
    Object.defineProperty(video, 'paused', { configurable: true, value: true })
    fireEvent.keyDown(document, { key: '5' })
    expect(HTMLMediaElement.prototype.play).toHaveBeenCalledTimes(2)
    Object.defineProperty(video, 'paused', { configurable: true, value: false })
    fireEvent.keyDown(document, { key: 'Enter' })
    expect(HTMLMediaElement.prototype.pause).toHaveBeenCalledOnce()
    video.currentTime = 115
    fireEvent.keyDown(document, { key: '6' })
    expect(video.currentTime).toBe(120)
    video.currentTime = 4
    fireEvent.keyDown(document, { key: '4' })
    expect(video.currentTime).toBe(0)
  })

  it('offers a focused Play action when autoplay is blocked and Retry on stream error', async () => {
    vi.mocked(HTMLMediaElement.prototype.play).mockRejectedValueOnce(new Error('blocked'))
    const { video } = renderPlayer()
    fireEvent.loadedMetadata(video)
    await screen.findByRole('button', { name: /Play episode/ })
    await waitFor(() => expect(screen.getByRole('button', { name: /Play episode/ }).getAttribute('data-remote-active')).toBe('true'))
    fireEvent.keyDown(document, { key: '5' })
    await waitFor(() => expect(screen.queryByRole('button', { name: /Play episode/ })).toBeNull())
    fireEvent.error(video)
    expect(screen.getByRole('alert').textContent).toContain('Playback stopped')
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(HTMLMediaElement.prototype.load).toHaveBeenCalledTimes(2)
  })

  it('counts down to the next ready episode across seasons and skips on OK', async () => {
    const { video, navigate } = renderPlayer()
    fireEvent.loadedMetadata(video)
    fireEvent.ended(video)
    expect(readProgress('first')).toBeNull()
    expect(screen.getByText('Starting in 5 seconds')).toBeTruthy()
    expect(screen.getByText('next')).toBeTruthy()
    await waitFor(() => expect(screen.getByRole('button', { name: 'Play now' }).getAttribute('data-remote-active')).toBe('true'))
    fireEvent.keyDown(document, { key: '5' })
    expect(navigate).toHaveBeenCalledWith('/watch/next', true)
  })

  it('advances at countdown zero and offers Replay at the final episode', () => {
    const { video, navigate } = renderPlayer()
    fireEvent.loadedMetadata(video)
    vi.useFakeTimers()
    fireEvent.ended(video)
    for (let second = 0; second < 5; second++) act(() => { vi.advanceTimersByTime(1000) })
    expect(navigate).toHaveBeenCalledWith('/watch/next', true)
    cleanup()
    vi.useRealTimers()
    const final = renderPlayer('next')
    fireEvent.loadedMetadata(final.video)
    fireEvent.ended(final.video)
    expect(screen.getByText('You’re all caught up')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: 'Replay' }))
    expect(final.video.currentTime).toBe(0)
    expect(HTMLMediaElement.prototype.play).toHaveBeenCalledTimes(3)
  })

  it('returns from the next-episode prompt', () => {
    const { video, back } = renderPlayer()
    fireEvent.loadedMetadata(video)
    fireEvent.ended(video)
    fireEvent.click(screen.getByRole('button', { name: 'Browse episodes' }))
    expect(back).toHaveBeenCalledOnce()
  })
})
