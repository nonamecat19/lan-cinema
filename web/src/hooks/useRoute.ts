import { useCallback, useEffect, useState } from 'react'

export type Screen = 'home' | 'downloads' | 'series' | 'watch' | 'remote'
export type Route = { screen: 'home' } | { screen: 'downloads' } | { screen: 'series'; id: string } | { screen: 'watch'; id: string } | { screen: 'remote' }
export type Navigate = (path: string, replace?: boolean) => void
export type GoBack = (fallback: string) => void

interface AppHistoryState {
  frameRoute: true
  previous: string
}

function isAppHistoryState(value: unknown): value is AppHistoryState {
  return typeof value === 'object' && value !== null && 'frameRoute' in value && value.frameRoute === true && 'previous' in value && typeof value.previous === 'string'
}

export function parseRoute(path: string): Route {
  if (path === '/downloads') return { screen: 'downloads' }
  if (path === '/remote') return { screen: 'remote' }
  const match = path.match(/^\/(series|watch)\/([^/]+)$/)
  if (!match) return { screen: 'home' }
  let id: string
  try {
    id = decodeURIComponent(match[2])
  } catch {
    return { screen: 'home' }
  }
  return match[1] === 'series' ? { screen: 'series', id } : { screen: 'watch', id }
}

export function useRoute() {
  const [path, setPath] = useState(window.location.pathname)

  useEffect(() => {
    const onPopState = () => setPath(window.location.pathname)
    window.addEventListener('popstate', onPopState)
    return () => window.removeEventListener('popstate', onPopState)
  }, [])

  const navigate = useCallback<Navigate>((next, replace = false) => {
    if (next === window.location.pathname) return
    const previous = replace && isAppHistoryState(window.history.state) ? window.history.state.previous : window.location.pathname
    window.history[replace ? 'replaceState' : 'pushState']({ frameRoute: true, previous } satisfies AppHistoryState, '', next)
    setPath(next)
  }, [])

  const goBack = useCallback<GoBack>((fallback) => {
    const state: unknown = window.history.state
    if (isAppHistoryState(state) && state.previous === fallback) window.history.back()
    else navigate(fallback, true)
  }, [navigate])

  return { path, route: parseRoute(path), navigate, goBack }
}
