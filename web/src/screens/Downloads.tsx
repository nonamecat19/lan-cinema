import { useRef } from 'react'
import type { ChangeEvent } from 'react'
import { Button, Footer, Header } from '../components/Chrome'
import { DownloadItem } from '../features/downloads/DownloadItem'
import { useDownloadManager } from '../features/downloads/useDownloadManager'
import type { TorrentFile } from '../domain/downloads'
import type { GoBack, Navigate } from '../hooks/useRoute'
import { pad } from '../lib/format'

function videoCount(files: TorrentFile[], season: number): number {
  return files.filter((file) => file.season === season && /\.(mp4|mkv|avi|mov|m4v|webm)$/i.test(file.name)).length
}

export function Downloads({ navigate, goBack }: { navigate: Navigate; goBack: GoBack }) {
  const manager = useDownloadManager()
  const uploadInput = useRef<HTMLInputElement | null>(null)

  function upload(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (file) void manager.upload(file)
  }

  const { sources, downloads, selectedSource, selection, selectedSeasons, seriesTitle, setSeriesTitle, requestError, actionError, busy, loading, inspect, start, toggleSeason, update } = manager
  return <main className="app-shell download-screen">
    <Header navigate={navigate} back onBack={() => goBack('/')} section="DOWNLOAD MANAGER" />
    <section className="download-heading"><span className="eyebrow">BUILD YOUR COLLECTION</span><h1>Downloads<span className="accent-dot">.</span></h1><p>Choose a torrent and the seasons you want. Completed episodes appear in your library.</p></section>
    {(actionError || requestError) && <div className="download-error" role="alert">{actionError || requestError}</div>}
    <section className="download-content">
      <div className="download-column"><div className="section-heading"><div><span className="eyebrow">01 / SELECT A SOURCE</span><h2>Torrent files</h2></div><input ref={uploadInput} type="file" accept=".torrent,application/x-bittorrent" onChange={upload} hidden /><Button className="secondary" focusKey="upload-torrent" disabled={busy} onClick={() => uploadInput.current?.click()}>↑ Add torrent</Button></div>
        <div className="torrent-source-list">{sources.map((source) => <button type="button" key={source.id} data-focusable="true" data-focus-key={`source-${source.id}`} className={`torrent-source ${selectedSource?.id === source.id ? 'selected' : ''}`} disabled={busy} onClick={() => { void inspect(source) }}><span className="torrent-source-icon">↓</span><span className="torrent-source-copy"><strong>{source.displayName || source.name}</strong><small>{source.name}</small></span><span className="torrent-source-action">INSPECT ↗</span></button>)}
          {!sources.length && <p className="download-empty">{loading ? 'Loading torrent files…' : 'No torrent files yet. Add one to get started.'}</p>}
        </div>
      </div>
      <div className="download-column season-picker"><div className="section-heading"><div><span className="eyebrow">02 / CHOOSE EPISODES</span><h2>{selection?.seriesTitle || 'Season selection'}</h2></div></div>
        {selection ? <><label className="series-title-field"><span>LIBRARY TITLE</span><input value={seriesTitle} onChange={(event) => setSeriesTitle(event.target.value)} /></label><div className="season-check-list">{selection.seasons.map((season) => <label key={season} className="season-check" tabIndex={0} data-focusable="true" data-focus-key={`download-season-${season}`}><input type="checkbox" checked={selectedSeasons.includes(season)} onChange={() => toggleSeason(season)} /><span className="season-check-mark">✓</span><span>SEASON {pad(season)}</span><small>{videoCount(selection.files, season)} EPISODES</small></label>)}</div><Button focusKey="download-start" disabled={!selectedSeasons.length || !seriesTitle.trim() || busy} onClick={() => { void start() }}>↓ Download selected seasons</Button></> : <div className="season-placeholder"><span>02</span><p>Select a torrent to inspect its seasons.</p></div>}
      </div>
    </section>
    <section className="active-downloads"><div className="section-heading"><div><span className="eyebrow">TRANSFER QUEUE</span><h2>Current downloads</h2></div><span className="episode-count">{pad(downloads.length)} TORRENTS</span></div>
      <div className="active-download-list">{downloads.map((download) => <DownloadItem key={download.hash} download={download} busy={busy} onToggle={(item) => { void update(item, item.paused ? 'resume' : 'pause') }} onRemove={(item) => { void update(item, 'delete') }} />)}{!downloads.length && <p className="download-empty">Your selected seasons will appear here.</p>}</div>
    </section>
    <Footer message="FINISHED EPISODES APPEAR IN YOUR LIBRARY" />
  </main>
}
