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

let tokenProvider: TokenProvider = () => ''

export function setTokenProvider(provider: TokenProvider): void {
  tokenProvider = provider
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
