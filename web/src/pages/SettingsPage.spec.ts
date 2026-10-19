import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import SettingsPage from './SettingsPage.vue'
import { endpoints } from '../api/endpoints'
import { pageHelp } from '../content/help'

vi.mock('../api/endpoints', () => ({
  endpoints: {
    targets: vi.fn(),
    stats: vi.fn(),
    listSettings: vi.fn(),
    updateSettings: vi.fn(),
  },
}))

const mockSetMode = vi.fn()
vi.mock('../composables/useThemeMode', () => ({
  useThemeMode: () => ({
    mode: { value: 'system' },
    isDark: { value: false },
    setMode: mockSetMode,
    cycle: vi.fn(),
  }),
}))

const vuetify = createVuetify({ components, directives })

interface SettingsPageVm {
  targets: string[]
  statsHistory: number
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
  })

  const sampleStats = { history: { total: 7 } } as any
  const sampleSettings = [
    { key: 'script.enabled', kind: 'bool', label: '脚本', description: '脚本开关', value: 'true', is_default: true, placeholder: '' },
    { key: 'rules.manifest_path', kind: 'path', label: 'manifest', description: 'manifest 路径', value: '/tmp/m', is_default: false, placeholder: '/default/m' },
  ] as any

  it('onMounted 调用 load：targets 与 stats 并发调用，结果写入，error 为空', async () => {
    vi.mocked(endpoints.targets).mockResolvedValue(['clash', 'yaml'])
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    const w = mount(SettingsPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as SettingsPageVm
    await flushPromises()
    expect(endpoints.targets).toHaveBeenCalledTimes(1)
    expect(endpoints.stats).toHaveBeenCalledTimes(1)
    expect(vm.targets).toEqual(['clash', 'yaml'])
    expect(vm.statsHistory).toBe(7)
    expect(vm.error).toBe('')
    w.unmount()
  })

  it('load 失败：error 为 errorMessage(err)', async () => {
    vi.mocked(endpoints.targets).mockRejectedValue(new Error('fail-load'))
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    const w = mount(SettingsPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as SettingsPageVm
    await flushPromises()
    expect(vm.error).toBe('fail-load')
    expect(vm.targets).toEqual([])
    w.unmount()
  })

  it('load 失败（非 Error）：error 为 String(err)', async () => {
    vi.mocked(endpoints.targets).mockResolvedValue(['clash'])
    vi.mocked(endpoints.stats).mockRejectedValue('str-fail')
    const w = mount(SettingsPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as SettingsPageVm
    await flushPromises()
    expect(vm.error).toBe('str-fail')
    w.unmount()
  })

  it('模板：targets 为空时显示「（无）」', async () => {
    vi.mocked(endpoints.targets).mockResolvedValue([])
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    const w = mount(SettingsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.text()).toContain('（无）')
    expect(w.text()).toContain('7')
    w.unmount()
  })

  it('模板：targets 非空时 join 渲染', async () => {
    vi.mocked(endpoints.targets).mockResolvedValue(['clash', 'yaml'])
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    const w = mount(SettingsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.text()).toContain('clash, yaml')
    expect(w.text()).not.toContain('（无）')
    w.unmount()
  })

  it('模板：error 为空时不显示错误块', async () => {
    vi.mocked(endpoints.targets).mockResolvedValue([])
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    const w = mount(SettingsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.text()).not.toContain('fail-load')
    expect(w.find('.text-error').exists()).toBe(false)
    w.unmount()
  })

  it('模板：error 非空时显示错误块', async () => {
    vi.mocked(endpoints.targets).mockRejectedValue(new Error('boom'))
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    const w = mount(SettingsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.text()).toContain('boom')
    expect(w.find('.text-error').exists()).toBe(true)
    w.unmount()
  })

  it('点击浅色按钮：theme.setMode("light")', async () => {
    vi.mocked(endpoints.targets).mockResolvedValue([])
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    const w = mount(SettingsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const btn = w.findAllComponents({ name: 'VBtn' }).find((b) => b.text().includes('浅色'))
    await btn!.trigger('click')
    expect(mockSetMode).toHaveBeenCalledWith('light')
    w.unmount()
  })

  it('点击深色按钮：theme.setMode("dark")', async () => {
    vi.mocked(endpoints.targets).mockResolvedValue([])
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    const w = mount(SettingsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const btn = w.findAllComponents({ name: 'VBtn' }).find((b) => b.text().includes('深色'))
    await btn!.trigger('click')
    expect(mockSetMode).toHaveBeenCalledWith('dark')
    w.unmount()
  })

  it('点击跟随系统按钮：theme.setMode("system")', async () => {
    vi.mocked(endpoints.targets).mockResolvedValue([])
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    const w = mount(SettingsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const btn = w.findAllComponents({ name: 'VBtn' }).find((b) => b.text().includes('跟随系统'))
    await btn!.trigger('click')
    expect(mockSetMode).toHaveBeenCalledWith('system')
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

  it('模板：fieldHelp 为 undefined 时 hint 回落空串', async () => {
    vi.mocked(endpoints.targets).mockResolvedValue([])
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    vi.mocked(endpoints.listSettings).mockResolvedValue({ settings: [{ key: 'unknown.key', kind: 'path', label: 'u', description: 'd', value: '', is_default: false }] } as any)
    const w = mount(SettingsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.find('[data-testid="setting-hint"]').text()).toBe('')
    w.unmount()
  })
})
