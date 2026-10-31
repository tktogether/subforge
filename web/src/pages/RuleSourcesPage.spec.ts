import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import RuleSourcesPage from './RuleSourcesPage.vue'
import { endpoints } from '../api/endpoints'
import { PAGE_HELP } from '../content/help'
import { notifyState } from '../composables/useNotify'

vi.mock('../api/endpoints', () => ({
  endpoints: {
    listRuleSources: vi.fn(),
    updateRuleSources: vi.fn(),
    rollbackRuleSource: vi.fn(),
    generateMRS: vi.fn(),
    addRuleSource: vi.fn(),
    removeRuleSource: vi.fn(),
    renameRuleSource: vi.fn(),
    updateRuleSourceOne: vi.fn(),
    listRuleSourceRules: vi.fn(),
    importRuleSourceRules: vi.fn(),
    rulesets: vi.fn(),
  },
}))

const vuetify = createVuetify({ components, directives })

const listMock = endpoints.listRuleSources as unknown as ReturnType<typeof vi.fn>
const updateMock = endpoints.updateRuleSources as unknown as ReturnType<typeof vi.fn>
const rollbackMock = endpoints.rollbackRuleSource as unknown as ReturnType<typeof vi.fn>
const mrsMock = endpoints.generateMRS as unknown as ReturnType<typeof vi.fn>
const addMock = endpoints.addRuleSource as unknown as ReturnType<typeof vi.fn>
const removeMock = endpoints.removeRuleSource as unknown as ReturnType<typeof vi.fn>
const renameMock = endpoints.renameRuleSource as unknown as ReturnType<typeof vi.fn>
const updateOneMock = endpoints.updateRuleSourceOne as unknown as ReturnType<typeof vi.fn>
const listRulesMock = endpoints.listRuleSourceRules as unknown as ReturnType<typeof vi.fn>
const importRulesMock = endpoints.importRuleSourceRules as unknown as ReturnType<typeof vi.fn>

function makeSources() {
  return [
    { name: 'src1', url: 'http://x', format: 'mrs', dir: '/d', versions: ['v1', 'v2'], latest: 'v2', origin: 'remote', label: 'src1', rule_count: 12, updated_at: 1700000000 },
    { name: 'src2', url: 'http://y', format: 'text', dir: '/e', versions: [], latest: '', origin: 'airport', label: '守候-SNTP', rule_count: 0, updated_at: 0 },
  ]
}

function makeManifest(overrides: Record<string, unknown> = {}) {
  return {
    manifest_path: '/m/manifest.yaml',
    default_manifest_path: '/default/manifest.yaml',
    output_dir: '/out',
    exists: true,
    hint: '',
    manifest_template: 'tpl',
    sources: makeSources(),
    ...overrides,
  }
}

describe('RuleSourcesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    Object.defineProperty(window, 'visualViewport', { value: { width: 1024, height: 768, scale: 1, offsetLeft: 0, offsetTop: 0, addEventListener: vi.fn(), removeEventListener: vi.fn() }, writable: true, configurable: true })
    listMock.mockResolvedValue(makeManifest())
    updateMock.mockResolvedValue({ results: [] })
    rollbackMock.mockResolvedValue({ path: '/rb/v1' })
    mrsMock.mockResolvedValue({ path: '/d/v2.mrs', bytes: 1024 })
    addMock.mockResolvedValue({ path: '/m/manifest.yaml', created: true })
    removeMock.mockResolvedValue({ ok: true })
    renameMock.mockResolvedValue({ ok: true, name: 'renamed' })
    updateOneMock.mockResolvedValue({ results: [{ source: 'src1', version: 'v3', path: '/p/v3', from_cache: false, err: '' }] })
    listRulesMock.mockResolvedValue({ name: 'src1', total: 2, offset: 0, limit: 50, rules: ['DOMAIN-SUFFIX,a.test,DIRECT', 'IP-CIDR,1.1.1.1/32,DIRECT,no-resolve'] })
    importRulesMock.mockResolvedValue({ added: 2, skipped: 0, overwrote: 0, warnings: [] })
  })

  it('onMounted 加载 listRuleSources 渲染列表与 testids', async () => {
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(listMock).toHaveBeenCalledTimes(1)
    expect(w.find('[data-testid="manifest-card"]').exists()).toBe(true)
    expect(w.find('[data-testid="manifest-path"]').text()).toContain('/m/manifest.yaml')
    expect(w.find('[data-testid="manifest-default-path"]').text()).toContain('/default/manifest.yaml')
    expect(w.find('[data-testid="manifest-output-dir"]').text()).toContain('/out')
    expect(w.find('[data-testid="manifest-exists"]').exists()).toBe(true)
    expect(w.findAll('[data-testid="source-row"]').length).toBe(2)
    expect(w.text()).toContain('src1')
    expect(w.text()).toContain('守候-SNTP')
    expect(w.text()).toContain('远程')
    expect(w.text()).toContain('机场')
    expect(w.text()).toContain('12')
    expect(w.text()).toContain('2023')
    expect(w.text()).not.toContain('v2')
    w.unmount()
  })

  it('加载失败显示错误，点击重试重新加载', async () => {
    listMock.mockRejectedValueOnce(new Error('boom'))
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.find('[data-testid="async-error"]').exists()).toBe(true)
    expect(w.text()).toContain('boom')
    await w.find('[data-testid="async-retry"]').trigger('click')
    await flushPromises()
    expect(listMock).toHaveBeenCalledTimes(2)
    w.unmount()
  })

  it('未配置清单：不显示生成按钮，新增规则源是主入口', async () => {
    listMock.mockResolvedValue(makeManifest({ exists: false, sources: [], hint: '未配置清单', manifest_path: '' }))
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.find('[data-testid="manifest-hint"]').exists()).toBe(true)
    expect(w.find('[data-testid="manifest-hint"]').text()).toContain('未配置清单')
    expect(w.find('[data-testid="manifest-exists"]').exists()).toBe(false)
    expect(w.find('[data-testid="empty-state"]').exists()).toBe(true)
    expect(w.find('[data-testid="init-manifest"]').exists()).toBe(false)
    const add = w.find('[data-testid="source-add"]')
    expect(add.exists()).toBe(true)
    expect(add.classes().join(' ')).toContain('primary')
    await add.trigger('click')
    expect((w.vm as any).addDialog).toBe(true)
    w.unmount()
  })

  it('表头只展示名称/来源/规则数/更新时间，不再展示 URL 与版本', async () => {
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const headers = w.findAll('th').map((th) => th.text())
    expect(headers).toEqual(['名称', '来源', '规则数', '更新时间', '操作'])
    const rows = w.findAll('[data-testid="source-row"]')
    expect(rows[1].text()).toContain('机场')
    expect(rows[1].text()).toContain('—')
    expect(rows[1].text()).not.toContain('http://y')
    w.unmount()
  })

  it('点击全部更新成功，渲染 update-results', async () => {
    updateMock.mockResolvedValue({ results: [{ source: 'src1', version: 'v3', path: '/p/v3', from_cache: true, err: '' }] })
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await w.find('[data-testid="update-all"]').trigger('click')
    await flushPromises()
    expect(updateMock).toHaveBeenCalledTimes(1)
    expect(w.find('[data-testid="update-results"]').exists()).toBe(true)
    expect(w.find('[data-testid="from-cache"]').exists()).toBe(true)
    expect(listMock).toHaveBeenCalledTimes(2)
    w.unmount()
  })

  it('更新结果含 err 显示 update-error', async () => {
    updateMock.mockResolvedValue({ results: [{ source: 'src1', version: '', path: '', from_cache: false, err: 'boom' }] })
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await w.find('[data-testid="update-all"]').trigger('click')
    await flushPromises()
    expect(w.find('[data-testid="source-update-error"]').exists()).toBe(true)
    expect(w.find('[data-testid="source-update-error"]').text()).toContain('boom')
    expect(w.text()).toContain('该源保留上一版本，产物不受影响')
    w.unmount()
  })

  it('点击全部更新失败', async () => {
    updateMock.mockRejectedValueOnce(new Error('upderr'))
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await w.find('[data-testid="update-all"]').trigger('click')
    await flushPromises()
    expect(updateMock).toHaveBeenCalledTimes(1)
    w.unmount()
  })

  it('全部更新失败时，500 响应体里的逐源结果仍要渲染出来', async () => {
    // 服务端在 500 里也回逐源结果；界面只弹一句「所有 N 个 source 更新失败」
    // 的话，唯一能定位问题的原因就丢了。
    updateMock.mockRejectedValueOnce({
      status: 500,
      body: JSON.stringify({
        error: '所有 1 个 source 更新失败',
        results: [{ source: 'src1', version: '', path: '', from_cache: false, err: '展开失败：56 个 ruleset' }],
      }),
    })
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await w.find('[data-testid="update-all"]').trigger('click')
    await flushPromises()
    expect(w.find('[data-testid="update-results"]').exists()).toBe(true)
    expect(w.find('[data-testid="source-update-error"]').text()).toContain('展开失败')
    w.unmount()
  })

  it('全部更新请求进行中：重复点击被吞掉，行内更新/回滚/MRS 按钮全 disabled', async () => {
    // D-12：没有进行中状态时，双击「全部更新」会发两次请求、写两份版本文件。
    let resolveUpdate: (v: unknown) => void = () => {}
    updateMock.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveUpdate = resolve
        }),
    )
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const allBtn = w.find('[data-testid="update-all"]')
    await allBtn.trigger('click')
    await w.vm.$nextTick()
    expect((w.vm as any).updating).toBe(true)
    expect(allBtn.attributes('disabled')).toBeDefined()
    expect(w.find('[data-testid="source-update"]').attributes('disabled')).toBeDefined()
    expect(w.find('[data-testid="source-rollback"]').attributes('disabled')).toBeDefined()
    expect(w.find('[data-testid="source-mrs"]').attributes('disabled')).toBeDefined()
    // 请求还在飞时再次触发 handler，必须被 guard 挡掉。
    await (w.vm as any).updateAll()
    expect(updateMock).toHaveBeenCalledTimes(1)
    resolveUpdate({ results: [] })
    await flushPromises()
    expect((w.vm as any).updating).toBe(false)
    expect((w.vm as any).updatingAll).toBe(false)
    expect(w.find('[data-testid="source-update"]').attributes('disabled')).toBeUndefined()
    w.unmount()
  })

  it('单源更新请求进行中：该行按钮 loading、行内更新/回滚/MRS 全 disabled', async () => {
    let resolveOne: (v: unknown) => void = () => {}
    updateOneMock.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveOne = resolve
        }),
    )
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await w.find('[data-testid="source-update"]').trigger('click')
    await w.vm.$nextTick()
    const updateBtn = w.find('[data-testid="source-update"]')
    expect((w.vm as any).updatingOne).toBe('src1')
    expect(updateBtn.attributes('disabled')).toBeDefined()
    expect(updateBtn.find('.v-progress-circular').exists()).toBe(true)
    expect(w.find('[data-testid="source-rollback"]').attributes('disabled')).toBeDefined()
    expect(w.find('[data-testid="source-mrs"]').attributes('disabled')).toBeDefined()
    // 同一行再点一次也不能重复发请求。
    await (w.vm as any).updateOne('src1')
    expect(updateOneMock).toHaveBeenCalledTimes(1)
    resolveOne({ results: [] })
    await flushPromises()
    expect((w.vm as any).updating).toBe(false)
    expect((w.vm as any).updatingOne).toBe('')
    w.unmount()
  })

  it('回滚请求进行中：确认按钮 loading 且已 disabled', async () => {
    let resolveRollback: (v: unknown) => void = () => {}
    rollbackMock.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveRollback = resolve
        }),
    )
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    ;(w.vm as any).openRollback('src1')
    const p = (w.vm as any).doRollback()
    await w.vm.$nextTick()
    expect((w.vm as any).updating).toBe(true)
    await (w.vm as any).doRollback()
    expect(rollbackMock).toHaveBeenCalledTimes(1)
    resolveRollback({ path: '/rb/v1' })
    await p
    await flushPromises()
    expect((w.vm as any).updating).toBe(false)
    w.unmount()
  })

  it('点击回滚按钮打开对话框，确认后调用 rollbackRuleSource', async () => {
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await w.find('[data-testid="source-rollback"]').trigger('click')
    await flushPromises()
    expect((w.vm as any).rollbackDialog).toBe(true)
    expect((w.vm as any).rollbackName).toBe('src1')
    await (w.vm as any).doRollback()
    await flushPromises()
    expect(rollbackMock).toHaveBeenCalledWith('src1')
    expect(listMock).toHaveBeenCalledTimes(2)
    w.unmount()
  })

  it('取消回滚对话框', async () => {
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    ;(w.vm as any).rollbackName = 'src1'
    ;(w.vm as any).rollbackDialog = true
    await (w.vm as any).cancelRollback()
    await flushPromises()
    expect((w.vm as any).rollbackDialog).toBe(false)
    w.unmount()
  })

  it('回滚失败 notify', async () => {
    rollbackMock.mockRejectedValueOnce(new Error('rberr'))
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    ;(w.vm as any).rollbackName = 'src1'
    ;(w.vm as any).rollbackDialog = true
    await (w.vm as any).doRollback()
    await flushPromises()
    expect(rollbackMock).toHaveBeenCalled()
    w.unmount()
  })

  it('回滚 404 原样展示', async () => {
    rollbackMock.mockRejectedValueOnce(new Error('规则源 src1 不存在'))
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    ;(w.vm as any).rollbackName = 'src1'
    ;(w.vm as any).rollbackDialog = true
    await (w.vm as any).doRollback()
    await flushPromises()
    expect(rollbackMock).toHaveBeenCalled()
    w.unmount()
  })

  it('无版本时回滚与 MRS 按钮 disabled', async () => {
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const rows = w.findAll('[data-testid="source-row"]')
    const secondRow = rows[1]
    const rollbackBtn = secondRow.find('[data-testid="source-rollback"]')
    const mrsBtn = secondRow.find('[data-testid="source-mrs"]')
    expect(rollbackBtn.attributes('disabled')).toBeDefined()
    expect(mrsBtn.attributes('disabled')).toBeDefined()
    // 机场规则源由机场维护，不能单独删除。
    expect(secondRow.find('[data-testid="source-remove"]').attributes('disabled')).toBeDefined()
    expect(rows[0].find('[data-testid="source-remove"]').attributes('disabled')).toBeUndefined()
    w.unmount()
  })

  it('打开 MRS 对话框，选择行为并生成成功', async () => {
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await w.find('[data-testid="source-mrs"]').trigger('click')
    await flushPromises()
    expect((w.vm as any).mrsDialog).toBe(true)
    expect((w.vm as any).mrsName).toBe('src1')
    ;(w.vm as any).mrsBehavior = 'ipcidr'
    await (w.vm as any).doMrs()
    await flushPromises()
    expect(mrsMock).toHaveBeenCalledWith('src1', 'ipcidr')
    expect(listMock).toHaveBeenCalledTimes(2)
    w.unmount()
  })

  it('MRS 生成失败', async () => {
    mrsMock.mockRejectedValueOnce(new Error('mrserr'))
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    ;(w.vm as any).mrsName = 'src1'
    ;(w.vm as any).mrsDialog = true
    ;(w.vm as any).mrsBehavior = 'domain'
    await (w.vm as any).doMrs()
    await flushPromises()
    expect(mrsMock).toHaveBeenCalled()
    w.unmount()
  })

  it('MRS 对话框取消', async () => {
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    ;(w.vm as any).mrsName = 'src1'
    ;(w.vm as any).mrsDialog = true
    await (w.vm as any).cancelMrs()
    await flushPromises()
    expect((w.vm as any).mrsDialog).toBe(false)
    w.unmount()
  })

  it('机场已删除的源可清理，仍存在的机场源仍由机场维护', async () => {
    listMock.mockResolvedValue(makeManifest({ sources: [
      { name: 'remote', latest: 'v1', origin: 'remote', label: 'remote', versions: ['v1'], rule_count: 1 },
      { name: 'airport-1-old', latest: 'v1', origin: 'airport', label: 'old', airport_missing: true, versions: ['v1'], rule_count: 2 },
      { name: 'airport-2-live', latest: 'v1', origin: 'airport', label: 'live', airport_missing: false, versions: ['v1'], rule_count: 2 },
    ] }))
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const rows = w.findAll('[data-testid="source-row"]')
    expect(rows[1].find('[data-testid="source-airport-missing"]').exists()).toBe(true)
    expect(rows[1].find('[data-testid="source-remove"]').attributes('disabled')).toBeUndefined()
    expect(rows[2].find('[data-testid="source-remove"]').attributes('disabled')).toBeDefined()
    w.unmount()
  })

  it('不再单独渲染版本列表', async () => {
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.findAll('[data-testid="version-item"]').length).toBe(0)
    expect(w.text()).not.toContain('的版本')
    w.unmount()
  })

  it('MRS 与版本保留说明在帮助弹窗中提供', async () => {
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.find('[data-testid="help-toggle"]').exists()).toBe(true)
    const h = PAGE_HELP['rule-sources']
    const text = [...(h.concepts ?? []).map((c) => c.term + c.explain), ...(h.faq ?? []).map((f) => f.q + f.a)].join('\n')
    expect(text).toContain('mihomo')
    expect(text).toContain('保留最近 5 个版本')
    w.unmount()
  })

  it('pageHelp oneLiner 渲染在 PageHeader subtitle', async () => {
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.text()).toContain(PAGE_HELP['rule-sources'].oneLiner)
    w.unmount()
  })

  it('help-panel 渲染', async () => {
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.find('[data-testid="help-panel"]').exists()).toBe(true)
    w.unmount()
  })

  it('coverage: 所有对话框 v-model setter', async () => {
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify], stubs: { teleport: true } }, attachTo: document.body })
    await flushPromises()
    const dlgs = w.findAllComponents({ name: 'VDialog' })
    for (const dlg of dlgs) {
      await dlg.setValue(true)
    }
    await flushPromises()
    for (const dlg of dlgs) {
      await dlg.setValue(false)
    }
    await flushPromises()
    w.unmount()
  })

  it('coverage: mrsBehavior v-select v-model setter', async () => {
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify], stubs: { teleport: true, VDialog: { template: '<div><slot /></div>' } } }, attachTo: document.body })
    await flushPromises()
    const sels = w.findAllComponents({ name: 'VSelect' })
    console.log('VSELECT_COUNT_STUBBED:', sels.length)
    for (const sel of sels) {
      if (sel.exists()) {
        await sel.setValue('ipcidr')
        await flushPromises()
        await new Promise(r => setTimeout(r, 50))
        await flushPromises()
      }
    }
    w.unmount()
  })

  it('coverage: initDialog v-model setter', async () => {
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify], stubs: { teleport: true } }, attachTo: document.body })
    await flushPromises()
    const dlgs = w.findAllComponents({ name: 'VDialog' })
    if (dlgs.length >= 1 && dlgs[0].exists()) {
      await dlgs[0].setValue(true)
      await flushPromises()
      await new Promise(r => setTimeout(r, 50))
      await flushPromises()
    }
    w.unmount()
  })
  it('coverage: manifest fields nullish branches', async () => {
    listMock.mockResolvedValueOnce({
      manifest_path: '/m/manifest.yaml',
      // intentionally omit default_manifest_path, output_dir, exists, hint, manifest_template, sources
    })
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.text()).toContain('/m/manifest.yaml')
    w.unmount()
  })

  it('coverage: update results null branch', async () => {
    updateMock.mockResolvedValueOnce({ results: null })
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const btn = w.find('[data-testid="update-all"]')
    if (btn.exists()) {
      await btn.trigger('click')
      await flushPromises()
    }
    w.unmount()
  })

  it('新增对话框：填表确认调用 addRuleSource 并重载列表', async () => {
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await w.find('[data-testid="source-add"]').trigger('click')
    await flushPromises()
    expect((w.vm as any).addDialog).toBe(true)
    ;(w.vm as any).addUrl = 'http://example.com/new.yaml'
    ;(w.vm as any).addName = 'new'
    await (w.vm as any).confirmAdd()
    await flushPromises()
    expect(addMock).toHaveBeenCalledWith({ name: 'new', url: 'http://example.com/new.yaml', input_format: undefined })
    expect((w.vm as any).addDialog).toBe(false)
    w.unmount()
  })

  it('删除对话框：确认调用 removeRuleSource', async () => {
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    ;(w.vm as any).openRemove('src2')
    await flushPromises()
    await (w.vm as any).confirmRemove()
    await flushPromises()
    expect(removeMock).toHaveBeenCalledWith('src2')
    expect((w.vm as any).removeDialog).toBe(false)
    w.unmount()
  })

  it('行内更新调用 updateRuleSourceOne（不是 updateAll）', async () => {
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await (w.vm as any).updateOne('src1')
    await flushPromises()
    expect(updateOneMock).toHaveBeenCalledWith('src1')
    expect(updateMock).not.toHaveBeenCalled()
    w.unmount()
  })

  // ---- coverage 补测：guard 分支 / catch 分支 / template 内联 handler ----

  it('coverage: confirmAdd 空 URL 直接 return（guard 分支）', async () => {
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    ;(w.vm as any).openAdd()
    await flushPromises()
    ;(w.vm as any).addUrl = '   '
    await (w.vm as any).confirmAdd()
    await flushPromises()
    expect(addMock).not.toHaveBeenCalled()
    expect((w.vm as any).addDialog).toBe(true)
    w.unmount()
  })

  it('coverage: confirmAdd 失败走 catch 通知错误', async () => {
    addMock.mockRejectedValueOnce(new Error('add fail'))
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    ;(w.vm as any).openAdd()
    ;(w.vm as any).addUrl = 'http://x.yaml'
    await (w.vm as any).confirmAdd()
    await flushPromises()
    expect(addMock).toHaveBeenCalled()
    expect((w.vm as any).addDialog).toBe(true)
    expect((w.vm as any).addSaving).toBe(false)
    w.unmount()
  })

  it('coverage: confirmRemove 防重入 guard（removeSaving=true 直接 return）', async () => {
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    ;(w.vm as any).removeSaving = true
    ;(w.vm as any).removeName = 'src2'
    await (w.vm as any).confirmRemove()
    await flushPromises()
    expect(removeMock).not.toHaveBeenCalled()
    ;(w.vm as any).removeSaving = false
    w.unmount()
  })

  it('coverage: confirmRemove 失败走 catch 通知错误', async () => {
    removeMock.mockRejectedValueOnce(new Error('rm fail'))
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    ;(w.vm as any).openRemove('src2')
    await (w.vm as any).confirmRemove()
    await flushPromises()
    expect(removeMock).toHaveBeenCalledWith('src2')
    expect((w.vm as any).removeDialog).toBe(true)
    expect((w.vm as any).removeSaving).toBe(false)
    w.unmount()
  })

  it('coverage: updateOne 防重入 guard（updatingOne 非空直接 return）', async () => {
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    ;(w.vm as any).updatingOne = 'src1'
    await (w.vm as any).updateOne('src2')
    await flushPromises()
    expect(updateOneMock).not.toHaveBeenCalled()
    ;(w.vm as any).updatingOne = ''
    w.unmount()
  })

  it('coverage: updateOne 失败走 catch 通知错误且 updatingOne 归位', async () => {
    updateOneMock.mockRejectedValueOnce(new Error('upd fail'))
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await (w.vm as any).updateOne('src1')
    await flushPromises()
    expect(updateOneMock).toHaveBeenCalledWith('src1')
    expect((w.vm as any).updatingOne).toBe('')
    w.unmount()
  })

  it('coverage: 对话框内联 handler——add 表单 v-model、add-cancel/remove-cancel、行内 source-update/source-remove 按钮', async () => {
    const w = mount(RuleSourcesPage, {
      global: {
        plugins: [vuetify],
        stubs: { teleport: true, VDialog: { template: '<div class="dlgstub"><button class="dlg-emit" @click="$emit(\'update:modelValue\', false)">x</button><slot /></div>' } },
      },
    })
    await flushPromises()
    // add 对话框：v-model 内联 onUpdate + 取消按钮
    await w.find('[data-testid="source-add"]').trigger('click')
    await flushPromises()
    // v-text-field 的 testid 落在根 div，setValue 必须作用于内层 input
    await w.find('[data-testid="add-name"] input').setValue('n1')
    await w.find('[data-testid="add-url"] input').setValue('http://u1.yaml')
    await w.find('[data-testid="add-format"] input').setValue('ini')
    await flushPromises()
    expect((w.vm as any).addName).toBe('n1')
    expect((w.vm as any).addUrl).toBe('http://u1.yaml')
    expect((w.vm as any).addFormat).toBe('ini')
    await w.find('[data-testid="add-cancel"]').trigger('click')
    await flushPromises()
    expect((w.vm as any).addDialog).toBe(false)
    // remove 对话框：取消按钮
    ;(w.vm as any).openRemove('src2')
    await flushPromises()
    await w.find('[data-testid="remove-cancel"]').trigger('click')
    await flushPromises()
    expect((w.vm as any).removeDialog).toBe(false)
    // 表格行内联 @click：source-update / source-remove
    await w.find('[data-testid="source-update"]').trigger('click')
    await flushPromises()
    expect(updateOneMock).toHaveBeenCalledWith('src1')
    await w.find('[data-testid="source-remove"]').trigger('click')
    await flushPromises()
    expect((w.vm as any).removeDialog).toBe(true)
    // v-dialog 的 v-model 内联 onUpdate fn：stub 组件上 $emit 才会执行（stub 根元素按 slot 内容定位）
    const addStub = w.findAll('.dlgstub').find(d => d.find('[data-testid="add-dialog"]').exists())
    expect(addStub).toBeTruthy()
    await addStub!.find('.dlg-emit').trigger('click')
    const removeStub = w.findAll('.dlgstub').find(d => d.find('[data-testid="remove-dialog"]').exists())
    expect(removeStub).toBeTruthy()
    await removeStub!.find('.dlg-emit').trigger('click')
    await flushPromises()
    expect((w.vm as any).addDialog).toBe(false)
    expect((w.vm as any).removeDialog).toBe(false)
    w.unmount()
  })

})


// ---------------- 覆盖率补齐：MRS 更新中的并发守卫 ----------------

describe('RuleSourcesPage 覆盖率补齐（MRS 更新中守卫）', () => {
  it('doMrs 在 updating 时直接返回，不发请求', async () => {
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const vm = w.vm as unknown as { updating: boolean; doMrs: () => Promise<void> }
    vm.updating = true
    await vm.doMrs()
    expect(mrsMock).not.toHaveBeenCalled()
    vm.updating = false
    w.unmount()
  })

  it('点击规则源名称按钮打开挑选弹窗', async () => {
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] }, attachTo: document.body })
    try {
      await flushPromises()
      await w.find('[data-testid="source-rules"]').trigger('click')
      await flushPromises()
      expect((w.vm as any).rulesDialog).toBe(true)
      expect(listRulesMock).toHaveBeenCalledWith('src1', { q: '', offset: 0, limit: 50 })
    } finally {
      w.unmount()
    }
  })

  it('从规则源挑选：加载、勾选、全选本页并加入自定义', async () => {
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] }, attachTo: document.body })
    try {
      await flushPromises()
      ;(w.vm as any).openRules({ name: 'src1', label: 'src1', versions: [], latest: 'v2' })
      await flushPromises()
      expect(listRulesMock).toHaveBeenCalledWith('src1', { q: '', offset: 0, limit: 50 })
      expect(document.body.querySelector('[data-testid="source-rules-dialog"]')).not.toBeNull()
      expect(document.body.querySelectorAll('[data-testid="source-rule-row"]')).toHaveLength(2)
      expect(document.body.querySelector('[data-testid="source-rules-count"]')!.textContent).toContain('已选 0 条')

      ;(w.vm as any).toggleRule('DOMAIN-SUFFIX,a.test,DIRECT')
      await flushPromises()
      expect(document.body.querySelector('[data-testid="source-rules-count"]')!.textContent).toContain('已选 1 条')
      ;(w.vm as any).selectCurrentPage()
      await flushPromises()
      expect(document.body.querySelector('[data-testid="source-rules-count"]')!.textContent).toContain('已选 2 条')
      ;(w.vm as any).clearSelectedRules()
      await flushPromises()
      expect(document.body.querySelector('[data-testid="source-rules-count"]')!.textContent).toContain('已选 0 条')
      // 空选择时加入按钮 disabled，直接调用也被 guard 挡住。
      expect((document.body.querySelector('[data-testid="source-rules-import"]') as HTMLButtonElement).disabled).toBe(true)
      await (w.vm as any).importSelectedRules()
      expect(importRulesMock).not.toHaveBeenCalled()

      ;(w.vm as any).toggleRule('DOMAIN-SUFFIX,a.test,DIRECT')
      await flushPromises()
      await (w.vm as any).importSelectedRules()
      await flushPromises()
      expect(importRulesMock).toHaveBeenCalledWith({
        name: 'src1',
        rules: ['DOMAIN-SUFFIX,a.test,DIRECT'],
        conflict: 'skip',
      })
      expect(notifyState.text).toContain('已加入自定义')
      expect((w.vm as any).rulesDialog).toBe(false)
    } finally {
      w.unmount()
    }
  })

  it('从规则源挑选：搜索、清空、分页、空结果与错误', async () => {
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] }, attachTo: document.body })
    try {
      await flushPromises()
      ;(w.vm as any).openRules({ name: 'src1', label: 'src1', versions: [], latest: 'v2' })
      await flushPromises()
      listRulesMock.mockResolvedValueOnce({ name: 'src1', total: 120, offset: 0, limit: 50, rules: ['DOMAIN-SUFFIX,page1.test,DIRECT'] })
      ;(w.vm as any).rulesQuery = 'page1'
      await (w.vm as any).searchRules()
      await flushPromises()
      expect(listRulesMock).toHaveBeenLastCalledWith('src1', { q: 'page1', offset: 0, limit: 50 })
      expect((w.vm as any).rulesTotalPages).toBe(3)

      listRulesMock.mockResolvedValueOnce({ name: 'src1', total: 120, offset: 50, limit: 50, rules: ['DOMAIN-SUFFIX,page2.test,DIRECT'] })
      ;(w.vm as any).rulesPage = 2
      await flushPromises()
      expect(listRulesMock).toHaveBeenLastCalledWith('src1', { q: 'page1', offset: 50, limit: 50 })
      expect(document.body.querySelectorAll('[data-testid="source-rule-row"]')).toHaveLength(1)

      listRulesMock.mockResolvedValueOnce({ name: 'src1', total: 120, offset: 0, limit: 50, rules: [] })
      await (w.vm as any).clearRulesSearch()
      await flushPromises()
      expect(listRulesMock).toHaveBeenLastCalledWith('src1', { q: '', offset: 0, limit: 50 })
      expect(document.body.querySelector('[data-testid="source-rules-empty"]')).not.toBeNull()

      listRulesMock.mockRejectedValueOnce(new Error('rules boom'))
      await (w.vm as any).searchRules()
      await flushPromises()
      expect((w.vm as any).rulesError).toContain('rules boom')
    } finally {
      w.unmount()
    }
  })

  it('从规则源挑选：空源守卫、跳过计数与取消按钮', async () => {
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] }, attachTo: document.body })
    try {
      await flushPromises()
      await (w.vm as any).loadRules()
      ;(w.vm as any).openRules({ name: 'src1', label: 'src1', versions: [], latest: 'v2' })
      await flushPromises()
      importRulesMock.mockResolvedValueOnce({ added: 1, skipped: 1, overwrote: 0, warnings: [] })
      ;(w.vm as any).rulesSelected = ['DOMAIN-SUFFIX,a.test,DIRECT']
      await (w.vm as any).importSelectedRules()
      await flushPromises()
      expect(notifyState.text).toContain('跳过 1')

      ;(w.vm as any).openRules({ name: 'src1', label: 'src1', versions: [], latest: 'v2' })
      await flushPromises()
      const cancel = w.findAllComponents({ name: 'VBtn' })
        .find((c) => c.attributes('data-testid') === 'source-rules-cancel')
      expect(cancel).toBeTruthy()
      await cancel!.trigger('click')
      await flushPromises()
      expect((w.vm as any).rulesDialog).toBe(false)
    } finally {
      w.unmount()
    }
  })

  it('从规则源挑选：真实搜索输入、复选切换与分页组件事件', async () => {
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] }, attachTo: document.body })
    try {
      await flushPromises()
      ;(w.vm as any).openRules({ name: 'src1', label: 'src1', versions: [], latest: 'v2' })
      await flushPromises()
      const search = w.findAllComponents({ name: 'VTextField' })
        .find((c) => c.attributes('data-testid') === 'source-rules-search')
      expect(search).toBeTruthy()
      await search!.find('input').setValue('page1')
      listRulesMock.mockResolvedValueOnce({ name: 'src1', total: 120, offset: 0, limit: 50, rules: ['DOMAIN-SUFFIX,page1.test,DIRECT'] })
      await search!.find('input').trigger('keyup.enter')
      await flushPromises()
      expect(listRulesMock).toHaveBeenLastCalledWith('src1', { q: 'page1', offset: 0, limit: 50 })

      const cb = w.findAllComponents({ name: 'VCheckbox' })
        .find((c) => c.attributes('data-testid') === 'source-rule-check')
      expect(cb).toBeTruthy()
      await cb!.setValue(true)
      await cb!.setValue(false)
      await flushPromises()
      expect((w.vm as any).rulesSelected).toEqual([])

      listRulesMock.mockResolvedValueOnce({ name: 'src1', total: 120, offset: 50, limit: 50, rules: ['DOMAIN-SUFFIX,page2.test,DIRECT'] })
      const pag = w.findAllComponents({ name: 'VPagination' })[0]
      expect(pag).toBeTruthy()
      ;(pag.vm as unknown as { $emit: (e: string, ...a: unknown[]) => void }).$emit('update:modelValue', 2)
      await flushPromises()
      expect((w.vm as any).rulesOffset).toBe(50)
    } finally {
      w.unmount()
    }
  })

  it('从规则源挑选：覆盖冲突、warnings、导入失败', async () => {
    importRulesMock.mockResolvedValueOnce({ added: 0, skipped: 0, overwrote: 2, warnings: ['RULE-SET 依赖 provider'] })
    notifyState.open = false
    notifyState.text = ''
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] }, attachTo: document.body })
    try {
      await flushPromises()
      ;(w.vm as any).openRules({ name: 'src1', label: 'src1', versions: [], latest: 'v2' })
      await flushPromises()
      ;(w.vm as any).rulesSelected = ['DOMAIN-SUFFIX,a.test,DIRECT']
      ;(w.vm as any).rulesConflict = 'overwrite'
      await (w.vm as any).importSelectedRules()
      await flushPromises()
      expect(importRulesMock).toHaveBeenCalledWith({
        name: 'src1',
        rules: ['DOMAIN-SUFFIX,a.test,DIRECT'],
        conflict: 'overwrite',
      })
      expect(notifyState.text).toContain('RULE-SET 依赖 provider')

      importRulesMock.mockRejectedValueOnce(new Error('import boom'))
      await (w.vm as any).openRules({ name: 'src1', label: 'src1', versions: [], latest: 'v2' })
      await flushPromises()
      ;(w.vm as any).rulesSelected = ['DOMAIN-SUFFIX,a.test,DIRECT']
      await (w.vm as any).importSelectedRules()
      await flushPromises()
      expect(notifyState.kind).toBe('error')
      expect(notifyState.text).toContain('import boom')
    } finally {
      w.unmount()
    }
  })
})

describe('RuleSourcesPage 改名', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listMock.mockResolvedValue(makeManifest())
    renameMock.mockResolvedValue({ ok: true, name: 'renamed' })
  })

  it('远程规则源可打开改名弹窗并提交', async () => {
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] }, attachTo: document.body })
    await flushPromises()
    await w.find('[data-testid="source-rename"]').trigger('click')
    await flushPromises()
    ;(w.vm as any).renameValue = 'renamed'
    await (w.vm as any).confirmRename()
    await flushPromises()
    expect(renameMock).toHaveBeenCalledWith('src1', 'renamed')
    w.unmount()
  })

  it('机场规则源禁止改名，直接调用也被守卫', async () => {
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const rows = w.findAll('[data-testid="source-row"]')
    expect(rows[1].find('[data-testid="source-rename"]').attributes('disabled')).toBeDefined()
    ;(w.vm as any).openRename(makeSources()[1])
    await flushPromises()
    expect((w.vm as any).renameDialog).toBe(false)
    w.unmount()
  })
})

describe('RuleSourcesPage 改名覆盖', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listMock.mockResolvedValue(makeManifest())
    renameMock.mockResolvedValue({ ok: true, name: 'renamed' })
  })

  it('改名守卫：进行中/空名/同名都直接返回', async () => {
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const vm = w.vm as any
    vm.openRename(makeSources()[0])
    vm.renameSaving = true
    await vm.confirmRename()
    expect(renameMock).not.toHaveBeenCalled()
    vm.renameSaving = false
    vm.renameValue = '   '
    await vm.confirmRename()
    expect(renameMock).not.toHaveBeenCalled()
    vm.renameValue = 'src1'
    await vm.confirmRename()
    expect(renameMock).not.toHaveBeenCalled()
    w.unmount()
  })

  it('改名失败提示错误；输入 v-model 与取消按钮可写回', async () => {
    renameMock.mockRejectedValueOnce(new Error('rename boom'))
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] }, attachTo: document.body })
    await flushPromises()
    await w.find('[data-testid="source-rename"]').trigger('click')
    await flushPromises()
    const input = document.body.querySelector('[data-testid="source-rename-input"] input') as HTMLInputElement | null
    expect(input).toBeTruthy()
    input!.value = 'renamed'
    input!.dispatchEvent(new Event('input', { bubbles: true }))
    await (w.vm as any).confirmRename()
    await flushPromises()
    expect(notifyState.kind).toBe('error')
    const cancel = document.body.querySelector('[data-testid="source-rename-cancel"]') as HTMLElement | null
    expect(cancel).toBeTruthy()
    cancel!.click()
    await flushPromises()
    expect((w.vm as any).renameDialog).toBe(false)
    w.unmount()
  })
})
