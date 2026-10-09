import { act, cleanup, renderHook } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { parseRoute, useRoute } from './useRoute'

describe('useRoute', () => {
  afterEach(() => { cleanup(); vi.restoreAllMocks(); window.history.replaceState(null, '', '/') })

  it('uses browser history for an in-app return', () => {
    window.history.replaceState(null, '', '/')
    const back = vi.spyOn(window.history, 'back').mockImplementation(() => undefined)
    const { result } = renderHook(useRoute)
    act(() => result.current.navigate('/series/example'))
    expect(result.current.route).toEqual({ screen: 'series', id: 'example' })
    act(() => result.current.goBack('/'))
    expect(back).toHaveBeenCalledOnce()
  })

  it('falls back to the series for a direct watch link', () => {
    window.history.replaceState(null, '', '/watch/example')
    const back = vi.spyOn(window.history, 'back').mockImplementation(() => undefined)
    const { result } = renderHook(useRoute)
    act(() => result.current.goBack('/series/example'))
    expect(result.current.route).toEqual({ screen: 'series', id: 'example' })
    expect(back).not.toHaveBeenCalled()
  })

  it('handles malformed route encoding', () => {
    expect(parseRoute('/series/%')).toEqual({ screen: 'home' })
  })
})
