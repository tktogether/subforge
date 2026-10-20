import { describe, it, expect, vi } from 'vitest'
import { errorDetail, errorMessage, useAsync } from './useAsync'
import { HttpError } from '../api/client'

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

describe('errorDetail（D-22）', () => {
  it('HttpError 时返回折叠用的原始报文', () => {
    const err = new HttpError(500, '{"error":"服务端内部错误（请求 ID r-1）","detail":"boom: 内部细节"}')
    expect(errorDetail(err)).toBe('boom: 内部细节')
  })

  it('普通 Error / 非 HttpError 时返回空串', () => {
    expect(errorDetail(new Error('boom'))).toBe('')
    expect(errorDetail('boom')).toBe('')
    expect(errorDetail(null)).toBe('')
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
