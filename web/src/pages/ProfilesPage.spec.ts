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
  },
}))

const vuetify = createVuetify({ components, directives })

interface Draft {
  name: string
  description: string
  config: string
  pipeline: string
  enabled: boolean
  sort_order: number
}

interface ProfilesPageVm {
  dialog: boolean
  confirmOpen: boolean
  draft: Draft
  ruleLayers: { key: string; enabled: boolean }[]
  groupLayers: { key: string; enabled: boolean }[]
  moveRuleLayer: (index: number, delta: number) => void
  moveGroupLayer: (index: number, delta: number) => void
  layerKeys: (rows: { key: string; enabled: boolean }[]) => string[]
}

const DEFAULT_LAYERS = ['custom', 'airport', 'template']

function makeProfile(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    name: '订阅A',
    description: '说明A',
    config: '',
    pipeline: '',
    enabled: true,
    sort_order: 3,
    rule_layers: [...DEFAULT_LAYERS],
    group_layers: [...DEFAULT_LAYERS],
    rule_count: 2,
    group_count: 1,
    ...overrides,
  }
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
    expect(ruleLayersText[0].text()).toBe('自定义 → 机场导入 → 模板')
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
    expect(help.text()).toContain('删除方案前要先把规则/分组移走或删除')
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
    const sw = w.findAllComponents({ name: 'VSwitch' })
    await sw[0].setValue(false)
    await w.find('[data-testid="profile-sort"]').find('input').setValue('9')

    // 新建弹窗里层编辑器默认全开（自定义→机场导入→模板）。
    const ruleBox = w.find('[data-testid="profile-rule-layers"]')
    const groupBox = w.find('[data-testid="profile-group-layers"]')
    expect(ruleBox.exists()).toBe(true)
    expect(groupBox.exists()).toBe(true)
    expect(ruleBox.find('[data-testid="layer-row-custom"]').exists()).toBe(true)
    expect(ruleBox.find('[data-testid="layer-row-template"]').exists()).toBe(true)
    expect(ruleBox.find('[data-testid="layer-row-custom"] .sf-layer-priority').text()).toBe('优先级 1')
    expect(ruleBox.find('[data-testid="layer-row-airport"] .sf-layer-priority').text()).toBe('优先级 2')
    expect(ruleBox.find('[data-testid="layer-row-template"] .sf-layer-priority').text()).toBe('优先级 3')
    // 分组层不受「模板恒最后」约束，template 行上下移可用，也不显示模板提示。
    const gTpl = groupBox.find('[data-testid="layer-row-template"]')
    // 分组层不锁模板：template 行上移可用（只在末位时才是下移边界）。
    expect(gTpl.find('[data-testid="layer-up-template"]').attributes('disabled')).toBeUndefined()
    expect(gTpl.find('[data-testid="layer-down-template"]').attributes('disabled')).not.toBeUndefined()
    expect(groupBox.text()).not.toContain('模板规则始终最后（MATCH/FINAL）')

    await flushPromises()
    await w.find('[data-testid="profile-save"]').trigger('click')
    await flushPromises()

    expect(endpoints.createProfile).toHaveBeenCalledWith({
      name: '新方案',
      description: '新说明',
      config: 'cfg',
      pipeline: 'pipe',
      enabled: false,
      sort_order: 9,
      rule_layers: ['custom', 'airport', 'template'],
      group_layers: ['custom', 'airport', 'template'],
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

    // 先关掉规则层的全部三层 → 保存被阻止。
    const ruleBox = w.find('[data-testid="profile-rule-layers"]')
    await ruleBox.find('[data-testid="layer-toggle-custom"] input').setValue(false)
    await ruleBox.find('[data-testid="layer-toggle-airport"] input').setValue(false)
    await ruleBox.find('[data-testid="layer-toggle-template"] input').setValue(false)
    await flushPromises()

    await w.find('[data-testid="profile-save"]').trigger('click')
    await flushPromises()

    expect(endpoints.createProfile).not.toHaveBeenCalled()
    expect(notifyState.kind).toBe('error')
    expect(notifyState.text).toBe('至少启用一层')

    // 规则层恢复全开，再关掉分组层的全部三层 → 同样被阻止。
    await ruleBox.find('[data-testid="layer-toggle-custom"] input').setValue(true)
    await ruleBox.find('[data-testid="layer-toggle-airport"] input').setValue(true)
    await ruleBox.find('[data-testid="layer-toggle-template"] input').setValue(true)
    const groupBox = w.find('[data-testid="profile-group-layers"]')
    await groupBox.find('[data-testid="layer-toggle-custom"] input').setValue(false)
    await groupBox.find('[data-testid="layer-toggle-airport"] input').setValue(false)
    await groupBox.find('[data-testid="layer-toggle-template"] input').setValue(false)
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

    // airport 在前（优先级 1），custom 在后，template 应被挪到末尾且未启用。
    const ruleBox = w.find('[data-testid="profile-rule-layers"]')
    const ruleRows = ruleBox.findAll('[data-testid^="layer-row-"]')
    expect(ruleRows).toHaveLength(3)
    expect(ruleRows[0].attributes('data-testid')).toBe('layer-row-airport')
    expect(ruleRows[0].find('.sf-layer-priority').text()).toBe('优先级 1')
    expect(ruleRows[1].attributes('data-testid')).toBe('layer-row-custom')
    expect(ruleRows[1].find('.sf-layer-priority').text()).toBe('优先级 2')
    expect(ruleRows[2].attributes('data-testid')).toBe('layer-row-template')
    expect(ruleRows[2].find('.sf-layer-priority').text()).toBe('未启用')
    // 模板规则始终最后，上下移按钮 disabled。
    const upTpl = ruleRows[2].find('[data-testid="layer-up-template"]')
    const downTpl = ruleRows[2].find('[data-testid="layer-down-template"]')
    expect(upTpl.attributes('disabled')).not.toBeUndefined()
    expect(downTpl.attributes('disabled')).not.toBeUndefined()
    expect(ruleRows[2].text()).toContain('模板规则始终最后（MATCH/FINAL）')

    await w.find('[data-testid="profile-save"]').trigger('click')
    await flushPromises()

    // 只提交启用的层（顺序即优先级）；未启用的 template 不进数组。
    expect(endpoints.updateProfile).toHaveBeenCalledWith(1, expect.objectContaining({
      rule_layers: ['airport', 'custom'],
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
    const upCustom = ruleBox.find('[data-testid="layer-row-custom"]').find('[data-testid="layer-up-custom"]')
    expect(upCustom.attributes('disabled')).not.toBeUndefined()

    await ruleBox.find('[data-testid="layer-row-airport"]').find('[data-testid="layer-up-airport"]').trigger('click')
    await flushPromises()
    expect(endpoints.updateProfile).not.toHaveBeenCalled()

    await w.find('[data-testid="profile-save"]').trigger('click')
    await flushPromises()
    expect(endpoints.updateProfile).toHaveBeenCalledWith(1, expect.objectContaining({
      rule_layers: ['airport', 'custom', 'template'],
      group_layers: ['custom', 'airport', 'template'],
    }))
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
    await groupBox.find('[data-testid="layer-row-custom"]').find('[data-testid="layer-down-custom"]').trigger('click')
    await flushPromises()

    await w.find('[data-testid="profile-save"]').trigger('click')
    await flushPromises()

    expect(endpoints.updateProfile).toHaveBeenCalledWith(1, expect.objectContaining({
      rule_layers: ['custom', 'airport', 'template'],
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
    const rows = ruleBox.findAll('[data-testid^="layer-row-"]')
    expect(rows[0].attributes('data-testid')).toBe('layer-row-custom')
    expect(rows[1].attributes('data-testid')).toBe('layer-row-airport')
    expect(rows[2].attributes('data-testid')).toBe('layer-row-template')
    // airport 不在后端数组里 → 未启用；template 被挪到末位且仍启用。
    expect(rows[1].find('.sf-layer-priority').text()).toBe('未启用')
    expect(rows[2].find('.sf-layer-priority').text()).toBe('优先级 2')
    // 分组层数组为空 → 按默认三层全开回填。
    const groupBox = w.find('[data-testid="profile-group-layers"]')
    expect(groupBox.find('[data-testid="layer-row-custom"] .sf-layer-priority').text()).toBe('优先级 1')
    expect(groupBox.find('[data-testid="layer-row-airport"] .sf-layer-priority').text()).toBe('优先级 2')
    expect(groupBox.find('[data-testid="layer-row-template"] .sf-layer-priority').text()).toBe('优先级 3')

    await w.find('[data-testid="profile-save"]').trigger('click')
    await flushPromises()

    expect(endpoints.updateProfile).toHaveBeenCalledWith(1, expect.objectContaining({
      rule_layers: ['custom', 'template'],
      group_layers: ['custom', 'airport', 'template'],
    }))
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

  it('moveRuleLayer/moveGroupLayer 边界守卫：越界不动、template 恒在末位', async () => {
    const w = mountPage()
    await flushPromises()
    const vm = w.vm as unknown as ProfilesPageVm
    const initial = ['custom', 'airport', 'template']
    const last = initial.length - 1

    // 越界：第一行上移、末行下移都不动。
    vm.moveRuleLayer(0, -1)
    vm.moveRuleLayer(last, 1)
    expect(vm.layerKeys(vm.ruleLayers)).toEqual(initial)
    // template 行上移被守卫（不允许把 template 换出末位）。
    vm.moveRuleLayer(last, -1)
    expect(vm.layerKeys(vm.ruleLayers)).toEqual(initial)
    // 其它行也不能越过 template。
    vm.moveRuleLayer(last - 1, 1)
    expect(vm.layerKeys(vm.ruleLayers)).toEqual(initial)
    // 正常交换：airport 上移到 custom 之前。
    vm.moveRuleLayer(1, -1)
    expect(vm.layerKeys(vm.ruleLayers)).toEqual(['airport', 'custom', 'template'])

    // 分组层：越界不动，正常交换生效。
    vm.moveGroupLayer(0, -1)
    vm.moveGroupLayer(last, 1)
    expect(vm.layerKeys(vm.groupLayers)).toEqual(initial)
    vm.moveGroupLayer(0, 1)
    expect(vm.layerKeys(vm.groupLayers)).toEqual(['airport', 'custom', 'template'])
  })
})
