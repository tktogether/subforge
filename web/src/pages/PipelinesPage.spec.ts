import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import PipelinesPage from './PipelinesPage.vue'
import { type Pipeline } from '../api/endpoints'
import { endpoints } from '../api/endpoints'

vi.mock('../api/endpoints', async () => {
  const actual = await vi.importActual<any>('../api/endpoints')
  return {
    ...actual,
    endpoints: { pipelines: vi.fn() },
  }
})

const pipelinesMock = endpoints.pipelines as unknown as ReturnType<typeof vi.fn>
const vuetify = createVuetify({ components, directives })

describe('PipelinesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('onMounted 调用 endpoints.pipelines("") 并把结果渲染成列表（empty 为假）', async () => {
    pipelinesMock.mockResolvedValue([{ id: 1, name: 'pipe-a', description: '', steps_json: '[{\"op\":\"filter\"},{\"op\":null}]', enabled: true, sort_order: 0, source: 'ui', read_only: false }, { id: 2, name: 'pipe-b', description: '', steps_json: 'not-json', enabled: true, sort_order: 1, source: 'template', read_only: true }])

    const w = mount(PipelinesPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    expect(pipelinesMock).toHaveBeenCalledTimes(1)
    expect(pipelinesMock).toHaveBeenCalledWith('')
    expect(w.findAllComponents({ name: 'VListItem' }).length).toBe(2)
    expect(w.text()).toContain('pipe-a')
    expect(w.text()).toContain('pipe-b')
    // 有数据时不该出现空态
    expect(w.find('[data-testid="async-empty"]').exists()).toBe(false)
    expect(w.find('[data-testid="async-loading"]').exists()).toBe(false)
    expect(w.find('[data-testid="async-error"]').exists()).toBe(false)
    w.unmount()
  })

  it('空列表时 empty 为真，渲染 emptyText，且不渲染任何列表项', async () => {
    pipelinesMock.mockResolvedValue([])

    const w = mount(PipelinesPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    expect(w.find('[data-testid="async-empty"]').exists()).toBe(true)
    expect(w.text()).toContain('在配置文件中定义管线')
    expect(w.findAllComponents({ name: 'VListItem' }).length).toBe(0)
    w.unmount()
  })

  it('请求未完成时 loading 为真，AsyncSection 渲染骨架屏', async () => {
    let resolveIt: (v: Pipeline[]) => void = () => {}
    pipelinesMock.mockImplementation(
      () => new Promise<Pipeline[]>((resolve) => { resolveIt = resolve }),
    )

    const w = mount(PipelinesPage, { global: { plugins: [vuetify] } })
    await nextTick()
    expect(w.find('[data-testid="async-loading"]').exists()).toBe(true)
    expect(w.find('[data-testid="async-empty"]').exists()).toBe(false)

    resolveIt([{ id: 3, name: 'late-pipe', description: '', steps_json: '[]', enabled: true, sort_order: 0, source: 'ui', read_only: false }])
    await flushPromises()
    expect(w.find('[data-testid="async-loading"]').exists()).toBe(false)
    expect(w.text()).toContain('late-pipe')
    w.unmount()
  })

  it('请求抛 Error 时显示其 message（catch + errorMessage 的 Error 分支）', async () => {
    pipelinesMock.mockRejectedValueOnce(new Error('配置文件缺少 pipelines'))

    const w = mount(PipelinesPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    expect(w.find('[data-testid="async-error"]').exists()).toBe(true)
    expect(w.text()).toContain('配置文件缺少 pipelines')
    // 失败时 loading 已被 finally 复位
    expect(w.find('[data-testid="async-loading"]').exists()).toBe(false)
    w.unmount()
  })

  it('请求抛非 Error 时用 String(err) 显示（errorMessage 的非 Error 分支）', async () => {
    pipelinesMock.mockRejectedValueOnce('plain-pipeline-failure')

    const w = mount(PipelinesPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    expect(w.text()).toContain('plain-pipeline-failure')
    w.unmount()
  })

  it('点击重试按钮再次 load：error 被清空，成功后渲染列表', async () => {
    pipelinesMock
      .mockRejectedValueOnce(new Error('boom'))
      .mockResolvedValueOnce([{ id: 4, name: 'recovered', description: '', steps_json: '[]', enabled: true, sort_order: 0, source: 'ui', read_only: false }])

    const w = mount(PipelinesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.text()).toContain('boom')

    const retry = w.find('[data-testid="async-retry"]')
    expect(retry.exists()).toBe(true)
    await retry.trigger('click')
    await flushPromises()

    expect(pipelinesMock).toHaveBeenCalledTimes(2)
    expect(w.find('[data-testid="async-error"]').exists()).toBe(false)
    expect(w.text()).toContain('recovered')
    w.unmount()
  })
})
