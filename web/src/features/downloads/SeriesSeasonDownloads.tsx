import { useCallback, useEffect, useRef, useState } from 'react'
import { Button } from '../../components/Chrome'
import type { TorrentDownload, TorrentDownloadFile } from '../../domain/downloads'
import type { Series, TorrentSeasonOption, TorrentSeriesSource } from '../../domain/media'
import { api, errorMessage } from '../../lib/api'
import { formatBytes, pad } from '../../lib/format'
import { torrentSeasonStatus } from '../../lib/seriesStatus'

function seasonProgress(files: TorrentDownloadFile[]): number {
  const size = files.reduce((sum, file) => sum + file.size, 0)
  if (size > 0) return files.reduce((sum, file) => sum + file.size * file.progress, 0) / size * 100
  return files.length ? files.reduce((sum, file) => sum + file.progress, 0) / files.length * 100 : 0
}

export interface SeriesSeasonDownloadManager {
  downloads: TorrentDownload[]
  error: string
  message: string
  busy: boolean
  expanded: string
  confirming: string
  setExpanded: (key: string) => void
  setConfirming: (key: string) => void
  act: (action: () => Promise<unknown>, success: string) => Promise<void>
}

export function useSeriesSeasonDownloads(series: Series, reload: () => Promise<void>): SeriesSeasonDownloadManager {
  const [downloads, setDownloads] = useState<TorrentDownload[]>([])
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)
  const [expanded, setExpanded] = useState('')
  const [confirming, setConfirming] = useState('')
  const mounted = useRef(false)
  const pending = useRef<Promise<void> | null>(null)
  const hasSources = Boolean(series.torrentSources?.length)

  const refresh = useCallback(async (force = false): Promise<void> => {
    if (pending.current) {
      await pending.current
      if (!force) return
    }
    const request = api.downloads().then((result) => {
      if (mounted.current) { setDownloads(result); setError('') }
    }).catch((reason: unknown) => {
      if (mounted.current) setError(errorMessage(reason))
    }).finally(() => { pending.current = null })
    pending.current = request
    await request
  }, [])

  useEffect(() => {
    mounted.current = true
    if (!hasSources) return () => { mounted.current = false }
    void refresh()
    const timer = window.setInterval(() => { void refresh() }, 4000)
    return () => { mounted.current = false; window.clearInterval(timer) }
  }, [hasSources, refresh])

  async function act(action: () => Promise<unknown>, success: string) {
    setBusy(true)
    setError('')
    setMessage('')
    try {
      await action()
      await reload()
      await refresh(true)
      setMessage(success)
      setConfirming('')
    } catch (reason) {
      setError(errorMessage(reason))
    } finally {
      setBusy(false)
    }
  }

  return { downloads, error, message, busy, expanded, confirming, setExpanded, setConfirming, act }
}

export function SeriesTorrentControls({ series, manager }: { series: Series; manager: SeriesSeasonDownloadManager }) {
  const sources = series.torrentSources || []
  if (!sources.length) return null

  return <div className="series-download-sources">
    {sources.map((source) => <TorrentSourceControls key={source.id} seriesId={series.id} source={source} manager={manager} />)}
    {manager.message && <p className="season-action-message" role="status">{manager.message}</p>}
    {manager.error && <p className="download-error" role="alert">{manager.error}</p>}
  </div>
}

function TorrentSourceControls({ seriesId, source, manager }: { seriesId: string; source: TorrentSeriesSource; manager: SeriesSeasonDownloadManager }) {
  const download = manager.downloads.find((item) => item.hash === source.hash)
  return <div className="series-download-source">
    <div className="series-download-heading"><div><strong>{source.name}</strong><span>{download ? `${formatBytes(download.downloadSpeed)}/s overall · ${download.paused ? 'Torrent paused' : download.state.replace(/_/g, ' ')}` : 'Transfer status loading'}</span></div><Button className="secondary" focusKey={`season-order-${source.id}`} disabled={manager.busy} aria-pressed={source.forceOneByOne} onClick={() => { void manager.act(() => api.setSeasonOrdering(seriesId, source.id, !source.forceOneByOne), `Force one by one ${source.forceOneByOne ? 'off' : 'on'}.`) }}>{source.forceOneByOne ? '✓ ' : ''}Force one by one</Button></div>
  </div>
}

export function SeriesSeasonDownloadCards({ series, seasonNumber, manager }: { series: Series; seasonNumber: number; manager: SeriesSeasonDownloadManager }) {
  const sources = series.torrentSources || []
  const matching = sources.flatMap((source) => source.seasons.filter((option) => option.number === seasonNumber).map((option) => ({ source, option })))
  return <SeasonDownloadCards series={series} matching={matching} manager={manager} />
}

function SeasonDownloadCards({ series, matching, manager }: { series: Series; matching: Array<{ source: TorrentSeriesSource; option: TorrentSeasonOption }>; manager: SeriesSeasonDownloadManager }) {
  if (!matching.length) return null
  return <div className="series-download-list">{matching.map(({ source, option }) => {
    const key = `${source.id}-${option.number}`
    const download = manager.downloads.find((item) => item.hash === source.hash)
    const files = download?.files.filter((file) => file.season === option.number) || []
    const progress = seasonProgress(files)
    const complete = files.filter((file) => file.progress >= 0.999999).length
    const ready = series.seasons.find((item) => item.number === option.number)?.episodes.filter((episode) => episode.status === 'ready').length || 0
    const selectedSize = files.reduce((sum, file) => sum + file.size, 0)
    const downloadedSize = files.reduce((sum, file) => sum + file.size * file.progress, 0)
    const state = torrentSeasonStatus(source, option.number, manager.downloads)
    return <div className="series-download-season" key={key}>
      <div className="series-download-main"><div className="series-download-copy"><span className="series-download-source-name">{source.name}</span><strong>Season {pad(option.number)}</strong><span className={`season-status status-${state.key}`}><i aria-hidden="true">{state.icon}</i>{state.label} · {complete}/{option.episodes} downloaded · {ready} ready in library</span>{option.selected && files.length > 0 && <span className="series-download-bytes">{progress.toFixed(1)}% · {formatBytes(downloadedSize)} / {formatBytes(selectedSize)}</span>}{!option.selected && ready > 0 && <span className="series-download-bytes">This season also has episodes in your library.</span>}</div><div className="series-download-actions">{!option.selected ? <Button className="secondary" focusKey={`season-download-${key}`} disabled={manager.busy} onClick={() => { void manager.act(() => api.addSeasons(series.id, source.id, [option.number]), `Season ${pad(option.number)} added to downloads.`) }}>↓ Download</Button> : <>{!option.deleting && <Button className="secondary" focusKey={`season-pause-${key}`} disabled={manager.busy} onClick={() => { void manager.act(() => api.updateSeason(series.id, source.id, option.number, option.paused ? 'resume' : 'pause'), `Season ${pad(option.number)} ${option.paused ? 'resumed' : 'paused'}.`) }}>{option.paused ? 'Resume' : 'Pause'}</Button>}<Button className="secondary danger" focusKey={`season-delete-${key}`} disabled={manager.busy} onClick={() => manager.setConfirming(key)}>{option.deleting ? 'Retry delete' : 'Delete'}</Button></>}{files.length > 0 && <Button className="secondary" focusKey={`season-details-${key}`} aria-expanded={manager.expanded === key} onClick={() => manager.setExpanded(manager.expanded === key ? '' : key)}>{manager.expanded === key ? 'Hide' : 'Files'}</Button>}</div></div>
      {option.selected && files.length > 0 && <div className="series-download-progress" role="progressbar" aria-label={`Season ${pad(option.number)} download progress`} aria-valuenow={progress} aria-valuemin={0} aria-valuemax={100}><i style={{ width: `${progress}%` }} /></div>}
      {manager.expanded === key && <div className="series-download-episodes">{files.map((file) => <div key={`${file.episode}-${file.name}`}><span>S{pad(file.season)}E{pad(file.episode)} · {file.name}</span><span>{file.detail} · {(file.progress * 100).toFixed(1)}%</span></div>)}</div>}
      {manager.confirming === key && <div className="download-remove-confirm"><span>Delete Season {pad(option.number)} from this download, its staged files, and its imported episodes from the library?</span><Button className="danger" focusKey={`season-confirm-delete-${key}`} disabled={manager.busy} onClick={() => { void manager.act(() => api.updateSeason(series.id, source.id, option.number, 'delete'), `Season ${pad(option.number)} deleted.`) }}>Delete season</Button><Button className="secondary" focusKey={`season-cancel-delete-${key}`} onClick={() => manager.setConfirming('')}>Keep</Button></div>}
    </div>
  })}</div>
}

export function SeriesSeasonDownloads({ series, reload }: { series: Series; reload: () => Promise<void> }) {
  const manager = useSeriesSeasonDownloads(series, reload)
  const matching = (series.torrentSources || []).flatMap((source) => source.seasons.map((option) => ({ source, option })))
  if (!matching.length) return null
  return <>
    <SeriesTorrentControls series={series} manager={manager} />
    <SeasonDownloadCards series={series} matching={matching} manager={manager} />
  </>
}
