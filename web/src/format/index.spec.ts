import { describe, it, expect } from 'vitest'
import {
  formatBytes,
  formatExpire,
  formatPercent,
  formatDuration,
  shortTime,
} from './index'

describe('formatBytes', () => {
  it('bytes <= 0 返回「无限」', () => {
    expect(formatBytes(0)).toBe('无限')
    expect(formatBytes(-1)).toBe('无限')
    expect(formatBytes(-1024)).toBe('无限')
  })

  it('unit === 0：小于 1024 时保留原始字节并带 B 单位', () => {
    expect(formatBytes(1)).toBe('1 B')
    expect(formatBytes(512)).toBe('512 B')
    expect(formatBytes(1023)).toBe('1023 B')
  })

  it('到达 1024 时进位到 KB', () => {
    expect(formatBytes(1024)).toBe('1.00 KB')
    expect(formatBytes(1024 * 1024)).toBe('1.00 MB')
    expect(formatBytes(1024 * 1024 * 1024)).toBe('1.00 GB')
  })

  it('digits 分档：value >= 100 取 0 位小数', () => {
    expect(formatBytes(100 * 1024)).toBe('100 KB')
  })

  it('digits 分档：value >= 10 取 1 位小数', () => {
    expect(formatBytes(10 * 1024)).toBe('10.0 KB')
  })

  it('digits 分档：其余取 2 位小数', () => {
    expect(formatBytes(1536)).toBe('1.50 KB')
  })

  it('极大值封顶在 PB，不再继续进位', () => {
    expect(formatBytes(1024 ** 6)).toBe('1024 PB')
    expect(formatBytes(1024 ** 5)).toBe('1.00 PB')
  })
})

describe('formatExpire', () => {
  const unixSeconds = 1700000000 // 2023-11-14T22:13:20Z
  const baseMs = unixSeconds * 1000
  const stamp = '2023-11-14'

  it('unixSeconds <= 0 返回「未知」', () => {
    expect(formatExpire(0, baseMs)).toBe('未知')
    expect(formatExpire(-1, baseMs)).toBe('未知')
  })

  it('days < 0：已过期', () => {
    expect(formatExpire(unixSeconds, baseMs + 86400000)).toBe(stamp + '（已过期）')
    expect(formatExpire(unixSeconds, baseMs + 86400000 * 30)).toBe(stamp + '（已过期）')
  })

  it('days >= 0：剩余 0 天与剩余多天', () => {
    expect(formatExpire(unixSeconds, baseMs)).toBe(stamp + '（剩余 0 天）')
    expect(formatExpire(unixSeconds, baseMs - 86400000 * 3)).toBe(stamp + '（剩余 3 天）')
  })
})

describe('formatPercent', () => {
  it('total <= 0 返回「—」', () => {
    expect(formatPercent(0, 0)).toBe('—')
    expect(formatPercent(10, -5)).toBe('—')
  })

  it('percent >= 100 时封顶为 100%', () => {
    expect(formatPercent(100, 100)).toBe('100%')
    expect(formatPercent(150, 100)).toBe('100%')
  })

  it('正常值保留 1 位小数', () => {
    expect(formatPercent(1, 3)).toBe('33.3%')
    expect(formatPercent(1, 4)).toBe('25.0%')
    expect(formatPercent(0, 100)).toBe('0.0%')
  })
})

describe('formatDuration', () => {
  it('ms < 1000 直接返回毫秒', () => {
    expect(formatDuration(0)).toBe('0 ms')
    expect(formatDuration(999)).toBe('999 ms')
  })

  it('seconds < 60 返回秒（1 位小数）', () => {
    expect(formatDuration(1000)).toBe('1.0 s')
    expect(formatDuration(1500)).toBe('1.5 s')
    expect(formatDuration(59900)).toBe('59.9 s')
  })

  it('>= 60 秒返回「分 秒」', () => {
    expect(formatDuration(60000)).toBe('1 分 0 秒')
    expect(formatDuration(65000)).toBe('1 分 5 秒')
    expect(formatDuration(125000)).toBe('2 分 5 秒')
  })
})

describe('shortTime', () => {
  it('空串返回「—」', () => {
    expect(shortTime('')).toBe('—')
  })

  it('非法日期串原样返回', () => {
    expect(shortTime('not-a-date')).toBe('not-a-date')
    expect(shortTime('2023-13-45')).toBe('2023-13-45')
  })

  it('合法 ISO 串返回本地化时间字符串', () => {
    const iso = '2023-11-14T22:13:20Z'
    const result = shortTime(iso)
    expect(result).toBe(new Date(iso).toLocaleString('zh-CN', { hour12: false }))
    expect(result).toContain('2023')
    expect(result).not.toBe(iso)
  })
})
