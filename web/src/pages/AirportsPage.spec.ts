import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import AirportsPage from './AirportsPage.vue'
import { endpoints } from '../api/endpoints'

vi.mock('../api/endpoints', () => ({
  endpoints: {
    listAirports: vi.fn(),
    createAirport: vi.fn(),
    updateAirport: vi.fn(),
    deleteAirport: vi.fn(),
    refreshAirport: vi.fn(),
    importAirportRules: vi.fn(),
    refreshAllAirports: vi.fn(),
    checkAirports: vi.fn(),
    lastAirportCheck: vi.fn(),
  },
}))

const vuetify = createVuetify({ components, directives })

const listMock = endpoints.listAirports as unknown as ReturnType<typeof vi.fn>
const createMock = endpoints.createAirport as unknown as ReturnType<typeof vi.fn>
const updateMock = endpoints.updateAirport as unknown as ReturnType<typeof vi.fn>
const deleteMock = endpoints.deleteAirport as unknown as ReturnType<typeof vi.fn>
const refreshMock = endpoints.refreshAirport as unknown as ReturnType<typeof vi.fn>
const importMock = endpoints.importAirportRules as unknown as ReturnType<typeof vi.fn>
const refreshAllMock = endpoints.refreshAllAirports as unknown as ReturnType<typeof vi.fn>
const checkMock = endpoints.checkAirports as unknown as ReturnType<typeof vi.fn>
const lastCheckMock = endpoints.lastAirportCheck as unknown as ReturnType<typeof vi.fn>

function makeAirport(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    name: 'HK',
    subscription_url: 'https://x.com/sub',
    remarks: 'r',
    enabled: true,
    sort_order: 0,
    upload: 100,
    download: 200,
    total: 1000,
    expire: 1700000000,
    node_count: 5,
    traffic_at: 1700000000,
    traffic_err: '',
    ...overrides,
  }
}

function makeCheckResult(overrides: Record<string, unknown> = {}) {
  return {
    url: 'https://x.com/sub',
    ok: true,
    latency: 123,
    bytes: 0,
    status_code: 200,
    err: '',
    ...overrides,
  }
}

beforeEach(() => {
  vi.clearAllMocks()
  Object.defineProperty(window, 'visualViewport', { value: { width: 1024, height: 768, scale: 1, offsetLeft: 0, offsetTop: 0, addEventListener: vi.fn(), removeEventListener: vi.fn() }, writable: true, configurable: true })
  listMock.mockResolvedValue([makeAirport()])
  createMock.mockResolvedValue(makeAirport({ id: 2, name: 'new' }))
  updateMock.mockResolvedValue(makeAirport())
  deleteMock.mockResolvedValue({ ok: true })
  refreshMock.mockResolvedValue({ result: { ok: true, err: '' } })
  importMock.mockResolvedValue({ rules_added: 1, rules_skipped: 2, providers_added: 1, warnings: [] })
  refreshAllMock.mockResolvedValue({ ok: 1, fail: 0, imports: [{ airport_id: 1, name: 'HK', rules_added: 1, rules_skipped: 0, providers_added: 1, warnings: [], err: '' }] })
  checkMock.mockResolvedValue({ results: [makeCheckResult()] })
  lastCheckMock.mockResolvedValue({ results: [makeCheckResult()] })
})

describe('AirportsPage', () => {
  it('onMounted 加载 listAirports 渲染表格与 testids', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(listMock).toHaveBeenCalledTimes(1)
    expect(lastCheckMock).toHaveBeenCalledTimes(1)
    expect(w.find('[data-testid="help-panel"]').exists()).toBe(true)
    expect(w.find('[data-testid="table"]').exists()).toBe(true)
    expect(w.text()).toContain('HK')
    expect(w.find('[data-testid="traffic-unknown"]').exists()).toBe(false)
    w.unmount()
  })

  it('加载失败显示错误，点击重试重新加载', async () => {
    listMock.mockRejectedValueOnce(new Error('boom'))
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.find('[data-testid="async-error"]').exists()).toBe(true)
    expect(w.text()).toContain('boom')
    await w.find('[data-testid="async-retry"]').trigger('click')
    await flushPromises()
    expect(listMock).toHaveBeenCalledTimes(2)
    w.unmount()
  })

  it('空列表渲染 empty-state', async () => {
    listMock.mockResolvedValue([])
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.find('[data-testid="empty-state"]').exists()).toBe(true)
    w.unmount()
  })

  it('traffic-unknown 与 traffic-at 与 expire-unknown 渲染', async () => {
    listMock.mockResolvedValue([makeAirport({ total: 0, expire: 0, traffic_at: 0, traffic_err: '' })])
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.find('[data-testid="traffic-unknown"]').exists()).toBe(true)
    expect(w.find('[data-testid="traffic-at"]').exists()).toBe(true)
    expect(w.find('[data-testid="traffic-at"]').text()).toBe('从未采集')
    expect(w.find('[data-testid="traffic-stale"]').exists()).toBe(false)
    expect(w.find('[data-testid="expire-unknown"]').exists()).toBe(true)
    w.unmount()
  })

  it('traffic-error 短文案 + title 悬浮完整原因', async () => {
    listMock.mockResolvedValue([makeAirport({ traffic_err: '拉取失败：connection refused' })])
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const el = w.find('[data-testid="traffic-error"]')
    expect(el.exists()).toBe(true)
    expect(el.text()).toContain('余额未更新')
    expect(el.attributes('title')).toContain('拉取失败：connection refused')
    expect(el.attributes('title')).toContain('余额仍是上次成功值，没有被清零')
    w.unmount()
  })

  it('traffic-stale 显示绝对时间与相对时间', async () => {
    const now = Math.floor(Date.now() / 1000)
    listMock.mockResolvedValue([makeAirport({ traffic_at: now - 120, traffic_err: '' })])
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const el = w.find('[data-testid="traffic-stale"]')
    expect(el.exists()).toBe(true)
    expect(el.text()).toContain('上次成功：')
    expect(el.text()).toContain('分钟前')
    expect(w.find('[data-testid="traffic-at"]').exists()).toBe(false)
    w.unmount()
  })

  it('relativeTime 三档：分钟 / 小时 / 天', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const now = Math.floor(Date.now() / 1000)
    expect((w.vm as any).relativeTime(now - 120)).toContain('分钟')
    expect((w.vm as any).relativeTime(now - 7200)).toContain('小时')
    expect((w.vm as any).relativeTime(now - 3 * 86400)).toContain('天')
    w.unmount()
  })

  it('traffic-err 色点+采集失败，title 悬浮原文', async () => {
    listMock.mockResolvedValue([makeAirport({ traffic_err: 'net down' })])
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.find('[data-testid="traffic-err"]').text()).toContain('采集失败')
    expect(w.find('[data-testid="traffic-err"]').attributes('title')).toContain('net down')
    w.unmount()
  })

  it('lastAirportCheck 恢复失败静默', async () => {
    lastCheckMock.mockRejectedValueOnce(new Error('no'))
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    w.unmount()
  })

  it('新增机场对话框提交 createAirport', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await w.find('[data-testid="add"]').trigger('click')
    await flushPromises()
    ;(w.vm as any).draft = { name: 'new', subscription_url: 'https://y.com', remarks: 'r2', enabled: true }
    await (w.vm as any).submit()
    await flushPromises()
    expect(createMock).toHaveBeenCalledTimes(1)
    expect(listMock).toHaveBeenCalledTimes(2)
    w.unmount()
  })

  it('编辑机场对话框提交 updateAirport', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    ;(w.vm as any).openEdit((w.vm as any).rows[0])
    await flushPromises()
    ;(w.vm as any).draft = { name: 'edited', subscription_url: 'https://z.com', remarks: 'r3', enabled: false }
    await (w.vm as any).submit()
    await flushPromises()
    expect(updateMock).toHaveBeenCalledWith(1, expect.any(Object))
    w.unmount()
  })

  it('取消新增对话框', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    ;(w.vm as any).dialog = true
    await flushPromises()
    ;(w.vm as any).dialog = false
    await flushPromises()
    expect((w.vm as any).dialog).toBe(false)
    w.unmount()
  })

  it('删除确认对话框确认后调用 deleteAirport', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    ;(w.vm as any).askDelete((w.vm as any).rows[0])
    await flushPromises()
    expect((w.vm as any).confirmOpen).toBe(true)
    await (w.vm as any).doDelete()
    await flushPromises()
    expect(deleteMock).toHaveBeenCalledWith(1)
    expect(listMock).toHaveBeenCalledTimes(2)
    w.unmount()
  })

  it('取消删除对话框', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    ;(w.vm as any).pendingDelete = (w.vm as any).rows[0]
    ;(w.vm as any).confirmOpen = true
    ;(w.vm as any).confirmOpen = false
    await flushPromises()
    expect((w.vm as any).confirmOpen).toBe(false)
    w.unmount()
  })

  it('删除时 pendingDelete 为 null 直接返回', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    ;(w.vm as any).pendingDelete = null
    ;(w.vm as any).confirmOpen = true
    await (w.vm as any).doDelete()
    await flushPromises()
    expect(deleteMock).not.toHaveBeenCalled()
    w.unmount()
  })

  it('刷新单个机场流量', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    ;(w.vm as any).refreshOne((w.vm as any).rows[0])
    await flushPromises()
    expect(refreshMock).toHaveBeenCalledWith(1)
    expect(listMock).toHaveBeenCalledTimes(2)
    w.unmount()
  })

  it('刷新单个机场流量失败', async () => {
    refreshMock.mockRejectedValueOnce(new Error('rerr'))
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    ;(w.vm as any).refreshOne((w.vm as any).rows[0])
    await flushPromises()
    expect(refreshMock).toHaveBeenCalled()
    w.unmount()
  })

  it('刷新失败 notify（result.ok=false）', async () => {
    refreshMock.mockResolvedValueOnce({ result: { ok: false, err: 'bad' } })
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    ;(w.vm as any).refreshOne((w.vm as any).rows[0])
    await flushPromises()
    expect(refreshMock).toHaveBeenCalled()
    w.unmount()
  })

  it('导入规则', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    ;(w.vm as any).importRules((w.vm as any).rows[0])
    await flushPromises()
    expect(importMock).toHaveBeenCalledWith(1)
    expect(listMock).toHaveBeenCalledTimes(2)
    w.unmount()
  })

  it('导入规则失败', async () => {
    importMock.mockRejectedValueOnce(new Error('ierr'))
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    ;(w.vm as any).importRules((w.vm as any).rows[0])
    await flushPromises()
    expect(importMock).toHaveBeenCalled()
    w.unmount()
  })

  it('checkOne 调用 checkAirports([id])', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await (w.vm as any).checkOne((w.vm as any).rows[0])
    await flushPromises()
    expect(checkMock).toHaveBeenCalledWith([1])
    w.unmount()
  })

  it('checkAll 调用 checkAirports(undefined)', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await (w.vm as any).checkAll()
    await flushPromises()
    expect(checkMock).toHaveBeenCalledWith(undefined)
    w.unmount()
  })

  it('checkAirports 失败写入 checkError', async () => {
    checkMock.mockRejectedValueOnce(new Error('cerr'))
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await (w.vm as any).checkAll()
    await flushPromises()
    expect(w.find('[data-testid="check-error"]').exists()).toBe(true)
    expect(w.find('[data-testid="check-error"]').text()).toContain('cerr')
    w.unmount()
  })

  it('checkResults 渲染 可用 与 不可用', async () => {
    checkMock.mockResolvedValueOnce({ results: [makeCheckResult({ ok: true }), makeCheckResult({ url: 'https://y.com', ok: false, err: 'timeout' })] })
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await (w.vm as any).checkAll()
    await flushPromises()
    expect(w.find('[data-testid="check-results"]').exists()).toBe(true)
    expect(w.findAll('[data-testid="check-result-row"]').length).toBe(2)
    expect(w.text()).toContain('可用')
    expect(w.text()).toContain('不可用')
    expect(w.text()).toContain('timeout')
    w.unmount()
  })

  it('refreshAll 成功渲染 summary 与 imports', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    ;(w.vm as any).withRules = true
    await (w.vm as any).refreshAll()
    await flushPromises()
    expect(refreshAllMock).toHaveBeenCalledWith(true)
    expect(w.find('[data-testid="refresh-summary"]').exists()).toBe(true)
    expect(w.find('[data-testid="imports"]').exists()).toBe(true)
    expect(w.text()).toContain('提供者')
    w.unmount()
  })

  it('refreshAll 结果表逐行显示 ok 与 err 原文', async () => {
    refreshAllMock.mockResolvedValueOnce({
      ok: 1,
      fail: 1,
      results: [
        { airport_id: 1, name: 'HK', ok: true, usage: { upload: '1', download: '2', total: '3', expire: '4' }, node_count: 5, err: '' },
        { airport_id: 2, name: 'JP', ok: false, usage: { upload: '', download: '', total: '', expire: '' }, node_count: 0, err: '拉取失败：timeout' },
      ],
      imports: [],
    })
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await (w.vm as any).refreshAll()
    await flushPromises()
    const rows = w.findAll('[data-testid="refresh-result-row"]')
    expect(rows.length).toBe(2)
    expect(rows[0].text()).toContain('HK')
    expect(rows[0].text()).toContain('成功')
    expect(rows[1].text()).toContain('JP')
    expect(rows[1].text()).toContain('失败')
    expect(rows[1].text()).toContain('拉取失败：timeout')
    w.unmount()
  })

  it('refreshOne 结果表在成功但有说明时显示 note，且不按失败渲染', async () => {
    refreshMock.mockResolvedValueOnce({
      result: {
        airport_id: 1, name: 'HK', ok: true,
        usage: { upload: '', download: '', total: '', expire: '' }, node_count: 12,
        err: '', note: '上游未提供 subscription-userinfo：本次只更新了节点数，流量信息保持未知',
      },
    })
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await (w.vm as any).refreshOne((w.vm as any).rows[0])
    await flushPromises()
    const rows = w.findAll('[data-testid="refresh-result-row"]')
    expect(rows.length).toBe(1)
    expect(rows[0].text()).toContain('成功')
    const note = w.find('[data-testid="refresh-result-note"]')
    expect(note.exists()).toBe(true)
    expect(note.text()).toContain('subscription-userinfo')
    w.unmount()
  })

  it('refreshOne 结果表显示单条 ok 与 err', async () => {
    refreshMock.mockResolvedValueOnce({ result: { airport_id: 1, name: 'HK', ok: false, usage: { upload: '', download: '', total: '', expire: '' }, node_count: 0, err: '单机场失败' } })
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await (w.vm as any).refreshOne((w.vm as any).rows[0])
    await flushPromises()
    const rows = w.findAll('[data-testid="refresh-result-row"]')
    expect(rows.length).toBe(1)
    expect(rows[0].text()).toContain('失败')
    expect(rows[0].text()).toContain('单机场失败')
    w.unmount()
  })

  it('refreshAll 失败写入 refreshError', async () => {
    refreshAllMock.mockRejectedValueOnce(new Error('503 转换引擎未启用'))
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await (w.vm as any).refreshAll()
    await flushPromises()
    expect(w.find('[data-testid="refresh-error"]').exists()).toBe(true)
    expect(w.find('[data-testid="refresh-error"]').text()).toContain('转换引擎未启用')
    w.unmount()
  })

  it('refreshAll 默认 withRules=false', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await (w.vm as any).refreshAll()
    await flushPromises()
    expect(refreshAllMock).toHaveBeenCalledWith(false)
    w.unmount()
  })

  it('imports 含 err 渲染', async () => {
    refreshAllMock.mockResolvedValueOnce({ ok: 1, fail: 1, imports: [{ airport_id: 1, name: 'HK', rules_added: 0, rules_skipped: 0, providers_added: 0, warnings: ['w'], err: 'boom' }] })
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await (w.vm as any).refreshAll()
    await flushPromises()
    expect(w.text()).toContain('boom')
    w.unmount()
  })

  it('with-rules 开关渲染', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.find('[data-testid="with-rules"]').exists()).toBe(true)
    w.unmount()
  })

  it('W8A 开关在 table 之前渲染', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const html = w.html()
    const switchIdx = html.indexOf('data-testid="with-rules"')
    const tableIdx = html.indexOf('data-testid="table"')
    expect(switchIdx).toBeGreaterThan(-1)
    expect(tableIdx).toBeGreaterThan(-1)
    expect(switchIdx).toBeLessThan(tableIdx)
    w.unmount()
  })

  it('pageHelp oneLiner 渲染在 PageHeader subtitle', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.text()).toContain('管理上游订阅地址')
    w.unmount()
  })

  it('help-panel 渲染', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.find('[data-testid="help-panel"]').exists()).toBe(true)
    w.unmount()
  })

  it('restoreLastCheck 成功写入 checkResults', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect((w.vm as any).checkResults.length).toBe(1)
    w.unmount()
  })

  it('dialog v-model setter coverage', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify], stubs: { teleport: true } }, attachTo: document.body })
    await flushPromises()
    ;(w.vm as any).dialog = true
    await flushPromises()
    await new Promise(r => setTimeout(r, 50))
    await flushPromises()
    ;(w.vm as any).dialog = false
    await flushPromises()
    w.unmount()
  })

  it('confirmOpen v-model setter coverage', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify], stubs: { teleport: true } }, attachTo: document.body })
    await flushPromises()
    ;(w.vm as any).confirmOpen = true
    await flushPromises()
    await new Promise(r => setTimeout(r, 50))
    await flushPromises()
    ;(w.vm as any).confirmOpen = false
    await flushPromises()
    w.unmount()
  })

  it('submit 失败 notify', async () => {
    createMock.mockRejectedValueOnce(new Error('crr'))
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    ;(w.vm as any).editing = null
    ;(w.vm as any).draft = { name: 'x', subscription_url: 'y', remarks: 'z', enabled: true }
    await (w.vm as any).submit()
    await flushPromises()
    expect(createMock).toHaveBeenCalled()
    w.unmount()
  })

  it('openCreate 重置 draft', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    ;(w.vm as any).draft = { name: 'x', subscription_url: 'y', remarks: 'z', enabled: false }
    ;(w.vm as any).openCreate()
    await flushPromises()
    expect((w.vm as any).draft.name).toBe('')
    expect((w.vm as any).editing).toBeNull()
    expect((w.vm as any).dialog).toBe(true)
    w.unmount()
  })

  it('openEdit 填充 draft', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    ;(w.vm as any).openEdit((w.vm as any).rows[0])
    await flushPromises()
    expect((w.vm as any).draft.name).toBe('HK')
    expect((w.vm as any).editing).not.toBeNull()
    expect((w.vm as any).dialog).toBe(true)
    w.unmount()
  })

  it('doDelete 失败 notify', async () => {
    deleteMock.mockRejectedValueOnce(new Error('derr'))
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    ;(w.vm as any).pendingDelete = (w.vm as any).rows[0]
    ;(w.vm as any).confirmOpen = true
    await (w.vm as any).doDelete()
    await flushPromises()
    expect(deleteMock).toHaveBeenCalled()
    w.unmount()
  })

  it('trafficText 与 expireText 与 relativeTime 覆盖', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const row = (w.vm as any).rows[0]
    expect((w.vm as any).trafficText(row)).toContain('/')
    expect((w.vm as any).expireText(row)).not.toBe('未知')
    expect((w.vm as any).relativeTime(row.traffic_at)).not.toBe('')
    w.unmount()
  })

  it('submit 编辑分支', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    ;(w.vm as any).editing = (w.vm as any).rows[0]
    ;(w.vm as any).draft = { name: 'ed', subscription_url: 'https://z.com', remarks: 'r', enabled: true }
    await (w.vm as any).submit()
    await flushPromises()
    expect(updateMock).toHaveBeenCalled()
    w.unmount()
  })

  it('UI 点击编辑按钮打开对话框', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await w.find('[data-testid="edit"]').trigger('click')
    await flushPromises()
    expect((w.vm as any).dialog).toBe(true)
    w.unmount()
  })

  it('UI 点击删除按钮打开确认', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await w.find('[data-testid="delete"]').trigger('click')
    await flushPromises()
    expect((w.vm as any).confirmOpen).toBe(true)
    w.unmount()
  })

  it('UI 点击刷新流量按钮', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await w.find('[data-testid="refresh-traffic"]').trigger('click')
    await flushPromises()
    expect(refreshMock).toHaveBeenCalled()
    w.unmount()
  })

  it('UI 点击导入规则按钮', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await w.find('[data-testid="import-rules"]').trigger('click')
    await flushPromises()
    expect(importMock).toHaveBeenCalled()
    w.unmount()
  })

  it('UI 点击测活按钮', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await w.find('[data-testid="check-one"]').trigger('click')
    await flushPromises()
    expect(checkMock).toHaveBeenCalled()
    w.unmount()
  })

  it('UI with-rules 开关切换', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const input = w.find('[data-testid="with-rules"] input')
    await input.setValue(true)
    await flushPromises()
    expect((w.vm as any).withRules).toBe(true)
    w.unmount()
  })

  it('UI 对话框 v-model 与取消按钮', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify] }, attachTo: document.body })
    await flushPromises()
    await w.find('[data-testid="add"]').trigger('click')
    await flushPromises()
    const dialogs = w.findAllComponents({ name: 'VDialog' })
    dialogs[0].vm.$emit('update:modelValue', true)
    await flushPromises()
    w.findAllComponents({ name: 'VTextField' }).forEach(f => f.vm.$emit('update:modelValue', 'zzz'))
    w.findAllComponents({ name: 'VSwitch' }).forEach(f => f.vm.$emit('update:modelValue', true))
    await flushPromises()
    const cancel = Array.from(document.body.querySelectorAll('button')).find(b => (b.textContent || '').trim() === '取消')
    expect(cancel).toBeTruthy()
    ;(cancel as HTMLElement).click()
    await flushPromises()
    expect((w.vm as any).dialog).toBe(false)
    w.unmount()
  })

  it('UI 确认对话框 v-model 与取消按钮', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify] }, attachTo: document.body })
    await flushPromises()
    ;(w.vm as any).askDelete((w.vm as any).rows[0])
    await flushPromises()
    const dialogs = w.findAllComponents({ name: 'VDialog' })
    dialogs[dialogs.length - 1].vm.$emit('update:modelValue', true)
    await flushPromises()
    const cancel = Array.from(document.body.querySelectorAll('button')).find(b => (b.textContent || '').trim() === '取消')
    expect(cancel).toBeTruthy()
    ;(cancel as HTMLElement).click()
    await flushPromises()
    expect((w.vm as any).confirmOpen).toBe(false)
    w.unmount()
  })

  it('UI 确认对话框删除按钮', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify] }, attachTo: document.body })
    await flushPromises()
    ;(w.vm as any).askDelete((w.vm as any).rows[0])
    await flushPromises()
    const dialogs = w.findAllComponents({ name: 'VDialog' })
    dialogs[dialogs.length - 1].vm.$emit('update:modelValue', true)
    await flushPromises()
    const del = Array.from(document.body.querySelectorAll('button')).find(b => (b.textContent || '').trim() === '删除')
    expect(del).toBeTruthy()
    ;(del as HTMLElement).click()
    await flushPromises()
    w.unmount()
  })

  it('with-rules 开关有 hint 文案', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.find('[data-testid="with-rules"]').exists()).toBe(true)
    // '全部刷新' 只出现在 airport.with_rules 的 hint 文案里（按钮是「刷新全部流量」），命中即证明 hint 渲染
    expect(w.text()).toContain('全部刷新')
    w.unmount()
  })

  it('行内开关调 updateAirport', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const target = w.findAllComponents({ name: 'VSwitch' })
      .find(s => s.attributes('data-testid') === 'airport-enabled')
    expect(target).toBeTruthy()
    await target!.vm.$emit('update:modelValue', false)
    await flushPromises()
    expect(updateMock).toHaveBeenCalledWith(1, expect.objectContaining({ enabled: false }))
    w.unmount()
  })

  it('行内开关失败：notify error 且 toggling 归位（T9 补 catch 分支）', async () => {
    updateMock.mockRejectedValueOnce(new Error('toggle fail'))
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const row = (w.vm as unknown as { rows: { id: number; name: string; subscription_url: string; remarks: string; enabled: boolean }[] }).rows[0]
    await (w.vm as unknown as { toggleEnabled: (r: unknown, v: boolean) => Promise<void> }).toggleEnabled(row, false)
    await flushPromises()
    expect(updateMock).toHaveBeenCalledWith(row.id, expect.objectContaining({ enabled: false }))
    expect((w.vm as unknown as { toggling: number | null }).toggling).toBeNull()
    w.unmount()
  })

})
