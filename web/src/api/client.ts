export class HttpError extends Error {
  readonly status: number
  readonly body: string

  constructor(status: number, body: string) {
    super('HTTP ' + status + (body === '' ? '' : ': ' + body))
    this.name = 'HttpError'
    this.status = status
    this.body = body
  }
}

export type QueryValue = string | number | boolean | null | undefined

export interface RequestOptions {
  query?: Record<string, QueryValue>
  body?: unknown
  headers?: Record<string, string>
  signal?: AbortSignal
}

export type TokenProvider = () => string

export type UnauthorizedHandler = () => void

let tokenProvider: TokenProvider = () => ''

// 服务端设了访问令牌时，未授权请求会拿到 401。请求层不认识界面，只把这件事
// 通知出去，由 api/token.ts 决定怎么提示用户（当前是弹出令牌输入框）。
// 没有注册处理器时静默跳过，纯请求层（含单元测试）不被迫关心界面。
let unauthorizedHandler: UnauthorizedHandler | null = null

export function setTokenProvider(provider: TokenProvider): void {
  tokenProvider = provider
}

export function setUnauthorizedHandler(handler: UnauthorizedHandler): void {
  unauthorizedHandler = handler
}

export function buildQuery(query?: Record<string, QueryValue>): string {
  if (query === undefined) {
    return ''
  }
  const params = new URLSearchParams()
  for (const key of Object.keys(query)) {
    const value = query[key]
    if (value === undefined || value === null) {
      continue
    }
    params.set(key, String(value))
  }
  const encoded = params.toString()
  return encoded === '' ? '' : '?' + encoded
}

export async function request<T>(method: string, path: string, options: RequestOptions = {}): Promise<T> {
  const headers: Record<string, string> = { ...options.headers }
  const token = tokenProvider()
  if (token !== '') {
    headers['X-Token'] = token
  }
  let payload: string | undefined
  if (options.body !== undefined) {
    headers['Content-Type'] = 'application/json'
    payload = JSON.stringify(options.body)
  }
  const response = await fetch(path + buildQuery(options.query), {
    method,
    headers,
    body: payload,
    signal: options.signal,
  })
  if (!response.ok) {
    if (response.status === 401 && unauthorizedHandler !== null) {
      unauthorizedHandler()
    }
    throw new HttpError(response.status, await response.text())
  }
  if (response.status === 204) {
    return undefined as T
  }
  const text = await response.text()
  if (text === '') {
    return undefined as T
  }
  const contentType = response.headers.get('Content-Type')
  if (contentType !== null && contentType.includes('application/json')) {
    return JSON.parse(text) as T
  }
  return text as unknown as T
}

export const api = {
  get<T>(path: string, options?: RequestOptions): Promise<T> {
    return request<T>('GET', path, options)
  },
  post<T>(path: string, body?: unknown, options?: RequestOptions): Promise<T> {
    return request<T>('POST', path, { ...options, body })
  },
  put<T>(path: string, body?: unknown, options?: RequestOptions): Promise<T> {
    return request<T>('PUT', path, { ...options, body })
  },
  del<T>(path: string, options?: RequestOptions): Promise<T> {
    return request<T>('DELETE', path, options)
  },
}
