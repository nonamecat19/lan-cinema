import { useEffect, useRef } from 'react'
import { closestItem, focusableItems } from './focus'
import { remoteAction } from './keys'

const focusByView = new Map<string, string>()

export function rememberFocus(viewKey: string, focusKey: string): void {
  focusByView.set(viewKey, focusKey)
}

export function useRemoteNavigation(active: boolean, viewKey: string, initialFocusKey?: string): void {
  const selected = useRef<HTMLElement | null>(null)

  useEffect(() => {
    if (!active) {
      selected.current?.removeAttribute('data-remote-active')
      selected.current = null
      return
    }

    const select = (item: HTMLElement | null) => {
      if (!item) return
      if (selected.current !== item) selected.current?.removeAttribute('data-remote-active')
      item.setAttribute('data-remote-active', 'true')
      selected.current = item
      if (item.dataset.focusKey) focusByView.set(viewKey, item.dataset.focusKey)
      if (document.activeElement !== item) item.focus({ preventScroll: true })
      item.scrollIntoView({ block: 'nearest', inline: 'nearest', behavior: 'auto' })
    }

    const restore = () => {
      const items = focusableItems()
      if (selected.current && items.includes(selected.current)) return
      const key = focusByView.get(viewKey)
      select(items.find((item) => item.dataset.focusKey === key) || items.find((item) => item.dataset.focusKey === initialFocusKey) || items[0] || null)
    }

    const onKeyDown = (event: KeyboardEvent) => {
      const action = remoteAction(event)
      if (!action || action.kind === 'back') return
      const items = focusableItems()
      const current = selected.current && items.includes(selected.current) ? selected.current : items.find((item) => item.dataset.focusKey === focusByView.get(viewKey)) || items[0]
      if (!current) return
      event.preventDefault()
      if (action.kind === 'select') {
        select(current)
        current.click()
        return
      }
      if (current.classList.contains('season-tab') && (action.direction === 'left' || action.direction === 'right')) {
        const tabs = items.filter((item) => item.classList.contains('season-tab'))
        const next = tabs[(tabs.indexOf(current) + (action.direction === 'right' ? 1 : -1) + tabs.length) % tabs.length]
        select(next)
        next.click()
        return
      }
      select(closestItem(items, current, action.direction) || current)
    }

    const onFocus = (event: Event) => {
      const target = event.target
      const item = target instanceof Element ? target.closest<HTMLElement>('[data-focusable="true"]') : null
      if (item && focusableItems().includes(item) && item !== selected.current) select(item)
    }

    let animation = requestAnimationFrame(restore)
    const observer = new MutationObserver(() => {
      cancelAnimationFrame(animation)
      animation = requestAnimationFrame(restore)
    })
    observer.observe(document.body, { subtree: true, childList: true, attributes: true, attributeFilter: ['disabled', 'hidden', 'aria-hidden', 'style', 'class'] })
    document.addEventListener('keydown', onKeyDown)
    document.addEventListener('focusin', onFocus)
    document.addEventListener('pointerdown', onFocus)
    return () => {
      cancelAnimationFrame(animation)
      observer.disconnect()
      document.removeEventListener('keydown', onKeyDown)
      document.removeEventListener('focusin', onFocus)
      document.removeEventListener('pointerdown', onFocus)
      selected.current?.removeAttribute('data-remote-active')
      selected.current = null
    }
  }, [active, viewKey, initialFocusKey])
}
