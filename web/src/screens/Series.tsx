import { Button, Footer, Header, StateScreen } from '../components/Chrome'
import type { Series as SeriesModel } from '../domain/media'
import { SeriesSeasonDownloadCards, SeriesTorrentControls, useSeriesSeasonDownloads } from '../features/downloads/SeriesSeasonDownloads'
import type { GoBack, Navigate } from '../hooks/useRoute'
import { artworkFor, episodeCount } from '../lib/catalog'
import { formatTime, pad } from '../lib/format'
import { progressPercent } from '../lib/progress'
import { episodeWatchPath, seriesSeasonPath } from '../lib/routes'
import { availableSeasonNumbers, readSelectedSeason, writeSelectedSeason } from '../lib/seasonSelection'
import { episodeRowsForSeason, seasonStatusFor } from '../lib/seriesStatus'

interface SeriesProps {
  series: SeriesModel
  seasonNumber?: number
  navigate: Navigate
  goBack: GoBack
  reload: () => Promise<void>
}

export function Series({ series, seasonNumber, navigate, goBack, reload }: SeriesProps) {
  const seasonNumbers = availableSeasonNumbers(series)
  const activeSeason = seasonNumbers.includes(seasonNumber ?? Number.NaN) ? seasonNumber : readSelectedSeason(series)
  const artwork = artworkFor(series)
  const downloadManager = useSeriesSeasonDownloads(series, reload)
  const episodeRows = activeSeason === undefined ? [] : episodeRowsForSeason(series, activeSeason, downloadManager.downloads)

  function selectSeason(number: number) {
    writeSelectedSeason(series.id, number)
    navigate(seriesSeasonPath(series.id, number))
  }

  if (activeSeason === undefined) return <StateScreen eyebrow="SERIES / EPISODES" title="No episodes yet" description="Episodes will appear here when they are added to your library." action={<Button focusKey="series-empty-back" onClick={() => goBack('/')}>Browse library</Button>} />

  return <main className="app-shell series-screen">
    <Header navigate={navigate} back onBack={() => goBack('/')} section="SERIES / EPISODES" />
    <section className="series-hero" style={artwork ? { backgroundImage: `linear-gradient(90deg, rgba(9,15,17,.98) 0%, rgba(9,15,17,.67) 55%, rgba(9,15,17,.3) 100%), url("${artwork}")` } : undefined}>
      <div className="hero-copy"><span className="eyebrow">THE COLLECTION</span><h1>{series.title}</h1><p>{series.seasons.length} {series.seasons.length === 1 ? 'season' : 'seasons'} in library <span>·</span> {episodeCount(series)} {episodeCount(series) === 1 ? 'episode' : 'episodes'}</p></div>
      <span className="hero-index">FRAME / SERIES</span>
    </section>
    <section className="episode-section"><div className="section-heading"><div><span className="eyebrow">CHOOSE YOUR MOMENT</span><h2>Episodes</h2></div><span className="episode-count">{pad(episodeRows.length)} EPISODES</span></div>
      <SeriesTorrentControls series={series} manager={downloadManager} />
      <div className="season-tabs" role="tablist" aria-label="Seasons">{seasonNumbers.map((number) => {
        const status = seasonStatusFor(series, number, downloadManager.downloads)
        return <button type="button" key={number} id={`season-tab-${number}`} role="tab" aria-label={`Season ${pad(number)} · ${status.label}`} aria-selected={activeSeason === number} aria-controls="episode-panel" data-focusable="true" data-focus-key={`season-${series.id}-${number}`} className={`season-tab ${activeSeason === number ? 'selected' : ''} status-${status.key}`} onClick={() => selectSeason(number)}><span>SEASON {pad(number)}</span><span className="season-tab-status"><i aria-hidden="true">{status.icon}</i>{status.label}</span></button>
      })}</div>
      <div id="episode-panel" role="tabpanel" aria-labelledby={`season-tab-${activeSeason}`}>
        <SeriesSeasonDownloadCards series={series} seasonNumber={activeSeason} manager={downloadManager} />
        {episodeRows.length ? <div className="episode-list">{episodeRows.map((row) => {
          const episode = row.episode
          const ready = episode?.status === 'ready'
          const progress = ready ? progressPercent(episode) : 0
          const preparing = episode?.status === 'preparing' || episode?.status === 'queued'
          const preparationProgress = episode?.preparationProgress || 0
          const detail = !episode ? '' : ready ? 'Ready to watch' : episode.status === 'unavailable' ? episode.unavailableReason || 'Unavailable' : `${episode.preparationStage || 'Waiting to prepare'}${preparationProgress > 0 ? ` · ${preparationProgress}%` : ''}`
          return <button type="button" key={`${activeSeason}-${row.number}`} data-focusable="true" data-focus-key={`episode-${series.id}-${activeSeason}-${row.number}`} className={`episode-row ${episode?.status === 'unavailable' ? 'is-unavailable' : ''} ${!episode ? 'is-torrent-only' : ''}`} onClick={() => ready && episode && navigate(episodeWatchPath(series.id, activeSeason, episode.id))} aria-disabled={!ready}>
            <span className="episode-thumb">{episode?.previewUrl && <img src={episode.previewUrl} alt="" loading="lazy" />}<span className="episode-number">{pad(row.number)}</span></span>
            <span className="episode-info"><span className="eyebrow">S{pad(activeSeason)} E{pad(row.number)} {progress > 0 ? ' · IN PROGRESS' : ''}</span><strong>{row.title}</strong>{episode && <span>{detail}</span>}{row.torrentStatuses.map((torrentStatus) => <span className={`episode-torrent-status status-${torrentStatus.status.key}`} key={torrentStatus.sourceId}><i aria-hidden="true">{torrentStatus.status.icon}</i><b>{torrentStatus.sourceName}</b><span>{torrentStatus.status.label}</span>{torrentStatus.detail && <small>{torrentStatus.detail}</small>}</span>)}{(progress > 0 || preparing && preparationProgress > 0) && <span className="episode-progress"><i style={{ width: `${ready ? progress : preparationProgress}%` }} /></span>}</span>
            <span className="episode-duration">{episode?.durationSeconds ? formatTime(episode.durationSeconds) : ''}</span><span className="episode-play" aria-hidden="true">{ready ? '▶' : '—'}</span>
          </button>
        })}</div> : <p className="season-episodes-empty">No episodes in this season yet.</p>}
      </div>
    </section>
    <Footer message="SELECT AN EPISODE TO WATCH" />
  </main>
}
