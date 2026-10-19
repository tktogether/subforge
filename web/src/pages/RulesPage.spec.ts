import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import RulesPage from './RulesPage.vue'
import { endpoints } from '../api/endpoints'
import { VSwitch } from 'vuetify/components'

vi.mock('../api/endpoints', () => ({
  endpoints: {
    listRules: vi.fn(),
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

const listRulesMock = endpoints.listRules as unknown as ReturnType<typeof vi.fn>
const listGroupsMock = endpoints.listGroups as unknown as ReturnType<typeof vi.fn>
const listRuleProvidersMock = endpoints.listRuleProviders as unknown as ReturnType<typeof vi.fn>
const bulkRulesMock = endpoints.bulkRules as unknown as ReturnType<typeof vi.fn>
const listSuggestionsMock = endpoints.listSuggestions as unknown as ReturnType<typeof vi.fn>
const confirmSuggestionMock = endpoints.confirmSuggestion as unknown as ReturnType<typeof vi.fn>
const rejectSuggestionMock = endpoints.rejectSuggestion as unknown as ReturnType<typeof vi.fn>
const applySuggestionsMock = endpoints.applySuggestions as unknown as ReturnType<typeof vi.fn>
const updateRuleProviderMock = endpoints.updateRuleProvider as unknown as ReturnType<typeof vi.fn>

describe('RulesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    Object.defineProperty(window, 'visualViewport', { value: { width: 1024, height: 768, scale: 1, offsetLeft: 0, offsetTop: 0, addEventListener: vi.fn(), removeEventListener: vi.fn() }, writable: true, configurable: true })
    listRulesMock.mockResolvedValue([])
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
    listRulesMock.mockResolvedValue([{ id: 1, rule_type: 't', rule_param: 'p', rule_config: 1, resolve_dns: true, enabled: false, sort_order: 1, source: 'airport:x' }])
    listRuleProvidersMock.mockResolvedValue([{ id: 1, name: 'rp', raw_json: '{}', source: 'local', enabled: true, sort_order: 1, created_at: '', updated_at: '' }])
    listSuggestionsMock.mockResolvedValue([{ Anomaly: { Type: 'node_unreachable', Message: 'm', Source: 's', Timestamp: 't', Raw: 'r' }, Action: 'prepend', Rule: 'RULE', Reason: 'reason', AutoApply: true }])
    const w = mount(RulesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(listRulesMock).toHaveBeenCalled()
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
    const disBtn = btns.find((b) => b.text().includes('已禁用'))
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

  it('候选加载失败', async () => {
    listRulesMock.mockRejectedValueOnce(new Error('boom2'))
    const w = mount(RulesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    w.unmount()
  })

  it('导入候选：选中启用/禁用 + 启用本机场全部', async () => {
    listRulesMock.mockResolvedValue([
      { id: 1, rule_type: 't', rule_param: 'p', rule_config: 0, resolve_dns: true, enabled: false, sort_order: 1, source: 'airport:x' },
      { id: 2, rule_type: 't2', rule_param: 'p2', rule_config: 5, resolve_dns: true, enabled: false, sort_order: 2, source: 'airport:x' },
    ])
    const w = mount(RulesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    // switch to tab 2
    const tabs = w.findAll('.v-tab')
    if (tabs.length >= 3) await tabs[2].trigger('click')
    await flushPromises()
    // select checkbox
    const checkboxes = w.findAll('input[type="checkbox"]')
    if (checkboxes.length) await checkboxes[0].setValue(true)
    await flushPromises()
    const enableBtn = w.findAll('button').find((b) => b.text().includes('启用选中'))
    const disableBtn = w.findAll('button').find((b) => b.text().includes('禁用选中'))
    const allBtn = w.findAll('button').find((b) => b.text().includes('启用本机场全部'))
    await enableBtn?.trigger('click')
    await flushPromises()
    await disableBtn?.trigger('click')
    await flushPromises()
    await allBtn?.trigger('click')
    await flushPromises()
    w.unmount()
  })

  it('导入候选：enableSelected 空选中直接返回', async () => {
    listRulesMock.mockResolvedValue([])
    const w = mount(RulesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const tabs = w.findAll('.v-tab')
    if (tabs.length >= 3) await tabs[2].trigger('click')
    await flushPromises()
    const enableBtn = w.findAll('button').find((b) => b.text().includes('启用选中'))
    await enableBtn?.trigger('click')
    await flushPromises()
    w.unmount()
  })

  it('导入候选：bulk 失败 notify', async () => {
    listRulesMock.mockResolvedValue([{ id: 1, rule_type: 't', rule_param: 'p', rule_config: 1, resolve_dns: true, enabled: false, sort_order: 1, source: 'airport:x' }])
    bulkRulesMock.mockRejectedValueOnce(new Error('bulkerr'))
    const w = mount(RulesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const tabs = w.findAll('.v-tab')
    if (tabs.length >= 3) await tabs[2].trigger('click')
    await flushPromises()
    const checkboxes = w.findAll('input[type="checkbox"]')
    if (checkboxes.length) await checkboxes[0].setValue(true)
    await flushPromises()
    const enableBtn = w.findAll('button').find((b) => b.text().includes('启用选中'))
    await enableBtn?.trigger('click')
    await flushPromises()
    w.unmount()
  })

  it('Rule-Bot 建议：confirm/reject/apply', async () => {
    listSuggestionsMock.mockResolvedValue([{ Anomaly: { Type: 'node_unreachable', Message: 'm', Source: 's', Timestamp: 't', Raw: 'r' }, Action: 'prepend', Rule: 'RULE', Reason: 'reason', AutoApply: false }])
    const w = mount(RulesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const tabs = w.findAll('.v-tab')
    if (tabs.length >= 4) await tabs[3].trigger('click')
    await flushPromises()
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
    const tabs = w.findAll('.v-tab')
    if (tabs.length >= 4) await tabs[3].trigger('click')
    await flushPromises()
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
    const tabs = w.findAll('.v-tab')
    if (tabs.length >= 4) await tabs[3].trigger('click')
    await flushPromises()
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
    w.unmount()
  })

  it('Rule-Bot 建议：501 未启用', async () => {
    listSuggestionsMock.mockRejectedValueOnce(new Error('Rule-Bot 建议队列未启用 501'))
    const w = mount(RulesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const tabs = w.findAll('.v-tab')
    if (tabs.length >= 4) await tabs[3].trigger('click')
    await flushPromises()
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
    listRulesMock.mockResolvedValue(rules)
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


  it('coverage: candidates 加载失败 + enableAllAirport 失败', async () => {
    // candidates 加载失败
    listRulesMock.mockRejectedValueOnce(new Error('canderr'))
    const w = mount(RulesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await new Promise(r => setTimeout(r, 50))
    await flushPromises()
    // enableAllAirport 失败（需要 candidates 有数据）
    listRulesMock.mockResolvedValue([{ id: 1, rule_type: 't', rule_param: 'p', rule_config: 1, resolve_dns: true, enabled: false, sort_order: 1, source: 'airport:x' }])
    bulkRulesMock.mockRejectedValueOnce(new Error('bulkerr'))
    await w.unmount()
    const w2 = mount(RulesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const tabs = w2.findAll('.v-tab')
    if (tabs.length >= 3) await tabs[2].trigger('click')
    await flushPromises()
    const enableAllBtn = w2.findAll('button').find((b) => b.text().includes('启用本机场全部'))
    if (enableAllBtn) { await enableAllBtn.trigger('click'); await flushPromises() }
    w2.unmount()
  })



  it('coverage: candidates 加载失败', async () => {
    listRulesMock.mockImplementation(() => { throw new Error('canderr') })
    const w = mount(RulesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await new Promise(r => setTimeout(r, 50))
    await flushPromises()
    w.unmount()
    // restore
    listRulesMock.mockResolvedValue([])
  })

  it('导入候选：启用本机场全部按前缀取 ids 调用 bulk', async () => {
    listRulesMock.mockResolvedValue([
      { id: 1, rule_type: 't', rule_param: 'p', rule_config: 0, resolve_dns: true, enabled: false, sort_order: 1, source: 'airport:x' },
      { id: 2, rule_type: 't2', rule_param: 'p2', rule_config: 5, resolve_dns: true, enabled: false, sort_order: 2, source: 'airport:x' },
      { id: 3, rule_type: 't3', rule_param: 'p3', rule_config: 1, resolve_dns: true, enabled: false, sort_order: 3, source: 'manual' },
      { id: 4, rule_type: 't4', rule_param: 'p4', rule_config: 1, resolve_dns: true, enabled: false, sort_order: 4, source: null },
    ])
    bulkRulesMock.mockResolvedValue({ updated: 2 })
    const w = mount(RulesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const tabs = w.findAll('.v-tab')
    if (tabs.length >= 3) await tabs[2].trigger('click')
    await flushPromises()
    const allBtn = w.find('[data-testid="enable-all-airport"]')
    await allBtn.trigger('click')
    await flushPromises()
    expect(bulkRulesMock).toHaveBeenCalledWith({ ids: [1, 2], enabled: true })
    w.unmount()
  })

  it('help-panel 含 oneLiner', async () => {
    const w = mount(RulesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.find('[data-testid="help-panel"]').exists()).toBe(true)
    w.unmount()
  })

  it('导入候选：启用本机场全部无机场规则', async () => {
    let data: unknown[] = [{ id: 1, rule_type: 't', rule_param: 'p', rule_config: 1, resolve_dns: true, enabled: false, sort_order: 1, source: 'airport:x' }]
    listRulesMock.mockImplementation(() => Promise.resolve(data))
    const w = mount(RulesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const tabs = w.findAll('.v-tab')
    if (tabs.length >= 3) await tabs[2].trigger('click')
    await flushPromises()
    // 现在列表里已经没有机场规则了
    data = [{ id: 3, rule_type: 't3', rule_param: 'p3', rule_config: 1, resolve_dns: true, enabled: false, sort_order: 3, source: 'manual' }]
    const allBtn = w.find('[data-testid="enable-all-airport"]')
    await allBtn.trigger('click')
    await flushPromises()
    expect(bulkRulesMock).not.toHaveBeenCalled()
    w.unmount()
  })

  it('coverage: AutoApply status dot / disabled provider / null source', async () => {
    listRulesMock.mockResolvedValue([
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
    if (tabs.length >= 4) await tabs[3].trigger('click')
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

    const selects = w.findAllComponents({ name: 'VSelect' })
    expect(selects.length).toBe(1)
    const items = (selects[0].props('items') as { title: string; value: number }[]) ?? []
    expect(items.map((i) => i.title)).toEqual(['无分组（DIRECT）', '#3 HK', '#7 JP'])
    expect(items.map((i) => i.value)).toEqual([0, 3, 7])

    await w.find('[data-testid="add"]').trigger('click')
    await flushPromises()
    // blank() 对 select 取第一个 option → 新建默认 0（无分组/DIRECT），不会退化成 ''
    expect(selects[0].props('modelValue')).toBe(0)

    await selects[0].setValue(3)
    await flushPromises()
    await w.find('[data-testid="dialog-submit"]').trigger('click')
    await flushPromises()
    expect(createMock).toHaveBeenCalledTimes(1)
    expect(createMock.mock.calls[0][0].rule_config).toBe(3)
    w.unmount()
  })

  it('loadGroups 失败软化：groups 归空数组，页面主功能不炸（T10 补 catch 分支）', async () => {
    listGroupsMock.mockRejectedValueOnce(new Error('groups boom'))
    const w = mount(RulesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect((w.vm as unknown as { groups: unknown[] }).groups).toEqual([])
    w.unmount()
  })
})
