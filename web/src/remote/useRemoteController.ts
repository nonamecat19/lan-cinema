import { useCallback, useEffect, useState } from 'react'
import type { Catalog } from '../domain/media'
import type { RemoteCommandPayload, RemoteDisplay } from '../domain/remote'
import { api, errorMessage } from '../lib/api'
import { findEpisode } from '../lib/catalog'
import { getSelectedRemoteDisplayId, setSelectedRemoteDisplayId } from './device'

export type RemoteControllerStatus = 'discovering' | 'noDisplays' | 'chooseDisplay' | 'connecting' | 'connected' | 'disconnected'

export function useRemoteController(catalog: Catalog | null) {
  const [selectedId, setSelectedId] = useState(getSelectedRemoteDisplayId)
  const [selectedDisplay, setSelectedDisplay] = useState<RemoteDisplay | null>(null)
  const [displays, setDisplays] = useState<RemoteDisplay[]>([])
  const [status, setStatus] = useState<RemoteControllerStatus>(selectedId ? 'connecting' : 'discovering')
  const [showChooser, setShowChooser] = useState(false)
  const [commandError, setCommandError] = useState<string | null>(null)
  const [refreshKey, setRefreshKey] = useState(0)

  const selectDisplay = useCallback((id: string) => {
    const display = displays.find((item) => item.id === id)
    if (!display) return
    setSelectedRemoteDisplayId(id)
    setSelectedId(id)
    setSelectedDisplay(display)
    setStatus('connecting')
    setShowChooser(false)
    setCommandError(null)
    setRefreshKey((key) => key + 1)
  }, [displays])

  const refresh = useCallback(() => {
    setCommandError(null)
    setRefreshKey((key) => key + 1)
  }, [])

  const openChooser = useCallback(() => {
    setShowChooser(true)
    setRefreshKey((key) => key + 1)
  }, [])

  useEffect(() => {
    if (selectedId && !showChooser) return
    let active = true
    let timer: ReturnType<typeof setTimeout>
    const poll = async () => {
      try {
        const found = await api.remoteDisplays()
        if (!active) return
        setDisplays(found)
        if (!selectedId && !showChooser && found.length === 1) {
          setSelectedRemoteDisplayId(found[0].id)
          setSelectedId(found[0].id)
          setSelectedDisplay(found[0])
          setStatus('connecting')
          return
        }
        if (!selectedId) setStatus(found.length ? 'chooseDisplay' : 'noDisplays')
      } catch {
        if (active && !selectedId) setStatus('noDisplays')
      }
      if (active) timer = setTimeout(poll, 2000)
    }
    void poll()
    return () => {
      active = false
      clearTimeout(timer)
    }
  }, [selectedId, showChooser, refreshKey])

  useEffect(() => {
    if (!selectedId) return
    let active = true
    let timer: ReturnType<typeof setTimeout>
    const poll = async () => {
      try {
        const display = await api.remoteDisplay(selectedId)
        if (!active) return
        setSelectedDisplay(display)
        setStatus('connected')
      } catch {
        if (active) setStatus('disconnected')
      }
      if (active) timer = setTimeout(poll, 1000)
    }
    void poll()
    return () => {
      active = false
      clearTimeout(timer)
    }
  }, [selectedId, refreshKey])

  const sendCommand = useCallback(async (command: RemoteCommandPayload) => {
    if (!selectedId || status !== 'connected') return
    if (command.type === 'playEpisode' && (!catalog || findEpisode(catalog.series, command.episodeId)?.episode.status !== 'ready')) return
    try {
      await api.sendRemoteCommand(selectedId, command)
      setCommandError(null)
    } catch (error) {
      setCommandError(errorMessage(error))
      setRefreshKey((key) => key + 1)
    }
  }, [catalog, selectedId, status, refresh])

  return { status, displays, selectedDisplay, selectedId, showChooser, commandError, selectDisplay, openChooser, refresh, sendCommand }
}
