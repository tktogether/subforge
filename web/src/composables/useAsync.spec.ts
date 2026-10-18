import { describe, it, expect, vi } from 'vitest'
import { errorMessage, useAsync } from './useAsync'

describe('errorMessage', () => {
  it('传入 Error 实例时返回 err.message', () => {
    expect(errorMessage(new Error('boom'))).toBe('boom')
    expect(errorMessage(new TypeError('wrong type'))).toBe('wrong type')
  })

  it('传入字符串时返回 String(err)', () => {
    expect(errorMessage('oops')).toBe('oops')
    expect(errorMessage('')).toBe('')
  })

  it('传入数字时返回 String(err)', () => {
    expect(errorMessage(0)).toBe('0')
    expect(errorMessage(42)).toBe('42')
    expect(errorMessage(-1)).toBe('-1')
  })

  it('传入普通对象（非 Error）时返回 String(err)', () => {
    expect(errorMessage({ a: 1 })).toBe('[object Object]')
    expect(errorMessage(null)).toBe('null')
    expect(errorMessage(undefined)).toBe('undefined')
  })
})

describe('useAsync', () => {
  it('成功：loader 挂起期间 loading=true；await load() 后 data 为返回值、loading=false、error 为空', async () => {
    let resolve!: (value: string) => void
    const pending = new Promise<string>((r) => {
      resolve = r
    })
    const loader = vi.fn(() => pending)
    const state = useAsync<string>(loader)

    const loadPromise = state.load()
    expect(state.loading.value).toBe(true)
    expect(state.error.value).toBe('')
    expect(state.data.value).toBeNull()

    resolve('hello')
    await loadPromise

    expect(loader).toHaveBeenCalledTimes(1)
    expect(state.loading.value).toBe(false)
    expect(state.error.value).toBe('')
    expect(state.data.value).toBe('hello')
  })

  it('失败（抛 Error）：error 为错误消息、data 仍为 null、loading=false', async () => {
    const state = useAsync<string>(async () => {
      throw new Error('boom')
    })

    await state.load()

    expect(state.error.value).toBe('boom')
    expect(state.data.value).toBeNull()
    expect(state.loading.value).toBe(false)
  })

  it('失败（抛非 Error，例如字符串）：error 为 String(该值)', async () => {
    const state = useAsync<number>(async () => {
      throw 'string-failure'
    })

    await state.load()

    expect(state.error.value).toBe('string-failure')
    expect(state.data.value).toBeNull()
    expect(state.loading.value).toBe(false)
  })

  it('连续调用两次 load：第二次先清空 error 再重新加载', async () => {
    let calls = 0
    const state = useAsync<string>(async () => {
      calls++
      if (calls === 1) {
        throw new Error('first fail')
      }
      return 'ok'
    })

    await state.load()
    expect(state.error.value).toBe('first fail')

    const secondLoad = state.load()
    expect(state.error.value).toBe('')
    expect(state.loading.value).toBe(true)

    await secondLoad
    expect(calls).toBe(2)
    expect(state.error.value).toBe('')
    expect(state.data.value).toBe('ok')
    expect(state.loading.value).toBe(false)
  })
})
