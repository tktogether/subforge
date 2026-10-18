import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import SettingsPage from './SettingsPage.vue'
import { endpoints } from '../api/endpoints'

vi.mock('../api/endpoints', () => ({
  endpoints: {
    targets: vi.fn(),
    stats: vi.fn(),
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
}

describe('SettingsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  const sampleStats = { history: { total: 7 } } as any

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
    const btns = w.findAllComponents({ name: 'VBtn' })
    await btns[0].trigger('click')
    expect(mockSetMode).toHaveBeenCalledWith('light')
    w.unmount()
  })

  it('点击深色按钮：theme.setMode("dark")', async () => {
    vi.mocked(endpoints.targets).mockResolvedValue([])
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    const w = mount(SettingsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const btns = w.findAllComponents({ name: 'VBtn' })
    await btns[1].trigger('click')
    expect(mockSetMode).toHaveBeenCalledWith('dark')
    w.unmount()
  })

  it('点击跟随系统按钮：theme.setMode("system")', async () => {
    vi.mocked(endpoints.targets).mockResolvedValue([])
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    const w = mount(SettingsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const btns = w.findAllComponents({ name: 'VBtn' })
    await btns[2].trigger('click')
    expect(mockSetMode).toHaveBeenCalledWith('system')
    w.unmount()
  })
})
