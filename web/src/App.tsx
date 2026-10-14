import { useCallback, useEffect } from 'react'
import { Button, StateScreen } from './components/Chrome'
import { useCatalog } from './hooks/useCatalog'
import { rememberFocus, useRemoteNavigation } from './remote/useRemoteNavigation'
import { remoteAction } from './remote/keys'
import { useFullscreen } from './remote/fullscreen'
import { useRoute } from './hooks/useRoute'
import { findEpisode } from './lib/catalog'
import { readSelectedSeason, writeSelectedSeason } from './lib/seasonSelection'
import { Downloads } from './screens/Downloads'
import { Library } from './screens/Library'
import { Player, UnavailableEpisode } from './screens/Player'
import { Series } from './screens/Series'

export function App() {
  const { path, route, navigate, goBack } = useRoute()
  const { catalog, error, loading, reload } = useCatalog(route.screen)
  const series = route.screen === 'series' ? catalog?.series.find((item) => item.id === route.id) : null
  const found = route.screen === 'watch' && catalog ? findEpisode(catalog.series, route.id) : null
  const playerReady = route.screen === 'watch' && found?.episode.status === 'ready'
  useFullscreen(route.screen === 'watch')
  const initialFocusKey = route.screen === 'home' ? 'featured-series' : route.screen === 'downloads' ? 'upload-torrent' : route.screen === 'series' && series ? `season-${series.id}-${readSelectedSeason(series)}` : undefined
  useRemoteNavigation(!playerReady, path, initialFocusKey)

  const returnToSeries = useCallback(() => {
    if (found) {
      writeSelectedSeason(found.series.id, found.season.number)
      rememberFocus(`/series/${found.series.id}`, `episode-${found.episode.id}`)
    }
    goBack(found ? `/series/${found.series.id}` : '/')
  }, [found, goBack])

  useEffect(() => {
    const onBack = (event: KeyboardEvent) => {
      if (remoteAction(event)?.kind !== 'back') return
      event.preventDefault()
      if (route.screen === 'watch') returnToSeries()
      else if (route.screen !== 'home') goBack('/')
    }
    document.addEventListener('keydown', onBack)
    return () => document.removeEventListener('keydown', onBack)
  }, [route.screen, goBack, returnToSeries])

  if (loading && !catalog) return <StateScreen loading eyebrow="FRAME / LIBRARY" title="Preparing your library" description="Your collection will be ready in a moment." />
  if (error && !catalog && route.screen !== 'downloads') return <StateScreen eyebrow="LIBRARY OFFLINE" title="We couldn’t reach your library" description="Check that the server is running, then try again." action={<Button focusKey="catalog-retry" onClick={reload}>Try again</Button>} />
  if (route.screen === 'downloads') return <Downloads navigate={navigate} goBack={goBack} />
  if (route.screen === 'series') return series ? <Series key={series.id} series={series} navigate={navigate} goBack={goBack} reload={reload} /> : <StateScreen eyebrow="SERIES NOT FOUND" title="This series is unavailable" action={<Button focusKey="series-missing-back" onClick={() => goBack('/')}>Browse library</Button>} />
  if (route.screen === 'watch') return playerReady ? <Player key={found.episode.id} {...found} navigate={navigate} back={returnToSeries} /> : <UnavailableEpisode episode={found?.episode} navigate={returnToSeries} />
  if (!catalog?.series.length) return <StateScreen eyebrow="YOUR LIBRARY" title="Nothing here yet" description="Add a series to your library or download one to get started." action={<Button focusKey="empty-downloads" onClick={() => navigate('/downloads')}>Open downloads</Button>} />
  return <Library series={catalog.series} navigate={navigate} />
}
