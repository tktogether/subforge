import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import ConvertPage from './ConvertPage.vue'
import { type Pipeline } from '../api/endpoints'
import { endpoints } from '../api/endpoints'

vi.mock('../api/endpoints', () => ({
  endpoints: {
    targets: vi.fn(),
    pipelines: vi.fn(),
    convert: vi.fn(),
  },
}))

const vuetify = createVuetify({ components, directives })

let writeTextMock: ReturnType<typeof vi.fn>

interface ConvertPageVm {
  targets: string[]
  pipelines: Pipeline[]
  selectedTarget: string
  urls: string
  text: string
  include: string
  exclude: string
  pipeline: string
  sort: string
  list: boolean
  loading: boolean
  error: string
  result: { output: string; warnings: unknown; failures: unknown; nodes: number; groups: number; rules: number } | null
  copied: boolean
  loadMeta: () => Promise<void>
  submit: () => Promise<void>
  copy: () => Promise<void>
  $nextTick: () => Promise<void>
}

describe('ConvertPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(endpoints.targets).mockResolvedValue([])
    vi.mocked(endpoints.pipelines).mockResolvedValue([])
    vi.mocked(endpoints.convert).mockResolvedValue({
      output: 'OUT',
      warnings: null,
      failures: null,
      nodes: 0,
      groups: 0,
      rules: 0,
    })
    writeTextMock = vi.fn()
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText: writeTextMock },
      configurable: true,
      writable: true,
    })
  })

  it('onMounted 调用 loadMeta：targets 与 pipelines("") 各被调用，成功时列表填充', async () => {
    vi.mocked(endpoints.targets).mockResolvedValue(['clash', 'singbox'])
    vi.mocked(endpoints.pipelines).mockResolvedValue([{ id: 1, name: 'p1', description: '', steps_json: '[]', enabled: true, sort_order: 0, source: 'ui', read_only: false }, { id: 2, name: 'p2', description: '', steps_json: '[]', enabled: true, sort_order: 1, source: 'template', read_only: true }])
    const w = mount(ConvertPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as ConvertPageVm
    await flushPromises()
    expect(endpoints.targets).toHaveBeenCalledTimes(1)
    expect(endpoints.pipelines).toHaveBeenCalledWith('')
    expect(vm.targets).toEqual(['clash', 'singbox'])
    expect(vm.pipelines).toEqual([{ id: 1, name: 'p1', description: '', steps_json: '[]', enabled: true, sort_order: 0, source: 'ui', read_only: false }, { id: 2, name: 'p2', description: '', steps_json: '[]', enabled: true, sort_order: 1, source: 'template', read_only: true }])
  })

  it('loadMeta 抛错时 error 显示 errorMessage(err) 文案（AsyncSection 可读）', async () => {
    vi.mocked(endpoints.targets).mockRejectedValue(new Error('boom'))
    vi.mocked(endpoints.pipelines).mockResolvedValue([])
    const w = mount(ConvertPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as ConvertPageVm
    await flushPromises()
    expect(vm.error).toBe('boom')
    expect(w.find('[data-testid="async-error"]').text()).toContain('boom')
  })

  it('submit 且 urls 与 text 都为空 → error 为指定文案且 convert 未被调用', async () => {
    const w = mount(ConvertPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as ConvertPageVm
    await flushPromises()
    await vm.submit()
    expect(vm.error).toBe('URL 与文本不能同时为空')
    expect(endpoints.convert).not.toHaveBeenCalled()
  })

  it('submit 成功路径：参数清洗与透传，并写入 result', async () => {
    const w = mount(ConvertPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as ConvertPageVm
    await flushPromises()
    vm.selectedTarget = 'clash'
    vm.urls = '  http://a.com  \n\n  http://b.com  '
    vm.text = 'some text'
    vm.include = ' foo , bar , , '
    vm.exclude = 'baz,qux'
    vm.pipeline = 'p1'
    vm.sort = 'true'
    vm.list = true
    const resp = { output: 'OUT', warnings: null, failures: null, nodes: 2, groups: 1, rules: 3 }
    vi.mocked(endpoints.convert).mockResolvedValue(resp)
    await vm.submit()
    expect(endpoints.convert).toHaveBeenCalledWith({
      target: 'clash',
      urls: ['http://a.com', 'http://b.com'],
      text: 'some text',
      types: [],
      include: ['foo', 'bar'],
      exclude: ['baz', 'qux'],
      pipeline: 'p1',
      config: '',
      sort: 'true',
      list: true,
      override: '',
    })
    expect(vm.result).toEqual(resp)
  })

  it('submit 失败：convert reject → error 显示错误文案，loading 归位 false', async () => {
    const w = mount(ConvertPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as ConvertPageVm
    await flushPromises()
    vm.urls = 'http://a.com'
    vi.mocked(endpoints.convert).mockRejectedValue(new Error('convert failed'))
    await vm.submit()
    expect(vm.error).toBe('convert failed')
    expect(vm.loading).toBe(false)
  })

  it('copy：result 为 null 时提前 return，不碰 clipboard', async () => {
    const w = mount(ConvertPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as ConvertPageVm
    await flushPromises()
    await vm.copy()
    expect(writeTextMock).not.toHaveBeenCalled()
  })

  it('copy：writeText 成功 → 按钮文案变「已复制」', async () => {
    const w = mount(ConvertPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as ConvertPageVm
    await flushPromises()
    vm.result = { output: 'hello', warnings: null, failures: null, nodes: 1, groups: 1, rules: 1 }
    writeTextMock.mockResolvedValue(undefined)
    await vm.copy()
    expect(writeTextMock).toHaveBeenCalledWith('hello')
    expect(vm.copied).toBe(true)
    expect(w.text()).toContain('已复制')
  })

  it('copy：writeText reject → 保持「复制」', async () => {
    const w = mount(ConvertPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as ConvertPageVm
    await flushPromises()
    vm.result = { output: 'hello', warnings: null, failures: null, nodes: 1, groups: 1, rules: 1 }
    writeTextMock.mockRejectedValue(new Error('no'))
    await vm.copy()
    expect(vm.copied).toBe(false)
    expect(w.text()).toContain('复制')
    expect(w.text()).not.toContain('已复制')
  })

  it('模板：result 为 null 时显示 empty-text「结果将显示在这里」', async () => {
    const w = mount(ConvertPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.find('[data-testid="async-empty"]').text()).toContain('结果将显示在这里')
  })

  it('模板：有结果时 textarea 的 value 等于 result.output', async () => {
    const w = mount(ConvertPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as ConvertPageVm
    await flushPromises()
    vm.text = 'hello'
    vi.mocked(endpoints.convert).mockResolvedValue({ output: 'THE OUTPUT', warnings: null, failures: null, nodes: 1, groups: 1, rules: 1 })
    await vm.submit()
    await flushPromises()
    await (w.vm as any).$nextTick()
    const ta = w.find('textarea[rows="12"]')
    expect((ta.element as HTMLTextAreaElement).value).toBe('THE OUTPUT')
  })



  it('交互：表单控件 v-model setter 覆盖', async () => {
    const w = mount(ConvertPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as ConvertPageVm
    await flushPromises()
    await w.find('textarea[rows="4"]').setValue('  http://a.com  ')
    await w.find('textarea[rows="3"]').setValue('some text')
    const textFields = w.findAllComponents({ name: 'VTextField' })
    for (const tf of textFields) await (tf as any).setValue('x')
    const selects = w.findAllComponents({ name: 'VSelect' })
    if (selects.length > 0) await (selects[0] as any).setValue('clash')
    if (selects.length > 1) await (selects[1] as any).setValue('p1')
    const switches = w.findAllComponents({ name: 'VSwitch' })
    if (switches.length) await (switches[0] as any).setValue(true)
    await vm.$nextTick()
    expect(vm.urls).toBe('  http://a.com  ')
    expect(vm.text).toBe('some text')
  })

})
