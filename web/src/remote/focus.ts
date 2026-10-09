import type { Direction } from './keys'

export function focusableItems(): HTMLElement[] {
  return Array.from(document.querySelectorAll<HTMLElement>('[data-focusable="true"]')).filter((element) => {
    const style = window.getComputedStyle(element)
    return !('disabled' in element && element.disabled) && element.getAttribute('aria-hidden') !== 'true' && style.display !== 'none' && style.visibility !== 'hidden' && element.getClientRects().length > 0
  })
}

export function closestItem(items: HTMLElement[], current: HTMLElement, direction: Direction): HTMLElement | null {
  const rect = current.getBoundingClientRect()
  const x = rect.left + rect.width / 2
  const y = rect.top + rect.height / 2
  const horizontal = direction === 'left' || direction === 'right'
  return items.filter((item) => item !== current).map((item) => {
    const bounds = item.getBoundingClientRect()
    const dx = bounds.left + bounds.width / 2 - x
    const dy = bounds.top + bounds.height / 2 - y
    const forward = direction === 'right' ? dx > 2 : direction === 'left' ? dx < -2 : direction === 'down' ? dy > 2 : dy < -2
    const axisDistance = horizontal ? Math.abs(dx) : Math.abs(dy)
    const crossDistance = horizontal ? Math.abs(dy) : Math.abs(dx)
    const overlap = horizontal ? bounds.top < rect.bottom && bounds.bottom > rect.top : bounds.left < rect.right && bounds.right > rect.left
    return { item, forward, score: axisDistance + crossDistance * (overlap ? 1.5 : 3) }
  }).filter(({ forward }) => forward).sort((a, b) => a.score - b.score)[0]?.item || null
}
