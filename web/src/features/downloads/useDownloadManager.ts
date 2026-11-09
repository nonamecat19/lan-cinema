import { useCallback, useEffect, useRef, useState } from 'react'
import type { TorrentDownload, TorrentSelection, TorrentSource } from '../../domain/downloads'
import { api, errorMessage } from '../../lib/api'

export function useDownloadManager() {
  const [sources, setSources] = useState<TorrentSource[]>([])
  const [downloads, setDownloads] = useState<TorrentDownload[]>([])
  const [selectedSource, setSelectedSource] = useState<TorrentSource | null>(null)
  const [selection, setSelection] = useState<TorrentSelection | null>(null)
  const [selectedSeasons, setSelectedSeasons] = useState<number[]>([])
  const [seriesTitle, setSeriesTitle] = useState('')
  const [forceOneByOne, setForceOneByOne] = useState(false)
  const [requestError, setRequestError] = useState('')
  const [actionError, setActionError] = useState('')
  const [busy, setBusy] = useState(false)
  const [loading, setLoading] = useState(true)
  const mounted = useRef(false)
  const pendingRefresh = useRef<Promise<unknown> | null>(null)

  const refresh = useCallback(async (force = false): Promise<void> => {
    if (pendingRefresh.current) {
      if (!force) { await pendingRefresh.current; return }
      await pendingRefresh.current
    }
    const request = Promise.allSettled([api.torrentSources(), api.downloads()])
    pendingRefresh.current = request
    const [sourceResult, downloadResult] = await request
    pendingRefresh.current = null
    if (!mounted.current) return
    if (sourceResult.status === 'fulfilled') setSources(sourceResult.value)
    if (downloadResult.status === 'fulfilled') setDownloads(downloadResult.value)
    const failure = sourceResult.status === 'rejected' ? sourceResult.reason : downloadResult.status === 'rejected' ? downloadResult.reason : null
    setRequestError(failure ? errorMessage(failure) : '')
    setLoading(false)
  }, [])

  useEffect(() => {
    mounted.current = true
    void refresh()
    const timer = window.setInterval(() => { void refresh() }, 4000)
    return () => { mounted.current = false; window.clearInterval(timer) }
  }, [refresh])

  async function run(action: () => Promise<void>) {
    setBusy(true)
    setActionError('')
    try {
      await action()
    } catch (reason) {
      setActionError(errorMessage(reason))
    } finally {
      setBusy(false)
    }
  }

  function showSelection(source: TorrentSource, result: TorrentSelection) {
    setSelectedSource(source)
    setSelection(result)
    setSelectedSeasons(result.seasons)
    setSeriesTitle(result.seriesTitle)
    setForceOneByOne(false)
  }

  function inspect(source: TorrentSource): Promise<void> {
    return run(async () => {
      setSelectedSource(source)
      setSelection(null)
      showSelection(source, await api.inspectTorrent(source.id))
    })
  }

  function upload(file: File): Promise<void> {
    if (!file.name.toLowerCase().endsWith('.torrent')) {
      setActionError('Choose a .torrent file')
      return Promise.resolve()
    }
    return run(async () => {
      const source = await api.uploadTorrent(file)
      await refresh(true)
      showSelection(source, await api.inspectTorrent(source.id))
    })
  }

  function start(): Promise<void> {
    if (!selectedSource || !selection) return Promise.resolve()
    return run(async () => {
      await api.startTorrent(selectedSource.id, { seriesTitle: seriesTitle.trim(), seasons: selectedSeasons, forceOneByOne })
      setSelection(null)
      setSelectedSource(null)
      await refresh(true)
    })
  }

  function toggleSeason(season: number): void {
    setSelectedSeasons((current) => current.includes(season) ? current.filter((item) => item !== season) : [...current, season].sort((a, b) => a - b))
  }

  function update(download: TorrentDownload, action: 'pause' | 'resume' | 'delete'): Promise<void> {
    return run(async () => { await api.updateDownload(download.hash, action); await refresh(true) })
  }

  return { sources, downloads, selectedSource, selection, selectedSeasons, seriesTitle, setSeriesTitle, forceOneByOne, setForceOneByOne, requestError, actionError, busy, loading, inspect, upload, start, toggleSeason, update }
}
