import { describe, it, expect, beforeEach } from 'vitest'
import { confirmAction, settleConfirm, confirmState } from './useConfirm'

describe('useConfirm', () => {
  beforeEach(() => {
    confirmState.open = false
    confirmState.options = null
    confirmState.resolve = null
  })

  it('confirmAction 打开确认框并暂存选项', () => {
    const pending = confirmAction({ title: '删除分组', body: '不可撤销', danger: true })
    expect(pending).toBeInstanceOf(Promise)
    expect(confirmState.open).toBe(true)
    expect(confirmState.options).toMatchObject({ title: '删除分组', danger: true })
  })

  it('确认 → Promise 解析为 true，并清理待决状态', async () => {
    const pending = confirmAction({ title: 't' })
    settleConfirm(true)
    await expect(pending).resolves.toBe(true)
    expect(confirmState.open).toBe(false)
    expect(confirmState.resolve).toBeNull()
  })

  it('取消 → Promise 解析为 false', async () => {
    const pending = confirmAction({ title: 't' })
    settleConfirm(false)
    await expect(pending).resolves.toBe(false)
  })

  it('没有待决确认时 settleConfirm 不抛错', () => {
    expect(() => settleConfirm(true)).not.toThrow()
    expect(confirmState.open).toBe(false)
  })

  it('连续两次 confirmAction 只有后一次被解析', async () => {
    const first = confirmAction({ title: 'first' })
    const second = confirmAction({ title: 'second' })
    settleConfirm(true)
    await expect(second).resolves.toBe(true)
    let firstSettled = false
    void first.then(() => {
      firstSettled = true
    })
    await Promise.resolve()
    expect(firstSettled).toBe(false)
  })
})
