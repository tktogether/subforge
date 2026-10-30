import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import OverviewPage from './OverviewPage.vue'
import { endpoints, type Stats, type HistoryEntry } from '../api/endpoints'
import { pageHelp } from '../content/help'

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
  expanded: string[]
  fmtTime: (at: string) => string
}

describe('OverviewPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  const sampleStats: Stats = {
    history: { total: 5, cachedHits: 2, urlTotal: 10, cacheHitRate: 0.4 },
    logs: 3,
    last: { nodes: 1, rawNodes: 2, groups: 3, rules: 4, cached: 1, urlCount: 2, cacheHitRate: 0.5 },
  }

  const sampleHistory: HistoryEntry[] = [
    { id: 1, at: '2024-01-01T00:00:00Z', target: 'clash', format: 'clash.meta', nodes: 2, rawNodes: 3, groups: 1, rules: 1, failures: ['f1'], warnings: ['w1'], cached: 0, urlCount: 1 },
  ]

  it('onMounted 调用 load：stats 与 history 被并发调用', async () => {
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

  it('load 失败：error 为 errorMessage(err)', async () => {
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

  it('empty()：stats 为 null 时返回 true', () => {
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    vi.mocked(endpoints.history).mockResolvedValue(sampleHistory)
    const w = mount(OverviewPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as OverviewVm
    expect(vm.empty()).toBe(true)
    w.unmount()
  })

  it('empty()：stats.last 为 null 时返回 false（显示占位 —）', async () => {
    const statsNoLast = { ...sampleStats, last: null }
    vi.mocked(endpoints.stats).mockResolvedValue(statsNoLast as unknown as Stats)
    vi.mocked(endpoints.history).mockResolvedValue([])
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

  it('模板：指标口径含「上限 20，重启后清空」', async () => {
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    vi.mocked(endpoints.history).mockResolvedValue(sampleHistory)
    const w = mount(OverviewPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.text()).toContain('上限 20，服务重启后清空')
    w.unmount()
  })

  it('模板：stats.last 为 null 时显示占位 —', async () => {
    const statsNoLast = { ...sampleStats, last: null }
    vi.mocked(endpoints.stats).mockResolvedValue(statsNoLast as unknown as Stats)
    vi.mocked(endpoints.history).mockResolvedValue([])
    const w = mount(OverviewPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.text()).toContain('—')
    expect(w.text()).toContain('还没有转换记录')
    w.unmount()
  })

  it('模板：history 表格渲染条目', async () => {
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    vi.mocked(endpoints.history).mockResolvedValue(sampleHistory)
    const w = mount(OverviewPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.text()).toContain('clash')
    w.unmount()
  })

  it('模板：history 空状态三段式', async () => {
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    vi.mocked(endpoints.history).mockResolvedValue([])
    const w = mount(OverviewPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.text()).toContain('还没有转换记录')
    expect(w.text()).toContain('在转换页执行一次转换')
    w.unmount()
  })

  it('页面副标题承载 oneLiner；help-panel 入口行=使用说明且详情默认收起', async () => {
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    vi.mocked(endpoints.history).mockResolvedValue(sampleHistory)
    const w = mount(OverviewPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const panel = w.find('[data-testid="help-panel"]')
    expect(panel.exists()).toBe(true)
    expect(w.find('.page-header__subtitle').text()).toContain(pageHelp('overview').oneLiner)
    expect(panel.find('[data-testid="help-toggle-label"]').text()).toBe('使用说明')
    expect(panel.find('[data-testid="help-body"]').exists()).toBe(false)
    w.unmount()
  })

  it('fmtTime：toLocaleString 抛错时回退返回原串', () => {
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    vi.mocked(endpoints.history).mockResolvedValue(sampleHistory)
    const w = mount(OverviewPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as OverviewVm
    const orig = Date.prototype.toLocaleString
    Date.prototype.toLocaleString = function () { throw new Error('boom') }
    try {
      expect(vm.fmtTime('2024-01-01T00:00:00Z')).toBe('2024-01-01T00:00:00Z')
    } finally {
      Date.prototype.toLocaleString = orig
    }
    w.unmount()
  })

  it('模板：展开行显示 failures 与 warnings 原文', async () => {
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    vi.mocked(endpoints.history).mockResolvedValue(sampleHistory)
    const w = mount(OverviewPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const btn = w.find('td.v-data-table__td--expanded-row button')
    await btn.trigger('click')
    await flushPromises()
    const text = w.text()
    expect(text).toContain('failures')
    expect(text).toContain('f1')
    expect(text).toContain('warnings')
    expect(text).toContain('w1')
    w.unmount()
  })

  it('模板：展开行全部成功时显示「本次全部来源成功」', async () => {
    const cleanHistory: HistoryEntry[] = [
      { id: 2, at: '2024-02-02T00:00:00Z', target: 'clash', format: 'clash.meta', nodes: 1, rawNodes: 1, groups: 1, rules: 1, failures: [], warnings: [], cached: 1, urlCount: 1 },
    ]
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    vi.mocked(endpoints.history).mockResolvedValue(cleanHistory)
    const w = mount(OverviewPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const btn2 = w.find('td.v-data-table__td--expanded-row button')
    await btn2.trigger('click')
    await flushPromises()
    expect(w.text()).toContain('本次全部来源成功，无跳过项')
    w.unmount()
  })

  it('模板：history 表格渲染 at 时间', async () => {
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    vi.mocked(endpoints.history).mockResolvedValue(sampleHistory)
    const w = mount(OverviewPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.text()).toContain('2024')
    w.unmount()
  })

})