import { useEffect } from 'react'

type TVDocument = Document & { webkitFullscreenElement?: Element | null }
type TVElement = HTMLElement & { webkitRequestFullscreen?: () => Promise<void> | void }

export function useFullscreen(watching: boolean): void {
  useEffect(() => {
    let pending = false
    const request = () => {
      const root = document.documentElement as TVElement
      const enter = root.requestFullscreen || root.webkitRequestFullscreen
      if (pending || document.fullscreenElement || (document as TVDocument).webkitFullscreenElement || !enter) return
      pending = true
      try {
        void Promise.resolve(enter.call(root)).catch(() => {}).finally(() => { pending = false })
      } catch {
        pending = false
      }
    }
    const onKeyDown = (event: KeyboardEvent) => {
      if (['Escape', 'Backspace', 'BrowserBack', 'Back', 'GoBack', '0', 'Numpad0'].includes(event.key)) return
      request()
    }
    document.addEventListener('keydown', onKeyDown, true)
    document.addEventListener('pointerdown', request, true)
    request()
    return () => {
      document.removeEventListener('keydown', onKeyDown, true)
      document.removeEventListener('pointerdown', request, true)
    }
  }, [watching])
}
