// 两个长度上限：详情进折叠块，可以长一点；展示文案是给用户看的一句话，必须短。
const DETAIL_LIMIT = 2000
const TEXT_LIMIT = 200

// 服务端 401 的纯文本是「401: 缺少或错误的访问令牌」，再拼上 HTTP 401
// 会变成「HTTP 401: 401: 缺少…」这种自说自话的重复，这里去掉重复的状态码前缀。
function stripStatusPrefix(text: string): string {
  return text.replace(/^\s*(?:http\s*)?\d{3}\s*[:：]\s*/i, '').trim()
}

function truncate(text: string, limit: number): string {
  return text.length <= limit ? text : text.slice(0, limit) + '…'
}

// parseErrorRecord 认出服务端的 JSON 错误体（形如 {"error":"…","detail":"…"}）。
// 认不出就返回 null，调用方按纯文本处理。
function parseErrorRecord(body: string): Record<string, unknown> | null {
  const trimmed = body.trim()
  if (trimmed === '' || !trimmed.startsWith('{')) {
    return null
  }
  try {
    const parsed: unknown = JSON.parse(trimmed)
    if (parsed !== null && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>
    }
  } catch {
    // 不是合法 JSON：交给纯文本分支。
  }
  return null
}

// errorText 从响应体里取出给用户看的一句话。
// 服务端的 JSON 错误体优先用 error 字段；纯文本直接用（去掉状态码前缀）。
export function errorText(body: string): string {
  const trimmed = body.trim()
  if (trimmed === '') {
    return ''
  }
  const record = parseErrorRecord(trimmed)
  if (record !== null) {
    const message = record['error']
    if (typeof message === 'string' && message.trim() !== '') {
      return truncate(stripStatusPrefix(message.trim()), TEXT_LIMIT)
    }
  }
  return truncate(stripStatusPrefix(trimmed), TEXT_LIMIT)
}

// errorDetail 取折叠展示的原文：服务端 JSON 体的 detail 字段（原始 Go 错误）优先，
// 没有就退回整个响应体。它只进折叠块，永远不当主文案展示。
export function errorDetail(body: string): string {
  const trimmed = body.trim()
  if (trimmed === '') {
    return ''
  }
  const record = parseErrorRecord(trimmed)
  if (record !== null) {
    const detail = record['detail']
    if (typeof detail === 'string' && detail.trim() !== '') {
      return truncate(detail.trim(), DETAIL_LIMIT)
    }
  }
  return truncate(trimmed, DETAIL_LIMIT)
}

export class HttpError extends Error {
  readonly status: number
  readonly body: string
  readonly detail: string

  constructor(status: number, body: string) {
    const text = errorText(body)
    const detail = errorDetail(body)
    super('HTTP ' + status + (text === '' ? '' : ': ' + text))
    this.name = 'HttpError'
    this.status = status
    this.body = body
    this.detail = detail
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
