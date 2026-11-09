import { Button, Footer, Header } from '../components/Chrome'
import type { Series } from '../domain/media'
import type { Navigate } from '../hooks/useRoute'
import { allEpisodes, artworkFor, episodeCount } from '../lib/catalog'
import { formatTime, pad } from '../lib/format'
import { continueWatchingItems } from '../lib/progress'
import { episodeWatchPath, seriesPath } from '../lib/routes'
import { isMobileDevice } from '../remote/device'

function greeting() {
  const hour = new Date().getHours()
  return hour < 12 ? 'Good morning' : hour < 17 ? 'Good afternoon' : 'Good evening'
}

function SeriesCard({ series, index, onOpen }: { series: Series; index: number; onOpen: () => void }) {
  const artwork = artworkFor(series)
  return <button type="button" data-focusable="true" data-focus-key={`series-${series.id}`} className={`series-card card-tone-${index % 6}`} onClick={onOpen}>
    <span className="card-art" style={artwork ? { backgroundImage: `linear-gradient(0deg, rgba(7,12,14,.48), rgba(7,12,14,.12)), url("${artwork}")` } : undefined}>
      {!artwork && <span className="art-glyph" aria-hidden="true">{series.title.trim().charAt(0).toUpperCase()}</span>}
      <span className="card-number">{pad(index + 1)} / COLLECTION</span>
    </span>
    <span className="card-copy"><strong>{series.title}</strong><span>{series.seasons.length} {series.seasons.length === 1 ? 'season' : 'seasons'} <i /> {episodeCount(series)} {episodeCount(series) === 1 ? 'episode' : 'episodes'}</span></span>
    <span className="card-arrow" aria-hidden="true">↗</span>
  </button>
}

export function Library({ series, navigate }: { series: Series[]; navigate: Navigate }) {
  const sorted = [...series].sort((a, b) => a.title.localeCompare(b.title, undefined, { sensitivity: 'base' }))
  const inProgress = continueWatchingItems(sorted)
  const featured = inProgress[0]
  const heroSeries = featured?.show || sorted[0]
  const heroEpisode = featured?.episode || allEpisodes(heroSeries)[0]
  const artwork = heroEpisode?.previewUrl

  return <main className="app-shell home-screen">
    <Header navigate={navigate} />
    <section className="welcome-row"><div><span className="eyebrow">YOUR PERSONAL SCREENING ROOM</span><h1>{greeting()}<span className="accent-dot">.</span></h1><p>Everything you love, ready when you are.</p></div><div className="library-count"><strong>{pad(series.length)}</strong><span>SERIES IN LIBRARY</span></div></section>
    <section className="feature-card" style={artwork ? { backgroundImage: `linear-gradient(90deg, rgba(10,16,17,.98) 0%, rgba(10,16,17,.77) 46%, rgba(10,16,17,.14) 100%), url("${artwork}")` } : undefined}>
      <div className="feature-copy"><span className="eyebrow">{featured ? 'CONTINUE WATCHING' : 'FROM YOUR LIBRARY'}</span><h2>{heroSeries.title}</h2><p>{heroEpisode ? `Season ${pad(heroEpisode.seasonNumber)} · Episode ${pad(heroEpisode.number)} · ${heroEpisode.title}` : `${episodeCount(heroSeries)} ${episodeCount(heroSeries) === 1 ? 'episode' : 'episodes'} waiting for you`}</p>
        <Button focusKey="featured-series" onClick={() => navigate(featured ? episodeWatchPath(featured.show.id, featured.episode.seasonNumber, featured.episode.id) : seriesPath(heroSeries.id))}>{featured ? '▶ Resume episode' : 'Explore series'} <span aria-hidden="true">↗</span></Button>
      </div><span className="feature-index">FEATURED / {pad(sorted.indexOf(heroSeries) + 1)}</span>
    </section>
    {isMobileDevice() && <button type="button" className="button remote-entry" onClick={() => navigate('/remote')}>TV remote mode</button>}
    {inProgress.length > 0 && <section className="continue-section" aria-label="Continue watching"><div className="section-heading"><div><span className="eyebrow">PICK UP WHERE YOU LEFT OFF</span><h2>Continue watching</h2></div><span className="episode-count">{pad(inProgress.length)} IN PROGRESS</span></div><div className="continue-grid">{inProgress.slice(0, 8).map(({ show, episode, progress }) => <button type="button" key={episode.id} className="continue-card" data-focusable="true" data-focus-key={`continue-${episode.id}`} onClick={() => navigate(episodeWatchPath(show.id, episode.seasonNumber, episode.id))}><span className="continue-art">{episode.previewUrl && <img src={episode.previewUrl} alt="" loading="lazy" />}<span className="continue-play" aria-hidden="true">▶</span></span><span className="continue-copy"><strong>{show.title}</strong><span>S{pad(episode.seasonNumber)} E{pad(episode.number)} · {episode.title}</span><small>{formatTime(Math.max(0, (episode.durationSeconds || progress.duration) - progress.position))} left</small></span><span className="continue-progress"><i style={{ width: `${Math.min(100, progress.position / (episode.durationSeconds || progress.duration) * 100)}%` }} /></span></button>)}</div></section>}
    <section className="collection"><div className="section-heading"><div><span className="eyebrow">BROWSE COLLECTION</span><h2>All series</h2></div><span className="episode-count">{pad(series.length)} TITLES · A—Z</span></div>
      <div className="series-grid">{sorted.map((show, index) => <SeriesCard key={show.id} series={show} index={index} onOpen={() => navigate(seriesPath(show.id))} />)}</div>
    </section>
    <Footer />
  </main>
}
