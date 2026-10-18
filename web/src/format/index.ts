export function formatBytes(bytes: number): string {
  if (bytes <= 0) {
    return '无限'
  }
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let value = bytes
  let unit = 0
  while (value >= 1024 && unit < units.length - 1) {
    value = value / 1024
    unit = unit + 1
  }
  if (unit === 0) {
    return value + ' B'
  }
  const digits = value >= 100 ? 0 : value >= 10 ? 1 : 2
  return value.toFixed(digits) + ' ' + units[unit]
}

export function formatExpire(unixSeconds: number, nowMs: number = Date.now()): string {
  if (unixSeconds <= 0) {
    return '未知'
  }
  const date = new Date(unixSeconds * 1000)
  const stamp = date.toISOString().slice(0, 10)
  const days = Math.floor((date.getTime() - nowMs) / 86400000)
  if (days < 0) {
    return stamp + '（已过期）'
  }
  return stamp + '（剩余 ' + days + ' 天）'
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
  return minutes + ' 分 ' + rest + ' 秒'
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
