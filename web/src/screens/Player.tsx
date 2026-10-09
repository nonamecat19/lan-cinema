import { useCallback, useEffect, useRef, useState } from 'react'
import type { CSSProperties, MouseEvent } from 'react'
import { Button, StateScreen } from '../components/Chrome'
import type { Episode, EpisodeLocation, EpisodeWithSeason } from '../domain/media'
import type { Navigate } from '../hooks/useRoute'
import { useRemoteNavigation } from '../remote/useRemoteNavigation'
import { remoteAction } from '../remote/keys'
import { nextReadyEpisode } from '../lib/catalog'
import { formatTime, pad } from '../lib/format'
import { clearProgress, resumePosition, writeProgress } from '../lib/progress'

function previewStyle(episode: Episode, time: number): CSSProperties | undefined {
  if (!episode.previewSpriteUrl) return undefined
  const columns = Math.max(1, episode.previewColumns || 10)
  const rows = Math.max(1, episode.previewRows || 1)
  const interval = Math.max(1, episode.previewIntervalSeconds || 10)
  const index = Math.max(0, Math.min(columns * rows - 1, Math.floor(time / interval)))
  const column = index % columns
  const row = Math.floor(index / columns)
  return {
    backgroundImage: `url("${episode.previewSpriteUrl}")`,
    backgroundSize: `${columns * 100}% ${rows * 100}%`,
    backgroundPosition: `${columns === 1 ? 0 : column / (columns - 1) * 100}% ${rows === 1 ? 0 : row / (rows - 1) * 100}%`,
  }
}

export function Player({ episode, season, series, navigate, back }: EpisodeLocation & { navigate: Navigate; back: () => void }) {
  const videoRef = useRef<HTMLVideoElement | null>(null)
  const hideTimer = useRef<number | undefined>(undefined)
  const previewTimer = useRef<number | undefined>(undefined)
  const ended = useRef(false)
  const [error, setError] = useState(false)
  const [autoplayBlocked, setAutoplayBlocked] = useState(false)
  const [controlsVisible, setControlsVisible] = useState(true)
  const [playing, setPlaying] = useState(false)
  const [position, setPosition] = useState(0)
  const [duration, setDuration] = useState(0)
  const [previewTime, setPreviewTime] = useState<number | null>(null)
  const [nextEpisode, setNextEpisode] = useState<EpisodeWithSeason | null>(null)
  const [countdown, setCountdown] = useState<number | null>(null)
  const overlay = error || autoplayBlocked || countdown !== null
  useRemoteNavigation(overlay, `player-${episode.id}-${error}-${autoplayBlocked}-${countdown !== null}`)

  const saveProgress = useCallback(() => {
    const video = videoRef.current
    if (!ended.current && video) writeProgress(episode.id, video.currentTime, video.duration)
  }, [episode.id])

  const revealControls = useCallback(() => {
    setControlsVisible(true)
    clearTimeout(hideTimer.current)
    hideTimer.current = window.setTimeout(() => setControlsVisible(false), 4500)
  }, [])

  useEffect(() => {
    const video = videoRef.current
    if (!video) return undefined
    ended.current = false
    setError(false)
    setAutoplayBlocked(false)
    setNextEpisode(null)
    setCountdown(null)
    setPosition(0)
    setDuration(0)
    video.src = episode.streamUrl || ''
    video.load()
    revealControls()
    return () => {
      if (!ended.current) writeProgress(episode.id, video.currentTime, video.duration)
      video.pause()
      video.removeAttribute('src')
      video.load()
      clearTimeout(hideTimer.current)
      clearTimeout(previewTimer.current)
    }
  }, [episode.id, episode.streamUrl, revealControls])

  useEffect(() => {
    const timer = setInterval(saveProgress, 5000)
    window.addEventListener('pagehide', saveProgress)
    return () => { clearInterval(timer); window.removeEventListener('pagehide', saveProgress) }
  }, [saveProgress])

  useEffect(() => {
    if (countdown === null || !nextEpisode || countdown <= 0) return undefined
    const timer = setTimeout(() => {
      if (countdown === 1) navigate(`/watch/${nextEpisode.id}`, true)
      else setCountdown(countdown - 1)
    }, 1000)
    return () => clearTimeout(timer)
  }, [countdown, nextEpisode, navigate])

  function onMetadata() {
    const video = videoRef.current
    if (!video) return
    const resume = resumePosition(episode)
    if (resume) video.currentTime = resume
    setPosition(resume)
    setDuration(Number.isFinite(video.duration) ? video.duration : Number(episode.durationSeconds) || 0)
    video.play().catch(() => setAutoplayBlocked(true))
  }

  function togglePlayback() {
    const video = videoRef.current
    if (!video) return
    if (video.paused) video.play().then(() => setAutoplayBlocked(false)).catch(() => setError(true))
    else video.pause()
    revealControls()
  }

  function seekTo(value: number) {
    const video = videoRef.current
    if (!video || !Number.isFinite(video.duration)) return
    const target = Math.max(0, Math.min(video.duration, value))
    video.currentTime = target
    setPosition(target)
    setPreviewTime(target)
    clearTimeout(previewTimer.current)
    previewTimer.current = window.setTimeout(() => setPreviewTime(null), 2200)
    revealControls()
  }

  function timeAtPointer(event: MouseEvent<HTMLDivElement>) {
    const bounds = event.currentTarget.getBoundingClientRect()
    return Math.max(0, Math.min(1, (event.clientX - bounds.left) / bounds.width)) * duration
  }

  function finishEpisode() {
    ended.current = true
    clearProgress(episode.id)
    const upcoming = nextReadyEpisode(series, episode.id)
    setNextEpisode(upcoming)
    setCountdown(upcoming ? 5 : 0)
    setControlsVisible(true)
  }

  function playNext() {
    if (nextEpisode) navigate(`/watch/${nextEpisode.id}`, true)
    else {
      ended.current = false
      setCountdown(null)
      const video = videoRef.current
      if (video) {
        video.currentTime = 0
        video.play().catch(() => setError(true))
      }
    }
  }

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (overlay) return
      const action = remoteAction(event)
      const direction = action?.kind === 'move' ? action.direction : null
      if (direction === 'left' || direction === 'right') {
        event.preventDefault()
        seekTo((videoRef.current?.currentTime || 0) + (direction === 'right' ? 10 : -10))
      } else if (direction === 'up' || direction === 'down') {
        event.preventDefault()
        revealControls()
      } else if (action?.kind === 'select') {
        event.preventDefault()
        togglePlayback()
      }
    }
    document.addEventListener('keydown', onKeyDown)
    return () => document.removeEventListener('keydown', onKeyDown)
  }, [overlay, revealControls])

  const total = duration || Number(episode.durationSeconds) || 0
  return <main className="player-screen" onMouseMove={revealControls}>
    <video ref={videoRef} playsInline preload="auto" onLoadedMetadata={onMetadata} onTimeUpdate={(event) => { setPosition(event.currentTarget.currentTime); if (Number.isFinite(event.currentTarget.duration)) setDuration(event.currentTarget.duration) }} onPlay={() => setPlaying(true)} onPause={() => { setPlaying(false); saveProgress() }} onEnded={finishEpisode} onError={() => { setError(true); setAutoplayBlocked(false) }} />
    <div className={`player-vignette ${controlsVisible || overlay ? 'visible' : ''}`} />
    {(controlsVisible || overlay) && <div className="player-top"><button type="button" className="back-icon" aria-label="Back to episodes" onClick={back}>←</button><div><span className="eyebrow">{series.title} / SEASON {pad(season.number)}</span><h1>E{pad(episode.number)} · {episode.title}</h1></div><span className="playing-label">NOW PLAYING</span></div>}
    {error && <div className="player-message" role="alert"><span className="eyebrow">CONNECTION INTERRUPTED</span><h2>Playback stopped</h2><p>Check the server and try this episode again.</p><div className="action-row"><Button focusKey="player-retry" onClick={() => { setError(false); videoRef.current?.load() }}>Retry</Button><Button className="secondary" focusKey="player-error-back" onClick={back}>Back to episodes</Button></div></div>}
    {autoplayBlocked && !error && <div className="center-action"><Button focusKey="player-play" onClick={togglePlayback}>▶ Play episode</Button></div>}
    {countdown !== null && !error && <div className="next-card"><span className="eyebrow">{nextEpisode ? 'UP NEXT' : 'EPISODE COMPLETE'}</span><h2>{nextEpisode?.title || 'You’re all caught up'}</h2><p>{nextEpisode ? `Starting in ${countdown} seconds` : 'Choose what to do next'}</p><div className="action-row"><Button focusKey="player-next" onClick={playNext}>{nextEpisode ? 'Play now' : 'Replay'}</Button><Button className="secondary" focusKey="player-browse" onClick={back}>Browse episodes</Button></div></div>}
    {!error && countdown === null && <div className={`player-controls ${controlsVisible ? 'visible' : ''}`}><div className="play-row"><button type="button" aria-label={playing ? 'Pause' : 'Play'} onClick={togglePlayback}>{playing ? 'Ⅱ' : '▶'}</button><span>{formatTime(position)}</span><div className="timeline-control" role="slider" aria-label="Playback position" aria-valuemin={0} aria-valuemax={Math.floor(total)} aria-valuenow={Math.floor(position)} tabIndex={-1} onPointerMove={(event) => setPreviewTime(timeAtPointer(event))} onPointerLeave={() => setPreviewTime(null)} onClick={(event) => seekTo(timeAtPointer(event))}>
      {previewTime !== null && <div className="seek-preview" style={{ left: `${Math.max(12, Math.min(88, total ? previewTime / total * 100 : 50))}%` }}><div className="seek-preview-frame" style={previewStyle(episode, previewTime)}>{!episode.previewSpriteUrl && episode.previewUrl && <img src={episode.previewUrl} alt="" />}</div><span>{formatTime(previewTime)}</span></div>}
      <div className="progress-track"><div className="progress-fill" style={{ width: `${total ? Math.min(100, position / total * 100) : 0}%` }} /></div>
    </div><span>{formatTime(total)}</span><span className="remote-hint">4 / ← 10s &nbsp; · &nbsp; OK / 5 PLAY / PAUSE &nbsp; · &nbsp; 10s 6 / →</span></div></div>}
  </main>
}

export function UnavailableEpisode({ episode, navigate }: { episode?: Episode; navigate: () => void }) {
  return <StateScreen eyebrow="PLAYBACK UNAVAILABLE" title="We can’t open this episode" description={episode?.unavailableReason || 'This episode is not in the current catalog.'} action={<Button focusKey="unavailable-back" onClick={navigate}>Back to episodes</Button>} />
}
