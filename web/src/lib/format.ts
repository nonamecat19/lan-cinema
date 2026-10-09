export function pad(value: number): string {
  return String(value).padStart(2, '0')
}

export function formatTime(value: number): string {
  const seconds = Math.max(0, Math.floor(Number(value) || 0))
  const minutes = Math.floor(seconds / 60)
  return `${minutes}:${pad(seconds % 60)}`
}

export function formatBytes(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const rank = Math.min(units.length - 1, Math.floor(Math.log(value) / Math.log(1024)))
  return `${(value / (1024 ** rank)).toFixed(rank > 1 ? 1 : 0)} ${units[rank]}`
}
