export type Direction = 'up' | 'down' | 'left' | 'right'
export type RemoteAction = { kind: 'move'; direction: Direction } | { kind: 'select' } | { kind: 'back' } | null

const directionKeys: Record<string, Direction> = {
  ArrowUp: 'up', ArrowDown: 'down', ArrowLeft: 'left', ArrowRight: 'right',
  '2': 'up', '8': 'down', '4': 'left', '6': 'right',
  Numpad2: 'up', Numpad8: 'down', Numpad4: 'left', Numpad6: 'right',
}

const legacyDirections: Record<number, Direction> = {
  38: 'up', 40: 'down', 37: 'left', 39: 'right',
  50: 'up', 56: 'down', 52: 'left', 54: 'right',
}

export function isEditingTarget(target: EventTarget | null): boolean {
  return target instanceof HTMLElement && (target.isContentEditable || target instanceof HTMLTextAreaElement || target instanceof HTMLInputElement && !['checkbox', 'radio', 'button', 'submit', 'file'].includes(target.type))
}

export function remoteAction(event: KeyboardEvent): RemoteAction {
  if (event.altKey || event.ctrlKey || event.metaKey || isEditingTarget(event.target)) return null
  const direction = directionKeys[event.key] || directionKeys[event.code] || legacyDirections[event.keyCode]
  if (direction) return { kind: 'move', direction }
  if (['Enter', ' ', 'OK', 'Accept', 'Select', '5', 'Numpad5'].includes(event.key) || event.code === 'Numpad5' || [13, 53].includes(event.keyCode)) return { kind: 'select' }
  if (['Escape', 'Backspace', 'BrowserBack', 'Back', 'GoBack', '0', 'Numpad0'].includes(event.key) || event.code === 'Numpad0' || [8, 48, 461, 10009].includes(event.keyCode)) return { kind: 'back' }
  return null
}
