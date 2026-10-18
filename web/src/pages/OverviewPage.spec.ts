import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import OverviewPage from './OverviewPage.vue'
import { endpoints, type Stats, type HistoryEntry } from '../api/endpoints'

vi.mock('../api/endpoints', () => ({
  endpoints: {
    stats: vi.fn(),
    history: vi.fn(),
  },
}))

const vuetify = createVuetify({ components, directives })

interface OverviewVm {
  loading: boolean
  error: string
  stats: Stats | null
  history: HistoryEntry[]
  load: () => Promise<void>
  empty: () => boolean
}

describe('OverviewPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  const sampleStats: Stats = {
    history: { total: 5, cachedHits: 2, urlTotal: 10, cacheHitRate: 0.4 },
    logs: 3,
    last: { nodes: 1, rawNodes: 1, groups: 1, rules: 1, cached: 0, urlCount: 1, cacheHitRate: 0.5 },
  }

  const sampleHistory: HistoryEntry[] = [
    { id: 1, at: '2024-01-01', target: 'clash', format: 'yaml', nodes: 2, rawNodes: 2, groups: 1, rules: 1, failures: null, warnings: null, cached: 0, urlCount: 1 },
  ]

  it('onMounted 调用 load：stats 与 history 被并发调用，结果写入 stats/history，loading 归 false', async () => {
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    vi.mocked(endpoints.history).mockResolvedValue(sampleHistory)
    const w = mount(OverviewPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as OverviewVm
    await flushPromises()
    expect(endpoints.stats).toHaveBeenCalledTimes(1)
    expect(endpoints.history).toHaveBeenCalledTimes(1)
    expect(vm.stats).toEqual(sampleStats)
    expect(vm.history).toEqual(sampleHistory)
    expect(vm.loading).toBe(false)
    expect(vm.error).toBe('')
    w.unmount()
  })

  it('load 失败：error 为 errorMessage(err)，loading 归 false，stats 仍为 null', async () => {
    vi.mocked(endpoints.stats).mockRejectedValue(new Error('boom'))
    vi.mocked(endpoints.history).mockResolvedValue(sampleHistory)
    const w = mount(OverviewPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as OverviewVm
    await flushPromises()
    expect(vm.error).toBe('boom')
    expect(vm.loading).toBe(false)
    expect(vm.stats).toBeNull()
    w.unmount()
  })

  it('load 失败（非 Error）：error 为 String(err)', async () => {
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    vi.mocked(endpoints.history).mockRejectedValue('str-fail')
    const w = mount(OverviewPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as OverviewVm
    await flushPromises()
    expect(vm.error).toBe('str-fail')
    expect(vm.loading).toBe(false)
    w.unmount()
  })

  it('empty()：stats 为 null 时返回 true', () => {
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    vi.mocked(endpoints.history).mockResolvedValue(sampleHistory)
    const w = mount(OverviewPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as OverviewVm
    // before load resolves, stats is null
    expect(vm.empty()).toBe(true)
    w.unmount()
  })

  it('empty()：stats.last 为 null 时返回 true', async () => {
    const statsNoLast = { ...sampleStats, last: null }
    vi.mocked(endpoints.stats).mockResolvedValue(statsNoLast)
    vi.mocked(endpoints.history).mockResolvedValue(sampleHistory)
    const w = mount(OverviewPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as OverviewVm
    await flushPromises()
    expect(vm.empty()).toBe(true)
    w.unmount()
  })

  it('empty()：stats.last 不为 null 时返回 false', async () => {
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    vi.mocked(endpoints.history).mockResolvedValue(sampleHistory)
    const w = mount(OverviewPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as OverviewVm
    await flushPromises()
    expect(vm.empty()).toBe(false)
    w.unmount()
  })

  it('模板：渲染 stats 的 history.total / cacheHitRate / urlTotal / logs', async () => {
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    vi.mocked(endpoints.history).mockResolvedValue(sampleHistory)
    const w = mount(OverviewPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const text = w.text()
    expect(text).toContain('5')
    expect(text).toContain('40.0%')
    expect(text).toContain('10')
    expect(text).toContain('3')
    w.unmount()
  })

  it('模板：history 表格渲染条目', async () => {
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    vi.mocked(endpoints.history).mockResolvedValue(sampleHistory)
    const w = mount(OverviewPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.text()).toContain('2024-01-01')
    expect(w.text()).toContain('clash')
    w.unmount()
  })

  it('模板：empty 状态显示「暂无转换记录」', async () => {
    vi.mocked(endpoints.stats).mockResolvedValue(null as unknown as Stats)
    vi.mocked(endpoints.history).mockResolvedValue([])
    const w = mount(OverviewPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.text()).toContain('暂无转换记录')
    w.unmount()
  })

  it('模板：stats 字段为 null 时 ?? 默认值 0 / 0.0% 渲染', async () => {
    const statsNullFields = {
      history: { total: null, cachedHits: 0, urlTotal: null, cacheHitRate: null },
      logs: null,
      last: { nodes: 1, rawNodes: 1, groups: 1, rules: 1, cached: 0, urlCount: 1, cacheHitRate: 0.5 },
    }
    vi.mocked(endpoints.stats).mockResolvedValue(statsNullFields as unknown as Stats)
    vi.mocked(endpoints.history).mockResolvedValue([])
    const w = mount(OverviewPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const text = w.text()
    expect(text).toContain('0')
    expect(text).toContain('0.0%')
    w.unmount()
  })

  it('retry：AsyncSection retry 事件触发 load', async () => {
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    vi.mocked(endpoints.history).mockResolvedValue(sampleHistory)
    const w = mount(OverviewPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as OverviewVm
    await flushPromises()
    const callsBefore = vi.mocked(endpoints.stats).mock.calls.length
    // trigger retry via load directly (AsyncSection @retry="load")
    await vm.load()
    await flushPromises()
    expect(vi.mocked(endpoints.stats).mock.calls.length).toBeGreaterThan(callsBefore)
    w.unmount()
  })
})
