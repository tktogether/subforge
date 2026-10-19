import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import ConvertPage from './ConvertPage.vue'
import { type Pipeline } from '../api/endpoints'
import { endpoints } from '../api/endpoints'
import { pageHelp } from '../content/help'

vi.mock('../api/endpoints', () => ({
  endpoints: {
    targets: vi.fn(),
    pipelines: vi.fn(),
    convert: vi.fn(),
    diff: vi.fn(),
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
  result: { body: string; warnings: unknown; failures: unknown; nodes: number; rawNodes: number; cached: number; format: string } | null
  copied: boolean
  types: string
  diffA: string
  diffB: string
  diffLines: { type: string; text: string }[]
  diffRan: boolean
  diffError: string
  loadMeta: () => Promise<void>
  submit: () => Promise<void>
  copy: () => Promise<void>
  download: () => void
  runDiff: () => Promise<void>
  lineClass: (t: string) => string
  filenameFor: (f: string) => string
  $nextTick: () => Promise<void>
}

describe('ConvertPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(endpoints.targets).mockResolvedValue([])
    vi.mocked(endpoints.pipelines).mockResolvedValue([])
    vi.mocked(endpoints.diff).mockResolvedValue({ lines: [] })
    vi.mocked(endpoints.convert).mockResolvedValue({
      body: 'OUT', format: '',
      warnings: null,
      failures: null,
      nodes: 0,
      rawNodes: 0,
      cached: 0,
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
    const resp = { body: 'OUT', format: '', warnings: null, failures: null, nodes: 2, rawNodes: 1, cached: 3 }
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
    vm.result = { body: 'hello', format: 'clash.meta', warnings: null, failures: null, nodes: 1, rawNodes: 1, cached: 1 }
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
    vm.result = { body: 'hello', format: 'clash.meta', warnings: null, failures: null, nodes: 1, rawNodes: 1, cached: 1 }
    writeTextMock.mockRejectedValue(new Error('no'))
    await vm.copy()
    expect(vm.copied).toBe(false)
    expect(w.text()).toContain('复制')
    expect(w.text()).not.toContain('已复制')
  })

  it('模板：result 为 null 时显示 empty-state', async () => {
    const w = mount(ConvertPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.find('[data-testid="empty-state"]').exists()).toBe(true)
    expect(w.find('[data-testid="empty-state-title"]').text()).toContain('转换结果')
  })

  it('模板：有结果时 textarea 的 value 等于 result.body', async () => {
    const w = mount(ConvertPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as ConvertPageVm
    await flushPromises()
    vm.text = 'hello'
    vi.mocked(endpoints.convert).mockResolvedValue({ body: 'THE OUTPUT', format: '', warnings: null, failures: null, nodes: 1, rawNodes: 1, cached: 1 })
    await vm.submit()
    await flushPromises()
    await (w.vm as any).$nextTick()
    const ta = w.find('textarea[rows="12"]')
    expect((ta.element as HTMLTextAreaElement).value).toBe('THE OUTPUT')
  })



  it('模板：target 含义表渲染 ≥14 行', async () => {
    const w = mount(ConvertPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const rows = w.findAll('[data-testid="target-meanings"] tbody tr')
    expect(rows.length).toBeGreaterThanOrEqual(14)
    w.unmount()
  })

  it('模板：target 含义表按 /api/targets 真实返回值生成，未知项归入「其他」', async () => {
    vi.mocked(endpoints.targets).mockResolvedValue(['clash.meta', 'surge', 'list', 'sing-box', 'brand-new-target'])
    const w = mount(ConvertPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await (w.vm as any).$nextTick()
    const rows = w.findAll('[data-testid="target-meanings"] tbody tr')
    expect(rows.length).toBe(5)
    const texts = rows.map((r) => r.text())
    expect(texts[0]).toContain('clash.meta')
    expect(texts[0]).toContain('Clash/Mihomo YAML 配置')
    expect(texts[3]).toContain('sing-box')
    expect(texts[4]).toContain('brand-new-target')
    expect(texts[4]).toContain('其他')
    w.unmount()
  })

  it('模板：help-panel 含 oneLiner 片段', async () => {
    const w = mount(ConvertPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const panel = w.find('[data-testid="help-panel"]')
    expect(panel.exists()).toBe(true)
    expect(panel.text()).toContain(pageHelp('convert').oneLiner)
    w.unmount()
  })

  it('download：生成 Blob 并 revokeObjectURL', async () => {
    const createSpy = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:mock')
    const revokeSpy = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
    const w = mount(ConvertPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as ConvertPageVm
    await flushPromises()
    vm.result = { body: 'hello', format: 'list', warnings: null, failures: null, nodes: 1, rawNodes: 1, cached: 1 }
    vm.download()
    expect(createSpy).toHaveBeenCalled()
    expect(revokeSpy).toHaveBeenCalled()
    createSpy.mockRestore()
    revokeSpy.mockRestore()
    clickSpy.mockRestore()
    w.unmount()
  })

  it('filenameFor：list/sing-box/其他', async () => {
    const w = mount(ConvertPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as ConvertPageVm
    await flushPromises()
    expect(vm.filenameFor('list')).toBe('list.txt')
    expect(vm.filenameFor('sing-box')).toBe('sing-box.json')
    expect(vm.filenameFor('clash.meta')).toBe('clash.meta.yaml')
    w.unmount()
  })

  it('lineClass：-/+/空格', async () => {
    const w = mount(ConvertPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as ConvertPageVm
    await flushPromises()
    expect(vm.lineClass('-')).toBe('line-del')
    expect(vm.lineClass('+')).toBe('line-add')
    expect(vm.lineClass(' ')).toBe('line-same')
    w.unmount()
  })

  it('runDiff：调用 endpoints.diff 并写入 diffLines', async () => {
    vi.mocked(endpoints.diff).mockResolvedValue({ lines: [{ type: ' ', text: 'same' }, { type: '-', text: 'old' }, { type: '+', text: 'new' }] })
    const w = mount(ConvertPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as ConvertPageVm
    await flushPromises()
    vm.diffA = 'a'
    vm.diffB = 'b'
    await vm.runDiff()
    await flushPromises()
    expect(endpoints.diff).toHaveBeenCalledWith('a', 'b')
    expect(vm.diffLines.length).toBe(3)
    expect(vm.diffRan).toBe(true)
    expect(w.find('[data-testid="diff-result"]').exists()).toBe(true)
    w.unmount()
  })

  it('runDiff：两侧都空 → diff-empty 显示「没有可对比的内容」', async () => {
    vi.mocked(endpoints.diff).mockResolvedValue({ lines: [] })
    const w = mount(ConvertPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as ConvertPageVm
    await flushPromises()
    await vm.runDiff()
    await flushPromises()
    expect(w.find('[data-testid="diff-empty"]').exists()).toBe(true)
    expect(w.find('[data-testid="diff-empty"]').text()).toContain('没有可对比的内容')
    w.unmount()
  })

  it('runDiff：失败写入 diffError', async () => {
    vi.mocked(endpoints.diff).mockRejectedValue(new Error('diff boom'))
    const w = mount(ConvertPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as ConvertPageVm
    await flushPromises()
    await vm.runDiff()
    await flushPromises()
    expect(vm.diffError).toBe('diff boom')
    expect(w.find('[data-testid="diff-error"]').exists()).toBe(true)
    w.unmount()
  })

  it('模板：result 摘要显示 format/nodes/rawNodes/cached', async () => {
    const w = mount(ConvertPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as ConvertPageVm
    await flushPromises()
    vm.result = { body: 'OUT', format: 'clash.meta', warnings: null, failures: null, nodes: 5, rawNodes: 7, cached: 2 }
    await vm.$nextTick()
    const s = w.find('[data-testid="result-summary"]')
    expect(s.text()).toContain('clash.meta')
    expect(s.text()).toContain('5')
    expect(s.text()).toContain('7')
    expect(s.text()).toContain('2')
    w.unmount()
  })

  it('模板：failures/warnings 逐条显示原文', async () => {
    const w = mount(ConvertPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as ConvertPageVm
    await flushPromises()
    vm.result = { body: 'OUT', format: 'clash.meta', warnings: ['w1'], failures: ['f1'], nodes: 1, rawNodes: 1, cached: 0 }
    await vm.$nextTick()
    expect(w.find('[data-testid="convert-failures"]').text()).toContain('f1')
    expect(w.find('[data-testid="convert-warnings"]').text()).toContain('w1')
    w.unmount()
  })

  it('模板：无 failures 显示全部来源成功', async () => {
    const w = mount(ConvertPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as ConvertPageVm
    await flushPromises()
    vm.result = { body: 'OUT', format: 'clash.meta', warnings: null, failures: null, nodes: 1, rawNodes: 1, cached: 0 }
    await vm.$nextTick()
    expect(w.find('[data-testid="convert-failures-ok"]').exists()).toBe(true)
    w.unmount()
  })

  it('types 输入框透传到 convert 请求', async () => {
    const w = mount(ConvertPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as ConvertPageVm
    await flushPromises()
    vm.urls = 'http://a.com'
    vm.types = 'ss, vmess ,'
    vi.mocked(endpoints.convert).mockResolvedValue({ body: 'OUT', format: 'clash.meta', warnings: null, failures: null, nodes: 1, rawNodes: 1, cached: 0 })
    await vm.submit()
    expect(endpoints.convert).toHaveBeenCalledWith(expect.objectContaining({ types: ['ss', 'vmess'] }))
    w.unmount()
  })

  it('download：result 为 null 时提前 return', async () => {
    const createSpy = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:mock')
    const w = mount(ConvertPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as ConvertPageVm
    await flushPromises()
    vm.download()
    expect(createSpy).not.toHaveBeenCalled()
    createSpy.mockRestore()
    w.unmount()
  })

  it('onUnmounted：objectUrl 非空时 revokeObjectURL', async () => {
    const revokeSpy = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})
    const w = mount(ConvertPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as ConvertPageVm
    await flushPromises()
    ;(vm as any).objectUrl = 'blob:leak'
    w.unmount()
    expect(revokeSpy).toHaveBeenCalledWith('blob:leak')
    revokeSpy.mockRestore()
  })

  it('交互：sort 下拉与 diff 文本框 setter', async () => {
    const w = mount(ConvertPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as ConvertPageVm
    await flushPromises()
    const selects = w.findAllComponents({ name: 'VSelect' })
    const sortSel = selects.find((s) => s.attributes('data-testid') === 'sort')
    if (sortSel) await (sortSel as any).setValue('true')
    await w.find('[data-testid="diff-a"] textarea').setValue('AAA')
    await w.find('[data-testid="diff-b"] textarea').setValue('BBB')
    await vm.$nextTick()
    expect(vm.diffA).toBe('AAA')
    expect(vm.diffB).toBe('BBB')
    w.unmount()
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
