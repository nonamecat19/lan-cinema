import { useState } from 'react'
import { Button } from '../../components/Chrome'
import type { TorrentDownload } from '../../domain/downloads'
import { formatBytes, pad } from '../../lib/format'

function transferState(download: TorrentDownload): string {
  const state = download.state.toLowerCase()
  if (state.includes('error') || state.includes('missing')) return 'Transfer needs attention'
  if (state.includes('check')) return 'Checking downloaded files'
  if (state.includes('meta')) return 'Reading torrent metadata'
  if (download.paused) return 'Paused'
  if (state.includes('stall')) return 'Waiting for peers'
  if (state.includes('queue')) return 'Queued by torrent client'
  if (state.includes('up') || state.includes('seed')) return 'Seeding'
  if (state.includes('download') || state.includes('forced')) return 'Downloading'
  return download.state.replace(/_/g, ' ')
}

function formatETA(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 0 || seconds >= 8640000) return ''
  if (seconds < 60) return `${seconds}s left`
  if (seconds < 3600) return `${Math.ceil(seconds / 60)}m left`
  return `${Math.floor(seconds / 3600)}h ${Math.ceil(seconds % 3600 / 60)}m left`
}

export function DownloadItem({ download, busy, onToggle, onRemove }: { download: TorrentDownload; busy: boolean; onToggle: (download: TorrentDownload) => void; onRemove: (download: TorrentDownload) => void }) {
  const [confirming, setConfirming] = useState(false)
  const [showFiles, setShowFiles] = useState(false)
  const progress = Math.max(0, Math.min(100, download.progress * 100))
  const ready = download.files.filter((file) => file.status === 'ready').length
  const preparing = download.files.filter((file) => file.status === 'preparing' || file.status === 'queued' || file.status === 'importing').length
  return <div className="active-download">
    <div className="active-download-title"><strong>{download.name}</strong><span>{transferState(download).toUpperCase()} · {ready} READY{preparing > 0 ? ` · ${preparing} PROCESSING` : ''}</span></div>
    <div className="download-actions"><Button className="secondary" focusKey={`download-toggle-${download.hash}`} disabled={busy} onClick={() => onToggle(download)}>{download.paused ? 'Resume' : 'Pause'}</Button><Button className="secondary danger" focusKey={`download-remove-${download.hash}`} disabled={busy} onClick={() => setConfirming(true)}>Remove</Button></div>
    <div className="download-progress" role="progressbar" aria-label={`${download.name} progress`} aria-valuenow={progress} aria-valuemin={0} aria-valuemax={100}><i style={{ width: `${progress}%` }} /></div>
    <div className="download-progress-meta"><span>{progress.toFixed(1)}% · {formatBytes(download.completed)} / {formatBytes(download.size)}</span><span>{formatBytes(download.downloadSpeed)}/s {formatETA(download.eta)}</span></div>
    {download.detail && <p className="download-file-warning">{download.detail}</p>}
    {download.files.length > 0 && <div className="download-file-section"><Button className="secondary" focusKey={`download-files-${download.hash}`} onClick={() => setShowFiles((current) => !current)}>{showFiles ? 'Hide' : 'Show'} {download.files.length} episode statuses</Button>
      {showFiles && <div className="download-file-list">{download.files.map((file) => <div className="download-file" key={`${file.season}-${file.episode}-${file.name}`}><span className="download-file-name">S{pad(file.season)}E{pad(file.episode)} · {file.name}</span><span className={`download-file-state ${file.status === 'unavailable' || file.status === 'import-error' ? 'is-error' : ''}`}>{file.detail || file.status}{file.status === 'downloading' || file.status === 'paused' ? ` · ${(file.progress * 100).toFixed(1)}%` : file.status === 'preparing' && (file.preparationProgress || 0) > 0 ? ` · ${file.preparationProgress}%` : ''}</span></div>)}</div>}
    </div>}
    {confirming && <div className="download-remove-confirm"><span>Remove this transfer and its staged files? Imported episodes stay in your library.</span><Button className="danger" focusKey={`confirm-remove-${download.hash}`} disabled={busy} onClick={() => onRemove(download)}>Remove files</Button><Button className="secondary" focusKey={`cancel-remove-${download.hash}`} onClick={() => setConfirming(false)}>Keep</Button></div>}
  </div>
}
