const displayIDKey = 'hisense.remote.displayId'
const selectedDisplayKey = 'hisense.remote.selectedDisplayId'

let memoryDisplayID: string | null = null
let memorySelectedDisplayID: string | null = null

function randomDisplayID(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(16))
  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('')
}

export function isMobileDevice(): boolean {
  const userAgentData = (navigator as Navigator & { userAgentData?: { mobile?: boolean } }).userAgentData
  return userAgentData?.mobile === true || (window.matchMedia('(pointer: coarse)').matches && window.innerWidth <= 768)
}

export function getRemoteDisplayId(): string {
  if (memoryDisplayID) return memoryDisplayID
  try {
    const stored = sessionStorage.getItem(displayIDKey)
    const navigation = performance.getEntriesByType('navigation')[0] as PerformanceNavigationTiming | undefined
    if (navigation?.type === 'reload' && stored && /^[A-Za-z0-9_-]{1,128}$/.test(stored)) {
      memoryDisplayID = stored
      return stored
    }
  } catch {
    memoryDisplayID = randomDisplayID()
    return memoryDisplayID
  }
  memoryDisplayID = randomDisplayID()
  try {
    sessionStorage.setItem(displayIDKey, memoryDisplayID)
  } catch {
    return memoryDisplayID
  }
  return memoryDisplayID
}

export function getSelectedRemoteDisplayId(): string | null {
  try {
    return localStorage.getItem(selectedDisplayKey) || memorySelectedDisplayID
  } catch {
    return memorySelectedDisplayID
  }
}

export function setSelectedRemoteDisplayId(id: string | null): void {
  memorySelectedDisplayID = id
  try {
    if (id === null) localStorage.removeItem(selectedDisplayKey)
    else localStorage.setItem(selectedDisplayKey, id)
  } catch {
    return
  }
}
