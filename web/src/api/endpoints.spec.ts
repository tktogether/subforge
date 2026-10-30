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

  it('overview → GET /api/overview', async () => {
    const data = { name: 'subforge', version: 'v1', startedAt: 't', uptime: '1m', counts: {} }
    fetchMock.mockResolvedValue(respond(data))
    const result = await endpoints.overview()
    expect(fetchMock).toHaveBeenCalledWith('/api/overview', expect.objectContaining({ method: 'GET' }))
    expect(result).toEqual(data)
  })

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
    const data = { body: 'out', format: 'clash.meta', nodes: 1, rawNodes: 2, failures: null, warnings: null, cached: 0 }
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

  it('listRulesPage 带分页/筛选参数并解包', async () => {
    fetchMock.mockResolvedValue(respond({ rules: [], total: 3, limit: 2, offset: 1 }))
    const r = await endpoints.listRulesPage({ source: 'manual', enabled: false, q: 'x', limit: 2, offset: 1 })
    expect(r.total).toBe(3)
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/rules?source=manual&enabled=0&q=x&limit=2&offset=1',
      expect.objectContaining({ method: 'GET' }),
    )
  })

  it('listRulesPage 全来源/enabled 未设置/空 q 时不带多余参数', async () => {
    fetchMock.mockResolvedValue(respond({ rules: [], total: 0, limit: 25, offset: 0 }))
    await endpoints.listRulesPage({ source: null, q: '', limit: 25, offset: 0 })
    expect(fetchMock).toHaveBeenCalledWith('/api/rules?limit=25&offset=0', expect.objectContaining({ method: 'GET' }))
  })

  it('checkRuleSourceConflicts 传 names 并解包', async () => {
    fetchMock.mockResolvedValue(respond({ total: 1, samples: [{ type: 'DOMAIN', param: 'dup', first: 'a', second: 'b' }] }))
    const r = await endpoints.checkRuleSourceConflicts(['a', 'b'])
    expect(r.total).toBe(1)
    expect(fetchMock).toHaveBeenCalledWith('/api/rule-sources/conflicts?names=a%2Cb', expect.objectContaining({ method: 'GET' }))
  })

  it('listRuleSourceCounts GET /api/rules/sources 解包', async () => {
    fetchMock.mockResolvedValue(respond({ sources: [{ source: '', count: 2 }] }))
    const r = await endpoints.listRuleSourceCounts()
    expect(r).toEqual([{ source: '', count: 2 }])
    expect(fetchMock).toHaveBeenCalledWith('/api/rules/sources', expect.objectContaining({ method: 'GET' }))
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

  it('updateToken("a b") → PUT /api/tokens/a%20b 并解包 .token', async () => {
    const token = { token: 't2', name: 'n2', target: 'mihomo' }
    const input = { name: 'n2', target: 'mihomo' }
    fetchMock.mockResolvedValue(respond({ token }))
    const result = await endpoints.updateToken('a b', input)
    expect(fetchMock).toHaveBeenCalledWith('/api/tokens/a%20b', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(input),
      signal: undefined,
    })
    expect(result).toEqual(token)
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

describe('endpoints fe1', () => {
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
    return { ok: true, status: 200, headers: { get: () => 'application/json' }, text: async () => JSON.stringify(data) }
  }
it('listScripts 解包 .scripts', async () => {
    fetchMock.mockResolvedValue(respond({ scripts: [{ id: 1, name: 's', kind: 'filter', code: 'x', enabled: true }] }))
    const r = await endpoints.listScripts()
    expect(r).toEqual([{ id: 1, name: 's', kind: 'filter', code: 'x', enabled: true }])
  })
  it('createScript POST /api/scripts 解包 .script', async () => {
    fetchMock.mockResolvedValue(respond({ script: { id: 2, name: 's2', kind: 'operate', code: 'y', enabled: false } }))
    const r = await endpoints.createScript({ name: 's2', kind: 'operate', code: 'y', enabled: false })
    expect(r.id).toBe(2)
  })
  it('getScript(3) GET /api/scripts/3 解包 .script', async () => {
    fetchMock.mockResolvedValue(respond({ script: { id: 3, name: 's3', kind: 'filter', code: 'z', enabled: true } }))
    const r = await endpoints.getScript(3)
    expect(r.id).toBe(3)
  })
  it('updateScript(4) PUT /api/scripts/4 解包 .script', async () => {
    fetchMock.mockResolvedValue(respond({ script: { id: 4, name: 's4', kind: 'filter', code: 'w', enabled: true } }))
    const r = await endpoints.updateScript(4, { name: 's4', kind: 'filter', code: 'w', enabled: true })
    expect(r.id).toBe(4)
  })
  it('deleteScript(5) DELETE /api/scripts/5', async () => {
    fetchMock.mockResolvedValue(respond({ ok: true }))
    const r = await endpoints.deleteScript(5)
    expect(r.ok).toBe(true)
  })
  it('testScript POST /api/scripts/test', async () => {
    fetchMock.mockResolvedValue(respond({ ok: true, kept: true, before: {}, after: {}, duration_ms: 10, error: '' }))
    const r = await endpoints.testScript({ code: 'x', kind: 'filter' })
    expect(r.ok).toBe(true)
    expect(r.kept).toBe(true)
  })
  it('listTasks GET /api/tasks 解包 .tasks', async () => {
    fetchMock.mockResolvedValue(respond({ tasks: [{ JobName: 'j', Success: true, Err: '', StartedAt: '2020-01-01', Duration: 1 }] }))
    const r = await endpoints.listTasks()
    expect(r.length).toBe(1)
  })
  it('listRuleProviders GET /api/rule-providers 解包 .rule_providers', async () => {
    fetchMock.mockResolvedValue(respond({ rule_providers: [{ id: 1, name: 'rp', source: 'local', enabled: true }] }))
    const r = await endpoints.listRuleProviders()
    expect(r.length).toBe(1)
  })
  it('listSuggestions GET /api/suggestions 解包 .suggestions', async () => {
    fetchMock.mockResolvedValue(respond({ suggestions: [{ index: 0, name: 'sg' }] }))
    const r = await endpoints.listSuggestions()
    expect(r.length).toBe(1)
  })
  it('confirmSuggestion(2) POST /api/suggestions/2/confirm', async () => {
    fetchMock.mockResolvedValue(respond({ ok: true }))
    const r = await endpoints.confirmSuggestion(2)
    expect(r.ok).toBe(true)
  })
  it('rejectSuggestion(3) POST /api/suggestions/3/reject', async () => {
    fetchMock.mockResolvedValue(respond({ ok: true }))
    const r = await endpoints.rejectSuggestion(3)
    expect(r.ok).toBe(true)
  })
  it('listSettings GET /api/settings 返回 { settings: [...] }', async () => {
    fetchMock.mockResolvedValue(respond({ settings: [{ key: 'k', value: 'v' }] }))
    const r = await endpoints.listSettings()
    expect(r.settings).toHaveLength(1)
    expect(r.settings[0].key).toBe('k')
  })
  it('updateSettings PUT /api/settings', async () => {
    fetchMock.mockResolvedValue(respond({ settings: [{ key: 'k', value: 'v', kind: 'string', label: 'K', description: '' }] }))
    const r = await endpoints.updateSettings({ k: 'v' })
    expect(r.settings[0].key).toBe('k')
    const call = fetchMock.mock.calls[fetchMock.mock.calls.length - 1]
    expect(call[0]).toBe('/api/settings')
    expect(call[1]).toMatchObject({ method: 'PUT' })
    expect(JSON.parse(call[1].body as string)).toEqual({ settings: { k: 'v' } })
  })

  it('bulkRules POST /api/rules/bulk', async () => {
    fetchMock.mockResolvedValue(respond({ ok: true, updated: 3 }))
    const r = await endpoints.bulkRules({ ids: [1, 2], enabled: true })
    expect(r.updated).toBe(3)
    expect(fetchMock).toHaveBeenCalledWith('/api/rules/bulk', expect.objectContaining({ method: 'POST' }))
  })

  it('applySuggestions POST /api/suggestions/apply', async () => {
    fetchMock.mockResolvedValue(respond({ ok: true, applied: 2 }))
    const r = await endpoints.applySuggestions()
    expect(r.applied).toBe(2)
    expect(fetchMock).toHaveBeenCalledWith('/api/suggestions/apply', expect.objectContaining({ method: 'POST' }))
  })

  it('listRuleSources GET /api/rule-sources 解包', async () => {
    fetchMock.mockResolvedValue(respond({ ok: true, manifest_path: '/m', sources: [{ name: 's1' }] }))
    const r = await endpoints.listRuleSources()
    expect(r.manifest_path).toBe('/m')
    expect(r.sources).toHaveLength(1)
  })

  it('listRuleSourceRules GET /api/rule-sources/rules 带 name/q/offset/limit', async () => {
    fetchMock.mockResolvedValue(respond({ name: 's1', total: 1, offset: 0, limit: 50, rules: ['DOMAIN,a.test,DIRECT'] }))
    const r = await endpoints.listRuleSourceRules('s1', { q: 'x', offset: 0, limit: 50 })
    expect(r.rules).toEqual(['DOMAIN,a.test,DIRECT'])
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/rule-sources/rules?name=s1&q=x&offset=0&limit=50',
      expect.objectContaining({ method: 'GET' }),
    )
  })

  it('importRuleSourceRules POST /api/rule-sources/import-rules', async () => {
    fetchMock.mockResolvedValue(respond({ added: 1, skipped: 0, overwrote: 0, warnings: [] }))
    const input = { name: 's1', rules: ['DOMAIN,a.test,DIRECT'], conflict: 'skip' as const }
    const r = await endpoints.importRuleSourceRules(input)
    expect(r.added).toBe(1)
    expect(fetchMock).toHaveBeenCalledWith('/api/rule-sources/import-rules', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(input),
      signal: undefined,
    })
  })

  it('updateRuleSources POST /api/rule-sources/update', async () => {
    fetchMock.mockResolvedValue(respond({ ok: true, results: [{ name: 's1', status: 'ok' }] }))
    const r = await endpoints.updateRuleSources()
    expect(r.results).toHaveLength(1)
    expect(fetchMock).toHaveBeenCalledWith('/api/rule-sources/update', expect.objectContaining({ method: 'POST' }))
  })

  it('rollbackRuleSource POST /api/rule-sources/rollback body {name}', async () => {
    fetchMock.mockResolvedValue(respond({ ok: true, path: '/rb/v1' }))
    const r = await endpoints.rollbackRuleSource('s1')
    expect(r.path).toBe('/rb/v1')
    const call = fetchMock.mock.calls[0]
    expect(JSON.parse(call[1].body as string)).toEqual({ name: 's1' })
  })

  it('generateMRS POST /api/rule-sources/mrs body {name,behavior}', async () => {
    fetchMock.mockResolvedValue(respond({ ok: true, path: '/p/v1.mrs', bytes: 1024 }))
    const r = await endpoints.generateMRS('s1', 'ipcidr')
    expect(r.path).toBe('/p/v1.mrs')
    const call = fetchMock.mock.calls[0]
    expect(JSON.parse(call[1].body as string)).toEqual({ name: 's1', behavior: 'ipcidr' })
  })


  it('refreshAllAirports() POST /api/airports/refresh', async () => {
    fetchMock.mockResolvedValue(respond({ results: [], ok: 0, fail: 0 }))
    const r = await endpoints.refreshAllAirports()
    expect(r.ok).toBe(0)
    expect(fetchMock).toHaveBeenCalledWith('/api/airports/refresh', expect.objectContaining({ method: 'POST' }))
  })

  it('refreshAllAirports(true) POST /api/airports/refresh?with_rules=1', async () => {
    fetchMock.mockResolvedValue(respond({ results: [], ok: 0, fail: 0 }))
    await endpoints.refreshAllAirports(true)
    expect(fetchMock).toHaveBeenCalledWith('/api/airports/refresh?with_rules=1', expect.objectContaining({ method: 'POST' }))
  })

  it('refreshAirport(3) POST /api/airports/3/refresh', async () => {
    fetchMock.mockResolvedValue(respond({ result: { airport_id: 3, name: 'a', ok: true, usage: { upload: 0, download: 0, total: 0, expire: 0 }, node_count: 1, err: '' } }))
    const r = await endpoints.refreshAirport(3)
    expect(r.result.airport_id).toBe(3)
    expect(fetchMock).toHaveBeenCalledWith('/api/airports/3/refresh', expect.objectContaining({ method: 'POST' }))
  })

  it('importAirportRules(3) POST /api/airports/3/import-rules', async () => {
    fetchMock.mockResolvedValue(respond({ rules_added: 1, rules_skipped: 0, providers_added: 0, warnings: [] }))
    const r = await endpoints.importAirportRules(3)
    expect(r.rules_added).toBe(1)
    expect(fetchMock).toHaveBeenCalledWith('/api/airports/3/import-rules', expect.objectContaining({ method: 'POST' }))
  })

  it('checkAirports([1,2]) POST /api/airports/check body {ids:[1,2]}', async () => {
    fetchMock.mockResolvedValue(respond({ results: [] }))
    await endpoints.checkAirports([1, 2])
    const call = fetchMock.mock.calls[0]
    expect(call[0]).toBe('/api/airports/check')
    expect(JSON.parse(call[1].body as string)).toEqual({ ids: [1, 2] })
  })

  it('checkAirports() POST /api/airports/check body {}', async () => {
    fetchMock.mockResolvedValue(respond({ results: [] }))
    await endpoints.checkAirports()
    const call = fetchMock.mock.calls[0]
    expect(JSON.parse(call[1].body as string)).toEqual({})
  })

  it('lastAirportCheck() GET /api/airports/check/last', async () => {
    fetchMock.mockResolvedValue(respond({ results: [], at: 123 }))
    const r = await endpoints.lastAirportCheck()
    expect(r.at).toBe(123)
    expect(fetchMock).toHaveBeenCalledWith('/api/airports/check/last', expect.objectContaining({ method: 'GET' }))
  })

  it('getTrafficStrategy() GET /api/traffic/strategy', async () => {
    fetchMock.mockResolvedValue(respond({ strategy: { preset: 'default' } }))
    const r = await endpoints.getTrafficStrategy()
    expect(r.strategy.preset).toBe('default')
    expect(fetchMock).toHaveBeenCalledWith('/api/traffic/strategy', expect.objectContaining({ method: 'GET' }))
  })

  it('setTrafficStrategy(s) PUT /api/traffic/strategy', async () => {
    fetchMock.mockResolvedValue(respond({ strategy: { preset: 'custom' } }))
    const r = await endpoints.setTrafficStrategy({ preset: 'custom' })
    expect(r.strategy.preset).toBe('custom')
    expect(fetchMock).toHaveBeenCalledWith('/api/traffic/strategy', expect.objectContaining({ method: 'PUT' }))
  })

  it('previewTraffic() GET /api/traffic/preview', async () => {
    fetchMock.mockResolvedValue(respond({ usage: { upload: 0, download: 0, total: 0, expire: 0 }, header: 'h', ok: true }))
    const r = await endpoints.previewTraffic()
    expect(r.ok).toBe(true)
    expect(fetchMock).toHaveBeenCalledWith('/api/traffic/preview', expect.objectContaining({ method: 'GET' }))
  })

  it('previewTraffic(s) GET /api/traffic/preview?strategy=...', async () => {
    fetchMock.mockResolvedValue(respond({ usage: { upload: 0, download: 0, total: 0, expire: 0 }, header: 'h', ok: true }))
    await endpoints.previewTraffic({ preset: 'x' })
    const call = fetchMock.mock.calls[0]
    expect(call[0]).toContain('/api/traffic/preview?strategy=')
  })

  it('getTemplate() GET /api/template', async () => {
    fetchMock.mockResolvedValue(respond({ path: '/t', content: 'c' }))
    const r = await endpoints.getTemplate()
    expect(r.path).toBe('/t')
    expect(fetchMock).toHaveBeenCalledWith('/api/template', expect.objectContaining({ method: 'GET' }))
  })

  it('putTemplate(content) PUT /api/template body {content}', async () => {
    fetchMock.mockResolvedValue(respond({ ok: true }))
    const r = await endpoints.putTemplate('hello')
    expect(r.ok).toBe(true)
    const call = fetchMock.mock.calls[0]
    expect(JSON.parse(call[1].body as string)).toEqual({ content: 'hello' })
  })

  it('getOverride() GET /api/override', async () => {
    fetchMock.mockResolvedValue(respond({ path: '/o', content: 'c' }))
    const r = await endpoints.getOverride()
    expect(r.path).toBe('/o')
    expect(fetchMock).toHaveBeenCalledWith('/api/override', expect.objectContaining({ method: 'GET' }))
  })

  it('putOverride(content) PUT /api/override body {content}', async () => {
    fetchMock.mockResolvedValue(respond({ ok: true }))
    const r = await endpoints.putOverride('x')
    expect(r.ok).toBe(true)
    const call = fetchMock.mock.calls[0]
    expect(JSON.parse(call[1].body as string)).toEqual({ content: 'x' })
  })

  it('listAlertChannels() GET /api/alert-channels', async () => {
    fetchMock.mockResolvedValue(respond({ channels: [{ kind: 'telegram', enabled: true }] }))
    const r = await endpoints.listAlertChannels()
    expect(r.channels).toHaveLength(1)
    expect(fetchMock).toHaveBeenCalledWith('/api/alert-channels', expect.objectContaining({ method: 'GET' }))
  })

  it('putAlertChannels(channels) PUT /api/alert-channels', async () => {
    fetchMock.mockResolvedValue(respond({ channels: [{ kind: 'telegram', enabled: false }] }))
    const r = await endpoints.putAlertChannels([{ kind: 'telegram', enabled: false }])
    expect(r.channels[0].enabled).toBe(false)
    expect(fetchMock).toHaveBeenCalledWith('/api/alert-channels', expect.objectContaining({ method: 'PUT' }))
  })

  it('testAlertChannel(0) POST /api/alert-channels/test body {index:0}', async () => {
    fetchMock.mockResolvedValue(respond({ ok: true, error: '', channel: 'telegram' }))
    const r = await endpoints.testAlertChannel(0)
    expect(r.ok).toBe(true)
    const call = fetchMock.mock.calls[0]
    expect(JSON.parse(call[1].body as string)).toEqual({ index: 0 })
  })

  it('createPipeline(body) → POST /api/pipelines 解出 pipeline', async () => {
    const body = { name: 'p', description: 'd', steps: [{ op: 'op.sort' }], enabled: true }
    fetchMock.mockResolvedValue(respond({ pipeline: { id: 1, name: 'p', description: 'd', steps_json: '[]', enabled: true, sort_order: 0, source: 'ui', read_only: false } }))
    const r = await endpoints.createPipeline(body)
    expect(r.id).toBe(1)
    expect(fetchMock).toHaveBeenCalledWith('/api/pipelines', expect.objectContaining({ method: 'POST' }))
  })

  it('updatePipeline(1,body) → PUT /api/pipelines/1 解出 pipeline', async () => {
    const body = { name: 'p2', enabled: false }
    fetchMock.mockResolvedValue(respond({ pipeline: { id: 1, name: 'p2', description: '', steps_json: '[]', enabled: false, sort_order: 0, source: 'ui', read_only: false } }))
    const r = await endpoints.updatePipeline(1, body)
    expect(r.name).toBe('p2')
    expect(fetchMock).toHaveBeenCalledWith('/api/pipelines/1', expect.objectContaining({ method: 'PUT' }))
  })

  it('deletePipeline(1) → DELETE /api/pipelines/1 解出 {ok:true}', async () => {
    fetchMock.mockResolvedValue(respond({ ok: true }))
    const r = await endpoints.deletePipeline(1)
    expect(r.ok).toBe(true)
    expect(fetchMock).toHaveBeenCalledWith('/api/pipelines/1', expect.objectContaining({ method: 'DELETE' }))
  })

  // 规则集/规则源写接口（T7/T3 的 endpoint 层契约，页面 spec 全 mock endpoints，这里是真调用点）
  it('updateRuleProvider(7,true) → PUT /api/rule-providers/7/enabled body {enabled:true}', async () => {
    fetchMock.mockResolvedValue(respond({ ok: true }))
    const r = await endpoints.updateRuleProvider(7, true)
    expect(r.ok).toBe(true)
    expect(fetchMock).toHaveBeenCalledWith('/api/rule-providers/7/enabled', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enabled: true }),
      signal: undefined,
    })
  })

  it('addRuleSource(body) → POST /api/rule-sources 解出 {path,created}', async () => {
    const body = { name: 'n', url: 'http://x', input_format: undefined }
    fetchMock.mockResolvedValue(respond({ path: '/m/manifest.yaml', created: true }))
    const r = await endpoints.addRuleSource(body)
    expect(r.created).toBe(true)
    expect(fetchMock).toHaveBeenCalledWith('/api/rule-sources', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
      signal: undefined,
    })
  })

  it('removeRuleSource("n") → POST /api/rule-sources/remove body {name:"n"}', async () => {
    fetchMock.mockResolvedValue(respond({ ok: true }))
    const r = await endpoints.removeRuleSource('n')
    expect(r.ok).toBe(true)
    expect(fetchMock).toHaveBeenCalledWith('/api/rule-sources/remove', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: 'n' }),
      signal: undefined,
    })
  })

  it('updateRuleSourceOne("n") → POST /api/rule-sources/update-one body {name:"n"}', async () => {
    const results = [{ source: 'n', version: 'v3', path: '/p/v3', from_cache: false, err: '' }]
    fetchMock.mockResolvedValue(respond({ results }))
    const r = await endpoints.updateRuleSourceOne('n')
    expect(r.results).toEqual(results)
    expect(fetchMock).toHaveBeenCalledWith('/api/rule-sources/update-one', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: 'n' }),
      signal: undefined,
    })
  })

})

// T3a：方案（profile）端点与 profile_id 归属。页面 spec 全 mock endpoints，
// 这里是真调用点，负责钉住 URL、方法与解包字段。
describe('endpoints 方案(profile) 与来源(source)分层（v2）', () => {
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
    return { ok: true, status: 200, headers: { get: () => 'application/json' }, text: async () => JSON.stringify(data) }
  }

  const DEFAULT_LAYERS = ['custom', 'airport', 'template']

  it('listProfiles → GET /api/profiles 解出 .profiles（含 rule_layers/group_layers）', async () => {
    const profiles = [{ id: 1, name: 'p1', description: '', config: '', pipeline: '', enabled: true, sort_order: 0, rule_layers: DEFAULT_LAYERS, group_layers: DEFAULT_LAYERS, rule_count: 2, group_count: 3 }]
    fetchMock.mockResolvedValue(respond({ profiles }))
    const r = await endpoints.listProfiles()
    expect(r).toEqual(profiles)
    expect(fetchMock).toHaveBeenCalledWith('/api/profiles', expect.objectContaining({ method: 'GET' }))
  })

  it('createProfile(body) → POST /api/profiles 解出 .profile（body 可带 rule_layers/group_layers）', async () => {
    const profile = { id: 2, name: 'p2', description: 'd', config: 'c', pipeline: 'pl', enabled: true, sort_order: 1, rule_layers: ['custom', 'airport', 'template'], group_layers: ['custom', 'airport'], rule_count: 0, group_count: 0 }
    const input = { name: 'p2', description: 'd', config: 'c', pipeline: 'pl', enabled: true, sort_order: 1, rule_layers: ['custom', 'airport', 'template'], group_layers: ['custom', 'airport'] }
    fetchMock.mockResolvedValue(respond({ profile }))
    const r = await endpoints.createProfile(input)
    expect(r).toEqual(profile)
    expect(fetchMock).toHaveBeenCalledWith('/api/profiles', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(input),
      signal: undefined,
    })
  })

  it('updateProfile(3,body) → PUT /api/profiles/3 解出 .profile', async () => {
    const profile = { id: 3, name: 'p3', description: '', config: '', pipeline: '', enabled: false, sort_order: 2, rule_layers: DEFAULT_LAYERS, group_layers: DEFAULT_LAYERS, rule_count: 0, group_count: 0 }
    const input = { name: 'p3', enabled: false }
    fetchMock.mockResolvedValue(respond({ profile }))
    const r = await endpoints.updateProfile(3, input)
    expect(r).toEqual(profile)
    expect(fetchMock).toHaveBeenCalledWith('/api/profiles/3', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(input),
      signal: undefined,
    })
  })

  it('deleteProfile(3) → DELETE /api/profiles/3', async () => {
    fetchMock.mockResolvedValue(respond({ ok: true }))
    const r = await endpoints.deleteProfile(3)
    expect(r).toEqual({ ok: true })
    expect(fetchMock).toHaveBeenCalledWith('/api/profiles/3', expect.objectContaining({ method: 'DELETE' }))
  })

  it('listGroups({source:"airport:A"}) → GET /api/groups?source=airport%3AA&source_prefix=0', async () => {
    const groups = [{ id: 1, name: 'g', type: 't', regex: 'r', url: 'u', interval: 1, sort_order: 1, source: 'airport:A' }]
    fetchMock.mockResolvedValue(respond({ groups }))
    const r = await endpoints.listGroups({ source: 'airport:A', sourcePrefix: false })
    expect(r).toEqual(groups)
    expect(fetchMock).toHaveBeenCalledWith('/api/groups?source=airport%3AA&source_prefix=0', expect.objectContaining({ method: 'GET' }))
  })

  it('listGroups({source:""}) → GET /api/groups?source=&source_prefix=0（空串保留 ?source=）', async () => {
    fetchMock.mockResolvedValue(respond({ groups: [] }))
    await endpoints.listGroups({ source: '', sourcePrefix: false })
    expect(fetchMock).toHaveBeenCalledWith('/api/groups?source=&source_prefix=0', expect.objectContaining({ method: 'GET' }))
  })

  it('listGroups() 不带参数 → GET /api/groups（无 query）', async () => {
    fetchMock.mockResolvedValue(respond({ groups: [] }))
    await endpoints.listGroups()
    expect(fetchMock).toHaveBeenCalledWith('/api/groups', expect.objectContaining({ method: 'GET' }))
  })

  it('listRules({source:"airport:B"}) → GET /api/rules?source=airport%3AB&source_prefix=0', async () => {
    const rules = [{ id: 1, rule_type: 't', rule_param: 'p', rule_config: 1, resolve_dns: true, enabled: true, sort_order: 1, source: 'airport:B' }]
    fetchMock.mockResolvedValue(respond({ rules }))
    const r = await endpoints.listRules({ source: 'airport:B', sourcePrefix: false })
    expect(r).toEqual(rules)
    expect(fetchMock).toHaveBeenCalledWith('/api/rules?source=airport%3AB&source_prefix=0', expect.objectContaining({ method: 'GET' }))
  })

  it('listRules({source:"", sourcePrefix:true}) → GET /api/rules?source=&source_prefix=1', async () => {
    fetchMock.mockResolvedValue(respond({ rules: [] }))
    await endpoints.listRules({ source: '', sourcePrefix: true })
    expect(fetchMock).toHaveBeenCalledWith('/api/rules?source=&source_prefix=1', expect.objectContaining({ method: 'GET' }))
  })

  it('listRules() 不带参数 → GET /api/rules（无 query）', async () => {
    fetchMock.mockResolvedValue(respond({ rules: [] }))
    await endpoints.listRules()
    expect(fetchMock).toHaveBeenCalledWith('/api/rules', expect.objectContaining({ method: 'GET' }))
  })

  it('bulkRules({ids,enabled}) 按 ids 整批启停原样透传', async () => {
    const body = { ids: [1, 2], enabled: true }
    fetchMock.mockResolvedValue(respond({ updated: 2 }))
    const r = await endpoints.bulkRules(body)
    expect(r).toEqual({ updated: 2 })
    expect(fetchMock).toHaveBeenCalledWith('/api/rules/bulk', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
      signal: undefined,
    })
  })

  it('bulkRules({source,sourcePrefix,enabled}) 按来源整批启停原样透传', async () => {
    const body = { source: 'airport:A', sourcePrefix: false, enabled: true }
    fetchMock.mockResolvedValue(respond({ updated: 3 }))
    const r = await endpoints.bulkRules(body)
    expect(r).toEqual({ updated: 3 })
    const call = fetchMock.mock.calls[0]
    expect(JSON.parse(call[1].body as string)).toEqual(body)
  })

  it('bulkRules({source,delete:true}) 按来源整批删除原样透传', async () => {
    const body = { source: 'airport:A', sourcePrefix: false, delete: true }
    fetchMock.mockResolvedValue(respond({ deleted: 3 }))
    const r = await endpoints.bulkRules(body)
    expect(r).toEqual({ deleted: 3 })
    const call = fetchMock.mock.calls[0]
    expect(JSON.parse(call[1].body as string)).toEqual(body)
  })

  it('importAirportRules(4,{create_groups}) → POST body 只含 create_groups', async () => {
    const body = { create_groups: true }
    fetchMock.mockResolvedValue(respond({ rules_added: 1, rules_skipped: 0, providers_added: 0, warnings: [], groups_created: 2 }))
    const r = await endpoints.importAirportRules(4, body)
    expect(r.groups_created).toBe(2)
    expect(fetchMock).toHaveBeenCalledWith('/api/airports/4/import-rules', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
      signal: undefined,
    })
  })

  it('importAirportRules(4) 不传 body 时发 {}', async () => {
    fetchMock.mockResolvedValue(respond({ rules_added: 0, rules_skipped: 0, providers_added: 0, warnings: [] }))
    const r = await endpoints.importAirportRules(4)
    expect(r.rules_added).toBe(0)
    const call = fetchMock.mock.calls[0]
    expect(call[0]).toBe('/api/airports/4/import-rules')
    expect(JSON.parse(call[1].body as string)).toEqual({})
  })

  it('createToken 带完整字段（target/pipeline/config/include/exclude/profile_id）原样 POST', async () => {
    const token = { token: 't2', name: 'n2', target: 'mihomo', pipeline: 'p', config: 'c', include: 'i', exclude: 'e', profile_id: 3 }
    const input = { token: 't2', name: 'n2', target: 'mihomo', pipeline: 'p', config: 'c', include: 'i', exclude: 'e', profile_id: 3 }
    fetchMock.mockResolvedValue(respond({ token }))
    const r = await endpoints.createToken(input)
    expect(r).toEqual(token)
    expect(fetchMock).toHaveBeenCalledWith('/api/tokens', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(input),
      signal: undefined,
    })
  })
})

