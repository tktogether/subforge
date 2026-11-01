import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import SettingsPage from './SettingsPage.vue'
import { endpoints } from '../api/endpoints'
import { pageHelp } from '../content/help'
import { notifyState } from '../composables/useNotify'

vi.mock('../api/endpoints', () => ({
  endpoints: {
    targets: vi.fn(),
    stats: vi.fn(),
    listSettings: vi.fn(),
    updateSettings: vi.fn(),
    getTemplate: vi.fn(),
    putTemplate: vi.fn(),
  },
}))

const vuetify = createVuetify({ components, directives })

interface SettingsPageVm {
  error: string
  settings: any[]
  saving: boolean
  load: () => Promise<void>
  save: () => Promise<void>
  resetDefaults: () => void
  setSwitch: (item: any, value: boolean) => void
}

describe('SettingsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(endpoints.listSettings).mockResolvedValue({ settings: sampleSettings } as any)
    vi.mocked(endpoints.updateSettings).mockResolvedValue({ ok: true } as any)
    vi.mocked(endpoints.getTemplate).mockResolvedValue({ path: '/data/template.ini', content: '[custom]\n' } as any)
    vi.mocked(endpoints.putTemplate).mockResolvedValue({ ok: true } as any)
  })

  const sampleStats = { history: { total: 7 } } as any
  const sampleSettings = [
    { key: 'script.enabled', kind: 'bool', label: '脚本', description: '脚本开关', value: 'true', is_default: true, placeholder: '' },
    { key: 'rules.manifest_path', kind: 'path', label: 'manifest', description: 'manifest 路径', value: '/tmp/m', is_default: false, placeholder: '/default/m' },
  ] as any

  it('模板卡片：已配置时加载路径/内容，保存调用 putTemplate 并提示', async () => {
    vi.mocked(endpoints.getTemplate).mockResolvedValue({ path: '/data/template.ini', content: '[custom]\n' } as any)
    const w = mount(SettingsPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as any
    await flushPromises()
    expect(vm.templateAvailable).toBe(true)
    expect(vm.templatePath).toBe('/data/template.ini')
    expect(w.find('[data-testid="template-content"]').exists()).toBe(true)
    await w.find('[data-testid="template-content"] textarea').setValue('[custom]\nruleset=x,[]FINAL')
    await flushPromises()
    expect(vm.templateContent).toBe('[custom]\nruleset=x,[]FINAL')
    await w.find('[data-testid="template-save"]').trigger('click')
    await flushPromises()
    expect(endpoints.putTemplate).toHaveBeenCalledWith('[custom]\nruleset=x,[]FINAL')
    expect(endpoints.getTemplate).toHaveBeenCalledTimes(2)
    expect(notifyState.text).toBe('模板已保存')
  })

  it('模板卡片：未配置 -config 时给出引导，不渲染编辑器', async () => {
    vi.mocked(endpoints.getTemplate).mockRejectedValueOnce(new Error('模板路径未配置'))
    const w = mount(SettingsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.find('[data-testid="template-unavailable"]').exists()).toBe(true)
    expect(w.find('[data-testid="template-content"]').exists()).toBe(false)
    expect(w.find('[data-testid="template-error"]').text()).toContain('模板路径未配置')
  })

  it('模板卡片：保存失败显示语法错误', async () => {
    vi.mocked(endpoints.getTemplate).mockResolvedValue({ path: '/data/template.ini', content: '[custom]' } as any)
    vi.mocked(endpoints.putTemplate).mockRejectedValueOnce(new Error('模板语法错误: bad'))
    const w = mount(SettingsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await w.find('[data-testid="template-save"]').trigger('click')
    await flushPromises()
    expect(w.find('[data-testid="template-error"]').text()).toContain('模板语法错误')
  })

  it('设置项集中渲染在单个卡片内', async () => {
    const w = mount(SettingsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(endpoints.listSettings).toHaveBeenCalledTimes(1)
    expect(w.findAll('[data-testid="setting-row"]').length).toBe(2)
    const cards = w.findAllComponents({ name: 'VCard' })
    const settingsCard = cards.find((c) => c.classes().includes('settings-card'))
    expect(settingsCard?.exists()).toBe(true)
    expect(settingsCard?.text()).toContain('manifest 路径')
    w.unmount()
  })

  it('加载设置失败显示错误', async () => {
    vi.mocked(endpoints.listSettings).mockRejectedValueOnce(new Error('fail-load'))
    const w = mount(SettingsPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as SettingsPageVm
    await flushPromises()
    expect(vm.error).toBe('fail-load')
    expect(w.text()).toContain('fail-load')
    w.unmount()
  })

  it('页面副标题承载 oneLiner；help-panel 入口行=使用说明且详情默认收起', async () => {
    vi.mocked(endpoints.listSettings).mockResolvedValue({ settings: sampleSettings })
    const w = mount(SettingsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const panel = w.find('[data-testid="help-panel"]')
    expect(panel.exists()).toBe(true)
    expect(w.find('.page-header__subtitle').text()).toContain(pageHelp('settings').oneLiner)
    expect(panel.find('[data-testid="help-toggle-label"]').text()).toBe('使用说明')
    expect(panel.find('[data-testid="help-body"]').exists()).toBe(false)
    w.unmount()
  })

  it('模板：渲染 bool switch 与 path input，is_default/placeholder 两侧', async () => {
    vi.mocked(endpoints.targets).mockResolvedValue([])
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    const w = mount(SettingsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.findAll('[data-testid="setting-row"]').length).toBe(2)
    expect(w.find('[data-testid="setting-switch"]').exists()).toBe(true)
    expect(w.find('[data-testid="setting-input"]').exists()).toBe(true)
    expect(w.find('[data-testid="setting-default-badge"]').exists()).toBe(true)
    expect(w.find('[data-testid="setting-placeholder"]').exists()).toBe(true)
    w.unmount()
  })

  it('数字设置项：渲染数字输入并写回字符串', async () => {
    vi.mocked(endpoints.listSettings).mockResolvedValueOnce({
      settings: [
        { key: 'fetch.refresh_minutes', kind: 'number', label: '刷新间隔', description: '分钟', value: '30', placeholder: '30', is_default: false },
      ],
    } as any)
    const w = mount(SettingsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const input = w.find('[data-testid="setting-number"] input')
    expect(input.exists()).toBe(true)
    await input.setValue('10')
    expect((w.vm as any).settings[0].value).toBe('10')
    await w.find('[data-testid="settings-save"]').trigger('click')
    await flushPromises()
    expect(endpoints.updateSettings).toHaveBeenCalledWith({ 'fetch.refresh_minutes': '10' })
    w.unmount()
  })

  it('下拉设置项：渲染候选并写回选中值', async () => {
    vi.mocked(endpoints.listSettings).mockResolvedValueOnce({
      settings: [
        {
          key: 'fetch.outbound_node',
          kind: 'select',
          label: '出网节点',
          description: '拉取出口',
          value: '',
          is_default: true,
          options: [
            { value: '', label: '直连（默认）' },
            { value: '3', label: '香港节点' },
          ],
        },
      ],
    } as any)
    const w = mount(SettingsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const select = w.findAllComponents({ name: 'VSelect' }).find((s) => s.attributes('data-testid') === 'setting-select')
    expect(select).toBeTruthy()
    await select!.setValue('3')
    expect((w.vm as any).settings[0].value).toBe('3')
    await w.find('[data-testid="settings-save"]').trigger('click')
    await flushPromises()
    expect(endpoints.updateSettings).toHaveBeenCalledWith({ 'fetch.outbound_node': '3' })
    w.unmount()
  })

  it('setSwitch：true/false 两个分支', async () => {
    vi.mocked(endpoints.targets).mockResolvedValue([])
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    const w = mount(SettingsPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as SettingsPageVm
    await flushPromises()
    const item = vm.settings[0]
    vm.setSwitch(item, true)
    expect(item.value).toBe('true')
    vm.setSwitch(item, false)
    expect(item.value).toBe('false')
    w.unmount()
  })

  it('setSwitch：通过 v-switch update:model-value 触发', async () => {
    vi.mocked(endpoints.targets).mockResolvedValue([])
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    const w = mount(SettingsPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as SettingsPageVm
    await flushPromises()
    const sw = w.findComponent({ name: 'VSwitch' })
    await sw.vm.$emit('update:model-value', false)
    await flushPromises()
    expect(vm.settings[0].value).toBe('false')
    w.unmount()
  })

  it('resetDefaults：所有项 value 置空', async () => {
    vi.mocked(endpoints.targets).mockResolvedValue([])
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    const w = mount(SettingsPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as SettingsPageVm
    await flushPromises()
    await w.find('[data-testid="settings-reset"]').trigger('click')
    expect(vm.settings[0].value).toBe('')
    expect(vm.settings[1].value).toBe('')
    w.unmount()
  })

  it('save：成功分支，updateSettings 调用、回填、notify ok', async () => {
    vi.mocked(endpoints.targets).mockResolvedValue([])
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    vi.mocked(endpoints.updateSettings).mockResolvedValue({ settings: sampleSettings } as any)
    const w = mount(SettingsPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as SettingsPageVm
    await flushPromises()
    vm.settings[0].value = 'false'
    await w.find('[data-testid="settings-save"]').trigger('click')
    await flushPromises()
    expect(endpoints.updateSettings).toHaveBeenCalled()
    expect(vm.saving).toBe(false)
    expect(vm.error).toBe('')
    w.unmount()
  })

  it('save：失败分支，error 写入、notify error、saving 复位', async () => {
    vi.mocked(endpoints.targets).mockResolvedValue([])
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    vi.mocked(endpoints.updateSettings).mockRejectedValue(new Error('savefail'))
    const w = mount(SettingsPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as SettingsPageVm
    await flushPromises()
    await w.find('[data-testid="settings-save"]').trigger('click')
    await flushPromises()
    expect(vm.error).toBe('savefail')
    expect(vm.saving).toBe(false)
    w.unmount()
  })

  it('save：updateSettings 返回无 settings 时回退 listSettings', async () => {
    vi.mocked(endpoints.targets).mockResolvedValue([])
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    vi.mocked(endpoints.updateSettings).mockResolvedValue({} as any)
    const w = mount(SettingsPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as SettingsPageVm
    await flushPromises()
    vi.mocked(endpoints.listSettings).mockResolvedValue({ settings: null } as any)
    await w.find('[data-testid="settings-save"]').trigger('click')
    await flushPromises()
    expect(endpoints.listSettings).toHaveBeenCalled()
    expect(vm.saving).toBe(false)
    w.unmount()
  })

  it('load：settings 为 null 时回落 []', async () => {
    vi.mocked(endpoints.targets).mockResolvedValue([])
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    vi.mocked(endpoints.listSettings).mockResolvedValue({ settings: null } as any)
    const w = mount(SettingsPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as SettingsPageVm
    await flushPromises()
    expect(vm.settings).toEqual([])
    w.unmount()
  })

  it('模板：path 项 v-text-field setValue 触发 v-model setter', async () => {
    vi.mocked(endpoints.targets).mockResolvedValue([])
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    const w = mount(SettingsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const tf = w.findComponent({ name: 'VTextField' })
    await tf.vm.$emit('update:model-value', '/new/path')
    await flushPromises()
    const vm = w.vm as unknown as SettingsPageVm
    expect(vm.settings[1].value).toBe('/new/path')
    w.unmount()
  })

  it('模板：fieldHelp 为 undefined 时不渲染说明图标', async () => {
    vi.mocked(endpoints.targets).mockResolvedValue([])
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    vi.mocked(endpoints.listSettings).mockResolvedValue({ settings: [{ key: 'unknown.key', kind: 'path', label: 'u', description: 'd', value: '', is_default: false }] } as any)
    const w = mount(SettingsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.find('[data-testid="info-tip"]').exists()).toBe(false)
    w.unmount()
  })
})
