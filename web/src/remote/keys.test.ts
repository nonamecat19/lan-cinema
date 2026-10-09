import { describe, expect, it } from 'vitest'
import { remoteAction } from './keys'

function keydown(key: string, code = ''): KeyboardEvent {
  return new KeyboardEvent('keydown', { key, code })
}

describe('remoteAction', () => {
  it('maps directional digits, select, and back', () => {
    expect(remoteAction(keydown('2'))).toEqual({ kind: 'move', direction: 'up' })
    expect(remoteAction(keydown('4'))).toEqual({ kind: 'move', direction: 'left' })
    expect(remoteAction(keydown('6'))).toEqual({ kind: 'move', direction: 'right' })
    expect(remoteAction(keydown('8'))).toEqual({ kind: 'move', direction: 'down' })
    expect(remoteAction(keydown('5'))).toEqual({ kind: 'select' })
    expect(remoteAction(keydown('0'))).toEqual({ kind: 'back' })
    expect(remoteAction(keydown('Unidentified', 'Numpad0'))).toEqual({ kind: 'back' })
  })

  it('leaves text input and modifier shortcuts alone', () => {
    const input = document.createElement('input')
    input.type = 'text'
    let action: ReturnType<typeof remoteAction> = null
    input.addEventListener('keydown', (event) => { action = remoteAction(event) })
    input.dispatchEvent(new KeyboardEvent('keydown', { key: '0', bubbles: true }))
    expect(action).toBeNull()
    expect(remoteAction(new KeyboardEvent('keydown', { key: '5', ctrlKey: true }))).toBeNull()
  })
})
