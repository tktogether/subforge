import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import OverviewPage from './OverviewPage.vue'
import { endpoints, type Stats, type HistoryEntry, type Overview } from '../api/endpoints'
import { pageHelp } from '../content/help'

vi.mock('../api/endpoints', () => ({
  endpoints: {
    overview: vi.fn(),
    stats: vi.fn(),
    history: vi.fn(),
  },
}))

const vuetify = createVuetify({ components, directives })

interface OverviewVm {
  loading: boolean
  error: string
  overview: Overview | null
  stats: Stats | null
  history: HistoryEntry[]
  load: () => Promise<void>
  empty: () => boolean
  expanded: string[]
  fmtTime: (at: string) => string
  uptimeText: () => string
  versionText: (v: string) => string
  scaleText: () => string
  countText: (n: number | undefined) => string
}

describe('OverviewPage', () => {
  const sampleStats: Stats = {
    history: { total: 5, cachedHits: 2, urlTotal: 10, cacheHitRate: 0.4 },
    logs: 3,
    last: { nodes: 1, rawNodes: 2, groups: 3, rules: 4, cached: 1, urlCount: 2, cacheHitRate: 0.5 },
  }

  const sampleHistory: HistoryEntry[] = [
    { id: 1, at: '2024-01-01T00:00:00Z', target: 'clash', format: 'clash.meta', nodes: 2, rawNodes: 3, groups: 1, rules: 1, failures: ['f1'], warnings: ['w1'], cached: 0, urlCount: 1 },
  ]

  const sampleOverview: Overview = {
    name: 'subforge',
    version: 'subforge dev (commit none, built unknown)',
    startedAt: new Date(Date.now() - 30 * 60000).toISOString(),
    uptime: '30m',
    counts: {
      airports: 2,
      airports_enabled: 2,
      airport_nodes: 70,
      self_nodes: 0,
      self_nodes_enabled: 0,
      profiles: 1,
      profiles_enabled: 1,
      tokens: 1,
      groups: 36,
      rules: 1,
      rules_enabled: 1,
      rule_sources: 10,
      rule_sources_ready: 9,
      rule_source_rules: 10442,
    },
  }

  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(endpoints.overview).mockResolvedValue(sampleOverview)
    vi.mocked(endpoints.stats).mockResolvedValue(sampleStats)
    vi.mocked(endpoints.history).mockResolvedValue(sampleHistory)
  })

  it('onMounted 并发加载 overview、stats 与 history', async () => {
    const w = mount(OverviewPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as OverviewVm
    await flushPromises()
    expect(endpoints.overview).toHaveBeenCalledTimes(1)
    expect(endpoints.stats).toHaveBeenCalledTimes(1)
    expect(endpoints.history).toHaveBeenCalledTimes(1)
    expect(vm.overview).toEqual(sampleOverview)
    expect(vm.stats).toEqual(sampleStats)
    expect(vm.history).toEqual(sampleHistory)
    expect(vm.loading).toBe(false)
    expect(vm.error).toBe('')
    w.unmount()
  })

  it('load 失败：error 为 errorMessage(err)，不展示旧数据', async () => {
    vi.mocked(endpoints.overview).mockRejectedValue(new Error('overview boom'))
    const w = mount(OverviewPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as OverviewVm
    await flushPromises()
    expect(vm.error).toBe('overview boom')
    expect(vm.loading).toBe(false)
    expect(vm.overview).toBeNull()
    w.unmount()
  })

  it('empty()：overview 为 null 时返回 true', () => {
    const w = mount(OverviewPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as OverviewVm
    expect(vm.empty()).toBe(true)
    w.unmount()
  })

  it('empty()：overview 加载后返回 false', async () => {
    const w = mount(OverviewPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as OverviewVm
    await flushPromises()
    expect(vm.empty()).toBe(false)
    w.unmount()
  })

  it('资源概览渲染持久化数量与启用数', async () => {
    const w = mount(OverviewPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.find('[data-testid="block-resources"]').exists()).toBe(true)
    expect(w.find('[data-testid="resource-airports"]').text()).toBe('2')
    expect(w.find('[data-testid="resource-airport-nodes"]').text()).toBe('70')
    expect(w.find('[data-testid="resource-self-nodes"]').text()).toBe('0')
    expect(w.find('[data-testid="resource-profiles"]').text()).toBe('1')
    expect(w.find('[data-testid="resource-tokens"]').text()).toBe('1')
    expect(w.find('[data-testid="resource-groups"]').text()).toBe('36')
    expect(w.find('[data-testid="resource-rules"]').text()).toBe('1')
    expect(w.find('[data-testid="resource-rule-sources"]').text()).toBe('10')
    expect(w.find('[data-testid="resource-rule-source-rules"]').text()).toBe('10442')
    expect(w.text()).toContain('启用 2 个')
    expect(w.text()).toContain('有产物 9 个')
    w.unmount()
  })

  it('模板：最近一次转换存在时渲染节点/分组/规则/缓存', async () => {
    const w = mount(OverviewPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.find('[data-testid="conversion-metrics"]').exists()).toBe(true)
    expect(w.find('[data-testid="metric-nodes"]').text()).toBe('1')
    expect(w.find('[data-testid="metric-raw-nodes"]').text()).toBe('2')
    expect(w.find('[data-testid="metric-groups"]').text()).toBe('3')
    expect(w.find('[data-testid="metric-rules"]').text()).toBe('4')
    expect(w.find('[data-testid="metric-last-cached"]').text()).toBe('1/2')
    expect(w.text()).toContain('最近记录 5 条 · 日志缓冲 3 行')
    w.unmount()
  })

  it('模板：本次启动没有转换记录时显示说明而非 0/— 指标', async () => {
    vi.mocked(endpoints.stats).mockResolvedValue({ ...sampleStats, last: null })
    vi.mocked(endpoints.history).mockResolvedValue([])
    const w = mount(OverviewPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.find('[data-testid="conversion-metrics"]').exists()).toBe(false)
    expect(w.find('[data-testid="no-records-hint"]').text()).toContain('本次启动还没有转换记录')
    expect(w.find('[data-testid="history-empty"]').exists()).toBe(true)
    w.unmount()
  })

  it('模板：history 表格渲染条目与时间', async () => {
    const w = mount(OverviewPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.text()).toContain('clash')
    expect(w.text()).toContain('2024')
    w.unmount()
  })

  it('模板：history 空状态三段式', async () => {
    vi.mocked(endpoints.history).mockResolvedValue([])
    const w = mount(OverviewPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.text()).toContain('还没有转换记录')
    expect(w.text()).toContain('在转换页执行一次转换')
    expect(w.find('[data-testid="history-empty-action"]').exists()).toBe(true)
    w.unmount()
  })

  it('服务状态卡展示时长、启动时间、版本与数据规模', async () => {
    const w = mount(OverviewPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.find('[data-testid="service-status"]').text()).toContain('正常')
    expect(w.find('[data-testid="service-uptime"]').text()).toContain('30 分钟')
    expect(w.find('[data-testid="service-started"]').text()).toContain('2026')
    expect(w.find('[data-testid="service-version"]').text()).toContain('开发版')
    expect(w.find('[data-testid="service-scale"]').text()).toContain('2 个机场 · 36 个分组 · 10 个规则源')
    w.unmount()
  })

  it('页面副标题承载 oneLiner；help-panel 入口行=使用说明且详情默认收起', async () => {
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
    vi.mocked(endpoints.history).mockResolvedValue(cleanHistory)
    const w = mount(OverviewPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const btn2 = w.find('td.v-data-table__td--expanded-row button')
    await btn2.trigger('click')
    await flushPromises()
    expect(w.text()).toContain('本次全部来源成功，无跳过项')
    w.unmount()
  })

  it('辅助函数覆盖：uptime、version、scale 与 count 的边界', async () => {
    const w = mount(OverviewPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as OverviewVm
    await flushPromises()
    expect(vm.countText(undefined)).toBe('—')
    expect(vm.versionText('')).toBe('开发版')
    expect(vm.versionText('subforge 0.2.1')).toBe('0.2.1')
    expect(vm.versionText('0.2.1')).toBe('0.2.1')

    vm.overview = { ...sampleOverview, startedAt: new Date(Date.now() - 120 * 60000).toISOString() }
    expect(vm.uptimeText()).toContain('2 小时')
    vm.overview = { ...sampleOverview, startedAt: new Date(Date.now() - 25 * 3600000).toISOString() }
    expect(vm.uptimeText()).toContain('1 天')
    vm.overview = { ...sampleOverview, startedAt: new Date(Date.now() - 10000).toISOString() }
    expect(vm.uptimeText()).toBe('不到 1 分钟')
    vm.overview = { ...sampleOverview, startedAt: 'bad', uptime: 'fallback' }
    expect(vm.uptimeText()).toBe('fallback')
    vm.overview = null
    expect(vm.uptimeText()).toBe('—')
    expect(vm.scaleText()).toBe('—')

    vm.overview = { ...sampleOverview, counts: { ...sampleOverview.counts, airports: 0, groups: 0, rule_sources: 0 } }
    expect(vm.scaleText()).toBe('暂无资源')
    w.unmount()
  })
})
