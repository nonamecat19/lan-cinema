import { useState } from 'react'
import { Button, Footer, Header, StateScreen } from '../components/Chrome'
import type { Series as SeriesModel, TorrentSeriesSource } from '../domain/media'
import type { GoBack, Navigate } from '../hooks/useRoute'
import { api, errorMessage } from '../lib/api'
import { artworkFor, episodeCount } from '../lib/catalog'
import { formatTime, pad } from '../lib/format'
import { progressPercent } from '../lib/progress'
import { readSelectedSeason, writeSelectedSeason } from '../lib/seasonSelection'

interface SeriesProps {
  series: SeriesModel
  navigate: Navigate
  goBack: GoBack
  reload: () => Promise<void>
}

export function Series({ series, navigate, goBack, reload }: SeriesProps) {
  const [selectedSeason, setSelectedSeason] = useState(() => readSelectedSeason(series))
  const [busySeason, setBusySeason] = useState('')
  const [seasonMessage, setSeasonMessage] = useState('')
  const [seasonError, setSeasonError] = useState('')
  const season = series.seasons.find((item) => item.number === selectedSeason) || series.seasons[0]
  const artwork = artworkFor(series)

  function selectSeason(number: number) {
    writeSelectedSeason(series.id, number)
    setSelectedSeason(number)
  }

  async function loadSeason(source: TorrentSeriesSource, number: number) {
    const key = `${source.id}-${number}`
    setBusySeason(key)
    setSeasonError('')
    setSeasonMessage('')
    try {
      await api.addSeasons(series.id, source.id, [number])
      setSeasonMessage(`Season ${pad(number)} added to the download queue.`)
      await reload()
    } catch (reason) {
      setSeasonError(errorMessage(reason))
    } finally {
      setBusySeason('')
    }
  }

  if (!season) return <StateScreen eyebrow="SERIES / EPISODES" title="No episodes yet" description="Episodes will appear here when they are added to your library." action={<Button focusKey="series-empty-back" onClick={() => goBack('/')}>Browse library</Button>} />

  return <main className="app-shell series-screen">
    <Header navigate={navigate} back onBack={() => goBack('/')} section="SERIES / EPISODES" />
    <section className="series-hero" style={artwork ? { backgroundImage: `linear-gradient(90deg, rgba(9,15,17,.98) 0%, rgba(9,15,17,.67) 55%, rgba(9,15,17,.3) 100%), url("${artwork}")` } : undefined}>
      <div className="hero-copy"><span className="eyebrow">THE COLLECTION</span><h1>{series.title}</h1><p>{series.seasons.length} {series.seasons.length === 1 ? 'season' : 'seasons'} <span>·</span> {episodeCount(series)} {episodeCount(series) === 1 ? 'episode' : 'episodes'}</p></div>
      <span className="hero-index">FRAME / SERIES</span>
    </section>
    {!!series.torrentSources?.length && <section className="torrent-seasons"><div className="section-heading"><div><span className="eyebrow">FROM YOUR TORRENT FILE</span><h2>More seasons</h2></div></div>
      {series.torrentSources.map((source) => <div className="torrent-series-source" key={source.id}><div><strong>{source.name}</strong><span>{source.availableSeasons.length ? 'Choose another season to download in the background.' : 'All seasons in this torrent are selected.'}</span></div><div className="torrent-season-actions">{source.availableSeasons.map((option) => <Button key={option.number} className="secondary" focusKey={`more-season-${source.id}-${option.number}`} disabled={!!busySeason} onClick={() => loadSeason(source, option.number)}>↓ Season {pad(option.number)} <small>{option.episodes} episodes</small></Button>)}</div></div>)}
      {seasonMessage && <p className="season-action-message" role="status">{seasonMessage}</p>}{seasonError && <p className="download-error" role="alert">{seasonError}</p>}
    </section>}
    <section className="episode-section"><div className="section-heading"><div><span className="eyebrow">CHOOSE YOUR MOMENT</span><h2>Episodes</h2></div><span className="episode-count">{pad(season.episodes.length)} EPISODES</span></div>
      <div className="season-tabs" role="tablist" aria-label="Seasons">{series.seasons.map((item) => <button type="button" key={item.number} id={`season-tab-${item.number}`} role="tab" aria-selected={season.number === item.number} aria-controls="episode-panel" data-focusable="true" data-focus-key={`season-${series.id}-${item.number}`} className={`season-tab ${season.number === item.number ? 'selected' : ''}`} onClick={() => selectSeason(item.number)}>SEASON {pad(item.number)}</button>)}</div>
      <div className="episode-list" id="episode-panel" role="tabpanel" aria-labelledby={`season-tab-${season.number}`}>{season.episodes.map((episode) => {
        const progress = episode.status === 'ready' ? progressPercent(episode) : 0
        const ready = episode.status === 'ready'
        const preparing = episode.status === 'preparing' || episode.status === 'queued'
        const preparationProgress = episode.preparationProgress || 0
        const detail = ready ? progress > 0 ? 'Continue watching' : 'Ready to watch' : episode.status === 'unavailable' ? episode.unavailableReason || 'Unavailable' : `${episode.preparationStage || 'Waiting to prepare'}${preparationProgress > 0 ? ` · ${preparationProgress}%` : ''}`
        return <button type="button" key={episode.id} data-focusable="true" data-focus-key={`episode-${episode.id}`} className={`episode-row ${episode.status === 'unavailable' ? 'is-unavailable' : ''}`} onClick={() => ready && navigate(`/watch/${episode.id}`)} aria-disabled={!ready}>
          <span className="episode-thumb">{episode.previewUrl && <img src={episode.previewUrl} alt="" loading="lazy" />}<span className="episode-number">{pad(episode.number)}</span></span>
          <span className="episode-info"><span className="eyebrow">S{pad(season.number)} E{pad(episode.number)} {progress > 0 ? ' · IN PROGRESS' : ''}</span><strong>{episode.title}</strong><span>{detail}</span>{(progress > 0 || preparing && preparationProgress > 0) && <span className="episode-progress"><i style={{ width: `${ready ? progress : preparationProgress}%` }} /></span>}</span>
          <span className="episode-duration">{episode.durationSeconds ? formatTime(episode.durationSeconds) : ''}</span><span className="episode-play" aria-hidden="true">{ready ? '▶' : '—'}</span>
        </button>
      })}</div>
    </section>
    <Footer message="SELECT AN EPISODE TO WATCH" />
  </main>
}
