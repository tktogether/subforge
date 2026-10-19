export function formatBytes(n: number): string {
  if (n <= 0) {
    return '0 B'
  }
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = n
  let unit = 0
  while (value >= 1024 && unit < units.length - 1) {
    value = value / 1024
    unit = unit + 1
  }
  if (unit === 0) {
    return value + ' B'
  }
  return value.toFixed(1) + ' ' + units[unit]
}

export function formatExpire(ts: number): string {
  if (ts <= 0) {
    return '未提供'
  }
  const d = new Date(ts * 1000)
  const pad = (x: number): string => (x < 10 ? '0' + x : String(x))
  return d.getFullYear() + '-' + pad(d.getMonth() + 1) + '-' + pad(d.getDate()) + ' ' + pad(d.getHours()) + ':' + pad(d.getMinutes())
}

export function formatPercent(used: number, total: number): string {
  if (total <= 0) {
    return '—'
  }
  const percent = (used / total) * 100
  return (percent >= 100 ? '100' : percent.toFixed(1)) + '%'
}

export function formatDuration(ms: number): string {
  if (ms < 1000) {
    return ms + ' ms'
  }
  const seconds = ms / 1000
  if (seconds < 60) {
    return seconds.toFixed(1) + ' s'
  }
  const minutes = Math.floor(seconds / 60)
  const rest = Math.floor(seconds % 60)
  return minutes + ' m ' + rest + ' s'
}

export function shortTime(iso: string): string {
  if (iso === '') {
    return '—'
  }
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) {
    return iso
  }
  return date.toLocaleString('zh-CN', { hour12: false })
}

export function formatTrafficPair(used: number, total: number): string {
  const usedStr = formatBytes(used)
  if (total <= 0) {
    return usedStr + ' / 未知'
  }
  const totalStr = formatBytes(total)
  const pct = (used / total) * 100
  return usedStr + ' / ' + totalStr + '（' + (pct >= 100 ? '100' : pct.toFixed(1)) + '%）'
}

export function formatRelative(ts: number, now?: number): string {
  const base = now === undefined ? Date.now() : now
  const diff = base - ts
  if (diff < 60000) {
    return '刚刚'
  }
  const minutes = Math.floor(diff / 60000)
  if (minutes < 60) {
    return minutes + ' 分钟前'
  }
  const hours = Math.floor(diff / 3600000)
  if (hours < 24) {
    return hours + ' 小时前'
  }
  const days = Math.floor(diff / 86400000)
  return days + ' 天前'
}
