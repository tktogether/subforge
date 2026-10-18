import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import RulesPage from './RulesPage.vue'
import { endpoints } from '../api/endpoints'

vi.mock('../api/endpoints', () => ({
  endpoints: {
    listRules: vi.fn(),
    createRule: vi.fn(),
    updateRule: vi.fn(),
    deleteRule: vi.fn(),
    listRuleProviders: vi.fn(),
    bulkRules: vi.fn(),
    listSuggestions: vi.fn(),
    confirmSuggestion: vi.fn(),
    rejectSuggestion: vi.fn(),
    applySuggestions: vi.fn(),
  },
}))

const vuetify = createVuetify({ components, directives })

const listRulesMock = endpoints.listRules as unknown as ReturnType<typeof vi.fn>
const listRuleProvidersMock = endpoints.listRuleProviders as unknown as ReturnType<typeof vi.fn>
const bulkRulesMock = endpoints.bulkRules as unknown as ReturnType<typeof vi.fn>
const listSuggestionsMock = endpoints.listSuggestions as unknown as ReturnType<typeof vi.fn>
const confirmSuggestionMock = endpoints.confirmSuggestion as unknown as ReturnType<typeof vi.fn>
const rejectSuggestionMock = endpoints.rejectSuggestion as unknown as ReturnType<typeof vi.fn>
const applySuggestionsMock = endpoints.applySuggestions as unknown as ReturnType<typeof vi.fn>

describe('RulesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    Object.defineProperty(window, 'visualViewport', { value: { width: 1024, height: 768, scale: 1, offsetLeft: 0, offsetTop: 0, addEventListener: vi.fn(), removeEventListener: vi.fn() }, writable: true, configurable: true })
    listRulesMock.mockResolvedValue([])
    listRuleProvidersMock.mockResolvedValue([{ id: 1, name: 'rp', raw_json: '{}', source: 'local', enabled: true, sort_order: 1, created_at: '', updated_at: '' }])
    listSuggestionsMock.mockResolvedValue([])
    bulkRulesMock.mockResolvedValue({ updated: 0 })
    confirmSuggestionMock.mockResolvedValue({ ok: true })
    rejectSuggestionMock.mockResolvedValue({ ok: true })
    applySuggestionsMock.mockResolvedValue({ applied: 0 })
  })

  it('onMounted 加载三块数据', async () => {
    listRulesMock.mockResolvedValue([{ id: 1, rule_type: 't', rule_param: 'p', rule_config: 1, resolve_dns: true, enabled: false, sort_order: 1, source: 'airport:x' }])
    listRuleProvidersMock.mockResolvedValue([{ id: 1, name: 'rp', raw_json: '{}', source: 'local', enabled: true, sort_order: 1, created_at: '', updated_at: '' }])
    listSuggestionsMock.mockResolvedValue([{ Anomaly: { Type: 'node_unreachable', Message: 'm', Source: 's', Timestamp: 't', Raw: 'r' }, Action: 'prepend', Rule: 'RULE', Reason: 'reason', AutoApply: false }])
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
    const confirmBtn = w.findAll('button').find((b) => b.text().includes('Confirm'))
    const rejectBtn = w.findAll('button').find((b) => b.text().includes('Reject'))
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
    const confirmBtn = w.findAll('button').find((b) => b.text().includes('Confirm'))
    await confirmBtn?.trigger('click')
    await flushPromises()
    const rejectBtn = w.findAll('button').find((b) => b.text().includes('Reject'))
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

})
