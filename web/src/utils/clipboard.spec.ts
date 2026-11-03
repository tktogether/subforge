import { afterEach, describe, expect, it, vi } from 'vitest'
import { copyText } from './clipboard'

const originalExecCommand = document.execCommand

function setClipboard(value: unknown): void {
  Object.defineProperty(navigator, 'clipboard', {
    value,
    configurable: true,
    writable: true,
  })
}

function setExecCommand(value: unknown): void {
  Object.defineProperty(document, 'execCommand', {
    value,
    configurable: true,
    writable: true,
  })
}

describe('copyText', () => {
  afterEach(() => {
    setClipboard(undefined)
    setExecCommand(originalExecCommand)
    vi.restoreAllMocks()
  })

  it('Clipboard API 可用时直接写入', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    setClipboard({ writeText })
    setExecCommand(vi.fn(() => false))
    await expect(copyText('hello')).resolves.toBe(true)
    expect(writeText).toHaveBeenCalledWith('hello')
    expect(document.execCommand).not.toHaveBeenCalled()
  })

  it('Clipboard API 拒绝时回退到 execCommand', async () => {
    setClipboard({ writeText: vi.fn().mockRejectedValue(new Error('denied')) })
    setExecCommand(vi.fn(() => true))
    await expect(copyText('fallback')).resolves.toBe(true)
    expect(document.execCommand).toHaveBeenCalledWith('copy')
    expect(document.querySelectorAll('textarea')).toHaveLength(0)
  })

  it('HTTP 下没有 Clipboard API 时使用兼容复制', async () => {
    setClipboard(undefined)
    setExecCommand(vi.fn(() => true))
    await expect(copyText('http copy')).resolves.toBe(true)
    expect(document.execCommand).toHaveBeenCalledWith('copy')
  })

  it('execCommand 返回 false 时报告失败', async () => {
    setClipboard(undefined)
    setExecCommand(vi.fn(() => false))
    await expect(copyText('x')).resolves.toBe(false)
  })

  it('execCommand 抛异常时也清理临时节点并报告失败', async () => {
    setClipboard(undefined)
    setExecCommand(vi.fn(() => {
      throw new Error('not supported')
    }))
    await expect(copyText('x')).resolves.toBe(false)
    expect(document.querySelectorAll('textarea')).toHaveLength(0)
  })
})
