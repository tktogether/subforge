import { describe, it, expect } from 'vitest'
import {
  formatBytes,
  formatTrafficPair,
  formatExpire,
  formatDuration,
  formatRelative,
  formatPercent,
  shortTime,
} from './index'

describe('formatBytes', () => {
  it('0 → 0 B', () => {
    expect(formatBytes(0)).toBe('0 B')
  })

  it('负数 → 0 B', () => {
    expect(formatBytes(-1)).toBe('0 B')
    expect(formatBytes(-1024)).toBe('0 B')
  })

  it('B 不带小数', () => {
    expect(formatBytes(1)).toBe('1 B')
    expect(formatBytes(512)).toBe('512 B')
    expect(formatBytes(1023)).toBe('1023 B')
  })

  it('KB 保留 1 位小数', () => {
    expect(formatBytes(1024)).toBe('1.0 KB')
    expect(formatBytes(1536)).toBe('1.5 KB')
  })

  it('MB/GB/TB 保留 1 位小数', () => {
    expect(formatBytes(1024 * 1024)).toBe('1.0 MB')
    expect(formatBytes(1024 * 1024 * 1024)).toBe('1.0 GB')
    expect(formatBytes(1024 * 1024 * 1024 * 1024)).toBe('1.0 TB')
  })

  it('封顶 TB', () => {
    expect(formatBytes(1024 ** 5)).toBe('1024.0 TB')
  })
})

describe('formatTrafficPair', () => {
  it('total <= 0 → used / 未知', () => {
    expect(formatTrafficPair(1024, 0)).toBe('1.0 KB / 未知')
    expect(formatTrafficPair(1024, -5)).toBe('1.0 KB / 未知')
  })

  it('正常：used / total（percent%）', () => {
    expect(formatTrafficPair(1024, 10240)).toBe('1.0 KB / 10.0 KB（10.0%）')
  })

  it('percent >= 100 封顶', () => {
    expect(formatTrafficPair(2048, 1024)).toBe('2.0 KB / 1.0 KB（100%）')
  })
})

describe('formatExpire', () => {
  it('0 → 未提供', () => {
    expect(formatExpire(0)).toBe('未提供')
  })

  it('负数 → 未提供', () => {
    expect(formatExpire(-1)).toBe('未提供')
  })

  it('正常 → YYYY-MM-DD HH:mm', () => {
    const ts = 1700000000 // 2023-11-14T22:13:20Z
    const d = new Date(ts * 1000)
    const pad = (x: number) => (x < 10 ? '0' + x : String(x))
    const expected = d.getFullYear() + '-' + pad(d.getMonth() + 1) + '-' + pad(d.getDate()) + ' ' + pad(d.getHours()) + ':' + pad(d.getMinutes())
    expect(formatExpire(ts)).toBe(expected)
  })
})

describe('formatDuration', () => {
  it('ms < 1000 → <ms> ms', () => {
    expect(formatDuration(0)).toBe('0 ms')
    expect(formatDuration(999)).toBe('999 ms')
  })

  it('seconds < 60 → x.x s', () => {
    expect(formatDuration(1000)).toBe('1.0 s')
    expect(formatDuration(1500)).toBe('1.5 s')
    expect(formatDuration(59900)).toBe('59.9 s')
  })

  it('>= 60 秒 → x m y s', () => {
    expect(formatDuration(60000)).toBe('1 m 0 s')
    expect(formatDuration(65000)).toBe('1 m 5 s')
    expect(formatDuration(125000)).toBe('2 m 5 s')
  })
})

describe('formatRelative', () => {
  it('diff < 60000 → 刚刚', () => {
    const now = 1000000
    expect(formatRelative(now - 30000, now)).toBe('刚刚')
    expect(formatRelative(now, now)).toBe('刚刚')
  })

  it('N 分钟前', () => {
    const now = 1000000
    expect(formatRelative(now - 5 * 60000, now)).toBe('5 分钟前')
    expect(formatRelative(now - 59 * 60000, now)).toBe('59 分钟前')
  })

  it('N 小时前', () => {
    const now = 1000000
    expect(formatRelative(now - 2 * 3600000, now)).toBe('2 小时前')
  })

  it('N 天前', () => {
    const now = 1000000
    expect(formatRelative(now - 3 * 86400000, now)).toBe('3 天前')
  })

  it('now 缺省使用 Date.now', () => {
    const ts = Date.now() - 1000
    expect(formatRelative(ts)).toBe('刚刚')
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
