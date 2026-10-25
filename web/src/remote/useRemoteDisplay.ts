import { useCallback, useEffect, useRef, useState } from 'react'
import type { RemoteCommand, RemotePlaybackState } from '../domain/remote'
import { api, ApiError } from '../lib/api'
import { getRemoteDisplayId } from './device'

const initialPlaybackState: RemotePlaybackState = {
  episodeId: null,
  playing: false,
  position: 0,
  duration: 0,
  volume: 1,
  muted: false,
  error: null,
}

export function useRemoteDisplay(enabled: boolean): {
  commands: RemoteCommand[]
  reportPlaybackState: (state: RemotePlaybackState) => void
  acknowledgeCommand: (id: string) => void
} {
  const [commands, setCommands] = useState<RemoteCommand[]>([])
  const playbackState = useRef<RemotePlaybackState>(initialPlaybackState)
  const acknowledgedIDs = useRef<Set<string>>(new Set())

  const reportPlaybackState = useCallback((state: RemotePlaybackState) => {
    playbackState.current = state
  }, [])

  const acknowledgeCommand = useCallback((id: string) => {
    acknowledgedIDs.current.add(id)
    setCommands((current) => current.filter((command) => command.id !== id))
  }, [])

  useEffect(() => {
    if (!enabled) {
      setCommands([])
      acknowledgedIDs.current.clear()
      return
    }

    let active = true
    let timer: ReturnType<typeof setTimeout> | undefined
    let registered = false
    let retryDelay = 500
    const id = getRemoteDisplayId()

    const poll = async () => {
      try {
        if (!registered) {
          await api.registerRemoteDisplay(id)
          registered = true
        }
        if (!active) return
        const sentAcknowledgements = [...acknowledgedIDs.current]
        const response = await api.syncRemoteDisplay(id, playbackState.current, sentAcknowledgements)
        if (!active) return
        for (const commandID of sentAcknowledgements) acknowledgedIDs.current.delete(commandID)
        setCommands(response.commands.filter((command) => !acknowledgedIDs.current.has(command.id)))
        retryDelay = 500
        timer = setTimeout(poll, 500)
      } catch (error) {
        if (!active) return
        if (error instanceof ApiError && error.status === 404) registered = false
        timer = setTimeout(poll, retryDelay)
        retryDelay = Math.min(retryDelay * 2, 8000)
      }
    }

    void poll()
    return () => {
      active = false
      if (timer) clearTimeout(timer)
    }
  }, [enabled])

  return { commands, reportPlaybackState, acknowledgeCommand }
}
