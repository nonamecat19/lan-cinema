import { useEffect, useState } from 'react'
import type { Catalog } from '../domain/media'
import type { RemoteCommandPayload } from '../domain/remote'
import { allEpisodes, findEpisode } from '../lib/catalog'
import { formatTime, pad } from '../lib/format'
import { useRemoteController } from '../remote/useRemoteController'

function clamp(value: number, minimum: number, maximum: number): number {
  return Math.min(maximum, Math.max(minimum, value))
}

function displayEpisode(episodeId: string | null, series: Catalog['series']): string {
  if (!episodeId) return 'Idle'
  const current = findEpisode(series, episodeId)
  return current ? `${current.series.title} · S${pad(current.season.number)} E${pad(current.episode.number)} · ${current.episode.title}` : `Episode ${episodeId}`
}

export function Remote({ catalog, goBack }: { catalog: Catalog | null; goBack: () => void }) {
  const { status, displays, selectedDisplay, selectedId, showChooser, commandError, selectDisplay, openChooser, refresh, sendCommand } = useRemoteController(catalog)
  const [selectedSeriesId, setSelectedSeriesId] = useState('')
  const [selectedSeasonNumber, setSelectedSeasonNumber] = useState<number | null>(null)
  const [selectedEpisodeId, setSelectedEpisodeId] = useState('')
  const series = catalog?.series || []

  useEffect(() => {
    if (!series.length) return
    const currentSeries = series.find((item) => item.id === selectedSeriesId)
    if (!currentSeries) {
      setSelectedSeriesId(series[0].id)
      setSelectedSeasonNumber(series[0].seasons[0]?.number ?? null)
      setSelectedEpisodeId(series[0].seasons[0]?.episodes[0]?.id || '')
      return
    }
    const currentSeason = currentSeries.seasons.find((item) => item.number === selectedSeasonNumber)
    if (!currentSeason) {
      setSelectedSeasonNumber(currentSeries.seasons[0]?.number ?? null)
      setSelectedEpisodeId(currentSeries.seasons[0]?.episodes[0]?.id || '')
      return
    }
    if (!currentSeason.episodes.some((item) => item.id === selectedEpisodeId)) setSelectedEpisodeId(currentSeason.episodes[0]?.id || '')
  }, [series, selectedSeriesId, selectedSeasonNumber, selectedEpisodeId])

  const selectedSeries = series.find((item) => item.id === selectedSeriesId)
  const seasons = selectedSeries?.seasons || []
  const selectedSeason = seasons.find((item) => item.number === selectedSeasonNumber)
  const episodes = selectedSeason?.episodes || []
  const selectedEpisode = episodes.find((item) => item.id === selectedEpisodeId)
  const playback = selectedDisplay?.state
  const current = playback?.episodeId && catalog ? findEpisode(series, playback.episodeId) : null
  const ordered = current ? allEpisodes(current.series).filter((item) => item.status === 'ready').sort((a, b) => a.seasonNumber - b.seasonNumber || a.number - b.number || a.id.localeCompare(b.id)) : []
  const currentIndex = ordered.findIndex((item) => item.id === playback?.episodeId)
  const previous = currentIndex > 0 ? ordered[currentIndex - 1] : null
  const next = currentIndex >= 0 ? ordered[currentIndex + 1] : null
  const connected = status === 'connected'
  const choose = showChooser || (!selectedId && status === 'chooseDisplay')
  const targetName = selectedDisplay?.label || (selectedId ? `Display ${selectedId.slice(0, 8)}` : 'No TV selected')
  const send = (command: RemoteCommandPayload) => { void sendCommand(command) }
  const changeSeries = (id: string) => {
    const nextSeries = series.find((item) => item.id === id)
    setSelectedSeriesId(id)
    setSelectedSeasonNumber(nextSeries?.seasons[0]?.number ?? null)
    setSelectedEpisodeId(nextSeries?.seasons[0]?.episodes[0]?.id || '')
  }
  const changeSeason = (number: number) => {
    const season = seasons.find((item) => item.number === number)
    setSelectedSeasonNumber(number)
    setSelectedEpisodeId(season?.episodes[0]?.id || '')
  }

  return <main className="app-shell remote-screen">
    <header className="remote-header"><button type="button" className="button secondary" onClick={goBack}>← Back</button><span className="eyebrow">TV REMOTE MODE</span></header>
    <section className="remote-heading"><h1>Control your TV</h1><p>Choose an episode and control playback on your screen.</p></section>
    <section className="remote-display" aria-live="polite">
      <div className="remote-display-heading"><div><span className="eyebrow">SELECTED TV</span><h2>{targetName}</h2></div><span className={`remote-connection remote-connection-${status}`}>{status === 'connected' ? 'Connected' : status === 'disconnected' ? 'Disconnected' : selectedId ? 'Connecting' : 'Searching'}</span></div>
      {selectedId && <div className="remote-display-actions"><button type="button" className="button secondary" onClick={refresh}>Retry connection</button><button type="button" className="button secondary" onClick={openChooser}>Change TV</button></div>}
      {!selectedId && status === 'discovering' && <p>Looking for TVs on this network…</p>}
      {!selectedId && status === 'noDisplays' && <><p>Open Frame on your TV and keep it on. This page will find it automatically.</p><div className="remote-display-actions"><button type="button" className="button secondary" onClick={refresh}>Retry search</button></div></>}
      {status === 'disconnected' && <p>This TV is offline. Turn it on, then retry, or choose another TV.</p>}
      {choose && <div className="remote-chooser"><h3>Choose a TV</h3>{displays.length ? <div className="remote-display-list">{displays.map((display) => <button type="button" className="button secondary" key={display.id} onClick={() => selectDisplay(display.id)}>{display.label}{display.id === selectedId ? ' · selected' : ''} · {displayEpisode(display.state.episodeId, series)}</button>)}</div> : <p>No active TVs found. Open Frame on your TV and keep this page open.</p>}</div>}
    </section>
    <section className="remote-now-playing"><span className="eyebrow">NOW ON TV</span><h2>{current ? `${current.series.title} · S${pad(current.season.number)} E${pad(current.episode.number)} · ${current.episode.title}` : playback?.episodeId ? 'Episode on TV' : 'Nothing playing'}</h2><p>{formatTime(playback?.position || 0)} / {formatTime(playback?.duration || 0)} · {playback?.playing ? 'Playing' : 'Paused'}</p>{playback?.error && <p role="alert">TV error: {playback.error}</p>}{commandError && <p role="alert">{commandError}</p>}</section>
    <section className="remote-picker" aria-label="Choose an episode"><h2>Play on TV</h2><div className="remote-selects"><label>Series<select value={selectedSeriesId} onChange={(event) => changeSeries(event.target.value)} disabled={!series.length}>{series.map((item) => <option key={item.id} value={item.id}>{item.title}</option>)}</select></label><label>Season<select value={selectedSeasonNumber ?? ''} onChange={(event) => changeSeason(Number(event.target.value))} disabled={!seasons.length}>{seasons.map((season) => <option key={season.number} value={season.number}>Season {season.number}</option>)}</select></label><label>Episode<select value={selectedEpisodeId} onChange={(event) => setSelectedEpisodeId(event.target.value)} disabled={!episodes.length}>{episodes.map((episode) => <option key={episode.id} value={episode.id}>Episode {episode.number}: {episode.title}{episode.status !== 'ready' ? ` (${episode.status})` : ''}</option>)}</select></label></div><button type="button" className="button remote-play" disabled={!connected || selectedEpisode?.status !== 'ready'} onClick={() => send({ type: 'playEpisode', episodeId: selectedEpisodeId })}>Play on TV</button>{selectedEpisode && selectedEpisode.status !== 'ready' && <p>This episode is {selectedEpisode.status} and cannot play yet.</p>}</section>
    <section className="remote-controls" aria-label="Playback controls"><h2>Playback</h2><div className="remote-control-grid"><button type="button" disabled={!connected || !previous} onClick={() => previous && send({ type: 'playEpisode', episodeId: previous.id })}>Previous episode</button><button type="button" disabled={!connected || !playback?.episodeId} onClick={() => send({ type: 'setPlaying', playing: !playback?.playing })}>{playback?.playing ? 'Pause' : 'Play'}</button><button type="button" disabled={!connected || !next} onClick={() => next && send({ type: 'playEpisode', episodeId: next.id })}>Next episode</button><button type="button" disabled={!connected || !playback?.episodeId} onClick={() => send({ type: 'seek', position: clamp((playback?.position || 0) - 10, 0, playback?.duration || 0) })}>Rewind 10 seconds</button><button type="button" disabled={!connected || !playback?.episodeId} onClick={() => send({ type: 'seek', position: clamp((playback?.position || 0) + 10, 0, playback?.duration || 0) })}>Forward 10 seconds</button><button type="button" disabled={!connected || !playback?.episodeId} onClick={() => send({ type: 'stop' })}>Stop</button><button type="button" disabled={!connected} onClick={() => send({ type: 'setMuted', muted: !playback?.muted })}>{playback?.muted ? 'Unmute' : 'Mute'}</button><button type="button" disabled={!connected} onClick={() => send({ type: 'setVolume', volume: clamp((playback?.volume || 0) - 0.1, 0, 1) })}>Volume down</button><button type="button" disabled={!connected} onClick={() => send({ type: 'setVolume', volume: clamp((playback?.volume || 0) + 0.1, 0, 1) })}>Volume up</button></div></section>
  </main>
}
