import { useCallback, useEffect, useMemo, useState } from 'react'

export type Screen = 'home' | 'downloads' | 'series' | 'watch' | 'remote'
export type Route = { screen: 'home' } | { screen: 'downloads' } | { screen: 'series'; id: string; seasonNumber?: number } | { screen: 'watch'; id: string; seriesId?: string; seasonNumber?: number } | { screen: 'remote' }
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
  const normalizedPath = path.length > 1 ? path.replace(/\/+$/, '') : path
  if (normalizedPath === '/downloads') return { screen: 'downloads' }
  if (normalizedPath === '/remote') return { screen: 'remote' }
  const nestedWatchMatch = normalizedPath.match(/^\/series\/([^/]+)\/season\/([^/]+)\/watch\/([^/]+)$/)
  if (nestedWatchMatch) {
    const seriesId = decodeSegment(nestedWatchMatch[1])
    const id = decodeSegment(nestedWatchMatch[3])
    if (seriesId === null || id === null) return { screen: 'home' }
    return { screen: 'watch', id, seriesId, seasonNumber: Number(nestedWatchMatch[2]) }
  }
  const seasonMatch = normalizedPath.match(/^\/series\/([^/]+)\/season\/([^/]+)$/)
  if (seasonMatch) {
    const id = decodeSegment(seasonMatch[1])
    if (id === null) return { screen: 'home' }
    return { screen: 'series', id, seasonNumber: Number(seasonMatch[2]) }
  }
  const match = normalizedPath.match(/^\/(series|watch)\/([^/]+)$/)
  if (!match) return { screen: 'home' }
  const id = decodeSegment(match[2])
  if (id === null) return { screen: 'home' }
  return match[1] === 'series' ? { screen: 'series', id } : { screen: 'watch', id }
}

function decodeSegment(segment: string): string | null {
  try {
    return decodeURIComponent(segment)
  } catch {
    return null
  }
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

  const route = useMemo(() => parseRoute(path), [path])

  return { path, route, navigate, goBack }
}
