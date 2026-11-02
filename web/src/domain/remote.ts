export interface RemotePlaybackState {
  episodeId: string | null
  playing: boolean
  position: number
  duration: number
  volume: number
  muted: boolean
  error: string | null
}

export type RemoteCommandPayload =
  | { type: 'playEpisode'; episodeId: string }
  | { type: 'setPlaying'; playing: boolean }
  | { type: 'seek'; position: number }
  | { type: 'setMuted'; muted: boolean }
  | { type: 'setVolume'; volume: number }
  | { type: 'stop' }

export type RemoteCommand = RemoteCommandPayload & { id: string }

export interface RemoteDisplay {
  id: string
  label: string
  state: RemotePlaybackState
}

export interface RemoteSyncResponse {
  commands: RemoteCommand[]
}
