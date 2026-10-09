import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { useRemoteNavigation } from './useRemoteNavigation'

function Harness({ showSecond, onFirst, onSecond }: { showSecond: boolean; onFirst: () => void; onSecond: () => void }) {
  useRemoteNavigation(true, 'focus-repair-test', 'second')
  return <main><button data-focusable="true" data-focus-key="first" onClick={onFirst}>First</button>{showSecond && <button data-focusable="true" data-focus-key="second" onClick={onSecond}>Second</button>}</main>
}

describe('useRemoteNavigation', () => {
  afterEach(() => { cleanup(); vi.restoreAllMocks(); vi.unstubAllGlobals(); Reflect.deleteProperty(HTMLElement.prototype, 'scrollIntoView') })

  it('repairs focus when the selected item disappears', async () => {
    vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => window.setTimeout(() => callback(performance.now()), 0))
    vi.stubGlobal('cancelAnimationFrame', (id: number) => window.clearTimeout(id))
    vi.spyOn(HTMLElement.prototype, 'getClientRects').mockImplementation(() => [new DOMRect(0, 0, 100, 40)] as unknown as DOMRectList)
    Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', { configurable: true, value: vi.fn() })
    const onFirst = vi.fn()
    const onSecond = vi.fn()
    const view = render(<Harness showSecond onFirst={onFirst} onSecond={onSecond} />)
    await waitFor(() => expect(screen.getByRole('button', { name: 'Second' }).getAttribute('data-remote-active')).toBe('true'))
    view.rerender(<Harness showSecond={false} onFirst={onFirst} onSecond={onSecond} />)
    await waitFor(() => expect(screen.getByRole('button', { name: 'First' }).getAttribute('data-remote-active')).toBe('true'))
    act(() => { fireEvent.keyDown(document, { key: '5' }) })
    expect(onFirst).toHaveBeenCalledOnce()
    expect(onSecond).not.toHaveBeenCalled()
  })
})
