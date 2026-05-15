export function formatDateTime(unixSec: number | string | undefined): string {
  if (!unixSec) return '-'
  const n = typeof unixSec === 'string' ? Date.parse(unixSec) / 1000 : unixSec
  if (!n || Number.isNaN(n)) return '-'
  const d = new Date(n * 1000)
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  const h = String(d.getHours()).padStart(2, '0')
  const mi = String(d.getMinutes()).padStart(2, '0')
  const s = String(d.getSeconds()).padStart(2, '0')
  return `${y}/${m}/${day} ${h}:${mi}:${s}`
}

export function formatRemaining(sec: number): string {
  if (sec <= 0) return '已过期'
  const d = Math.floor(sec / 86400)
  const h = Math.floor((sec % 86400) / 3600)
  const m = Math.floor((sec % 3600) / 60)
  if (d > 0) return `剩余 ${d}天${h}小时`
  if (h > 0) return `剩余 ${h}小时${m}分钟`
  return `剩余 ${m} 分钟`
}

// Token typical lifetime is ~7 days. Use 7 days as full-bar reference.
const FULL_BAR_SEC = 7 * 24 * 3600

export function tokenProgressPercent(sec: number): number {
  if (sec <= 0) return 0
  return Math.min(100, Math.max(2, (sec / FULL_BAR_SEC) * 100))
}

export function tokenProgressColor(sec: number): string {
  if (sec <= 0) return 'bg-gray-300'
  if (sec < 24 * 3600) return 'bg-red-500'
  if (sec < 3 * 24 * 3600) return 'bg-amber-500'
  return 'bg-emerald-500'
}
