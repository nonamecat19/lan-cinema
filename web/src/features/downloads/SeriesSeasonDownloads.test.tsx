import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import type { TorrentDownload } from '../../domain/downloads'
import type { Series } from '../../domain/media'
import { SeriesSeasonDownloads } from './SeriesSeasonDownloads'

const sourceId = 'source-id'
const hash = 'a'.repeat(40)
const series: Series = {
  id: 'bones',
  title: 'Bones',
  seasons: [],
  torrentSources: [{
    id: sourceId,
    hash,
    name: 'Bones torrent',
    forceOneByOne: false,
    seasons: [
      { number: 7, episodes: 1, selected: true, paused: false, deleting: false },
      { number: 8, episodes: 1, selected: true, paused: false, deleting: false },
      { number: 9, episodes: 1, selected: true, paused: false, deleting: false },
    ],
    availableSeasons: [],
  }],
}

const download: TorrentDownload = {
  hash,
  name: 'Bones torrent',
  progress: 0.43,
  state: 'downloading',
  downloadSpeed: 1024,
  uploadSpeed: 0,
  eta: 100,
  size: 300,
  completed: 129,
  paused: false,
  files: [
    { name: 'S07E01.mkv', season: 7, episode: 1, size: 100, progress: 1, status: 'ready', detail: 'Ready to watch' },
    { name: 'S08E01.mkv', season: 8, episode: 1, size: 100, progress: 0.2, status: 'downloading', detail: 'Downloading from peers' },
    { name: 'S09E01.mkv', season: 9, episode: 1, size: 100, progress: 0.09, status: 'downloading', detail: 'Downloading from peers' },
  ],
}

afterEach(() => { cleanup(); vi.unstubAllGlobals() })

it('shows selected seasons before they enter the library', async () => {
  vi.stubGlobal('fetch', vi.fn(async () => ({ ok: true, status: 200, json: async () => [download] })))
  render(<SeriesSeasonDownloads series={series} reload={async () => {}} />)
  const season8 = (await screen.findByText('Season 08')).closest('.series-download-season') as HTMLElement
  const season9 = screen.getByText('Season 09').closest('.series-download-season') as HTMLElement
  expect(within(season8).getByText(/Downloading · 0\/1 downloaded/)).toBeTruthy()
  expect(within(season8).getByText(/20.0%/)).toBeTruthy()
  expect(within(season9).getByText(/Downloading · 0\/1 downloaded/)).toBeTruthy()
  expect(within(season8).getByRole('progressbar').getAttribute('aria-valuenow')).toBe('20')
})

it('requires confirmation before deleting a season and exposes stable TV controls', async () => {
  const fetchMock = vi.fn(async (input: RequestInfo | URL) => ({ ok: true, status: 200, json: async () => String(input) === '/api/downloads' ? [download] : { updated: true } }))
  vi.stubGlobal('fetch', fetchMock)
  render(<SeriesSeasonDownloads series={series} reload={async () => {}} />)
  const season8 = (await screen.findByText('Season 08')).closest('.series-download-season') as HTMLElement
  const pause = within(season8).getByRole('button', { name: 'Pause' })
  expect(pause.getAttribute('data-focus-key')).toBe(`season-pause-${sourceId}-8`)
  fireEvent.click(within(season8).getByRole('button', { name: 'Delete' }))
  expect(fetchMock.mock.calls.some(([input]) => String(input) === '/api/series/bones/seasons/8/delete')).toBe(false)
  fireEvent.click(within(season8).getByRole('button', { name: 'Delete season' }))
  await waitFor(() => expect(fetchMock.mock.calls.some(([input]) => String(input) === '/api/series/bones/seasons/8/delete')).toBe(true))
})
