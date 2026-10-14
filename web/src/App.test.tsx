import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Catalog, Episode } from './domain/media'
import { useCatalog } from './hooks/useCatalog'
import { App } from './App'

vi.mock('./hooks/useCatalog', () => ({ useCatalog: vi.fn() }))
vi.mock('./screens/Downloads', () => ({ Downloads: () => null }))

function episode(id: string, number: number, status: Episode['status'] = 'ready'): Episode {
  return { id, number, title: id, durationSeconds: 120, status, unavailableReason: status === 'unavailable' ? 'Cannot prepare' : null, streamUrl: status === 'ready' ? `/stream/${id}` : null, previewUrl: null, previewSpriteUrl: null, previewIntervalSeconds: 10, previewColumns: 10, previewRows: 1 }
}

const catalog: Catalog = { series: [{ id: 'show', title: 'Show', seasons: [
  { number: 1, episodes: [episode('first', 1), episode('blocked', 2, 'unavailable')] },
  { number: 2, episodes: [episode('next', 1)] },
] }] }

describe('App player navigation', () => {
  beforeEach(() => {
    localStorage.clear()
    window.history.replaceState(null, '', '/watch/first')
    vi.mocked(useCatalog).mockReturnValue({ catalog, error: '', loading: false, reload: vi.fn().mockResolvedValue(undefined) })
    vi.spyOn(HTMLMediaElement.prototype, 'load').mockImplementation(() => {})
    vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(() => {})
    vi.spyOn(HTMLElement.prototype, 'getClientRects').mockImplementation(() => [new DOMRect(0, 0, 100, 40)] as unknown as DOMRectList)
    Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', { configurable: true, value: vi.fn() })
  })

  afterEach(() => {
    cleanup()
    vi.useRealTimers()
    vi.restoreAllMocks()
    Reflect.deleteProperty(HTMLElement.prototype, 'scrollIntoView')
    window.history.replaceState(null, '', '/')
    localStorage.clear()
  })

  it('cancels the next-episode countdown on 0 and restores the focused episode', () => {
    const view = render(<App />)
    vi.useFakeTimers()
    fireEvent.ended(view.container.querySelector('video')!)
    expect(screen.getByText('Starting in 5 seconds')).toBeTruthy()
    fireEvent.keyDown(document, { key: '0' })
    expect(window.location.pathname).toBe('/series/show')
    act(() => { vi.advanceTimersByTime(6000) })
    expect(document.querySelector('[data-focus-key="episode-first"]')?.getAttribute('data-remote-active')).toBe('true')
    expect(window.location.pathname).toBe('/series/show')
  })

  it('shows the unavailable reason and returns with Back', () => {
    window.history.replaceState(null, '', '/watch/blocked')
    render(<App />)
    expect(screen.getByText('Cannot prepare')).toBeTruthy()
    fireEvent.keyDown(document, { key: 'Backspace' })
    expect(window.location.pathname).toBe('/series/show')
  })
})
