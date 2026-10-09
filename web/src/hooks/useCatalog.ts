import { useCallback, useEffect, useRef, useState } from 'react'
import type { Catalog } from '../domain/media'
import { api, ApiError, errorMessage } from '../lib/api'
import type { Screen } from './useRoute'

export function useCatalog(screen: Screen) {
  const [catalog, setCatalog] = useState<Catalog | null>(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const previousScreen = useRef(screen)
  const mounted = useRef(false)
  const inFlight = useRef<Promise<void> | null>(null)

  const reload = useCallback((): Promise<void> => {
    if (inFlight.current) return inFlight.current
    const request = api.catalog().then((result) => {
      if (!mounted.current) return
      setCatalog(result)
      setError('')
      setLoading(false)
    }).catch((reason: unknown) => {
      if (!mounted.current) return
      if (reason instanceof ApiError && reason.status === 503) {
        setLoading(true)
        setError('')
      } else {
        setError(errorMessage(reason))
        setLoading(false)
      }
    }).finally(() => {
      inFlight.current = null
    })
    inFlight.current = request
    return request
  }, [])

  useEffect(() => {
    mounted.current = true
    void reload()
    const interval = window.setInterval(() => { void reload() }, 3000)
    return () => { mounted.current = false; window.clearInterval(interval) }
  }, [reload])

  useEffect(() => {
    if (previousScreen.current === 'downloads' && screen !== 'downloads') void reload()
    previousScreen.current = screen
  }, [screen, reload])

  return { catalog, error, loading, reload }
}
