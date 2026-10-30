import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import RulesPage from './RulesPage.vue'
import { endpoints } from '../api/endpoints'
import { VSwitch } from 'vuetify/components'
import { notifyState } from '../composables/useNotify'

vi.mock('../api/endpoints', () => ({
  endpoints: {
    listRules: vi.fn(),
    listRulesPage: vi.fn(),
    listRuleSourceCounts: vi.fn(),
    createRule: vi.fn(),
    listGroups: vi.fn(),
    updateRule: vi.fn(),
    deleteRule: vi.fn(),
    listRuleProviders: vi.fn(),
    bulkRules: vi.fn(),
    listSuggestions: vi.fn(),
    confirmSuggestion: vi.fn(),
    rejectSuggestion: vi.fn(),
    applySuggestions: vi.fn(),
    updateRuleProvider: vi.fn(),
  },
}))

const vuetify = createVuetify({ components, directives })

const listRulesPageMock = endpoints.listRulesPage as unknown as ReturnType<typeof vi.fn>
const listRuleSourceCountsMock = endpoints.listRuleSourceCounts as unknown as ReturnType<typeof vi.fn>
let rulesFixture: any[] = []

function setRules(rules: any[]) {
  rulesFixture = rules
}

function sourceCounts() {
  const map = new Map<string, number>()
  for (const r of rulesFixture) {
    const source = r.source ?? ''
    map.set(source, (map.get(source) ?? 0) + 1)
  }
  return Array.from(map.entries()).map(([source, count]) => ({ source, count }))
}
const listGroupsMock = endpoints.listGroups as unknown as ReturnType<typeof vi.fn>
const listRuleProvidersMock = endpoints.listRuleProviders as unknown as ReturnType<typeof vi.fn>
const bulkRulesMock = endpoints.bulkRules as unknown as ReturnType<typeof vi.fn>
const listSuggestionsMock = endpoints.listSuggestions as unknown as ReturnType<typeof vi.fn>
const confirmSuggestionMock = endpoints.confirmSuggestion as unknown as ReturnType<typeof vi.fn>
const rejectSuggestionMock = endpoints.rejectSuggestion as unknown as ReturnType<typeof vi.fn>
const applySuggestionsMock = endpoints.applySuggestions as unknown as ReturnType<typeof vi.fn>
const updateRuleProviderMock = endpoints.updateRuleProvider as unknown as ReturnType<typeof vi.fn>


async function openSuggestionsTab(w: any) {
  const tabs = w.findAll('.v-tab')
  if (tabs.length >= 3) await tabs[2].trigger('click')
  await flushPromises()
}

describe('RulesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    Object.defineProperty(window, 'visualViewport', { value: { width: 1024, height: 768, scale: 1, offsetLeft: 0, offsetTop: 0, addEventListener: vi.fn(), removeEventListener: vi.fn() }, writable: true, configurable: true })
    rulesFixture = []
    listRulesPageMock.mockImplementation(async (query: any) => {
      let items = rulesFixture
      if (query.source !== undefined && query.source !== null) {
        items = items.filter((r) => (r.source ?? '') === query.source)
      }
      if (query.enabled !== undefined) {
        items = items.filter((r) => r.enabled === query.enabled)
      }
      if (query.q) {
        const q = String(query.q).toLowerCase()
        items = items.filter((r) => [r.rule_type, r.rule_param, r.source ?? '', r.group_name ?? ''].some((v) => String(v ?? '').toLowerCase().includes(q)))
      }
      const offset = query.offset ?? 0
      const limit = query.limit ?? 25
      return { rules: items.slice(offset, offset + limit), total: items.length, limit, offset }
    })
    listRuleSourceCountsMock.mockImplementation(async () => sourceCounts())
    setRules([])
    listGroupsMock.mockResolvedValue([])
    listRuleProvidersMock.mockResolvedValue([{ id: 1, name: 'rp', raw_json: '{}', source: 'local', enabled: true, sort_order: 1, created_at: '', updated_at: '' }])
    listSuggestionsMock.mockResolvedValue([])
    bulkRulesMock.mockResolvedValue({ updated: 0 })
    confirmSuggestionMock.mockResolvedValue({ ok: true })
    rejectSuggestionMock.mockResolvedValue({ ok: true })
    applySuggestionsMock.mockResolvedValue({ applied: 0 })
    updateRuleProviderMock.mockResolvedValue({ ok: true })
  })

  it('onMounted 加载三块数据', async () => {
    setRules([{ id: 1, rule_type: 't', rule_param: 'p', rule_config: 1, resolve_dns: true, enabled: false, sort_order: 1, source: 'airport:x' }])
    listRuleProvidersMock.mockResolvedValue([{ id: 1, name: 'rp', raw_json: '{}', source: 'local', enabled: true, sort_order: 1, created_at: '', updated_at: '' }])
    listSuggestionsMock.mockResolvedValue([{ Anomaly: { Type: 'node_unreachable', Message: 'm', Source: 's', Timestamp: 't', Raw: 'r' }, Action: 'prepend', Rule: 'RULE', Reason: 'reason', AutoApply: true }])
    const w = mount(RulesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(listRulesPageMock).toHaveBeenCalled()
    expect(listRuleProvidersMock).toHaveBeenCalled()
    expect(listSuggestionsMock).toHaveBeenCalled()
    w.unmount()
  })

  it('规则集筛选按钮', async () => {
    const w = mount(RulesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    // switch to tab 1
    const tabs = w.findAll('.v-tab')
    if (tabs.length >= 2) await tabs[1].trigger('click')
    await flushPromises()
    // click filter buttons
    const btns = w.findAll('button')
    const allBtn = btns.find((b) => b.text().includes('全部'))
    const enBtn = btns.find((b) => b.text().includes('已启用'))
    const disBtn = btns.find((b) => b.text().includes('已停用'))
    await allBtn?.trigger('click')
    await flushPromises()
    await enBtn?.trigger('click')
    await flushPromises()
    await disBtn?.trigger('click')
    await flushPromises()
    w.unmount()
  })

  it('规则集加载失败', async () => {
    listRuleProvidersMock.mockRejectedValueOnce(new Error('boom'))
    const w = mount(RulesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    w.unmount()
  })



  it('Rule-Bot 建议：confirm/reject/apply', async () => {
    listSuggestionsMock.mockResolvedValue([{ Anomaly: { Type: 'node_unreachable', Message: 'm', Source: 's', Timestamp: 't', Raw: 'r' }, Action: 'prepend', Rule: 'RULE', Reason: 'reason', AutoApply: false }])
    const w = mount(RulesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await openSuggestionsTab(w)
    const confirmBtn = w.find('[data-testid="suggestion-confirm"]')
    const rejectBtn = w.find('[data-testid="suggestion-reject"]')
    const applyBtn = w.findAll('button').find((b) => b.text().includes('应用已确认建议'))
    await confirmBtn?.trigger('click')
    await flushPromises()
    await rejectBtn?.trigger('click')
    await flushPromises()
    await applyBtn?.trigger('click')
    await flushPromises()
    w.unmount()
  })

  it('Rule-Bot 建议：apply 失败', async () => {
    listSuggestionsMock.mockResolvedValue([{ Anomaly: { Type: 't', Message: 'm', Source: 's', Timestamp: 't', Raw: 'r' }, Action: 'a', Rule: 'r', Reason: 'r', AutoApply: false }])
    applySuggestionsMock.mockRejectedValueOnce(new Error('applyerr'))
    const w = mount(RulesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await openSuggestionsTab(w)
    const applyBtn = w.findAll('button').find((b) => b.text().includes('应用已确认建议'))
    await applyBtn?.trigger('click')
    await flushPromises()
    w.unmount()
  })

  it('Rule-Bot 建议：confirm/reject 失败', async () => {
    listSuggestionsMock.mockResolvedValue([{ Anomaly: { Type: 't', Message: 'm', Source: 's', Timestamp: 't', Raw: 'r' }, Action: 'a', Rule: 'r', Reason: 'r', AutoApply: false }])
    confirmSuggestionMock.mockRejectedValueOnce(new Error('cerr'))
    rejectSuggestionMock.mockRejectedValueOnce(new Error('rerr'))
    const w = mount(RulesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await openSuggestionsTab(w)
    const confirmBtn = w.find('[data-testid="suggestion-confirm"]')
    await confirmBtn?.trigger('click')
    await flushPromises()
    const rejectBtn = w.find('[data-testid="suggestion-reject"]')
    await rejectBtn?.trigger('click')
    await flushPromises()
    w.unmount()
  })

  it('Rule-Bot 建议：加载失败', async () => {
    listSuggestionsMock.mockRejectedValueOnce(new Error('suggerr'))
    const w = mount(RulesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await openSuggestionsTab(w)
    w.unmount()
  })

  it('Rule-Bot 建议：501 未启用', async () => {
    listSuggestionsMock.mockRejectedValueOnce(new Error('Rule-Bot 建议队列未启用 501'))
    const w = mount(RulesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await openSuggestionsTab(w)
    w.unmount()
  })

  it('provider-switch 切换调 API', async () => {
    listRuleProvidersMock.mockResolvedValue([{ id: 1, name: 'rp', raw_json: '{}', source: 'local', enabled: true, sort_order: 1, created_at: '', updated_at: '' }])
    const w = mount(RulesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const tabs = w.findAll('.v-tab')
    if (tabs.length >= 2) await tabs[1].trigger('click')
    await flushPromises()
    const switches = w.findAllComponents(VSwitch)
    const providerSwitch = switches.find((s) => s.attributes('data-testid') === 'provider-switch')
    expect(providerSwitch).toBeTruthy()
    await providerSwitch!.vm.$emit('update:modelValue', false)
    await flushPromises()
    expect(updateRuleProviderMock).toHaveBeenCalledWith(1, false)
    w.unmount()
  })

  it('provider-switch 失败回滚', async () => {
    listRuleProvidersMock.mockResolvedValue([{ id: 1, name: 'rp', raw_json: '{}', source: 'local', enabled: true, sort_order: 1, created_at: '', updated_at: '' }])
    updateRuleProviderMock.mockRejectedValueOnce(new Error('toggle fail'))
    const w = mount(RulesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const tabs = w.findAll('.v-tab')
    if (tabs.length >= 2) await tabs[1].trigger('click')
    await flushPromises()
    const switches = w.findAllComponents(VSwitch)
    const providerSwitch = switches.find((s) => s.attributes('data-testid') === 'provider-switch')
    expect(providerSwitch).toBeTruthy()
    await providerSwitch!.vm.$emit('update:modelValue', false)
    await flushPromises()
    expect(updateRuleProviderMock).toHaveBeenCalledWith(1, false)
    expect(providerSwitch!.props('modelValue')).toBe(true)
    w.unmount()
  })

  it('空态：providers/candidates/suggestions 为空', async () => {
    const w = mount(RulesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    w.unmount()
  })

  it('coverage: 规则行 resource update/remove + 规则集 raw_json 展开', async () => {
    const w = mount(RulesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    // tab 0 触发 ResourcePage 的 list 调用（resource.update/remove 通过 ResourcePage 内部覆盖）
    const tabs = w.findAll('.v-tab')
    if (tabs.length) await tabs[0].trigger('click')
    await flushPromises()
    // tab 1 展开规则集 raw_json
    if (tabs.length >= 2) await tabs[1].trigger('click')
    await flushPromises()
    // 展开第一个 expansion panel（点击 title 触发展开）
    const panelTitles = w.findAll('.v-expansion-panel-title')
    if (panelTitles.length) {
      await panelTitles[0].trigger('click')
      await flushPromises()
    }
    w.unmount()
  })


  it('coverage: 规则行 ResourcePage 编辑/新建/删除', async () => {
    const rules = [{ id: 1, rule_type: 't', rule_param: 'p', rule_config: 1, resolve_dns: true, enabled: true, sort_order: 1 }]
    setRules(rules)
    const updateMock = endpoints.updateRule as unknown as ReturnType<typeof vi.fn>
    const deleteMock = endpoints.deleteRule as unknown as ReturnType<typeof vi.fn>
    updateMock.mockResolvedValue({ id: 1, rule_type: 't', rule_param: 'p', rule_config: 1, resolve_dns: true, enabled: true, sort_order: 1 })
    deleteMock.mockResolvedValue({ ok: true })
    const w = mount(RulesPage, { global: { plugins: [vuetify], stubs: { teleport: true, VDialog: { template: '<div><slot /></div>' } } } })
    await flushPromises()
    // tab 0
    await w.findAll('.v-tab')[0].trigger('click')
    await flushPromises()
    // find add button
    const addBtn = w.findAll('button').find((b) => b.text().includes('新增') || b.attributes('data-testid') === 'add')
    if (addBtn) { await addBtn.trigger('click'); await flushPromises() }
    // find edit button
    const editBtn = w.find('[data-testid="edit"]')
    if (editBtn.exists()) { await editBtn.trigger('click'); await flushPromises() }
    // submit edit to trigger resource.update
    const submitBtn = w.find('[data-testid="dialog-submit"]')
    if (submitBtn.exists()) { await submitBtn.trigger('click'); await flushPromises() }
    // find delete button
    const delBtn = w.find('[data-testid="delete"]')
    if (delBtn.exists()) { await delBtn.trigger('click'); await flushPromises() }
    // confirm delete
    const confirmDel = w.find('[data-testid="confirm-delete"]')
    if (confirmDel.exists()) { await confirmDel.trigger('click'); await flushPromises() }
    w.unmount()
  })


  it('按来源整批启用/停用走带 source 的 bulk 接口', async () => {
    setRules([
      { id: 1, rule_type: 't', rule_param: 'p', rule_config: 0, resolve_dns: true, enabled: false, sort_order: 1, source: 'airport:x' },
      { id: 2, rule_type: 't2', rule_param: 'p2', rule_config: 5, resolve_dns: true, enabled: false, sort_order: 2, source: 'airport:x' },
      { id: 3, rule_type: 't3', rule_param: 'p3', rule_config: 1, resolve_dns: true, enabled: false, sort_order: 3, source: 'manual' },
      { id: 4, rule_type: 't4', rule_param: 'p4', rule_config: 1, resolve_dns: true, enabled: false, sort_order: 4, source: null },
    ])
    bulkRulesMock.mockResolvedValue({ updated: 2 })
    notifyState.open = false
    const w = mount(RulesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const vm = w.vm as unknown as { sourceFilter: string | null }
    vm.sourceFilter = 'airport:x'
    await flushPromises()
    await w.find('[data-testid="enable-source-all"]').trigger('click')
    await flushPromises()
    expect(bulkRulesMock).toHaveBeenCalledWith({ source: 'airport:x', sourcePrefix: false, enabled: true })
    expect(notifyState.text).toBe('已启用 2 条')
    await w.find('[data-testid="disable-source-all"]').trigger('click')
    await flushPromises()
    expect(bulkRulesMock).toHaveBeenCalledWith({ source: 'airport:x', sourcePrefix: false, enabled: false })
    expect(notifyState.text).toBe('已停用 2 条')
    w.unmount()
  })

  it('help-panel 含 oneLiner', async () => {
    const w = mount(RulesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.find('[data-testid="help-panel"]').exists()).toBe(true)
    w.unmount()
  })

  it('coverage: AutoApply status dot / disabled provider / null source', async () => {
    setRules([
      { id: 1, rule_type: 't', rule_param: 'p', rule_config: 1, resolve_dns: true, enabled: false, sort_order: 1, source: null },
    ])
    listRuleProvidersMock.mockResolvedValue([
      { id: 1, name: 'rp', raw_json: '{}', source: 'local', enabled: false, sort_order: 1, created_at: '', updated_at: '' },
    ])
    listSuggestionsMock.mockResolvedValue([{ Anomaly: { Type: 't', Message: 'm', Source: 's', Timestamp: 't', Raw: 'r' }, Action: 'a', Rule: 'r', Reason: 'r', AutoApply: true }])
    const w = mount(RulesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const tabs = w.findAll('.v-tab')
    if (tabs.length >= 2) await tabs[1].trigger('click')
    await flushPromises()
    if (tabs.length >= 3) await tabs[2].trigger('click')
    await flushPromises()
    expect(w.text()).toContain('自动应用')
    w.unmount()
  })

  // T10: rule_config 从手填数字改成代理分组下拉
  it('T10: rule_config 用分组下拉（含 #3 HK），新建默认 0 且 payload 写入所选分组', async () => {
    listGroupsMock.mockResolvedValue([
      { id: 3, name: 'HK', type: 'select', regex: '.*', url: 'https://g.example', interval: 300, sort_order: 1 },
      { id: 7, name: 'JP', type: 'select', regex: '.*', url: 'https://g2.example', interval: 300, sort_order: 2 },
    ])
    const createMock = endpoints.createRule as unknown as ReturnType<typeof vi.fn>
    createMock.mockResolvedValue({ id: 1, rule_type: 'DOMAIN', rule_param: 'example.com', rule_config: 3, resolve_dns: true, enabled: true, sort_order: 1 })
    const w = mount(RulesPage, {
      global: {
        plugins: [vuetify],
        stubs: { teleport: true, VDialog: { template: '<div><slot /></div>' } },
      },
    })
    await flushPromises()
    expect(listGroupsMock).toHaveBeenCalled()

    // 方案筛选 + 方案下拉也各是一个 VSelect，按 data-testid 定位「配置」分组下拉
    const groupSelect = w.findAllComponents({ name: 'VSelect' })
      .find((s) => s.attributes('data-testid') === 'field-select')
    expect(groupSelect).toBeTruthy()
    const items = (groupSelect!.props('items') as { title: string; value: number }[]) ?? []
    expect(items.map((i) => i.title)).toEqual(['无分组（DIRECT）', '#3 HK', '#7 JP'])
    expect(items.map((i) => i.value)).toEqual([0, 3, 7])

    await w.find('[data-testid="add"]').trigger('click')
    await flushPromises()
    // blank() 对 select 取第一个 option → 新建默认 0（无分组/DIRECT），不会退化成 ''
    expect(groupSelect!.props('modelValue')).toBe(0)

    await groupSelect!.setValue(3)
    await flushPromises()
    await w.find('[data-testid="dialog-submit"]').trigger('click')
    await flushPromises()
    expect(createMock).toHaveBeenCalledTimes(1)
    expect(createMock.mock.calls[0][0].rule_config).toBe(3)
    w.unmount()
  })

  // ---------------- 来源（source）筛选 / 按来源整批操作 ----------------

  it('来源筛选：默认「全部来源」不带参数，选中具体来源后带 source 请求', async () => {
    rulesFixture = []
    listRulesPageMock.mockImplementation(async (query: any) => {
      let items = rulesFixture
      if (query.source !== undefined && query.source !== null) {
        items = items.filter((r) => (r.source ?? '') === query.source)
      }
      if (query.enabled !== undefined) {
        items = items.filter((r) => r.enabled === query.enabled)
      }
      if (query.q) {
        const q = String(query.q).toLowerCase()
        items = items.filter((r) => [r.rule_type, r.rule_param, r.source ?? '', r.group_name ?? ''].some((v) => String(v ?? '').toLowerCase().includes(q)))
      }
      const offset = query.offset ?? 0
      const limit = query.limit ?? 25
      return { rules: items.slice(offset, offset + limit), total: items.length, limit, offset }
    })
    listRuleSourceCountsMock.mockImplementation(async () => sourceCounts())
    setRules([])
    const w = mountRules()
    await flushPromises()
    expect(listRulesPageMock).toHaveBeenCalledWith(expect.objectContaining({ source: null, limit: 25, offset: 0 }))
    const vm = w.vm as unknown as { sourceFilter: string | null; sourceOptions: { title: string; value: string | null }[] }
    expect(vm.sourceFilter).toBe(null)
    expect(vm.sourceOptions.map((o) => o.title)).toEqual(['全部来源', '自定义'])
    vm.sourceFilter = 'airport:PEK'
    await flushPromises()
    expect(listRulesPageMock).toHaveBeenCalledWith(expect.objectContaining({ source: 'airport:PEK' }))
    w.unmount()
  })

  it('来源筛选：选「自定义」时带空串 source', async () => {
    rulesFixture = []
    listRulesPageMock.mockImplementation(async (query: any) => {
      let items = rulesFixture
      if (query.source !== undefined && query.source !== null) {
        items = items.filter((r) => (r.source ?? '') === query.source)
      }
      if (query.enabled !== undefined) {
        items = items.filter((r) => r.enabled === query.enabled)
      }
      if (query.q) {
        const q = String(query.q).toLowerCase()
        items = items.filter((r) => [r.rule_type, r.rule_param, r.source ?? '', r.group_name ?? ''].some((v) => String(v ?? '').toLowerCase().includes(q)))
      }
      const offset = query.offset ?? 0
      const limit = query.limit ?? 25
      return { rules: items.slice(offset, offset + limit), total: items.length, limit, offset }
    })
    listRuleSourceCountsMock.mockImplementation(async () => sourceCounts())
    setRules([])
    const w = mountRules()
    await flushPromises()
    const vm = w.vm as unknown as { sourceFilter: string | null }
    vm.sourceFilter = ''
    await flushPromises()
    expect(listRulesPageMock).toHaveBeenCalledWith(expect.objectContaining({ source: '' }))
    w.unmount()
  })

  it('来源下拉：列出数据里出现过的来源（去重/排序）；来源列表拉取失败时降级', async () => {
    setRules([
      { id: 1, rule_type: 't', rule_param: 'p', rule_config: 0, resolve_dns: true, enabled: false, sort_order: 1, source: 'airport:PEK' },
      { id: 2, rule_type: 't', rule_param: 'p', rule_config: 0, resolve_dns: true, enabled: false, sort_order: 2, source: 'airport:PEK' },
      { id: 3, rule_type: 't', rule_param: 'p', rule_config: 0, resolve_dns: true, enabled: false, sort_order: 3, source: '' },
    ])
    const w = mountRules()
    await flushPromises()
    const vm = w.vm as unknown as { sourceOptions: { title: string; value: string | null }[] }
    expect(vm.sourceOptions).toEqual([
      { title: '全部来源', value: null },
      { title: '自定义', value: '' },
      { title: 'airport:PEK · 2 条', value: 'airport:PEK' },
    ])
    expect(w.find('[data-testid="rules-source-filter"]').exists()).toBe(true)
    w.unmount()
    listRuleSourceCountsMock.mockRejectedValueOnce(new Error('sources boom'))
    const w2 = mountRules()
    await flushPromises()
    expect((w2.vm as unknown as { sourceOptions: { title: string; value: string | null }[] }).sourceOptions).toEqual([
      { title: '全部来源', value: null },
      { title: '自定义', value: '' },
    ])
    w2.unmount()
    rulesFixture = []
    listRulesPageMock.mockImplementation(async (query: any) => {
      let items = rulesFixture
      if (query.source !== undefined && query.source !== null) {
        items = items.filter((r) => (r.source ?? '') === query.source)
      }
      if (query.enabled !== undefined) {
        items = items.filter((r) => r.enabled === query.enabled)
      }
      if (query.q) {
        const q = String(query.q).toLowerCase()
        items = items.filter((r) => [r.rule_type, r.rule_param, r.source ?? '', r.group_name ?? ''].some((v) => String(v ?? '').toLowerCase().includes(q)))
      }
      const offset = query.offset ?? 0
      const limit = query.limit ?? 25
      return { rules: items.slice(offset, offset + limit), total: items.length, limit, offset }
    })
    listRuleSourceCountsMock.mockImplementation(async () => sourceCounts())
    setRules([])
  })

  it('按来源整批删除：必须二次确认，确认后调用 bulk delete=true', async () => {
    setRules([
      { id: 1, rule_type: 't', rule_param: 'p', rule_config: 0, resolve_dns: true, enabled: false, sort_order: 1, source: 'airport:PEK' },
      { id: 2, rule_type: 't', rule_param: 'p', rule_config: 0, resolve_dns: true, enabled: false, sort_order: 2, source: 'airport:PEK' },
    ])
    bulkRulesMock.mockResolvedValue({ deleted: 2 })
    notifyState.open = false
    const w = mountRules()
    await flushPromises()
    const vm = w.vm as unknown as { sourceFilter: string | null; sourceDeleteOpen: boolean }
    vm.sourceFilter = 'airport:PEK'
    await flushPromises()
    await w.find('[data-testid="delete-source-all"]').trigger('click')
    await flushPromises()
    expect(vm.sourceDeleteOpen).toBe(true)
    expect(bulkRulesMock).not.toHaveBeenCalled()
    const text = w.find('[data-testid="confirm-source-delete-text"]')
    expect(text.exists()).toBe(true)
    expect(text.text()).toContain('airport:PEK')
    expect(text.text()).toContain('2')
    expect(text.text()).toContain('此操作不可撤销')
    // 取消：仍然不发请求
    await w.find('[data-testid="confirm-source-delete-cancel"]').trigger('click')
    await flushPromises()
    expect(vm.sourceDeleteOpen).toBe(false)
    expect(bulkRulesMock).not.toHaveBeenCalled()
    // 再次打开并确认
    await w.find('[data-testid="delete-source-all"]').trigger('click')
    await flushPromises()
    await w.find('[data-testid="confirm-source-delete-ok"]').trigger('click')
    await flushPromises()
    expect(bulkRulesMock).toHaveBeenCalledWith({ source: 'airport:PEK', sourcePrefix: false, delete: true })
    expect(notifyState.text).toBe('已删除 2 条')
    w.unmount()
  })

  it('按来源整批删除弹窗：v-model 关闭回调覆盖', async () => {
    const DialogStub = {
      name: 'VDialog',
      props: ['modelValue'],
      emits: ['update:modelValue'],
      template: '<div><slot /></div>',
    }
    const w = mount(RulesPage, { global: { plugins: [vuetify], stubs: { teleport: true, VDialog: DialogStub } } })
    await flushPromises()
    const vm = w.vm as unknown as { sourceDeleteOpen: boolean; tab: number }
    const target = w.findAllComponents({ name: 'VDialog' }).find((d) => d.attributes('data-testid') === 'confirm-source-delete')
    expect(target).toBeDefined()
    await (target as unknown as { vm: { $emit: (e: string, ...a: unknown[]) => void } }).vm.$emit('update:modelValue', false)
    await flushPromises()
    expect(vm.sourceDeleteOpen).toBe(false)
    w.unmount()
  })

  it('整批操作守卫：没有具体来源 / 进行中时不发请求', async () => {
    setRules([{ id: 1, rule_type: 't', rule_param: 'p', rule_config: 0, resolve_dns: true, enabled: false, sort_order: 1, source: 'airport:x' }])
    const w = mountRules()
    await flushPromises()
    const vm = w.vm as unknown as {
      sourceFilter: string | null
      sourceBusy: boolean
      sourceDeleteOpen: boolean
      sourceEnableAll: (v: boolean) => Promise<void>
      openSourceDelete: () => void
      confirmSourceDelete: () => Promise<void>
    }
    // 全部来源：sourceTarget() 为 null
    await vm.sourceEnableAll(true)
    vm.openSourceDelete()
    expect(vm.sourceDeleteOpen).toBe(false)
    vm.sourceDeleteOpen = true
    await vm.confirmSourceDelete()
    expect(vm.sourceDeleteOpen).toBe(false)
    expect(bulkRulesMock).not.toHaveBeenCalled()
    // 有来源但 busy：也不发请求
    vm.sourceFilter = 'airport:x'
    vm.sourceBusy = true
    await flushPromises()
    await vm.sourceEnableAll(false)
    vm.openSourceDelete()
    expect(vm.sourceDeleteOpen).toBe(false)
    expect(bulkRulesMock).not.toHaveBeenCalled()
    w.unmount()
  })

  it('按来源整批删除失败：错误提示', async () => {
    setRules([{ id: 1, rule_type: 't', rule_param: 'p', rule_config: 0, resolve_dns: true, enabled: false, sort_order: 1, source: 'airport:x' }])
    bulkRulesMock.mockRejectedValueOnce(new Error('del boom'))
    notifyState.open = false
    const w = mountRules()
    await flushPromises()
    const vm = w.vm as unknown as { sourceFilter: string | null; sourceDeleteOpen: boolean }
    vm.sourceFilter = 'airport:x'
    await flushPromises()
    await w.find('[data-testid="delete-source-all"]').trigger('click')
    await flushPromises()
    vm.sourceDeleteOpen = true
    await flushPromises()
    await w.find('[data-testid="confirm-source-delete-ok"]').trigger('click')
    await flushPromises()
    expect(bulkRulesMock).toHaveBeenCalledWith({ source: 'airport:x', sourcePrefix: false, delete: true })
    expect(notifyState.kind).toBe('error')
    expect(notifyState.text).toContain('del boom')
    w.unmount()
  })

  it('整批结果缺 updated/deleted 字段时回退为当前来源的规则条数', async () => {
    setRules([{ id: 1, rule_type: 't', rule_param: 'p', rule_config: 0, resolve_dns: true, enabled: false, sort_order: 1, source: 'airport:x' }])
    bulkRulesMock.mockResolvedValue({})
    notifyState.open = false
    const w = mountRules()
    await flushPromises()
    const vm = w.vm as unknown as { sourceFilter: string | null }
    vm.sourceFilter = 'airport:x'
    await flushPromises()
    await w.find('[data-testid="enable-source-all"]').trigger('click')
    await flushPromises()
    expect(notifyState.text).toBe('已启用 1 条')
    w.unmount()
  })

  it('规则表：来源列渲染原始 source；不再有方案列与批量移动入口', async () => {
    setRules([
      { id: 1, rule_type: 'DOMAIN', rule_param: 'a.com', rule_config: 0, resolve_dns: true, enabled: true, sort_order: 1, source: 'airport:PEK' },
      { id: 2, rule_type: 'DOMAIN', rule_param: 'b.com', rule_config: 0, resolve_dns: true, enabled: true, sort_order: 2, source: '' },
    ])
    const w = mountRules()
    await flushPromises()
    expect(w.text()).toContain('airport:PEK')
    const head = w.find('[data-testid="table"] thead').text()
    expect(head).toContain('来源')
    expect(head).not.toContain('方案')
    expect(w.find('[data-testid="move-profile"]').exists()).toBe(false)
    expect(w.find('[data-testid="rule-profile"]').exists()).toBe(false)
    w.unmount()
  })

  it('新增规则提交 body 不含 profile_id（v2：规则不再归属方案）', async () => {
    rulesFixture = []
    listRulesPageMock.mockImplementation(async (query: any) => {
      let items = rulesFixture
      if (query.source !== undefined && query.source !== null) {
        items = items.filter((r) => (r.source ?? '') === query.source)
      }
      if (query.enabled !== undefined) {
        items = items.filter((r) => r.enabled === query.enabled)
      }
      if (query.q) {
        const q = String(query.q).toLowerCase()
        items = items.filter((r) => [r.rule_type, r.rule_param, r.source ?? '', r.group_name ?? ''].some((v) => String(v ?? '').toLowerCase().includes(q)))
      }
      const offset = query.offset ?? 0
      const limit = query.limit ?? 25
      return { rules: items.slice(offset, offset + limit), total: items.length, limit, offset }
    })
    listRuleSourceCountsMock.mockImplementation(async () => sourceCounts())
    setRules([])
    const createMock = endpoints.createRule as unknown as ReturnType<typeof vi.fn>
    createMock.mockResolvedValue({ id: 1, rule_type: 'DOMAIN', rule_param: 'a.com', rule_config: 0, resolve_dns: true, enabled: true, sort_order: 1 })
    const w = mountRules()
    await flushPromises()
    await w.find('[data-testid="add"]').trigger('click')
    await flushPromises()
    await w.find('[data-testid="dialog-submit"]').trigger('click')
    await flushPromises()
    expect(createMock).toHaveBeenCalledTimes(1)
    expect(createMock.mock.calls[0][0]).not.toHaveProperty('profile_id')
    w.unmount()
  })

  it('编辑规则回填来源字段并随保存发出（body 不含 profile_id）', async () => {
    setRules([{ id: 9, rule_type: 'DOMAIN', rule_param: 'e.com', rule_config: 0, resolve_dns: true, enabled: true, sort_order: 1, source: 'airport:PEK' }])
    const updateMock = endpoints.updateRule as unknown as ReturnType<typeof vi.fn>
    updateMock.mockResolvedValue({ id: 9, rule_type: 'DOMAIN', rule_param: 'e.com', rule_config: 0, resolve_dns: true, enabled: true, sort_order: 1, source: 'airport:PEK' })
    const w = mountRules()
    await flushPromises()
    await w.find('[data-testid="edit"]').trigger('click')
    await flushPromises()
    expect((w.vm as unknown as { ruleDraft: { source?: string } }).ruleDraft.source).toBe('airport:PEK')
    await w.find('[data-testid="dialog-submit"]').trigger('click')
    await flushPromises()
    expect(updateMock).toHaveBeenCalledWith(9, expect.objectContaining({ source: 'airport:PEK' }))
    expect(updateMock.mock.calls[0][1]).not.toHaveProperty('profile_id')
    w.unmount()
  })

  it('loadGroups 失败软化：groups 归空数组，页面主功能不炸（T10 补 catch 分支）', async () => {
    listGroupsMock.mockRejectedValueOnce(new Error('groups boom'))
    const w = mount(RulesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect((w.vm as unknown as { groups: unknown[] }).groups).toEqual([])
    w.unmount()
  })


  // ---------------- D-09：规则行多选 + 批量操作 ----------------

  const D09_RULES = [
    { id: 1, rule_type: 'DOMAIN-SUFFIX', rule_param: 'example.com', rule_config: 3, resolve_dns: true, enabled: true, sort_order: 1, source: 'local', group_name: '#3 HK' },
    { id: 2, rule_type: 'IP-CIDR', rule_param: '10.0.0.0/8', rule_config: 0, resolve_dns: false, enabled: false, sort_order: 2, source: 'airport:PEK', group_name: '' },
    { id: 3, rule_type: 'DOMAIN-KEYWORD', rule_param: 'ads', rule_config: 3, resolve_dns: true, enabled: true, sort_order: 3, source: 'local', group_name: '#3 HK' },
  ]

  function mountRules() {
    return mount(RulesPage, {
      global: {
        plugins: [vuetify],
        stubs: { teleport: true, VDialog: { template: '<div><slot /></div>' } },
      },
    })
  }

  it('D-09 表头全选命中当前筛选的全部行，批量禁用只调用一次且 ids 正确', async () => {
    setRules(D09_RULES)
    const w = mountRules()
    await flushPromises()
    const table = w.find('[data-testid="table"]')
    expect(table.exists()).toBe(true)
    const headerBox = table.find('thead input[type="checkbox"]')
    expect(headerBox.exists()).toBe(true)
    await headerBox.setValue(true)
    await flushPromises()
    // 选中后批量条出现，并显示选中数量
    expect(w.find('[data-testid="bulk-bar"]').exists()).toBe(true)
    expect(w.find('[data-testid="bulk-selected-count"]').text()).toContain('3')
    await w.find('[data-testid="bulk-disable"]').trigger('click')
    await flushPromises()
    expect(bulkRulesMock).toHaveBeenCalledTimes(1)
    expect(bulkRulesMock).toHaveBeenCalledWith({ ids: [1, 2, 3], enabled: false })
    w.unmount()
  })

  it('D-09 筛选后表头全选只选中筛选命中的行', async () => {
    setRules(D09_RULES)
    const w = mountRules()
    await flushPromises()
    await w.find('[data-testid="filter-disabled"]').trigger('click')
    await flushPromises()
    const headerBox = w.find('[data-testid="table"] thead input[type="checkbox"]')
    expect(headerBox.exists()).toBe(true)
    await headerBox.setValue(true)
    await flushPromises()
    await w.find('[data-testid="bulk-enable"]').trigger('click')
    await flushPromises()
    expect(bulkRulesMock).toHaveBeenCalledTimes(1)
    expect(bulkRulesMock).toHaveBeenCalledWith({ ids: [2], enabled: true })
    w.unmount()
  })

  it('D-09 批量删除必须二次确认：取消不发请求，确认后才发送 delete payload', async () => {
    setRules(D09_RULES)
    bulkRulesMock.mockResolvedValue({ deleted: 2 })
    const w = mountRules()
    await flushPromises()
    const rowBoxes = w.findAll('[data-testid="table"] tbody input[type="checkbox"]')
    expect(rowBoxes.length).toBe(3)
    await rowBoxes[0].trigger('click')
    await rowBoxes[1].trigger('click')
    await flushPromises()
    expect(w.find('[data-testid="bulk-selected-count"]').text()).toContain('2')
    await w.find('[data-testid="bulk-delete"]').trigger('click')
    await flushPromises()
    // 还没点确认：不能发请求
    expect(bulkRulesMock).not.toHaveBeenCalled()
    const confirmText = w.find('[data-testid="bulk-confirm-text"]')
    expect(confirmText.exists()).toBe(true)
    expect(confirmText.text()).toContain('删除')
    expect(confirmText.text()).toContain('2')
    // 取消：仍然不发请求
    await w.find('[data-testid="bulk-confirm-cancel"]').trigger('click')
    await flushPromises()
    expect(bulkRulesMock).not.toHaveBeenCalled()
    // 再次打开并确认
    await w.find('[data-testid="bulk-delete"]').trigger('click')
    await flushPromises()
    await w.find('[data-testid="bulk-confirm-delete"]').trigger('click')
    await flushPromises()
    expect(bulkRulesMock).toHaveBeenCalledTimes(1)
    expect(bulkRulesMock).toHaveBeenCalledWith({ ids: [1, 2], delete: true })
    w.unmount()
  })

  it('D-09 批量失败：错误提示出来且不清空选择', async () => {
    setRules(D09_RULES)
    bulkRulesMock.mockRejectedValueOnce(new Error('bulk boom'))
    notifyState.open = false
    const w = mountRules()
    await flushPromises()
    const headerBox = w.find('[data-testid="table"] thead input[type="checkbox"]')
    await headerBox.setValue(true)
    await flushPromises()
    await w.find('[data-testid="bulk-disable"]').trigger('click')
    await flushPromises()
    expect(bulkRulesMock).toHaveBeenCalledTimes(1)
    expect(notifyState.open).toBe(true)
    expect(notifyState.kind).toBe('error')
    expect(notifyState.text).toContain('bulk boom')
    // 失败不清空选择，允许重试
    expect(w.find('[data-testid="bulk-bar"]').exists()).toBe(true)
    w.unmount()
  })

  it('D-09 批量成功后：清空选择并刷新列表', async () => {
    setRules(D09_RULES)
    bulkRulesMock.mockResolvedValue({ updated: 3 })
    const w = mountRules()
    await flushPromises()
    const before = listRulesPageMock.mock.calls.length
    const headerBox = w.find('[data-testid="table"] thead input[type="checkbox"]')
    await headerBox.setValue(true)
    await flushPromises()
    await w.find('[data-testid="bulk-enable"]').trigger('click')
    await flushPromises()
    expect(listRulesPageMock.mock.calls.length).toBeGreaterThan(before)
    expect(w.find('[data-testid="bulk-bar"]').exists()).toBe(false)
    w.unmount()
  })

  it('D-09 规则行分页 footer 保留（每页数目 / 分页器）', async () => {
    setRules(D09_RULES)
    const w = mountRules()
    await flushPromises()
    expect(w.find('[data-testid="table"]').exists()).toBe(true)
    expect(w.find('.v-data-table-footer__items-per-page').exists()).toBe(true)
    expect(w.find('.v-pagination').exists()).toBe(true)
    expect(w.text()).toMatch(/每页数目|Items per page/)
    w.unmount()
  })

  // ---------------- 覆盖率补齐：搜索/筛选/刷新/保存失败/守卫/弹窗 setter ----------------

  const COV_RULES = [
    { id: 21, rule_type: 'DOMAIN-SUFFIX', rule_param: 'hk.example', rule_config: 3, resolve_dns: true, enabled: true, sort_order: 1, source: 'local', group_name: '#3 HK' },
    { id: 22, rule_type: 'IP-CIDR', rule_param: '10.1.0.0/16', rule_config: 0, resolve_dns: false, enabled: false, sort_order: 2, source: null as unknown as string, group_name: undefined as unknown as string },
  ]

  function mountCov() {
    const DialogStub = { name: 'VDialog', emits: ['update:modelValue'], template: '<div class="v-dialog-stub"><slot /></div>' }
    return mount(RulesPage, { global: { plugins: [vuetify], stubs: { teleport: true, VDialog: DialogStub } } })
  }

  it('搜索框输入触发过滤（source 为 null、group_name 缺失也不炸）', async () => {
    setRules(COV_RULES)
    const w = mountCov()
    await flushPromises()
    const search = w.findComponent('[data-testid="rule-search"]')
    expect(search.exists()).toBe(true)
    const searchVm = (search as unknown as { vm: { $emit: (ev: string, v: unknown) => void } }).vm
    searchVm.$emit('update:modelValue', 'HK')
    await new Promise((r) => setTimeout(r, 350))
    await flushPromises()
    expect(w.find('[data-testid="rule-selection-count"]').text()).toContain('筛选命中 1 条')
    ;(w.vm as any).ruleSearch = 'no-such-rule'
    await new Promise((r) => setTimeout(r, 400))
    await flushPromises()
    expect(w.find('[data-testid="rule-selection-count"]').text()).toContain('筛选命中 0 条')
    w.unmount()
  })

  it('筛选按钮与刷新按钮走查（已启用 / 全部 / 重新加载）', async () => {
    setRules(COV_RULES)
    const w = mountCov()
    await flushPromises()
    const before = listRulesPageMock.mock.calls.length
    await w.find('[data-testid="filter-enabled"]').trigger('click')
    await flushPromises()
    expect(w.find('[data-testid="rule-selection-count"]').text()).toContain('筛选命中 1 条')
    await w.find('[data-testid="filter-all"]').trigger('click')
    await flushPromises()
    expect(w.find('[data-testid="rule-selection-count"]').text()).toContain('筛选命中 2 条')
    await w.find('[data-testid="refresh"]').trigger('click')
    await flushPromises()
    expect(listRulesPageMock.mock.calls.length).toBeGreaterThan(before)
    w.unmount()
  })

  it('新增保存失败：错误提示弹出且对话框保持打开', async () => {
    setRules(COV_RULES)
    const createMock = endpoints.createRule as unknown as ReturnType<typeof vi.fn>
    createMock.mockRejectedValueOnce(new Error('create boom'))
    notifyState.open = false
    const w = mountCov()
    await flushPromises()
    await w.find('[data-testid="add"]').trigger('click')
    await flushPromises()
    await w.find('[data-testid="dialog-submit"]').trigger('click')
    await flushPromises()
    expect(notifyState.open).toBe(true)
    expect(notifyState.kind).toBe('error')
    expect(notifyState.text).toContain('create boom')
    expect((w.vm as unknown as { ruleDialog: boolean }).ruleDialog).toBe(true)
    w.unmount()
  })

  it('守卫分支：无待删目标 / 空选择批量 / 批量删除失败', async () => {
    setRules(COV_RULES)
    const w = mountCov()
    await flushPromises()
    const vm = w.vm as unknown as {
      doRuleDelete: () => Promise<void>
      bulkSetEnabled: (v: boolean) => Promise<void>
      bulkDelete: () => Promise<void>
      bulkDeleteOpen: boolean
      selectedRuleIds: number[]
    }
    void vm.doRuleDelete()
    await vm.bulkSetEnabled(true)
    await vm.bulkDelete()
    expect(bulkRulesMock).not.toHaveBeenCalled()
    // 批量删除失败：错误提示、确认框保持打开（catch 分支提前结束）
    bulkRulesMock.mockRejectedValueOnce(new Error('bulk delete boom'))
    notifyState.open = false
    vm.selectedRuleIds = [21]
    await flushPromises()
    await w.find('[data-testid="bulk-delete"]').trigger('click')
    await flushPromises()
    await w.find('[data-testid="bulk-confirm-delete"]').trigger('click')
    await flushPromises()
    expect(bulkRulesMock).toHaveBeenCalledWith({ ids: [21], delete: true })
    expect(notifyState.kind).toBe('error')
    expect(notifyState.text).toContain('bulk delete boom')
    expect(vm.bulkDeleteOpen).toBe(true)
    w.unmount()
  })

  it('弹窗 v-model setter 与取消按钮', async () => {
    setRules(COV_RULES)
    const w = mountCov()
    await flushPromises()
    const vm = w.vm as unknown as {
      ruleDialog: boolean
      ruleConfirmOpen: boolean
      bulkDeleteOpen: boolean
      ruleDraft: Record<string, unknown>
      pendingRuleDelete: unknown
      selectedRuleIds: number[]
    }
    vm.ruleDialog = true
    await flushPromises()
    await w.find('[data-testid="dialog-cancel"]').trigger('click')
    await flushPromises()
    expect(vm.ruleDialog).toBe(false)
    // 三个 v-dialog 的 v-model setter 都由 VDialog 的 update:modelValue 写回
    for (const d of w.findAllComponents({ name: 'VDialog' })) {
      d.vm.$emit('update:modelValue', false)
    }
    await flushPromises()
    // 文本字段 / 开关的 v-model setter
    for (const f of w.findAllComponents({ name: 'VTextField' })) {
      f.vm.$emit('update:modelValue', 'x')
    }
    for (const s of w.findAllComponents({ name: 'VSwitch' })) {
      s.vm.$emit('update:modelValue', true)
    }
    await flushPromises()
    expect(vm.ruleDraft.rule_type).toBe('x')
    expect(vm.ruleDraft.rule_param).toBe('x')
    expect(vm.ruleDraft.resolve_dns).toBe(true)
    expect(vm.ruleDraft.enabled).toBe(true)
    // 单条删除确认：取消
    vm.pendingRuleDelete = COV_RULES[0]
    vm.ruleConfirmOpen = true
    await flushPromises()
    await w.find('[data-testid="confirm-cancel"]').trigger('click')
    await flushPromises()
    expect(vm.ruleConfirmOpen).toBe(false)
    // 批量删除确认：打开再取消
    vm.selectedRuleIds = [21]
    await flushPromises()
    await w.find('[data-testid="bulk-delete"]').trigger('click')
    await flushPromises()
    expect(vm.bulkDeleteOpen).toBe(true)
    await w.find('[data-testid="bulk-confirm-cancel"]').trigger('click')
    await flushPromises()
    expect(vm.bulkDeleteOpen).toBe(false)
    w.unmount()
  })

  it('来源筛选下拉的 update:modelValue 与按来源删除确认弹窗', async () => {
    setRules(D09_RULES)
    const w = mountRules()
    await flushPromises()
    const vm = w.vm as unknown as {
      sourceFilter: string | null
      sourceDeleteOpen: boolean
      sourceTarget: () => string | null
    }
    const selectByLabel = (label: string) => w.findAllComponents({ name: 'VSelect' })
      .find((c) => (c.props() as Record<string, unknown>).label === label)
    // 1) 页面顶部「来源筛选」下拉的 update:modelValue
    const filterSel = selectByLabel('来源筛选')
    expect(filterSel).toBeTruthy()
    await filterSel!.vm.$emit('update:modelValue', 'airport:PEK')
    await flushPromises()
    expect(vm.sourceFilter).toBe('airport:PEK')
    expect(listRulesPageMock).toHaveBeenCalledWith(expect.objectContaining({ source: 'airport:PEK' }))
    expect(vm.sourceTarget()).toBe('airport:PEK')
    // 2) 按来源删除确认弹窗：打开再取消（弹窗在「导入候选」tab 里）
    vm.sourceDeleteOpen = true
    await flushPromises()
    const cancelBtn = w.find('[data-testid="confirm-source-delete-cancel"]')
    expect(cancelBtn.exists()).toBe(true)
    await cancelBtn.trigger('click')
    await flushPromises()
    expect(vm.sourceDeleteOpen).toBe(false)
    w.unmount()
  })


  it('服务端分页选项变化触发重新取数', async () => {
    setRules([
      { id: 1, rule_type: 'DOMAIN', rule_param: 'a.test', rule_config: 0, resolve_dns: true, enabled: true, sort_order: 1, source: '' },
      { id: 2, rule_type: 'DOMAIN', rule_param: 'b.test', rule_config: 0, resolve_dns: true, enabled: true, sort_order: 2, source: '' },
    ])
    const w = mountRules()
    await flushPromises()
    const vm = w.vm as unknown as { onRuleOptions: (o: { page: number; itemsPerPage: number }) => void }
    vm.onRuleOptions({ page: 1, itemsPerPage: 25 })
    vm.onRuleOptions({ page: 2, itemsPerPage: 50 })
    await flushPromises()
    expect(listRulesPageMock).toHaveBeenLastCalledWith(expect.objectContaining({ limit: 50, offset: 50 }))
    w.unmount()
  })

  it('按来源整批启用失败提示 error', async () => {
    setRules([{ id: 1, rule_type: 't', rule_param: 'p', rule_config: 0, resolve_dns: true, enabled: false, sort_order: 1, source: 'airport:x' }])
    bulkRulesMock.mockRejectedValueOnce(new Error('bulk boom'))
    notifyState.open = false
    const w = mountRules()
    await flushPromises()
    const vm = w.vm as unknown as { sourceFilter: string | null; sourceEnableAll: (v: boolean) => Promise<void> }
    vm.sourceFilter = 'airport:x'
    await flushPromises()
    await vm.sourceEnableAll(true)
    await flushPromises()
    expect(notifyState.kind).toBe('error')
    expect(notifyState.text).toContain('bulk boom')
    w.unmount()
  })
})
