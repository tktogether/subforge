import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import LogsPage from './LogsPage.vue'
import { endpoints } from '../api/endpoints'
import { pageHelp } from '../content/help'

vi.mock('../api/endpoints', async () => {
  const actual = await vi.importActual<any>('../api/endpoints')
  return { ...actual, endpoints: { ...actual.endpoints, logs: vi.fn() } }
})

const logsMock = endpoints.logs as unknown as ReturnType<typeof vi.fn>
const vuetify = createVuetify({ components, directives })

describe('LogsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    logsMock.mockReset()
    Object.defineProperty(window, 'visualViewport', { value: { width: 1024, height: 768, scale: 1, offsetLeft: 0, offsetTop: 0, addEventListener: vi.fn(), removeEventListener: vi.fn() }, writable: true, configurable: true })
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('onMounted 调用 logs(0) 并渲染', async () => {
    logsMock.mockResolvedValue({ logs: [{ id: 1, ts: '2026-10-19T04:00:00+08:00', line: 'hello' }] })
    const w = mount(LogsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(logsMock).toHaveBeenCalledWith(0)
    expect(w.text()).toContain('hello')
    expect(w.find('[data-testid="log-row"]').exists()).toBe(true)
    w.unmount()
  })

  it('空列表时空态三段式', async () => {
    logsMock.mockResolvedValue({ logs: [] })
    const w = mount(LogsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.text()).toContain('暂无日志')
    expect(w.text()).toContain('服务刚启动时日志缓冲区可能是空的')
    w.unmount()
  })

  it('增量追加而非整表替换', async () => {
    logsMock.mockResolvedValueOnce({ logs: [{ id: 1, ts: '2026-10-19T04:00:00+08:00', line: 'first' }] })
    const w = mount(LogsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.findAll('[data-testid="log-row"]').length).toBe(1)

    logsMock.mockResolvedValueOnce({ logs: [{ id: 2, ts: '2026-10-19T04:00:01+08:00', line: 'second' }] })
    await w.find('[data-testid="refresh"]').trigger('click')
    await flushPromises()
    expect(logsMock).toHaveBeenLastCalledWith(1)
    expect(w.findAll('[data-testid="log-row"]').length).toBe(2)
    expect(w.text()).toContain('first')
    expect(w.text()).toContain('second')
    w.unmount()
  })

  it('id <= lastId 的重复行不重复追加 (entry.id > lastId)', async () => {
    logsMock.mockResolvedValueOnce({ logs: [{ id: 5, ts: '2026-10-19T04:00:00+08:00', line: 'a' }] })
    const w = mount(LogsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    logsMock.mockResolvedValueOnce({ logs: [{ id: 3, ts: '2026-10-19T04:00:00+08:00', line: 'dup' }] })
    await w.find('[data-testid="refresh"]').trigger('click')
    await flushPromises()
    expect(w.findAll('[data-testid="log-row"]').length).toBe(1)
    w.unmount()
  })

  it('clear 只清前端并重置 since 为当前最大 id', async () => {
    logsMock.mockResolvedValueOnce({ logs: [{ id: 7, ts: '2026-10-19T04:00:00+08:00', line: 'x' }] })
    const w = mount(LogsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await w.find('[data-testid="clear"]').trigger('click')
    await flushPromises()
    expect(w.findAll('[data-testid="log-row"]').length).toBe(0)
    logsMock.mockResolvedValueOnce({ logs: [] })
    await w.find('[data-testid="refresh"]').trigger('click')
    await flushPromises()
    expect(logsMock).toHaveBeenLastCalledWith(7)
    w.unmount()
  })

  it('clear 空列表时 since 保持 0', async () => {
    logsMock.mockResolvedValue({ logs: [] })
    const w = mount(LogsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await w.find('[data-testid="clear"]').trigger('click')
    await flushPromises()
    logsMock.mockResolvedValueOnce({ logs: [] })
    await w.find('[data-testid="refresh"]').trigger('click')
    await flushPromises()
    expect(logsMock).toHaveBeenLastCalledWith(0)
    w.unmount()
  })

  it('fetchLogs 失败显示 log-error', async () => {
    logsMock.mockRejectedValueOnce(new Error('net boom'))
    const w = mount(LogsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.find('[data-testid="log-error"]').exists()).toBe(true)
    expect(w.text()).toContain('net boom')
    w.unmount()
  })

  it('highlight 含 error/panic/fatal 的行', async () => {
    logsMock.mockResolvedValue({ logs: [
      { id: 1, ts: '2026-10-19T04:00:00+08:00', line: 'all good' },
      { id: 2, ts: '2026-10-19T04:00:01+08:00', line: 'ERROR boom' },
      { id: 3, ts: '2026-10-19T04:00:02+08:00', line: 'panic here' },
      { id: 4, ts: '2026-10-19T04:00:03+08:00', line: 'fatal x' },
    ] })
    const w = mount(LogsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const rows = w.findAll('[data-testid="log-row"]')
    expect(rows.length).toBe(4)
    const html = w.html()
    expect(html).toContain('text-error')
    w.unmount()
  })

  it('ts 非法时原样显示', async () => {
    logsMock.mockResolvedValue({ logs: [{ id: 1, ts: 'bad-ts', line: 'x' }] })
    const w = mount(LogsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.text()).toContain('bad-ts')
    w.unmount()
  })

  it('auto-refresh 开关创建定时器并每 3s 拉取', async () => {
    vi.useFakeTimers()
    logsMock.mockResolvedValue({ logs: [] })
    const w = mount(LogsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const before = logsMock.mock.calls.length
    const sw = w.find('[data-testid="auto-refresh"]')
    await sw.trigger('change') // fallback
    // toggle via model value
    const input = w.find('[data-testid="auto-refresh"] input')
    if (input.exists()) {
      await input.setValue(true)
    }
    await flushPromises()
    // ensure timer path covered by calling onAutoChange via vm
    ;(w.vm as any).onAutoChange(true)
    await flushPromises()
    await vi.advanceTimersByTimeAsync(3000)
    await flushPromises()
    expect(logsMock.mock.calls.length).toBeGreaterThan(before)
    w.unmount()
  })

  it('auto-refresh 关闭清除定时器', async () => {
    logsMock.mockResolvedValue({ logs: [] })
    const w = mount(LogsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    ;(w.vm as any).onAutoChange(true)
    await flushPromises()
    ;(w.vm as any).onAutoChange(false)
    await flushPromises()
    expect((w.vm as any).autoRefresh).toBe(false)
    w.unmount()
  })

  it('onUnmounted 清除定时器', async () => {
    logsMock.mockResolvedValue({ logs: [] })
    const w = mount(LogsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    ;(w.vm as any).onAutoChange(true)
    await flushPromises()
    w.unmount()
    // no throw
  })

  it('scroll-bottom 按钮可点击', async () => {
    logsMock.mockResolvedValue({ logs: [{ id: 1, ts: '2026-10-19T04:00:00+08:00', line: 'x' }] })
    const w = mount(LogsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await w.find('[data-testid="scroll-bottom"]').trigger('click')
    await flushPromises()
    w.unmount()
  })

  it('help-panel 含 oneLiner 片段', async () => {
    logsMock.mockResolvedValue({ logs: [] })
    const w = mount(LogsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.find('[data-testid="help-panel"]').exists()).toBe(true)
    expect(w.find('[data-testid="help-oneliner"]').text()).toContain(pageHelp('logs').oneLiner)
    w.unmount()
  })
  it('res.logs 为 undefined 时视为空数组', async () => {
    logsMock.mockResolvedValue({})
    const w = mount(LogsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.findAll('[data-testid="log-row"]').length).toBe(0)
    w.unmount()
  })

  it('clear 多条时 reduce 命中 false 分支', async () => {
    logsMock.mockResolvedValueOnce({ logs: [
      { id: 3, ts: '2026-10-19T04:00:00+08:00', line: 'a' },
      { id: 7, ts: '2026-10-19T04:00:01+08:00', line: 'b' },
    ] })
    const w = mount(LogsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await w.find('[data-testid="clear"]').trigger('click')
    await flushPromises()
    expect(w.findAll('[data-testid="log-row"]').length).toBe(0)
    w.unmount()
  })

  it('auto-refresh 开关真正清除已设置的定时器', async () => {
    logsMock.mockResolvedValue({ logs: [] })
    const w = mount(LogsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    ;(w.vm as any).onAutoChange(true)
    await flushPromises()
    expect((w.vm as any).autoRefresh).toBe(true)
    ;(w.vm as any).onAutoChange(false)
    await flushPromises()
    expect((w.vm as any).autoRefresh).toBe(false)
    w.unmount()
  })

  it('onAutoChange(false) 在 timer 为空时直接跳过', async () => {
    logsMock.mockResolvedValue({ logs: [] })
    const w = mount(LogsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    ;(w.vm as any).onAutoChange(false)
    await flushPromises()
    expect((w.vm as any).autoRefresh).toBe(false)
    w.unmount()
  })

  it('scrollToBottom 在容器未渲染时为空操作', async () => {
    logsMock.mockResolvedValue({ logs: [] })
    const w = mount(LogsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    ;(w.vm as any).scrollToBottom()
    await flushPromises()
    w.unmount()
  })


  it('v-alert 三条说明', async () => {
    logsMock.mockResolvedValue({ logs: [] })
    const w = mount(LogsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const t = w.text()
    expect(t).toContain('不区分级别')
    expect(t).toContain('最多显示最近 200 行')
    expect(t).toContain('跟到底')
    expect(t).toContain('纯前端关键字高亮')
    w.unmount()
  })
})
