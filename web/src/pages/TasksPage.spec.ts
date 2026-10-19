import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import TasksPage from './TasksPage.vue'
import { endpoints } from '../api/endpoints'
import { pageHelp } from '../content/help'

vi.mock('../api/endpoints', async () => {
  const actual = await vi.importActual<any>('../api/endpoints')
  return { ...actual, endpoints: { ...actual.endpoints, listTasks: vi.fn() } }
})

const listTasksMock = endpoints.listTasks as unknown as ReturnType<typeof vi.fn>
const vuetify = createVuetify({ components, directives })

describe('TasksPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listTasksMock.mockReset()
    Object.defineProperty(window, 'visualViewport', { value: { width: 1024, height: 768, scale: 1, offsetLeft: 0, offsetTop: 0, addEventListener: vi.fn(), removeEventListener: vi.fn() }, writable: true, configurable: true })
  })

  it('onMounted 调用 listTasks 并渲染表格', async () => {
    listTasksMock.mockResolvedValue([
      { JobName: 'rule-sources', Success: true, Err: '', StartedAt: '2026-10-19T04:00:00+08:00', Duration: 1234567 },
      { JobName: 'subscheck', Success: false, Err: 'boom', StartedAt: '2026-10-19T04:30:00+08:00', Duration: 2500000000 },
    ])
    const w = mount(TasksPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(listTasksMock).toHaveBeenCalledTimes(1)
    expect(w.text()).toContain('规则源更新')
    expect(w.text()).toContain('订阅测活')
    expect(w.text()).toContain('成功')
    expect(w.text()).toContain('失败')
    expect(w.text()).toContain('boom')
    expect(w.text()).toContain('1.23 ms')
    expect(w.text()).toContain('2.50 s')
    w.unmount()
  })

  it('JobName 未知时原样显示', async () => {
    listTasksMock.mockResolvedValue([{ JobName: 'custom-job', Success: true, Err: '', StartedAt: '2026-10-19T04:00:00+08:00', Duration: 0 }])
    const w = mount(TasksPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.text()).toContain('custom-job')
    w.unmount()
  })

  it('StartedAt 非法时原样显示', async () => {
    listTasksMock.mockResolvedValue([{ JobName: 'rule-sources', Success: true, Err: '', StartedAt: 'not-a-date', Duration: 1000000 }])
    const w = mount(TasksPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.text()).toContain('not-a-date')
    expect(w.text()).toContain('1.00 ms')
    w.unmount()
  })

  it('Err 为空显示占位', async () => {
    listTasksMock.mockResolvedValue([{ JobName: 'rule-sources', Success: true, Err: '', StartedAt: '2026-10-19T04:00:00+08:00', Duration: 0 }])
    const w = mount(TasksPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.text()).toContain('—')
    w.unmount()
  })

  it('空列表时空态三段式', async () => {
    listTasksMock.mockResolvedValue([])
    const w = mount(TasksPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.text()).toContain('还没有任务执行记录')
    expect(w.text()).toContain('任务按 cron 到点才会执行')
    expect(w.find('[data-testid="empty-state-action"]').exists()).toBe(true)
    w.unmount()
  })

  it('503 渲染引导卡而非红色错误', async () => {
    listTasksMock.mockRejectedValueOnce(new Error('503 调度器未启用'))
    const w = mount(TasksPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.find('[data-testid="scheduler-unavailable"]').exists()).toBe(true)
    expect(w.text()).toContain('调度器未启用')
    expect(w.text()).toContain('当前进程没有装配调度器')
    expect(w.find('[data-testid="async-error"]').exists()).toBe(false)
    w.unmount()
  })

  it('503 重试按钮重新 load', async () => {
    listTasksMock.mockRejectedValueOnce(new Error('503 调度器未启用')).mockResolvedValueOnce([])
    const w = mount(TasksPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await w.find('[data-testid="scheduler-retry"]').trigger('click')
    await flushPromises()
    expect(listTasksMock).toHaveBeenCalledTimes(2)
    w.unmount()
  })

  it('其他错误渲染 async-error', async () => {
    listTasksMock.mockRejectedValueOnce(new Error('boom'))
    const w = mount(TasksPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.find('[data-testid="async-error"]').exists()).toBe(true)
    expect(w.text()).toContain('boom')
    w.unmount()
  })

  it('help-panel 含 oneLiner 片段', async () => {
    listTasksMock.mockResolvedValue([])
    const w = mount(TasksPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.find('[data-testid="help-panel"]').exists()).toBe(true)
    expect(w.find('[data-testid="help-oneliner"]').text()).toContain(pageHelp('tasks').oneLiner)
    w.unmount()
  })

  it('v-alert 说明卡四件事', async () => {
    listTasksMock.mockResolvedValue([])
    const w = mount(TasksPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const t = w.text()
    expect(t).toContain('这里只显示')
    expect(t).toContain('规则源更新')
    expect(t).toContain('订阅测活')
    expect(t).toContain('cron 怎么改')
    expect(t).toContain('手动触发的后果')
    expect(t).toContain('rules.update_cron')
    expect(t).toContain('subscheck.cron')
    expect(t).toContain('0 4 * * *')
    expect(t).toContain('*/30 * * * *')
    w.unmount()
  })

  it('task-retry-note 说明失败重试 3 次指数退避', async () => {
    listTasksMock.mockResolvedValue([])
    const w = mount(TasksPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const note = w.find('[data-testid="task-retry-note"]')
    expect(note.exists()).toBe(true)
    expect(note.text()).toContain('自动重试 3 次')
    expect(note.text()).toContain('指数退避')
    expect(note.text()).toContain('仍失败才告警')
    w.unmount()
  })
})
