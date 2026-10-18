import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import TasksPage from './TasksPage.vue'
import { endpoints } from '../api/endpoints'

vi.mock('../api/endpoints', async () => {
  const actual = await vi.importActual<any>('../api/endpoints')
  return {
    ...actual,
    endpoints: { listTasks: vi.fn() },
  }
})

const listTasksMock = endpoints.listTasks as unknown as ReturnType<typeof vi.fn>
const vuetify = createVuetify({ components, directives })

function button(w: ReturnType<typeof mount>, text: string) {
  const b = w.findAll('button').find((x) => x.text().includes(text))
  if (!b) throw new Error('button not found: ' + text)
  return b
}

describe('TasksPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('onMounted 调用 listTasks 并渲染列表', async () => {
    listTasksMock.mockResolvedValue([{ JobName: 'job1', Success: true, Err: '', StartedAt: '2026-01-01', Duration: 10 }])

    const w = mount(TasksPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    expect(listTasksMock).toHaveBeenCalledTimes(1)
    expect(w.text()).toContain('job1')
    expect(w.text()).toContain('成功')
    w.unmount()
  })

  it('空列表时空态', async () => {
    listTasksMock.mockResolvedValue([])

    const w = mount(TasksPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    expect(w.text()).toContain('暂无任务记录')
    w.unmount()
  })

  it('load 失败显示错误', async () => {
    listTasksMock.mockRejectedValueOnce(new Error('load boom'))

    const w = mount(TasksPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    expect(w.text()).toContain('load boom')
    w.unmount()
  })

  it('503 时显示调度器未启用', async () => {
    listTasksMock.mockRejectedValueOnce(new Error('503 调度器未启用'))

    const w = mount(TasksPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    expect(w.text()).toContain('调度器未启用')
    w.unmount()
  })

  it('手动刷新按钮重新 load', async () => {
    listTasksMock.mockResolvedValue([])

    const w = mount(TasksPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(listTasksMock).toHaveBeenCalledTimes(1)

    await button(w, '手动刷新').trigger('click')
    await flushPromises()

    expect(listTasksMock).toHaveBeenCalledTimes(2)
    w.unmount()
  })

  it('失败任务显示失败与错误', async () => {
    listTasksMock.mockResolvedValue([{ JobName: 'job2', Success: false, Err: 'boom', StartedAt: '2026-01-02', Duration: 5 }])

    const w = mount(TasksPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    expect(w.text()).toContain('job2')
    expect(w.text()).toContain('失败')
    expect(w.text()).toContain('boom')
    w.unmount()
  })
})
