import { beforeEach, describe, expect, it } from 'vitest'
import { closeNotify, notify, notifyState, snackbarColor } from './useNotify'

describe('useNotify', () => {
  beforeEach(() => {
    notifyState.open = false
    notifyState.text = ''
    notifyState.kind = 'ok'
  })

  it('notify 不传 kind 时默认为 ok，并写入 text、打开弹层', () => {
    notify('x')
    expect(notifyState.kind).toBe('ok')
    expect(notifyState.text).toBe('x')
    expect(notifyState.open).toBe(true)
  })

  it('notify 支持 error 与 info 两种 kind', () => {
    notify('y', 'error')
    expect(notifyState.kind).toBe('error')
    expect(notifyState.text).toBe('y')
    expect(notifyState.open).toBe(true)

    notify('z', 'info')
    expect(notifyState.kind).toBe('info')
    expect(notifyState.text).toBe('z')
    expect(notifyState.open).toBe(true)
  })

  it('closeNotify 只关闭弹层，保留 text 与 kind', () => {
    notify('y', 'error')
    closeNotify()
    expect(notifyState.open).toBe(false)
    expect(notifyState.text).toBe('y')
    expect(notifyState.kind).toBe('error')
  })

  it('snackbarColor 按 kind 映射颜色', () => {
    expect(snackbarColor('error')).toBe('error')
    expect(snackbarColor('info')).toBe('info')
    expect(snackbarColor('ok')).toBe('success')
  })
})
