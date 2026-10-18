import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'
import LogsPage from './LogsPage.vue'
import { endpoints } from '../api/endpoints'
import { vuetify } from '../plugins/vuetify'

vi.mock('../api/endpoints', async () => {
  const actual = await vi.importActual<any>('../api/endpoints')
  return {
    ...actual,
    endpoints: { logs: vi.fn() },
  }
})

const logsMock = endpoints.logs as unknown as ReturnType<typeof vi.fn>

describe('LogsPage', () => {
  let intervals: Array<() => void> = []
  let seq = 1

  beforeEach(() => {
    intervals = []
    seq = 1
    vi.clearAllMocks()
    // 捕获 onMounted 注册的轮询回调，便于确定性触发；避免真实 2s 等待
    vi.spyOn(globalThis, 'setInterval').mockImplementation(((fn: any) => {
      intervals.push(fn as () => void)
      return seq++
    }) as any)
    vi.spyOn(globalThis, 'clearInterval').mockImplementation((() => {}) as any)
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  function tickPoll(): void {
    // 触发一次 setInterval 回调（LogsPage.vue:37 的箭头函数）
    expect(intervals.length).toBeGreaterThan(0)
    intervals[0]()
  }

  function button(w: ReturnType<typeof mount>, text: string) {
    const b = w.findAll('button').find((x) => x.text().includes(text))
    if (!b) throw new Error('button not found: ' + text)
    return b
  }

  it('onMounted 拉取日志并累加，lastId 取最大值；轮询回调再次拉取（id > lastId 为真）', async () => {
    logsMock
      .mockResolvedValueOnce({ logs: [{ id: 1, ts: 't1', line: 'l1' }] })
      .mockResolvedValueOnce({ logs: [{ id: 2, ts: 't2', line: 'l2' }] })

    const w = mount(LogsPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    expect(logsMock).toHaveBeenCalledTimes(1)
    expect(logsMock).toHaveBeenNthCalledWith(1, 0)
    // 有日志：渲染 ts 与 line，不显示空态
    expect(w.text()).toContain('t1 l1')
    expect(w.text()).not.toContain('暂无日志')

    tickPoll()
    await flushPromises()

    expect(logsMock).toHaveBeenCalledTimes(2)
    // lastId 已更新为 1，第二次用 since=1
    expect(logsMock).toHaveBeenNthCalledWith(2, 1)
    expect(w.text()).toContain('t1 l1')
    expect(w.text()).toContain('t2 l2')
    w.unmount()
  })

  it('新条目 id 不大于 lastId 时 lastId 保持不变（id > lastId 为假）', async () => {
    logsMock
      .mockResolvedValueOnce({ logs: [{ id: 5, ts: 'a', line: 'x' }] })
      .mockResolvedValueOnce({ logs: [{ id: 3, ts: 'b', line: 'y' }] })
      .mockResolvedValueOnce({ logs: [] })

    const w = mount(LogsPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    tickPoll()
    await flushPromises()
    expect(logsMock).toHaveBeenNthCalledWith(2, 5)

    tickPoll()
    await flushPromises()
    // id=3 未大于 lastId=5，因此第三次仍然 since=5
    expect(logsMock).toHaveBeenNthCalledWith(3, 5)
    w.unmount()
  })

  it('点击暂停后 paused=true，轮询回调直接 return，不再请求（paused 分支）', async () => {
    logsMock.mockResolvedValue({ logs: [] })

    const w = mount(LogsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(logsMock).toHaveBeenCalledTimes(1)
    expect(button(w, '暂停').exists()).toBe(true)

    await button(w, '暂停').trigger('click')
    await nextTick()
    // 三元的真分支：暂停后按钮文案变为「继续」
    expect(button(w, '继续').exists()).toBe(true)

    tickPoll()
    await flushPromises()
    expect(logsMock).toHaveBeenCalledTimes(1)

    // 再点一次恢复轮询
    await button(w, '继续').trigger('click')
    await nextTick()
    tickPoll()
    await flushPromises()
    expect(logsMock).toHaveBeenCalledTimes(2)
    w.unmount()
  })

  it('点击清屏重置 logs 与 lastId，模板显示空态，之后轮询用 since=0', async () => {
    logsMock.mockResolvedValueOnce({ logs: [{ id: 7, ts: 't7', line: 'l7' }] }).mockResolvedValue({ logs: [] })

    const w = mount(LogsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.text()).toContain('t7 l7')

    await button(w, '清屏').trigger('click')
    await nextTick()
    expect(w.text()).not.toContain('t7 l7')
    expect(w.text()).toContain('暂无日志')

    tickPoll()
    await flushPromises()
    // clear() 把 lastId 归零
    expect(logsMock).toHaveBeenLastCalledWith(0)
    w.unmount()
  })

  it('请求抛 Error 时把 message 写入 error 并渲染（catch + error !== "" 为真）', async () => {
    logsMock.mockRejectedValueOnce(new Error('boom'))

    const w = mount(LogsPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    expect(w.text()).toContain('boom')
    expect(w.find('.text-error').exists()).toBe(true)
    w.unmount()
  })

  it('请求抛非 Error 时用 String(err) 渲染错误', async () => {
    logsMock.mockRejectedValueOnce('plain-failure')

    const w = mount(LogsPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    expect(w.text()).toContain('plain-failure')
    w.unmount()
  })

  it('卸载时清理定时器（timer !== null 为真）', async () => {
    logsMock.mockResolvedValue({ logs: [] })

    const w = mount(LogsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(intervals.length).toBe(1)

    w.unmount()
    expect(clearInterval).toHaveBeenCalledWith(1)
  })
})
