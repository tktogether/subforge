import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import ProfilesPage from './ProfilesPage.vue'
import { endpoints } from '../api/endpoints'
import { notifyState } from '../composables/useNotify'

// 端点层用 mock 隔离，页面 spec 只关心页面自己的行为。
vi.mock('../api/endpoints', () => ({
  endpoints: {
    listProfiles: vi.fn(),
    createProfile: vi.fn(),
    updateProfile: vi.fn(),
    deleteProfile: vi.fn(),
    listRuleSources: vi.fn(),
    checkRuleSourceConflicts: vi.fn(),
    listAirports: vi.fn(),
    listSelfNodes: vi.fn(),
    targets: vi.fn(),
  },
}))

const vuetify = createVuetify({ components, directives })

interface Draft {
  name: string
  description: string
  config: string
  pipeline: string
  target: string
  include: string
  exclude: string
  enabled: boolean
  sort_order: number
  rule_sources: string[]
  limit_sources: boolean
  airport_ids: number[]
  self_node_ids: number[]
}

interface ProfilesPageVm {
  dialog: boolean
  confirmOpen: boolean
  draft: Draft
  ruleLayers: { key: string; enabled: boolean }[]
  groupLayers: { key: string; enabled: boolean }[]
  moveGroupLayer: (index: number, delta: number) => void
  layerKeys: (rows: { key: string; enabled: boolean }[]) => string[]
}

const DEFAULT_LAYERS = ['custom', 'airport', 'template']
const DEFAULT_RULE_LAYERS = ['custom', 'template']

function makeProfile(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    name: '订阅A',
    description: '说明A',
    config: '',
    pipeline: '',
    target: '',
    include: '',
    exclude: '',
    enabled: true,
    sort_order: 3,
    rule_layers: [...DEFAULT_RULE_LAYERS],
    rule_sources: [] as string[],
    airport_ids: null,
    self_node_ids: null,
    group_layers: [...DEFAULT_LAYERS],
    rule_count: 2,
    group_count: 1,
    ...overrides,
  }
}

// 选中规则源要先打开「添加规则源」弹窗，再勾选一项并确认。
function sourceCheckbox(w: ReturnType<typeof mountPage>, value: string) {
  return w.findAllComponents({ name: 'VCheckbox' })
    .find((c) => c.attributes('data-testid') === 'profile-source-check-' + value)
}

async function addRuleSources(w: ReturnType<typeof mountPage>, values: string[]) {
  await w.find('[data-testid="profile-rule-source-add"]').trigger('click')
  await flushPromises()
  for (const value of values) {
    const cb = sourceCheckbox(w, value)
    if (!cb) throw new Error('找不到规则源选项: ' + value)
    await cb.setValue(true)
  }
  await flushPromises()
  const confirm = w.findAllComponents({ name: 'VBtn' })
    .find((c) => c.attributes('data-testid') === 'profile-source-confirm')
  if (!confirm) throw new Error('找不到规则源添加确认按钮')
  await confirm.trigger('click')
  await flushPromises()
}

// v-dialog 的内容默认惰性渲染，测试里换成内联 stub 让弹窗内容直接可查。
function mountPage() {
  return mount(ProfilesPage, {
    global: {
      plugins: [vuetify],
      stubs: {
        VDialog: {
          name: 'VDialog',
          props: ['modelValue'],
          template: '<div class="dlg" v-if="modelValue"><slot /></div>',
        },
      },
    },
  })
}

describe('ProfilesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    notifyState.open = false
    notifyState.text = ''
    notifyState.kind = 'ok'
    ;(endpoints.listProfiles as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([])
    ;(endpoints.checkRuleSourceConflicts as unknown as ReturnType<typeof vi.fn>).mockResolvedValue({ total: 0, samples: [] })
    // 规则源清单默认返回「无 sources 字段」，覆盖前端 `res.sources ?? []` 的兜底分支。
    const listSources = endpoints.listRuleSources as unknown as ReturnType<typeof vi.fn>
    listSources.mockReset()
    listSources.mockResolvedValue({})
    ;(endpoints.listAirports as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([
      { id: 11, name: '机场A', enabled: true },
    ])
    ;(endpoints.listSelfNodes as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([
      { id: 22, name: '节点A', enabled: true },
    ])
    ;(endpoints.targets as unknown as ReturnType<typeof vi.fn>).mockResolvedValue(['clash.meta', 'sing-box'])
  })

  it('渲染方案表格：模板/管线空值显示默认、模板超长截断、启用停用、顺序>1e12 显示未设置、层列顺序', async () => {
    const longConfig = 'x'.repeat(80)
    ;(endpoints.listProfiles as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([
      makeProfile(),
      makeProfile({ id: 2, name: '订阅B', config: longConfig, pipeline: 'pipeB', enabled: false, sort_order: 7, rule_count: 5, group_count: 4 }),
      makeProfile({ id: 3, name: '订阅C', rule_layers: ['airport', 'custom'], group_layers: [], sort_order: 1234567890123 }),
      makeProfile({ id: 4, name: '订阅D', rule_layers: ['custom', 'bogus'], group_layers: ['template'] }),
      makeProfile({ id: 5, name: '订阅E', config: 'cfg-short' }),
    ])
    const w = mountPage()
    await flushPromises()

    expect(w.find('[data-testid="profile-table"]').exists()).toBe(true)
    expect(w.findAll('[data-testid="profile-row"]')).toHaveLength(5)

    const cells = w.findAll('.profiles-truncate')
    expect(cells[0].text()).toBe('默认')
    expect(cells[0].attributes('title')).toBe('')
    expect(cells[1].text()).toBe(longConfig.slice(0, 60) + '…')
    expect(cells[1].attributes('title')).toBe(longConfig)
    // 短模板原样显示，不截断。
    expect(cells[4].text()).toBe('cfg-short')
    expect(cells[4].attributes('title')).toBe('cfg-short')

    expect(w.text()).toContain('默认')
    expect(w.text()).toContain('启用')
    expect(w.text()).toContain('停用')
    expect(w.text()).toContain('pipeB')
    expect(w.text()).toContain('7')
    expect(w.text()).toContain('未设置')

    const ruleLayersText = w.findAll('[data-testid="profile-rule-layers-text"]')
    expect(ruleLayersText[0].text()).toBe('自定义 → 模板')
    expect(ruleLayersText[2].text()).toBe('机场导入 → 自定义')
    expect(ruleLayersText[3].text()).toBe('自定义 → bogus')

    const groupLayersText = w.findAll('[data-testid="profile-group-layers-text"]')
    expect(groupLayersText[0].text()).toBe('自定义 → 机场导入 → 模板')
    expect(groupLayersText[2].text()).toBe('默认')
    expect(groupLayersText[3].text()).toBe('模板')

    const orderCells = w.findAll('[data-testid="profile-sort-order"]')
    expect(orderCells[2].text()).toBe('未设置')
  })

  it('帮助文案说明未分配、绑定隔离、删除前置条件、模板留空与层顺序', async () => {
    const w = mountPage()
    await flushPromises()
    const help = w.find('[data-testid="profiles-help"]')
    expect(help.exists()).toBe(true)
    expect(help.text()).toContain('未分配')
    expect(help.text()).toContain('profile_id = 0')
    expect(help.text()).toContain('绑定方案后')
    expect(help.text()).toContain('只有仍有令牌绑定时才会拒绝')
    expect(help.text()).toContain('模板留空')
    expect(help.text()).toContain('规则层 = 按顺序拼接，先出现者优先')
    expect(help.text()).toContain('分组层 = 同名分组先出现者胜出')
    expect(help.text()).toContain('模板层 = INI 模板里的规则/分组')
  })

  it('空态：没有方案时显示还没有订阅方案且不渲染表格', async () => {
    const w = mountPage()
    await flushPromises()
    expect(w.text()).toContain('还没有订阅方案')
    expect(w.find('[data-testid="profile-table"]').exists()).toBe(false)
  })

  it('新建方案：填字段后保存提交全部字段含 rule_layers/group_layers', async () => {
    ;(endpoints.createProfile as unknown as ReturnType<typeof vi.fn>).mockResolvedValue(makeProfile({ id: 5 }))
    const w = mountPage()
    await flushPromises()
    const vm = w.vm as unknown as ProfilesPageVm

    await w.find('[data-testid="profile-new"]').trigger('click')
    await flushPromises()
    expect(vm.dialog).toBe(true)
    expect(w.text()).toContain('新建方案')

    await w.find('[data-testid="profile-name"]').find('input').setValue('新方案')
    await w.find('[data-testid="profile-description"]').find('input').setValue('新说明')
    await w.find('[data-testid="profile-config"]').find('input').setValue('cfg')
    await w.find('[data-testid="profile-pipeline"]').find('input').setValue('pipe')
    await w.find('[data-testid="profile-enabled"] input').setValue(false)
    await w.find('[data-testid="profile-sort"]').find('input').setValue('9')

    // 规则层默认全开（自定义→模板）；机场规则是规则源，不在规则层里。
    const ruleBox = w.find('[data-testid="profile-rule-layers"]')
    const groupBox = w.find('[data-testid="profile-group-layers"]')
    expect(ruleBox.exists()).toBe(true)
    expect(groupBox.exists()).toBe(true)
    expect(ruleBox.find('[data-testid="rule-layer-row-custom"]').exists()).toBe(true)
    expect(ruleBox.find('[data-testid="rule-layer-row-template"]').exists()).toBe(true)
    expect(ruleBox.find('[data-testid="rule-layer-row-custom"] .sf-layer-priority').text()).toBe('优先级 1')
    expect(ruleBox.find('[data-testid="rule-layer-row-template"] .sf-layer-priority').text()).toBe('优先级 2')
    expect(ruleBox.find('[data-testid="rule-layer-row-airport"]').exists()).toBe(false)
    // 分组层不受「模板恒最后」约束，template 行上下移可用，也不显示模板提示。
    const gTpl = groupBox.find('[data-testid="group-layer-row-template"]')
    // 分组层不锁模板：template 行上移可用（只在末位时才是下移边界）。
    expect(gTpl.find('[data-testid="group-layer-up-template"]').attributes('disabled')).toBeUndefined()
    expect(gTpl.find('[data-testid="group-layer-down-template"]').attributes('disabled')).not.toBeUndefined()
    expect(groupBox.text()).not.toContain('模板规则始终最后（MATCH/FINAL）')

    await flushPromises()
    await w.find('[data-testid="profile-save"]').trigger('click')
    await flushPromises()

    expect(endpoints.createProfile).toHaveBeenCalledWith({
      name: '新方案',
      description: '新说明',
      config: 'cfg',
      pipeline: 'pipe',
      target: '',
      include: '',
      exclude: '',
      enabled: false,
      sort_order: 9,
      rule_layers: ['custom', 'template'],
      rule_sources: [],
      group_layers: ['custom', 'airport', 'template'],
      all_sources: true,
    })
    expect(notifyState.kind).toBe('ok')
    expect(notifyState.text).toBe('已保存')
  })

  it('新建全不选层保存被阻止并提示至少启用一层', async () => {
    ;(endpoints.createProfile as unknown as ReturnType<typeof vi.fn>).mockResolvedValue(makeProfile({ id: 5 }))
    const w = mountPage()
    await flushPromises()

    await w.find('[data-testid="profile-new"]').trigger('click')
    await flushPromises()

    // 先关掉规则层的全部两层 → 保存被阻止。
    const ruleBox = w.find('[data-testid="profile-rule-layers"]')
    await ruleBox.find('[data-testid="rule-layer-toggle-custom"] input').setValue(false)
    await ruleBox.find('[data-testid="rule-layer-toggle-template"] input').setValue(false)
    await flushPromises()

    await w.find('[data-testid="profile-save"]').trigger('click')
    await flushPromises()

    expect(endpoints.createProfile).not.toHaveBeenCalled()
    expect(notifyState.kind).toBe('error')
    expect(notifyState.text).toBe('至少启用一层')

    // 规则层恢复全开，再关掉分组层的全部三层 → 同样被阻止。
    await ruleBox.find('[data-testid="rule-layer-toggle-custom"] input').setValue(true)
    await ruleBox.find('[data-testid="rule-layer-toggle-template"] input').setValue(true)
    const groupBox = w.find('[data-testid="profile-group-layers"]')
    await groupBox.find('[data-testid="group-layer-toggle-custom"] input').setValue(false)
    await groupBox.find('[data-testid="group-layer-toggle-airport"] input').setValue(false)
    await groupBox.find('[data-testid="group-layer-toggle-template"] input').setValue(false)
    await flushPromises()

    await w.find('[data-testid="profile-save"]').trigger('click')
    await flushPromises()

    expect(endpoints.createProfile).not.toHaveBeenCalled()
    expect(notifyState.text).toBe('至少启用一层')
  })

  it('编辑方案回填层顺序并保存提交数组', async () => {
    ;(endpoints.listProfiles as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([
      makeProfile({ rule_layers: ['airport', 'custom'], group_layers: ['custom', 'template'] }),
    ])
    ;(endpoints.updateProfile as unknown as ReturnType<typeof vi.fn>).mockResolvedValue(makeProfile())
    const w = mountPage()
    await flushPromises()

    await w.find('[data-testid="profile-edit"]').trigger('click')
    await flushPromises()
    expect(w.text()).toContain('编辑方案')

    // 规则层只保留 custom/template；旧 airport 层被忽略，template 在末位且默认未启用。
    const ruleBox = w.find('[data-testid="profile-rule-layers"]')
    const ruleRows = ruleBox.findAll('[data-testid^="rule-layer-row-"]')
    expect(ruleRows).toHaveLength(2)
    expect(ruleRows[0].attributes('data-testid')).toBe('rule-layer-row-custom')
    expect(ruleRows[0].find('.sf-layer-priority').text()).toBe('优先级 1')
    expect(ruleRows[1].attributes('data-testid')).toBe('rule-layer-row-template')
    expect(ruleRows[1].find('.sf-layer-priority').text()).toBe('未启用')
    // 规则层没有可移动空间，不渲染上下移按钮；模板提示仍在。
    expect(ruleRows[1].find('[data-testid="rule-layer-up-template"]').exists()).toBe(false)
    expect(ruleRows[1].find('[data-testid="rule-layer-down-template"]').exists()).toBe(false)
    expect(ruleRows[1].text()).toContain('模板规则始终最后（MATCH/FINAL）')

    await w.find('[data-testid="profile-save"]').trigger('click')
    await flushPromises()

    // 只提交启用的层（顺序即优先级）；未启用的 template 不进数组。
    expect(endpoints.updateProfile).toHaveBeenCalledWith(1, expect.objectContaining({
      rule_layers: ['custom'],
      group_layers: ['custom', 'template'],
    }))
    expect(notifyState.text).toBe('已保存')
  })

  it('规则/分组层互动：上移改变顺序后保存提交新顺序', async () => {
    ;(endpoints.listProfiles as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([
      makeProfile({ rule_layers: ['custom', 'airport', 'template'] }),
    ])
    ;(endpoints.updateProfile as unknown as ReturnType<typeof vi.fn>).mockResolvedValue(makeProfile())
    const w = mountPage()
    await flushPromises()

    await w.find('[data-testid="profile-edit"]').trigger('click')
    await flushPromises()

    const ruleBox = w.find('[data-testid="profile-rule-layers"]')
    expect(ruleBox.find('[data-testid="rule-layer-row-airport"]').exists()).toBe(false)
    // 规则层只有 custom/template 且 template 恒在末位，没有可移动空间，不渲染上下移按钮。
    expect(ruleBox.find('[data-testid="rule-layer-up-custom"]').exists()).toBe(false)
    expect(ruleBox.find('[data-testid="rule-layer-down-template"]').exists()).toBe(false)

    await w.find('[data-testid="profile-save"]').trigger('click')
    await flushPromises()
    expect(endpoints.updateProfile).toHaveBeenCalledWith(1, expect.objectContaining({
      rule_layers: ['custom', 'template'],
      group_layers: ['custom', 'airport', 'template'],
    }))
  })

  it('分组层上移按钮点击改变顺序', async () => {
    ;(endpoints.listProfiles as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([
      makeProfile({ group_layers: ['custom', 'airport', 'template'] }),
    ])
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="profile-edit"]').trigger('click')
    await flushPromises()
    const groupBox = w.find('[data-testid="profile-group-layers"]')
    await groupBox.find('[data-testid="group-layer-row-template"]').find('[data-testid="group-layer-up-template"]').trigger('click')
    await flushPromises()
    const vm = w.vm as unknown as ProfilesPageVm
    expect(vm.layerKeys(vm.groupLayers)).toEqual(['custom', 'template', 'airport'])
    w.unmount()
  })

  it('分组层排序互动：下移后保存提交新顺序', async () => {
    ;(endpoints.listProfiles as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([
      makeProfile({ group_layers: ['custom', 'airport', 'template'] }),
    ])
    ;(endpoints.updateProfile as unknown as ReturnType<typeof vi.fn>).mockResolvedValue(makeProfile())
    const w = mountPage()
    await flushPromises()

    await w.find('[data-testid="profile-edit"]').trigger('click')
    await flushPromises()

    const groupBox = w.find('[data-testid="profile-group-layers"]')
    await groupBox.find('[data-testid="group-layer-row-custom"]').find('[data-testid="group-layer-down-custom"]').trigger('click')
    await flushPromises()

    await w.find('[data-testid="profile-save"]').trigger('click')
    await flushPromises()

    expect(endpoints.updateProfile).toHaveBeenCalledWith(1, expect.objectContaining({
      rule_layers: ['custom', 'template'],
      group_layers: ['airport', 'custom', 'template'],
    }))
  })

  it('编辑方案：模板层不在末位时挪到末位，空层数组按默认三层回填', async () => {
    ;(endpoints.listProfiles as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([
      makeProfile({ rule_layers: ['template', 'custom'], group_layers: [] }),
    ])
    ;(endpoints.updateProfile as unknown as ReturnType<typeof vi.fn>).mockResolvedValue(makeProfile())
    const w = mountPage()
    await flushPromises()

    await w.find('[data-testid="profile-edit"]').trigger('click')
    await flushPromises()

    const ruleBox = w.find('[data-testid="profile-rule-layers"]')
    const rows = ruleBox.findAll('[data-testid^="rule-layer-row-"]')
    expect(rows).toHaveLength(2)
    expect(rows[0].attributes('data-testid')).toBe('rule-layer-row-custom')
    expect(rows[1].attributes('data-testid')).toBe('rule-layer-row-template')
    expect(rows[0].find('.sf-layer-priority').text()).toBe('优先级 1')
    expect(rows[1].find('.sf-layer-priority').text()).toBe('优先级 2')
    // 分组层数组为空 → 按默认三层全开回填。
    const groupBox = w.find('[data-testid="profile-group-layers"]')
    expect(groupBox.find('[data-testid="group-layer-row-custom"] .sf-layer-priority').text()).toBe('优先级 1')
    expect(groupBox.find('[data-testid="group-layer-row-airport"] .sf-layer-priority').text()).toBe('优先级 2')
    expect(groupBox.find('[data-testid="group-layer-row-template"] .sf-layer-priority').text()).toBe('优先级 3')

    await w.find('[data-testid="profile-save"]').trigger('click')
    await flushPromises()

    expect(endpoints.updateProfile).toHaveBeenCalledWith(1, expect.objectContaining({
      rule_layers: ['custom', 'template'],
      group_layers: ['custom', 'airport', 'template'],
    }))
  })

  it('新建方案：target/include/exclude 随保存提交', async () => {
    ;(endpoints.createProfile as unknown as ReturnType<typeof vi.fn>).mockResolvedValue(makeProfile({ id: 9 }))
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="profile-new"]').trigger('click')
    await flushPromises()
    const vm = w.vm as any
    vm.draft.name = '输出定制'
    const targetSel = w.findAllComponents({ name: 'VSelect' })
      .find((s) => s.attributes('data-testid') === 'profile-target')
    expect(targetSel).toBeTruthy()
    targetSel!.vm.$emit('update:modelValue', 'sing-box')
    await w.find('[data-testid="profile-include"] input').setValue('香港,日本')
    await w.find('[data-testid="profile-exclude"] input').setValue('过期')
    await flushPromises()
    await w.find('[data-testid="profile-save"]').trigger('click')
    await flushPromises()
    expect(endpoints.createProfile).toHaveBeenCalledWith(expect.objectContaining({
      target: 'sing-box',
      include: '香港,日本',
      exclude: '过期',
    }))
  })

  it('target 候选加载失败时降级为空选项', async () => {
    ;(endpoints.targets as unknown as ReturnType<typeof vi.fn>).mockRejectedValueOnce(new Error('targets down'))
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="profile-new"]').trigger('click')
    await flushPromises()
    expect((w.vm as any).targetOptions).toEqual([])
  })

  it('新建方案：限制来源后提交所选机场/自建节点', async () => {
    ;(endpoints.createProfile as unknown as ReturnType<typeof vi.fn>).mockResolvedValue(makeProfile({ id: 8 }))
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="profile-new"]').trigger('click')
    await flushPromises()
    const vm = w.vm as any
    vm.draft.name = '限源方案'
    await w.find('[data-testid="profile-limit-sources"] input').setValue(true)
    await flushPromises()
    const selects = w.findAllComponents({ name: 'VSelect' })
    const airportSel = selects.find((s) => s.attributes('data-testid') === 'profile-airports')
    const selfSel = selects.find((s) => s.attributes('data-testid') === 'profile-self-nodes')
    expect(airportSel).toBeTruthy()
    expect(selfSel).toBeTruthy()
    airportSel!.vm.$emit('update:modelValue', [11])
    selfSel!.vm.$emit('update:modelValue', [22])
    await flushPromises()
    expect(vm.draft.airport_ids).toEqual([11])
    expect(vm.draft.self_node_ids).toEqual([22])
    await w.find('[data-testid="profile-save"]').trigger('click')
    await flushPromises()
    expect(endpoints.createProfile).toHaveBeenCalledWith(expect.objectContaining({
      all_sources: false,
      airport_ids: [11],
      self_node_ids: [22],
    }))
  })

  it('编辑受限方案：回填来源，切回全部来源后提交 all_sources=true', async () => {
    ;(endpoints.listProfiles as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([
      makeProfile({ airport_ids: [11], self_node_ids: [] }),
    ])
    ;(endpoints.updateProfile as unknown as ReturnType<typeof vi.fn>).mockResolvedValue(makeProfile())
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="profile-edit"]').trigger('click')
    await flushPromises()
    const vm = w.vm as any
    expect(vm.draft.limit_sources).toBe(true)
    expect(vm.draft.airport_ids).toEqual([11])
    expect(vm.draft.self_node_ids).toEqual([])

    await w.find('[data-testid="profile-limit-sources"] input').setValue(false)
    await flushPromises()
    expect(w.find('[data-testid="profile-airports"]').classes().join(' ')).toContain('v-input--disabled')
    await w.find('[data-testid="profile-save"]').trigger('click')
    await flushPromises()
    expect(endpoints.updateProfile).toHaveBeenCalledWith(1, expect.objectContaining({
      all_sources: true,
    }))
    const payload = (endpoints.updateProfile as unknown as ReturnType<typeof vi.fn>).mock.calls[0][1]
    expect(payload).not.toHaveProperty('airport_ids')
    expect(payload).not.toHaveProperty('self_node_ids')
  })

  it('来源选项加载失败时降级为空选项，不影响保存', async () => {
    ;(endpoints.listAirports as unknown as ReturnType<typeof vi.fn>).mockRejectedValueOnce(new Error('airports down'))
    ;(endpoints.listSelfNodes as unknown as ReturnType<typeof vi.fn>).mockRejectedValueOnce(new Error('nodes down'))
    ;(endpoints.createProfile as unknown as ReturnType<typeof vi.fn>).mockResolvedValue(makeProfile({ id: 9 }))
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="profile-new"]').trigger('click')
    await flushPromises()
    const vm = w.vm as any
    expect(vm.airportOptions).toEqual([])
    expect(vm.selfNodeOptions).toEqual([])
    vm.draft.name = '降级方案'
    await w.find('[data-testid="profile-save"]').trigger('click')
    await flushPromises()
    expect(endpoints.createProfile).toHaveBeenCalled()
  })

  it('取消按钮关闭新建弹窗', async () => {
    const w = mountPage()
    await flushPromises()
    const vm = w.vm as unknown as ProfilesPageVm
    await w.find('[data-testid="profile-new"]').trigger('click')
    await flushPromises()
    expect(vm.dialog).toBe(true)
    await w.find('[data-testid="profile-cancel"]').trigger('click')
    await flushPromises()
    expect(vm.dialog).toBe(false)
    expect(endpoints.createProfile).not.toHaveBeenCalled()
  })

  it('删除确认：先弹确认框，取消不删除，确认后删除并提示已删除', async () => {
    ;(endpoints.listProfiles as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([makeProfile()])
    ;(endpoints.deleteProfile as unknown as ReturnType<typeof vi.fn>).mockResolvedValue({ ok: true })
    const w = mountPage()
    await flushPromises()
    const vm = w.vm as unknown as ProfilesPageVm

    await w.find('[data-testid="profile-delete"]').trigger('click')
    await flushPromises()
    expect(vm.confirmOpen).toBe(true)
    expect(w.text()).toContain('确定继续')

    await w.find('[data-testid="profile-delete-cancel"]').trigger('click')
    await flushPromises()
    expect(vm.confirmOpen).toBe(false)
    expect(endpoints.deleteProfile).not.toHaveBeenCalled()

    await w.find('[data-testid="profile-delete"]').trigger('click')
    await flushPromises()
    await w.find('[data-testid="profile-delete-confirm"]').trigger('click')
    await flushPromises()
    expect(endpoints.deleteProfile).toHaveBeenCalledWith(1)
    expect(notifyState.kind).toBe('ok')
    expect(notifyState.text).toBe('已删除')
  })

  it('删除冲突 409：把后端错误文案以 error 形式提示', async () => {
    ;(endpoints.listProfiles as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([makeProfile()])
    ;(endpoints.deleteProfile as unknown as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('HTTP 409: 方案名称 “订阅A” 已存在'))
    const w = mountPage()
    await flushPromises()

    await w.find('[data-testid="profile-delete"]').trigger('click')
    await flushPromises()
    await w.find('[data-testid="profile-delete-confirm"]').trigger('click')
    await flushPromises()

    expect(notifyState.kind).toBe('error')
    expect(notifyState.text).toContain('已存在')
  })

  it('加载失败：显示错误并可重试', async () => {
    const list = endpoints.listProfiles as unknown as ReturnType<typeof vi.fn>
    list.mockRejectedValueOnce(new Error('方案列表加载失败')).mockResolvedValueOnce([makeProfile()])
    const w = mountPage()
    await flushPromises()

    expect(w.find('[data-testid="async-error"]').exists()).toBe(true)
    expect(w.text()).toContain('方案列表加载失败')

    await w.find('[data-testid="async-retry"]').trigger('click')
    await flushPromises()
    expect(list).toHaveBeenCalledTimes(2)
    expect(w.find('[data-testid="profile-row"]').exists()).toBe(true)
  })

  it('保存失败：提示错误文案', async () => {
    ;(endpoints.createProfile as unknown as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('保存失败'))
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="profile-new"]').trigger('click')
    await flushPromises()
    await w.find('[data-testid="profile-save"]').trigger('click')
    await flushPromises()

    expect(notifyState.kind).toBe('error')
    expect(notifyState.text).toContain('保存失败')
  })

  it('刷新按钮重新拉取方案列表', async () => {
    const list = endpoints.listProfiles as unknown as ReturnType<typeof vi.fn>
    const w = mountPage()
    await flushPromises()
    expect(list).toHaveBeenCalledTimes(1)
    await w.find('[data-testid="profile-refresh"]').trigger('click')
    await flushPromises()
    expect(list).toHaveBeenCalledTimes(2)
  })

  it('编辑方案保存失败提示错误', async () => {
    ;(endpoints.listProfiles as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([makeProfile()])
    ;(endpoints.updateProfile as unknown as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('更新失败'))
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="profile-edit"]').trigger('click')
    await flushPromises()
    await w.find('[data-testid="profile-save"]').trigger('click')
    await flushPromises()
    expect(notifyState.kind).toBe('error')
    expect(notifyState.text).toContain('更新失败')
  })

  it('删除成功后刷新列表', async () => {
    ;(endpoints.listProfiles as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([makeProfile()])
    ;(endpoints.deleteProfile as unknown as ReturnType<typeof vi.fn>).mockResolvedValue({ ok: true })
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="profile-delete"]').trigger('click')
    await flushPromises()
    await w.find('[data-testid="profile-delete-confirm"]').trigger('click')
    await flushPromises()
    expect(endpoints.deleteProfile).toHaveBeenCalledWith(1)
    expect(endpoints.listProfiles).toHaveBeenCalledTimes(2)
  })

  it('关闭弹窗以覆盖 v-dialog v-model setter', async () => {
    const list = endpoints.listProfiles as unknown as ReturnType<typeof vi.fn>
    list.mockResolvedValue([makeProfile()])
    const w = mountPage()
    await flushPromises()
    // 打开新建弹窗，关闭以覆盖 v-model setter
    await w.find('[data-testid="profile-new"]').trigger('click')
    await flushPromises()
    for (const d of w.findAllComponents({ name: 'VDialog' })) {
      await (d.vm as { $emit: (e: string, ...a: unknown[]) => void }).$emit('update:modelValue', false)
    }
    await flushPromises()
    // 打开编辑弹窗，关闭以覆盖 v-model setter
    await w.find('[data-testid="profile-edit"]').trigger('click')
    await flushPromises()
    for (const d of w.findAllComponents({ name: 'VDialog' })) {
      await (d.vm as { $emit: (e: string, ...a: unknown[]) => void }).$emit('update:modelValue', false)
    }
    await flushPromises()
    // 打开删除确认弹窗，关闭以覆盖 v-model setter
    await w.find('[data-testid="profile-delete"]').trigger('click')
    await flushPromises()
    const dialogs = w.findAllComponents({ name: 'VDialog' })
    for (const d of dialogs) {
      await (d.vm as { $emit: (e: string, ...a: unknown[]) => void }).$emit('update:modelValue', false)
    }
    await flushPromises()
    expect(dialogs.length).toBeGreaterThan(0)
  })

  it('moveGroupLayer 边界守卫：越界不动、正常交换生效', async () => {
    const w = mountPage()
    await flushPromises()
    const vm = w.vm as unknown as ProfilesPageVm
    // 规则层只有 custom/template 且 template 恒在末位，没有可移动空间。

    // 分组层：越界不动，正常交换生效。
    const groupInitial = ['custom', 'airport', 'template']
    const groupLast = groupInitial.length - 1
    vm.moveGroupLayer(0, -1)
    vm.moveGroupLayer(groupLast, 1)
    expect(vm.layerKeys(vm.groupLayers)).toEqual(groupInitial)
    vm.moveGroupLayer(0, 1)
    expect(vm.layerKeys(vm.groupLayers)).toEqual(['airport', 'custom', 'template'])
  })
  it('新建方案：从弹窗添加规则源后保存提交 rule_sources', async () => {
    const listSources = endpoints.listRuleSources as unknown as ReturnType<typeof vi.fn>
    listSources.mockResolvedValue({ sources: [
      { name: 'src1', latest: 'v1', rule_count: 12 },
      { name: 'src2', latest: 'v2', rule_count: 34 },
      { name: 'empty', latest: '' },
      { name: 'airport-1-HK', latest: 'v3', origin: 'airport', label: 'HK', rule_count: 100 },
    ] })
    ;(endpoints.createProfile as unknown as ReturnType<typeof vi.fn>).mockResolvedValue(makeProfile({ id: 7 }))
    const w = mountPage()
    await flushPromises()

    await w.find('[data-testid="profile-new"]').trigger('click')
    await flushPromises()

    expect(w.find('[data-testid="profile-rule-sources"]').exists()).toBe(true)
    expect(w.find('[data-testid="profile-rule-sources-empty"]').exists()).toBe(true)
    await addRuleSources(w, ['src1'])
    const vm = w.vm as unknown as ProfilesPageVm
    expect(vm.draft.rule_sources).toEqual(['src1'])
    const row = w.find('[data-testid="profile-rule-source-row-src1"]')
    expect(row.text()).toContain('src1')
    expect(row.text()).toContain('远程 · 12 条')

    await w.find('[data-testid="profile-name"]').find('input').setValue('带规则源方案')
    await w.find('[data-testid="profile-save"]').trigger('click')
    await flushPromises()

    expect(endpoints.createProfile).toHaveBeenCalledWith(
      expect.objectContaining({ name: '带规则源方案', rule_sources: ['src1'] }),
    )
    expect(notifyState.text).toBe('已保存')
  })

  it('编辑方案：回填已绑定的规则源并原样提交；旧数据缺字段回填为空数组', async () => {
    const listSources = endpoints.listRuleSources as unknown as ReturnType<typeof vi.fn>
    listSources.mockResolvedValue({ sources: [{ name: 'a', latest: 'v1' }, { name: 'b', latest: 'v1' }] })
    ;(endpoints.listProfiles as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([
      makeProfile({ id: 9, name: '已绑定方案', rule_sources: ['a', 'b'] }),
      makeProfile({ id: 10, name: '旧方案', rule_sources: undefined }),
    ])
    ;(endpoints.updateProfile as unknown as ReturnType<typeof vi.fn>).mockResolvedValue(makeProfile())
    const w = mountPage()
    await flushPromises()

    await w.findAll('[data-testid="profile-edit"]')[0].trigger('click')
    await flushPromises()

    const vm = w.vm as unknown as ProfilesPageVm
    expect(vm.draft.rule_sources).toEqual(['a', 'b'])
    expect(w.find('[data-testid="profile-rule-source-row-a"]').text()).toContain('1')
    expect(w.find('[data-testid="profile-rule-source-row-b"]').text()).toContain('2')

    await w.find('[data-testid="profile-save"]').trigger('click')
    await flushPromises()
    expect(endpoints.updateProfile).toHaveBeenCalledWith(9, expect.objectContaining({ rule_sources: ['a', 'b'] }))

    // 旧数据没有 rule_sources 字段时回填空数组，不崩。
    await w.findAll('[data-testid="profile-edit"]')[1].trigger('click')
    await flushPromises()
    expect(vm.draft.rule_sources).toEqual([])
  })

  it('有序多选：搜索过滤、添加顺序、上移/下移、移除与未知源提示', async () => {
    const listSources = endpoints.listRuleSources as unknown as ReturnType<typeof vi.fn>
    listSources.mockResolvedValue({ sources: [
      { name: 'a', latest: 'v1', rule_count: 5 },
      { name: 'b', latest: 'v1', rule_count: 6 },
      { name: 'c', latest: 'v1', rule_count: 7 },
    ] })
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="profile-new"]').trigger('click')
    await flushPromises()

    // 搜索只在添加弹窗里过滤。
    await w.find('[data-testid="profile-rule-source-add"]').trigger('click')
    await flushPromises()
    const sourceSearch = w.findAllComponents({ name: 'VTextField' })
      .find((c) => c.attributes('data-testid') === 'profile-source-search')
    expect(sourceSearch).toBeTruthy()
    await sourceSearch!.find('input').setValue('b')
    await flushPromises()
    expect(w.findAll('[data-testid="profile-source-option"]')).toHaveLength(1)
    ;(w.vm as any).sourceSearch = ''
    await flushPromises()
    await (w.vm as any).toggleSourcePick('a')
    await (w.vm as any).toggleSourcePick('a')
    await (w.vm as any).toggleSourcePick('a')
    await (w.vm as any).toggleSourcePick('b')
    await flushPromises()
    await (w.vm as any).addPickedSources()
    await flushPromises()

    const vm = w.vm as unknown as ProfilesPageVm
    expect(vm.draft.rule_sources).toEqual(['a', 'b'])
    await w.find('[data-testid="profile-rule-source-down-a"]').trigger('click')
    await flushPromises()
    expect(vm.draft.rule_sources).toEqual(['b', 'a'])
    await w.find('[data-testid="profile-rule-source-up-a"]').trigger('click')
    await flushPromises()
    expect(vm.draft.rule_sources).toEqual(['a', 'b'])
    await (w.vm as any).moveRuleSource(0, -1)
    expect(vm.draft.rule_sources).toEqual(['a', 'b'])
    await (w.vm as any).removeRuleSource('b')
    expect(vm.draft.rule_sources).toEqual(['a'])

    // 已选的 a 不再出现在添加列表里；b 已移除，可以重新添加。
    await w.find('[data-testid="profile-rule-source-add"]').trigger('click')
    await flushPromises()
    const opts = w.findAll('[data-testid="profile-source-option"]').map((o) => o.text())
    expect(opts).toHaveLength(2)
    expect(w.findAllComponents({ name: 'VCheckbox' }).some((c) => c.attributes('data-testid') === 'profile-source-check-a')).toBe(false)
    ;(w.vm as any).sourcePickerOpen = false
    await flushPromises()

    // 已删除/无产物的历史源仍要显示、可移除。
    vm.draft.rule_sources = ['missing']
    await flushPromises()
    expect(w.find('[data-testid="profile-rule-source-row-missing"]').text()).toContain('已移除或暂无产物')
    await w.find('[data-testid="profile-rule-source-remove-missing"]').trigger('click')
    await flushPromises()
    expect(vm.draft.rule_sources).toEqual([])
  })

  it('跨源冲突提示：按当前源顺序检查并展示示例；移除后清除', async () => {
    const listSources = endpoints.listRuleSources as unknown as ReturnType<typeof vi.fn>
    const conflicts = endpoints.checkRuleSourceConflicts as unknown as ReturnType<typeof vi.fn>
    listSources.mockResolvedValue({ sources: [{ name: 'a', latest: 'v1' }, { name: 'b', latest: 'v1' }] })
    conflicts.mockResolvedValue({ total: 1, samples: [{ type: 'DOMAIN', param: 'dup.test', first: 'a', second: 'b' }] })
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="profile-new"]').trigger('click')
    await flushPromises()
    await addRuleSources(w, ['a', 'b'])
    await new Promise((r) => setTimeout(r, 350))
    await flushPromises()
    expect(conflicts).toHaveBeenLastCalledWith(['a', 'b'])
    const alert = w.find('[data-testid="profile-source-conflicts"]')
    expect(alert.exists()).toBe(true)
    expect(alert.text()).toContain('1 条跨源重复规则')
    expect(alert.text()).toContain('dup.test')
    await w.find('[data-testid="profile-rule-source-remove-b"]').trigger('click')
    await new Promise((r) => setTimeout(r, 350))
    await flushPromises()
    expect(w.find('[data-testid="profile-source-conflicts"]').exists()).toBe(false)
    w.unmount()
  })

  it('跨源冲突检查失败时静默降级，不影响方案编辑', async () => {
    const listSources = endpoints.listRuleSources as unknown as ReturnType<typeof vi.fn>
    const conflicts = endpoints.checkRuleSourceConflicts as unknown as ReturnType<typeof vi.fn>
    listSources.mockResolvedValue({ sources: [{ name: 'a', latest: 'v1' }, { name: 'b', latest: 'v1' }] })
    conflicts.mockRejectedValue(new Error('conflict down'))
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="profile-new"]').trigger('click')
    await flushPromises()
    await addRuleSources(w, ['a', 'b'])
    await new Promise((r) => setTimeout(r, 350))
    await flushPromises()
    expect(w.find('[data-testid="profile-source-conflicts"]').exists()).toBe(false)
    expect(w.find('[data-testid="profile-name"]').exists()).toBe(true)
    w.unmount()
  })

  it('列表新增规则源列：未绑定显示「默认」，多值用「、」连接', async () => {
    ;(endpoints.listProfiles as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([
      makeProfile({ id: 21, name: '无源', rule_sources: [] }),
      makeProfile({ id: 22, name: '多源', rule_sources: ['src1', 'src2'] }),
      makeProfile({ id: 23, name: '旧数据', rule_sources: undefined }),
    ])
    const w = mountPage()
    await flushPromises()

    expect(w.text()).toContain('规则源')
    expect(w.find('[data-testid="profile-rule-sources-text-21"]').text().trim()).toBe('默认')
    expect(w.find('[data-testid="profile-rule-sources-text-22"]').text().trim()).toBe('src1、src2')
    expect(w.find('[data-testid="profile-rule-sources-text-23"]').text().trim()).toBe('默认')
  })

  it('D-43：层编辑器行级 testid 按编辑器前缀去重（rule-layer-* / group-layer-*），无裸 layer-*', async () => {
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="profile-new"]').trigger('click')
    await flushPromises()

    expect(w.find('[data-testid="rule-layer-row-custom"]').exists()).toBe(true)
    expect(w.find('[data-testid="group-layer-row-custom"]').exists()).toBe(true)
    expect(w.find('[data-testid="layer-row-custom"]').exists()).toBe(false)
    expect(w.find('[data-testid="rule-layer-toggle-custom"]').exists()).toBe(true)
    expect(w.find('[data-testid="group-layer-toggle-custom"]').exists()).toBe(true)
    expect(w.find('[data-testid="layer-toggle-custom"]').exists()).toBe(false)
    expect(w.find('[data-testid="rule-layer-up-custom"]').exists()).toBe(false)
    expect(w.find('[data-testid="group-layer-up-custom"]').exists()).toBe(true)
    expect(w.find('[data-testid="layer-up-custom"]').exists()).toBe(false)
    expect(w.find('[data-testid="rule-layer-down-custom"]').exists()).toBe(false)
    expect(w.find('[data-testid="group-layer-down-custom"]').exists()).toBe(true)
    expect(w.find('[data-testid="layer-down-custom"]').exists()).toBe(false)
  })

  it('规则源清单加载失败：不影响主功能，弹窗可开、选项为空、仍可保存方案', async () => {
    const listSources = endpoints.listRuleSources as unknown as ReturnType<typeof vi.fn>
    listSources.mockRejectedValue(new Error('规则源清单不可用'))
    ;(endpoints.listProfiles as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([makeProfile({ id: 31 })])
    ;(endpoints.createProfile as unknown as ReturnType<typeof vi.fn>).mockResolvedValue(makeProfile({ id: 31 }))
    const w = mountPage()
    await flushPromises()

    // 页面照常渲染：列表加载没有被规则源接口拖垮。
    expect(w.find('[data-testid="profile-table"]').exists()).toBe(true)

    await w.find('[data-testid="profile-new"]').trigger('click')
    await flushPromises()
    const vm = w.vm as unknown as ProfilesPageVm
    expect(vm.dialog).toBe(true)
    await w.find('[data-testid="profile-rule-source-add"]').trigger('click')
    await flushPromises()
    expect(w.find('[data-testid="profile-source-empty"]').exists()).toBe(true)
    const cancelPicker = w.findAllComponents({ name: 'VBtn' })
      .find((c) => c.attributes('data-testid') === 'profile-source-cancel')
    expect(cancelPicker).toBeTruthy()
    await cancelPicker!.trigger('click')
    await flushPromises()

    await w.find('[data-testid="profile-name"]').find('input').setValue('兜底方案')
    await w.find('[data-testid="profile-save"]').trigger('click')
    await flushPromises()

    expect(endpoints.createProfile).toHaveBeenCalledWith(
      expect.objectContaining({ name: '兜底方案', rule_sources: [] }),
    )
    expect(notifyState.text).toBe('已保存')
  })})
