import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import {
  HttpError,
  buildQuery,
  request,
  api,
  setTokenProvider,
  setUnauthorizedHandler,
} from './client'

function mockResponse(init: {
  ok: boolean
  status: number
  text?: string
  contentType?: string
}): Response {
  const headers = new Headers()
  if (init.contentType !== undefined) {
    headers.set('Content-Type', init.contentType)
  }
  return {
    ok: init.ok,
    status: init.status,
    statusText: '',
    headers,
    text: async () => init.text ?? '',
  } as unknown as Response
}

describe('HttpError', () => {
  it('body 为空串时 message 恰为 "HTTP 404"', () => {
    const err = new HttpError(404, '')
    expect(err.message).toBe('HTTP 404')
  })

  it('body 非空时 message 为 "HTTP 500: boom"', () => {
    const err = new HttpError(500, 'boom')
    expect(err.message).toBe('HTTP 500: boom')
  })

  it('name / status / body 字段正确', () => {
    const err = new HttpError(400, 'bad request')
    expect(err.name).toBe('HttpError')
    expect(err.status).toBe(400)
    expect(err.body).toBe('bad request')
  })
})

describe('buildQuery', () => {
  it('参数 undefined → 返回空串', () => {
    expect(buildQuery(undefined)).toBe('')
  })

  it('undefined / null 的键被跳过', () => {
    expect(buildQuery({ a: undefined, b: null, c: 1 })).toBe('?c=1')
  })

  it('全部被跳过 → 空串', () => {
    expect(buildQuery({ a: undefined, b: null })).toBe('')
  })

  it('正常对象 → "?a=1&b=x&c=true"', () => {
    expect(buildQuery({ a: 1, b: 'x', c: true })).toBe('?a=1&b=x&c=true')
  })

  it('含特殊字符的值被 URL 编码', () => {
    expect(buildQuery({ msg: 'hello world', q: 'a&b' })).toBe('?msg=hello+world&q=a%26b')
  })
})

describe('request', () => {
  beforeEach(() => {
    setTokenProvider(() => '')
    setUnauthorizedHandler(() => {})
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('401 时调用未授权处理器（供界面弹出令牌输入框）', async () => {
    const onUnauthorized = vi.fn()
    setUnauthorizedHandler(onUnauthorized)
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(mockResponse({ ok: false, status: 401, text: '401: 缺少或错误的访问令牌' })),
    )
    await expect(request('GET', '/api/version')).rejects.toThrow('HTTP 401')
    expect(onUnauthorized).toHaveBeenCalledTimes(1)
  })

  it('其它错误状态不触发未授权处理器', async () => {
    const onUnauthorized = vi.fn()
    setUnauthorizedHandler(onUnauthorized)
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(mockResponse({ ok: false, status: 500, text: 'boom' })))
    await expect(request('GET', '/api/version')).rejects.toThrow('HTTP 500')
    expect(onUnauthorized).not.toHaveBeenCalled()
  })

  it('401 的响应体进入 HttpError.body，便于界面显示服务端提示', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(mockResponse({ ok: false, status: 401, text: '401: 缺少或错误的访问令牌' })),
    )
    await expect(request('GET', '/api/version')).rejects.toMatchObject({
      status: 401,
      body: '401: 缺少或错误的访问令牌',
    })
  })

  it('token 为空 → 请求头没有 X-Token', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      mockResponse({ ok: true, status: 200, text: 'ok', contentType: 'text/plain' }),
    )
    vi.stubGlobal('fetch', fetchMock)
    await request('GET', '/x')
    expect(fetchMock).toHaveBeenCalledTimes(1)
    const init = fetchMock.mock.calls[0][1] as RequestInit
    const headers = init.headers as Record<string, string>
    expect(headers['X-Token']).toBeUndefined()
  })

  it('setTokenProvider(() => "tok") 后 X-Token = "tok"', async () => {
    setTokenProvider(() => 'tok')
    const fetchMock = vi.fn().mockResolvedValue(
      mockResponse({ ok: true, status: 200, text: 'ok', contentType: 'text/plain' }),
    )
    vi.stubGlobal('fetch', fetchMock)
    await request('GET', '/x')
    const init = fetchMock.mock.calls[0][1] as RequestInit
    const headers = init.headers as Record<string, string>
    expect(headers['X-Token']).toBe('tok')
  })

  it('body 非 undefined → Content-Type application/json 且 body 为 JSON 字符串', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      mockResponse({ ok: true, status: 200, text: '{"ok":true}', contentType: 'application/json' }),
    )
    vi.stubGlobal('fetch', fetchMock)
    await request('POST', '/x', { body: { a: 1 } })
    const init = fetchMock.mock.calls[0][1] as RequestInit
    const headers = init.headers as Record<string, string>
    expect(headers['Content-Type']).toBe('application/json')
    expect(init.body).toBe('{"a":1}')
  })

  it('response.ok 为 false → 抛出 HttpError 且 status/body 正确', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      mockResponse({ ok: false, status: 500, text: 'boom', contentType: 'text/plain' }),
    )
    vi.stubGlobal('fetch', fetchMock)
    let caught: unknown
    try {
      await request('GET', '/x')
    } catch (e) {
      caught = e
    }
    expect(caught).toBeInstanceOf(HttpError)
    const err = caught as HttpError
    expect(err.status).toBe(500)
    expect(err.body).toBe('boom')
  })

  it('status 204 → 返回 undefined', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      mockResponse({ ok: true, status: 204, text: '' }),
    )
    vi.stubGlobal('fetch', fetchMock)
    const result = await request('DELETE', '/x')
    expect(result).toBeUndefined()
  })

  it('200 且空文本 → 返回 undefined', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      mockResponse({ ok: true, status: 200, text: '' }),
    )
    vi.stubGlobal('fetch', fetchMock)
    const result = await request('GET', '/x')
    expect(result).toBeUndefined()
  })

  it('Content-Type 含 application/json → 返回 JSON.parse 的对象', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      mockResponse({ ok: true, status: 200, text: '{"a":1,"b":"x"}', contentType: 'application/json' }),
    )
    vi.stubGlobal('fetch', fetchMock)
    const result = await request<{ a: number; b: string }>('GET', '/x')
    expect(result).toEqual({ a: 1, b: 'x' })
  })

  it('Content-Type 为 text/plain → 原样返回字符串', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      mockResponse({ ok: true, status: 200, text: 'hello world', contentType: 'text/plain' }),
    )
    vi.stubGlobal('fetch', fetchMock)
    const result = await request<string>('GET', '/x')
    expect(result).toBe('hello world')
  })

  it('Content-Type 头缺失 → 原样返回字符串', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      mockResponse({ ok: true, status: 200, text: 'raw text' }),
    )
    vi.stubGlobal('fetch', fetchMock)
    const result = await request<string>('GET', '/x')
    expect(result).toBe('raw text')
  })
})

describe('api', () => {
  beforeEach(() => {
    setTokenProvider(() => '')
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('api.get method 为 GET', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      mockResponse({ ok: true, status: 200, text: 'ok', contentType: 'text/plain' }),
    )
    vi.stubGlobal('fetch', fetchMock)
    await api.get('/x')
    const call = fetchMock.mock.calls[0] as unknown as [string, RequestInit]
    expect(call[0]).toBe('/x')
    expect(call[1].method).toBe('GET')
  })

  it('api.post method 为 POST 且 body 透传进 fetch', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      mockResponse({ ok: true, status: 200, text: '{}', contentType: 'application/json' }),
    )
    vi.stubGlobal('fetch', fetchMock)
    await api.post('/x', { name: 'n' })
    const call = fetchMock.mock.calls[0] as unknown as [string, RequestInit]
    expect(call[0]).toBe('/x')
    expect(call[1].method).toBe('POST')
    expect(call[1].body).toBe('{"name":"n"}')
  })

  it('api.put method 为 PUT 且 body 透传进 fetch', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      mockResponse({ ok: true, status: 200, text: '{}', contentType: 'application/json' }),
    )
    vi.stubGlobal('fetch', fetchMock)
    await api.put('/x', { name: 'n' })
    const call = fetchMock.mock.calls[0] as unknown as [string, RequestInit]
    expect(call[0]).toBe('/x')
    expect(call[1].method).toBe('PUT')
    expect(call[1].body).toBe('{"name":"n"}')
  })

  it('api.del method 为 DELETE', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      mockResponse({ ok: true, status: 204, text: '' }),
    )
    vi.stubGlobal('fetch', fetchMock)
    await api.del('/x')
    const call = fetchMock.mock.calls[0] as unknown as [string, RequestInit]
    expect(call[0]).toBe('/x')
    expect(call[1].method).toBe('DELETE')
  })
})


describe('default tokenProvider (fresh module)', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('未调用 setTokenProvider 时默认返回空串 → 无 X-Token', async () => {
    vi.resetModules()
    const mod = await import('./client')
    const fetchMock = vi.fn().mockResolvedValue(
      mockResponse({ ok: true, status: 200, text: 'ok', contentType: 'text/plain' }),
    )
    vi.stubGlobal('fetch', fetchMock)
    await mod.request('GET', '/x')
    const init = fetchMock.mock.calls[0][1] as RequestInit
    const headers = init.headers as Record<string, string>
    expect(headers['X-Token']).toBeUndefined()
  })
})
