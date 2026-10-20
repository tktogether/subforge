import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { request, setTokenProvider, setUnauthorizedHandler } from './client'
import {
  TOKEN_STORAGE_KEY,
  closeTokenDialog,
  initToken,
  logout,
  openTokenDialog,
  readStoredToken,
  saveToken,
  tokenDialogOpen,
  tokenRejected,
  tokenState,
} from './token'

function response(ok: boolean, status: number, text = ''): Response {
  return {
    ok,
    status,
    statusText: '',
    headers: new Headers(),
    text: async () => text,
  } as unknown as Response
}

describe('api/token', () => {
  beforeEach(() => {
    window.localStorage.clear()
    tokenState.value = ''
    tokenDialogOpen.value = false
    tokenRejected.value = false
    // 每个用例都从干净的请求层开始，避免上一个用例注册的提供者残留。
    setTokenProvider(() => '')
    setUnauthorizedHandler(() => {})
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('readStoredToken：未写入时返回空串', () => {
    expect(readStoredToken()).toBe('')
  })

  it('readStoredToken：写入后返回该值', () => {
    window.localStorage.setItem(TOKEN_STORAGE_KEY, 'abc123')
    expect(readStoredToken()).toBe('abc123')
  })

  it('saveToken：去掉首尾空白后写入 localStorage 与 tokenState', () => {
    saveToken('  tok-1  ')
    expect(window.localStorage.getItem(TOKEN_STORAGE_KEY)).toBe('tok-1')
    expect(tokenState.value).toBe('tok-1')
  })

  it('saveToken：空串清除 localStorage 中的令牌', () => {
    saveToken('tok-1')
    saveToken('   ')
    expect(window.localStorage.getItem(TOKEN_STORAGE_KEY)).toBeNull()
    expect(tokenState.value).toBe('')
  })

  it('saveToken：清掉上一次的 401 标记', () => {
    tokenRejected.value = true
    saveToken('tok-1')
    expect(tokenRejected.value).toBe(false)
  })

  it('openTokenDialog / closeTokenDialog 切换输入框显示状态', () => {
    openTokenDialog()
    expect(tokenDialogOpen.value).toBe(true)
    closeTokenDialog()
    expect(tokenDialogOpen.value).toBe(false)
  })

  it('initToken：把 localStorage 里的令牌读进内存状态', () => {
    window.localStorage.setItem(TOKEN_STORAGE_KEY, 'stored-tok')
    initToken()
    expect(tokenState.value).toBe('stored-tok')
  })

  it('initToken 后请求自动带 X-Token，且用的是当前内存状态', async () => {
    const fetchMock = vi.fn().mockResolvedValue(response(true, 200, 'ok'))
    vi.stubGlobal('fetch', fetchMock)
    initToken()
    expect((fetchMock.mock.calls[0] ?? [])[1]).toBeUndefined()
    await request('GET', '/api/version')
    let headers = (fetchMock.mock.calls[0][1] as RequestInit).headers as Record<string, string>
    expect(headers['X-Token']).toBeUndefined()

    saveToken('tok-2')
    await request('GET', '/api/version')
    headers = (fetchMock.mock.calls[1][1] as RequestInit).headers as Record<string, string>
    expect(headers['X-Token']).toBe('tok-2')
  })

  it('initToken：请求收到 401 时自动标记被拒并弹出输入框', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(response(false, 401, '401: 缺少或错误的访问令牌')))
    initToken()
    await expect(request('GET', '/api/version')).rejects.toThrow('HTTP 401')
    expect(tokenRejected.value).toBe(true)
    expect(tokenDialogOpen.value).toBe(true)
  })

  it('D-20：logout 清空 localStorage 与内存令牌，并复位 401 标记', () => {
    saveToken('tok-1')
    tokenRejected.value = true
    logout()
    expect(window.localStorage.getItem(TOKEN_STORAGE_KEY)).toBeNull()
    expect(tokenState.value).toBe('')
    expect(tokenRejected.value).toBe(false)
  })

  it('D-20：logout 在没有令牌时也不报错', () => {
    expect(() => logout()).not.toThrow()
    expect(tokenState.value).toBe('')
  })

  it('initToken：非 401 的失败不会弹出输入框', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(response(false, 500, 'boom')))
    initToken()
    await expect(request('GET', '/api/version')).rejects.toThrow('HTTP 500')
    expect(tokenRejected.value).toBe(false)
    expect(tokenDialogOpen.value).toBe(false)
  })
})
