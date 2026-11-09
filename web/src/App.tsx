import { useCallback, useEffect, useMemo } from 'react'
import { Button, StateScreen } from './components/Chrome'
import { useCatalog } from './hooks/useCatalog'
import { rememberFocus, useRemoteNavigation } from './remote/useRemoteNavigation'
import { remoteAction } from './remote/keys'
import { useFullscreen } from './remote/fullscreen'
import { useRoute } from './hooks/useRoute'
import { findEpisode } from './lib/catalog'
import { isMobileDevice } from './remote/device'
import { useRemoteDisplay } from './remote/useRemoteDisplay'
import { availableSeasonNumbers, readSelectedSeason, writeSelectedSeason } from './lib/seasonSelection'
import { episodeWatchPath, seriesPath, seriesSeasonPath } from './lib/routes'
import { Downloads } from './screens/Downloads'
import { Library } from './screens/Library'
import { Player, UnavailableEpisode } from './screens/Player'
import { Remote } from './screens/Remote'
import { Series } from './screens/Series'

export function App() {
  const { path, route, navigate, goBack } = useRoute()
  const { commands, reportPlaybackState, acknowledgeCommand } = useRemoteDisplay(!isMobileDevice() && path !== '/remote')
  const { catalog, error, loading, reload } = useCatalog(route.screen)
  const series = route.screen === 'series' ? catalog?.series.find((item) => item.id === route.id) : null
  const found = route.screen === 'watch' && catalog ? findEpisode(catalog.series, route.id) : null
  const playerReady = route.screen === 'watch' && found?.episode.status === 'ready'
  useFullscreen(route.screen === 'watch')
  const focusSeason = route.screen === 'series' && series && route.seasonNumber !== undefined && availableSeasonNumbers(series).includes(route.seasonNumber) ? route.seasonNumber : route.screen === 'series' && series ? readSelectedSeason(series) : undefined
  const initialFocusKey = route.screen === 'home' ? 'featured-series' : route.screen === 'downloads' ? 'upload-torrent' : route.screen === 'series' && series && focusSeason !== undefined ? `season-${series.id}-${focusSeason}` : undefined
  useRemoteNavigation(!playerReady, path, initialFocusKey)
  const navigationCommands = useMemo(() => commands.filter((command) => command.type === 'playEpisode' || command.type === 'stop'), [commands])
  const latestNavigationCommand = navigationCommands[navigationCommands.length - 1]
  const currentCommands = commands.filter((command) => (command.type !== 'playEpisode' && command.type !== 'stop') || command.id === latestNavigationCommand?.id)

  useEffect(() => {
    if (!catalog || route.screen !== 'series') return
    if (!series) {
      navigate('/', true)
      return
    }
    const seasonNumbers = availableSeasonNumbers(series)
    if (seasonNumbers.length === 0) {
      if (route.seasonNumber !== undefined) navigate(seriesPath(series.id), true)
      return
    }
    if (route.seasonNumber !== undefined && seasonNumbers.includes(route.seasonNumber)) {
      writeSelectedSeason(series.id, route.seasonNumber)
      const targetPath = seriesSeasonPath(series.id, route.seasonNumber)
      if (path !== targetPath) navigate(targetPath, true)
      return
    }
    const seasonNumber = readSelectedSeason(series)
    if (seasonNumber !== undefined) navigate(seriesSeasonPath(series.id, seasonNumber), true)
  }, [catalog, route, series, navigate, path])

  useEffect(() => {
    if (!catalog || route.screen !== 'watch') return
    if (found) {
      writeSelectedSeason(found.series.id, found.season.number)
      const targetPath = episodeWatchPath(found.series.id, found.season.number, found.episode.id)
      if (path !== targetPath) navigate(targetPath, true)
      return
    }
    if (!route.seriesId) {
      navigate('/', true)
      return
    }
    const parentSeries = catalog.series.find((item) => item.id === route.seriesId)
    if (!parentSeries) {
      navigate('/', true)
      return
    }
    const seasonNumbers = availableSeasonNumbers(parentSeries)
    const requestedSeason = route.seasonNumber
    if (requestedSeason !== undefined && seasonNumbers.includes(requestedSeason)) {
      navigate(seriesSeasonPath(parentSeries.id, requestedSeason), true)
      return
    }
    const seasonNumber = readSelectedSeason(parentSeries)
    navigate(seasonNumber === undefined ? seriesPath(parentSeries.id) : seriesSeasonPath(parentSeries.id, seasonNumber), true)
  }, [catalog, route, found, navigate, path])

  const returnToSeries = useCallback(() => {
    const fallback = found ? seriesSeasonPath(found.series.id, found.season.number) : '/'
    if (found) {
      writeSelectedSeason(found.series.id, found.season.number)
      rememberFocus(fallback, `episode-${found.episode.id}`)
    }
    goBack(fallback)
  }, [found, goBack])

  useEffect(() => {
    for (const command of navigationCommands.slice(0, -1)) acknowledgeCommand(command.id)
    if (latestNavigationCommand?.type === 'playEpisode') {
      if (!catalog) return
      const target = findEpisode(catalog.series, latestNavigationCommand.episodeId)
      if (!target || target.episode.status !== 'ready') {
        acknowledgeCommand(latestNavigationCommand.id)
      } else if (route.screen !== 'watch' || route.id !== latestNavigationCommand.episodeId || route.seriesId !== target.series.id || route.seasonNumber !== target.season.number) {
        navigate(episodeWatchPath(target.series.id, target.season.number, target.episode.id))
      }
    } else if (latestNavigationCommand?.type === 'stop') {
      if (found) {
        const targetPath = seriesSeasonPath(found.series.id, found.season.number)
        writeSelectedSeason(found.series.id, found.season.number)
        rememberFocus(targetPath, `episode-${found.episode.id}`)
        navigate(targetPath, true)
      }
      acknowledgeCommand(latestNavigationCommand.id)
    }
  }, [navigationCommands, catalog, route, found, navigate, acknowledgeCommand])

  useEffect(() => {
    if (playerReady) return
    reportPlaybackState({ episodeId: null, playing: false, position: 0, duration: 0, volume: 1, muted: false, error: null })
  }, [playerReady, reportPlaybackState])

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

  if (route.screen === 'remote') return <Remote catalog={catalog} goBack={() => goBack('/')} />
  if (loading && !catalog) return <StateScreen loading eyebrow="FRAME / LIBRARY" title="Preparing your library" description="Your collection will be ready in a moment." />
  if (error && !catalog && route.screen !== 'downloads') return <StateScreen eyebrow="LIBRARY OFFLINE" title="We couldn’t reach your library" description="Check that the server is running, then try again." action={<Button focusKey="catalog-retry" onClick={reload}>Try again</Button>} />
  if (route.screen === 'downloads') return <Downloads navigate={navigate} goBack={goBack} />
  if (route.screen === 'series') return series ? <Series key={series.id} series={series} seasonNumber={route.seasonNumber} navigate={navigate} goBack={goBack} reload={reload} /> : null
  if (route.screen === 'watch') return found ? playerReady ? <Player key={found.episode.id} {...found} navigate={navigate} back={returnToSeries} remoteCommands={currentCommands} acknowledgeRemoteCommand={acknowledgeCommand} reportPlaybackState={reportPlaybackState} /> : <UnavailableEpisode episode={found.episode} navigate={returnToSeries} /> : null
  if (!catalog?.series.length) return <StateScreen eyebrow="YOUR LIBRARY" title="Nothing here yet" description="Add a series to your library or download one to get started." action={<Button focusKey="empty-downloads" onClick={() => navigate('/downloads')}>Open downloads</Button>} />
  return <Library series={catalog.series} navigate={navigate} />
}
