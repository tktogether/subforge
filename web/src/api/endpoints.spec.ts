import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { endpoints, subscribeUrl } from './endpoints'

describe('endpoints', () => {
  let fetchMock: ReturnType<typeof vi.fn>

  beforeEach(() => {
    fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.clearAllMocks()
  })

  function respond(data: unknown) {
    return {
      ok: true,
      status: 200,
      headers: { get: () => 'application/json' },
      text: async () => JSON.stringify(data),
    }
  }

  it('stats → GET /api/stats', async () => {
    const data = { history: { total: 1, cachedHits: 0, urlTotal: 1, cacheHitRate: 0 }, logs: 2, last: null }
    fetchMock.mockResolvedValue(respond(data))
    const result = await endpoints.stats()
    expect(fetchMock).toHaveBeenCalledWith('/api/stats', {
      method: 'GET',
      headers: {},
      body: undefined,
      signal: undefined,
    })
    expect(result).toEqual(data)
  })

  it('history → GET /api/history', async () => {
    const data = [{ id: 1, at: 't', target: 't', format: 'f', nodes: 1, rawNodes: 1, groups: 1, rules: 1, failures: null, warnings: null, cached: 0, urlCount: 1 }]
    fetchMock.mockResolvedValue(respond(data))
    const result = await endpoints.history()
    expect(fetchMock).toHaveBeenCalledWith('/api/history', expect.objectContaining({ method: 'GET' }))
    expect(result).toEqual(data)
  })

  it('pipelines("cfg") → GET /api/pipelines?config=cfg 解出数组', async () => {
    const data = ['a', 'b']
    fetchMock.mockResolvedValue(respond(data))
    const result = await endpoints.pipelines('cfg')
    expect(fetchMock).toHaveBeenCalledWith('/api/pipelines?config=cfg', expect.objectContaining({ method: 'GET' }))
    expect(result).toEqual(data)
  })

  it('targets → GET /api/targets', async () => {
    const data = ['x']
    fetchMock.mockResolvedValue(respond(data))
    const result = await endpoints.targets()
    expect(fetchMock).toHaveBeenCalledWith('/api/targets', expect.objectContaining({ method: 'GET' }))
    expect(result).toEqual(data)
  })

  it('rulesets("cfg") → GET /api/rulesets?config=cfg 解出 {rulesets,rules}', async () => {
    const data = { rulesets: { a: 'b' }, rules: ['r'] }
    fetchMock.mockResolvedValue(respond(data))
    const result = await endpoints.rulesets('cfg')
    expect(fetchMock).toHaveBeenCalledWith('/api/rulesets?config=cfg', expect.objectContaining({ method: 'GET' }))
    expect(result).toEqual(data)
  })

  it('logs(123) → GET /api/logs?since=123', async () => {
    const data = { logs: [{ id: 1, ts: 't', line: 'l' }] }
    fetchMock.mockResolvedValue(respond(data))
    const result = await endpoints.logs(123)
    expect(fetchMock).toHaveBeenCalledWith('/api/logs?since=123', expect.objectContaining({ method: 'GET' }))
    expect(result).toEqual(data)
  })

  it('convert(body) → POST /api/convert，body 为 JSON 字符串', async () => {
    const body = { target: 't', urls: [], text: '', types: [], include: [], exclude: [], pipeline: '', config: '', sort: '', list: false, override: '' }
    const data = { output: 'out', warnings: null, failures: null, nodes: 1, groups: 1, rules: 1 }
    fetchMock.mockResolvedValue(respond(data))
    const result = await endpoints.convert(body)
    expect(fetchMock).toHaveBeenCalledWith('/api/convert', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
      signal: undefined,
    })
    expect(result).toEqual(data)
  })

  it('diff("a","b") → POST /api/diff 且 body 为 {"a":"a","b":"b"}', async () => {
    const data = { lines: [{ type: 'add', text: 'x' }] }
    fetchMock.mockResolvedValue(respond(data))
    const result = await endpoints.diff('a', 'b')
    expect(fetchMock).toHaveBeenCalledWith('/api/diff', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ a: 'a', b: 'b' }),
      signal: undefined,
    })
    expect(result).toEqual(data)
  })

  // 机场
  it('listAirports 从 {airports:[...]} 解出数组', async () => {
    const airports = [{ id: 1, name: 'n', subscription_url: 'u', remarks: '', enabled: true, sort_order: 1 }]
    fetchMock.mockResolvedValue(respond({ airports }))
    const result = await endpoints.listAirports()
    expect(fetchMock).toHaveBeenCalledWith('/api/airports', expect.objectContaining({ method: 'GET' }))
    expect(result).toEqual(airports)
  })

  it('createAirport POST /api/airports 解出 .airport', async () => {
    const airport = { id: 2, name: 'n2', subscription_url: 'u2', remarks: '', enabled: true, sort_order: 2 }
    const input = { name: 'n2', subscription_url: 'u2', remarks: '', enabled: true }
    fetchMock.mockResolvedValue(respond({ airport }))
    const result = await endpoints.createAirport(input)
    expect(fetchMock).toHaveBeenCalledWith('/api/airports', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(input),
      signal: undefined,
    })
    expect(result).toEqual(airport)
  })

  it('updateAirport(7,...) PUT /api/airports/7 解出 .airport', async () => {
    const airport = { id: 7, name: 'n7', subscription_url: 'u7', remarks: '', enabled: true, sort_order: 7 }
    const input = { name: 'n7', subscription_url: 'u7', remarks: '', enabled: true }
    fetchMock.mockResolvedValue(respond({ airport }))
    const result = await endpoints.updateAirport(7, input)
    expect(fetchMock).toHaveBeenCalledWith('/api/airports/7', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(input),
      signal: undefined,
    })
    expect(result).toEqual(airport)
  })

  it('deleteAirport(7) DELETE /api/airports/7', async () => {
    fetchMock.mockResolvedValue(respond({ ok: true }))
    const result = await endpoints.deleteAirport(7)
    expect(fetchMock).toHaveBeenCalledWith('/api/airports/7', expect.objectContaining({ method: 'DELETE' }))
    expect(result).toEqual({ ok: true })
  })

  // 分组
  it('listGroups 解包 .groups', async () => {
    const groups = [{ id: 1, name: 'g', type: 't', regex: 'r', url: 'u', interval: 1, sort_order: 1 }]
    fetchMock.mockResolvedValue(respond({ groups }))
    const result = await endpoints.listGroups()
    expect(fetchMock).toHaveBeenCalledWith('/api/groups', expect.objectContaining({ method: 'GET' }))
    expect(result).toEqual(groups)
  })

  it('createGroup 解包 .group', async () => {
    const group = { id: 2, name: 'g2', type: 't', regex: 'r', url: 'u', interval: 1, sort_order: 2 }
    const input = { name: 'g2', type: 't', regex: 'r', url: 'u', interval: 1 }
    fetchMock.mockResolvedValue(respond({ group }))
    const result = await endpoints.createGroup(input)
    expect(fetchMock).toHaveBeenCalledWith('/api/groups', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(input),
      signal: undefined,
    })
    expect(result).toEqual(group)
  })

  it('updateGroup(8) 对应 /api/groups/8 解包 .group', async () => {
    const group = { id: 8, name: 'g8', type: 't', regex: 'r', url: 'u', interval: 1, sort_order: 8 }
    const input = { name: 'g8', type: 't', regex: 'r', url: 'u', interval: 1 }
    fetchMock.mockResolvedValue(respond({ group }))
    const result = await endpoints.updateGroup(8, input)
    expect(fetchMock).toHaveBeenCalledWith('/api/groups/8', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(input),
      signal: undefined,
    })
    expect(result).toEqual(group)
  })

  it('deleteGroup(8) 对应 /api/groups/8', async () => {
    fetchMock.mockResolvedValue(respond({ ok: true }))
    const result = await endpoints.deleteGroup(8)
    expect(fetchMock).toHaveBeenCalledWith('/api/groups/8', expect.objectContaining({ method: 'DELETE' }))
    expect(result).toEqual({ ok: true })
  })

  // 规则
  it('listRules 解包 .rules', async () => {
    const rules = [{ id: 1, rule_type: 't', rule_param: 'p', rule_config: 1, resolve_dns: true, enabled: true, sort_order: 1 }]
    fetchMock.mockResolvedValue(respond({ rules }))
    const result = await endpoints.listRules()
    expect(fetchMock).toHaveBeenCalledWith('/api/rules', expect.objectContaining({ method: 'GET' }))
    expect(result).toEqual(rules)
  })

  it('createRule 解包 .rule', async () => {
    const rule = { id: 2, rule_type: 't', rule_param: 'p', rule_config: 1, resolve_dns: true, enabled: true, sort_order: 2 }
    const input = { rule_type: 't', rule_param: 'p', rule_config: 1, resolve_dns: true, enabled: true }
    fetchMock.mockResolvedValue(respond({ rule }))
    const result = await endpoints.createRule(input)
    expect(fetchMock).toHaveBeenCalledWith('/api/rules', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(input),
      signal: undefined,
    })
    expect(result).toEqual(rule)
  })

  it('updateRule(9) 对应 /api/rules/9 解包 .rule', async () => {
    const rule = { id: 9, rule_type: 't', rule_param: 'p', rule_config: 1, resolve_dns: true, enabled: true, sort_order: 9 }
    const input = { rule_type: 't', rule_param: 'p', rule_config: 1, resolve_dns: true, enabled: true }
    fetchMock.mockResolvedValue(respond({ rule }))
    const result = await endpoints.updateRule(9, input)
    expect(fetchMock).toHaveBeenCalledWith('/api/rules/9', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(input),
      signal: undefined,
    })
    expect(result).toEqual(rule)
  })

  it('deleteRule(9) 对应 /api/rules/9', async () => {
    fetchMock.mockResolvedValue(respond({ ok: true }))
    const result = await endpoints.deleteRule(9)
    expect(fetchMock).toHaveBeenCalledWith('/api/rules/9', expect.objectContaining({ method: 'DELETE' }))
    expect(result).toEqual({ ok: true })
  })

  // 令牌
  it('listTokens 解包 .tokens', async () => {
    const tokens = [{ token: 't', name: 'n' }]
    fetchMock.mockResolvedValue(respond({ tokens }))
    const result = await endpoints.listTokens()
    expect(fetchMock).toHaveBeenCalledWith('/api/tokens', expect.objectContaining({ method: 'GET' }))
    expect(result).toEqual(tokens)
  })

  it('createToken 解包 .token', async () => {
    const token = { token: 't2', name: 'n2' }
    const input = { token: 't2', name: 'n2' }
    fetchMock.mockResolvedValue(respond({ token }))
    const result = await endpoints.createToken(input)
    expect(fetchMock).toHaveBeenCalledWith('/api/tokens', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(input),
      signal: undefined,
    })
    expect(result).toEqual(token)
  })

  it('deleteToken("a b") → DELETE /api/tokens/a%20b（encodeURIComponent）', async () => {
    fetchMock.mockResolvedValue(respond({ ok: true }))
    const result = await endpoints.deleteToken('a b')
    expect(fetchMock).toHaveBeenCalledWith('/api/tokens/a%20b', expect.objectContaining({ method: 'DELETE' }))
    expect(result).toEqual({ ok: true })
  })

  // 自建节点
  it('listSelfNodes 解包 .self_nodes', async () => {
    const self_nodes = [{ id: 1, link: 'l', name: 'n', enabled: true, sort_order: 1 }]
    fetchMock.mockResolvedValue(respond({ self_nodes }))
    const result = await endpoints.listSelfNodes()
    expect(fetchMock).toHaveBeenCalledWith('/api/self-nodes', expect.objectContaining({ method: 'GET' }))
    expect(result).toEqual(self_nodes)
  })

  it('createSelfNode 解包 .self_node', async () => {
    const self_node = { id: 2, link: 'l2', name: 'n2', enabled: true, sort_order: 2 }
    const input = { link: 'l2', name: 'n2', enabled: true }
    fetchMock.mockResolvedValue(respond({ self_node }))
    const result = await endpoints.createSelfNode(input)
    expect(fetchMock).toHaveBeenCalledWith('/api/self-nodes', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(input),
      signal: undefined,
    })
    expect(result).toEqual(self_node)
  })

  it('updateSelfNode(3) → PUT /api/self-nodes/3', async () => {
    const input = { link: 'l3', name: 'n3', enabled: true }
    fetchMock.mockResolvedValue(respond({ ok: true }))
    const result = await endpoints.updateSelfNode(3, input)
    expect(fetchMock).toHaveBeenCalledWith('/api/self-nodes/3', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(input),
      signal: undefined,
    })
    expect(result).toEqual({ ok: true })
  })

  it('deleteSelfNode(3) → DELETE /api/self-nodes/3', async () => {
    fetchMock.mockResolvedValue(respond({ ok: true }))
    const result = await endpoints.deleteSelfNode(3)
    expect(fetchMock).toHaveBeenCalledWith('/api/self-nodes/3', expect.objectContaining({ method: 'DELETE' }))
    expect(result).toEqual({ ok: true })
  })
})

describe('subscribeUrl', () => {
  it('subscribeUrl("http://h","a b") → "http://h/subscribe?token=a%20b"', () => {
    expect(subscribeUrl('http://h', 'a b')).toBe('http://h/subscribe?token=a%20b')
  })
})
