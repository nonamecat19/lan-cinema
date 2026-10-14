import { cleanup, fireEvent, renderHook, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { useFullscreen } from './fullscreen'

describe('useFullscreen', () => {
  afterEach(() => {
    cleanup()
    vi.restoreAllMocks()
    Reflect.deleteProperty(document.documentElement, 'requestFullscreen')
    Reflect.deleteProperty(document.documentElement, 'webkitRequestFullscreen')
    Reflect.deleteProperty(document, 'fullscreenElement')
  })

  it('tries on entry and retries from a user gesture after denial', async () => {
    const request = vi.fn().mockRejectedValueOnce(new Error('gesture required')).mockResolvedValue(undefined)
    Object.defineProperty(document.documentElement, 'requestFullscreen', { configurable: true, value: request })
    renderHook(() => useFullscreen(false))
    expect(request).toHaveBeenCalledTimes(1)
    await waitFor(() => { fireEvent.keyDown(document, { key: '5' }); expect(request).toHaveBeenCalledTimes(2) })
    Object.defineProperty(document, 'fullscreenElement', { configurable: true, value: document.documentElement })
    fireEvent.pointerDown(document.body)
    expect(request).toHaveBeenCalledTimes(2)
  })

  it('tries again on player entry and keeps working without the API', () => {
    const request = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(document.documentElement, 'requestFullscreen', { configurable: true, value: request })
    const view = renderHook(({ watching }) => useFullscreen(watching), { initialProps: { watching: false } })
    view.rerender({ watching: true })
    expect(request).toHaveBeenCalledTimes(2)
    view.unmount()
    Reflect.deleteProperty(document.documentElement, 'requestFullscreen')
    expect(() => renderHook(() => useFullscreen(true))).not.toThrow()
  })

  it('uses the WebKit fullscreen API when it is the only available API', () => {
    const request = vi.fn()
    Object.defineProperty(document.documentElement, 'webkitRequestFullscreen', { configurable: true, value: request })
    renderHook(() => useFullscreen(true))
    expect(request).toHaveBeenCalledOnce()
  })
})
