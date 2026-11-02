import { parseResponse, responseGuards } from '../domain/parse'
import type { Catalog } from '../domain/media'
import type { TorrentDownload, TorrentSelection, TorrentSource } from '../domain/downloads'
import type { RemoteCommandPayload, RemoteDisplay, RemotePlaybackState, RemoteSyncResponse } from '../domain/remote'

export class ApiError extends Error {
  constructor(message: string, readonly status: number) {
    super(message)
  }
}

export function errorMessage(reason: unknown): string {
  return reason instanceof Error ? reason.message : 'The request could not be completed'
}

async function request<T>(path: string, guard: (value: unknown) => value is T, options?: RequestInit): Promise<T> {
  const response = await fetch(path, options)
  const payload: unknown = await response.json().catch(() => null)
  if (!response.ok) {
    const message = typeof payload === 'object' && payload !== null && 'error' in payload && typeof payload.error === 'string' ? payload.error : 'The request could not be completed'
    throw new ApiError(message, response.status)
  }
  return parseResponse(payload, guard)
}

export const api = {
  catalog: (): Promise<Catalog> => request('/api/catalog', responseGuards.catalog, { cache: 'no-store' }),
  torrentSources: (): Promise<TorrentSource[]> => request('/api/torrents', responseGuards.torrentSources),
  inspectTorrent: (id: string): Promise<TorrentSelection> => request(`/api/torrents/${encodeURIComponent(id)}`, responseGuards.torrentSelection),
  uploadTorrent: (file: File): Promise<TorrentSource> => {
    const body = new FormData()
    body.append('torrent', file)
    return request('/api/torrents', responseGuards.torrentSource, { method: 'POST', body })
  },
  startTorrent: (id: string, selection: Pick<TorrentSelection, 'seriesTitle' | 'seasons'>) => request(`/api/torrents/${encodeURIComponent(id)}/start`, responseGuards.accepted, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(selection),
  }),
  downloads: (): Promise<TorrentDownload[]> => request('/api/downloads', responseGuards.downloads),
  updateDownload: (hash: string, action: 'pause' | 'resume' | 'delete') => request(`/api/downloads/${encodeURIComponent(hash)}/${action}`, responseGuards.accepted, { method: 'POST' }),
  addSeasons: (seriesId: string, sourceId: string, seasons: number[]) => request(`/api/series/${encodeURIComponent(seriesId)}/seasons`, responseGuards.accepted, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ sourceId, seasons }),
  }),
  registerRemoteDisplay: (id: string): Promise<RemoteDisplay> => request('/api/remote/displays', responseGuards.remoteDisplay, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id }),
  }),
  remoteDisplays: (): Promise<RemoteDisplay[]> => request('/api/remote/displays', responseGuards.remoteDisplays, { cache: 'no-store' }),
  remoteDisplay: (id: string): Promise<RemoteDisplay> => request(`/api/remote/displays/${encodeURIComponent(id)}`, responseGuards.remoteDisplay, { cache: 'no-store' }),
  syncRemoteDisplay: (id: string, state: RemotePlaybackState, acknowledgedIDs: string[]): Promise<RemoteSyncResponse> => request(`/api/remote/displays/${encodeURIComponent(id)}/sync`, responseGuards.remoteSync, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ state, acknowledgedIds: acknowledgedIDs }),
  }),
  sendRemoteCommand: (id: string, command: RemoteCommandPayload): Promise<{ queued: true }> => request(`/api/remote/displays/${encodeURIComponent(id)}/commands`, responseGuards.remoteCommandQueued, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(command),
  }),
}
