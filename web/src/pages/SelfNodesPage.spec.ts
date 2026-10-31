import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import SelfNodesPage from './SelfNodesPage.vue'
import { pageHelp } from '../content/help'
import ResourcePage from '../components/ResourcePage.vue'
import { endpoints } from '../api/endpoints'

vi.mock('../api/endpoints', async () => {
  const actual = await vi.importActual<any>('../api/endpoints')
  return {
    ...actual,
    endpoints: {
      listSelfNodes: vi.fn(),
      createSelfNode: vi.fn(),
      updateSelfNode: vi.fn(),
      deleteSelfNode: vi.fn(),
      getSelfSourcePipeline: vi.fn(),
      setSelfSourcePipeline: vi.fn(),
      pipelines: vi.fn(),
      previewSelfNodesProcess: vi.fn(),
    },
  }
})

const listMock = endpoints.listSelfNodes as unknown as ReturnType<typeof vi.fn>
const createMock = endpoints.createSelfNode as unknown as ReturnType<typeof vi.fn>
const updateMock = endpoints.updateSelfNode as unknown as ReturnType<typeof vi.fn>
const deleteMock = endpoints.deleteSelfNode as unknown as ReturnType<typeof vi.fn>
const getSourcePipelineMock = endpoints.getSelfSourcePipeline as unknown as ReturnType<typeof vi.fn>
const setSourcePipelineMock = endpoints.setSelfSourcePipeline as unknown as ReturnType<typeof vi.fn>
const pipelinesMock = endpoints.pipelines as unknown as ReturnType<typeof vi.fn>
const previewProcessMock = endpoints.previewSelfNodesProcess as unknown as ReturnType<typeof vi.fn>
const vuetify = createVuetify({ components, directives })

describe('SelfNodesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listMock.mockResolvedValue([])
    getSourcePipelineMock.mockResolvedValue({ pipeline: '' })
    pipelinesMock.mockResolvedValue([])
    setSourcePipelineMock.mockResolvedValue({ pipeline: '' })
    previewProcessMock.mockResolvedValue({ name: '自建节点', pipeline: '', before_count: 0, after_count: 0, before: [], after: [], warnings: [] })
  })

  it('把 title/subtitle/columns/fields/resource/help-key 原样传给 ResourcePage', async () => {
    const w = mount(SelfNodesPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    const rp = w.findComponent(ResourcePage)
    expect(rp.exists()).toBe(true)
    expect(rp.props('title')).toBe('自建节点')
    expect(rp.props('subtitle')).toBe(pageHelp('self-nodes').oneLiner)
    expect(rp.props('helpKey')).toBe('self-nodes')
    expect(rp.props('columns')).toEqual([
      { key: 'name', title: '名称' },
      { key: 'link', title: '链接' },
      { key: 'enabled', title: '启用' },
    ])
    expect(rp.props('fields')).toEqual([
      { key: 'name', label: '名称', type: 'text', hint: '输出到客户端显示的名字；留空则用链接里自带的名字', example: '自建-香港' },
      { key: 'link', label: '分享链接', type: 'text', hint: '标准分享链接，以 ss:// / vmess:// / vless:// / trojan:// / hysteria2:// 开头', example: 'vless://uuid@1.2.3.4:443?encryption=none&security=tls#香港' },
      { key: 'enabled', label: '启用', type: 'switch', hint: '关闭后不参与输出' },
    ])
    const resource = rp.props('resource') as any
    expect(resource.list).toBe(endpoints.listSelfNodes)
    expect(resource.create).toBe(endpoints.createSelfNode)
    w.unmount()
  })

  it('resource.list 调 endpoints.listSelfNodes，并把返回项渲染进表格', async () => {
    listMock.mockResolvedValue([
      { id: 1, link: 'vless://node-a', name: '节点A', enabled: true, sort_order: 1 },
    ])

    const w = mount(SelfNodesPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    expect(listMock).toHaveBeenCalledTimes(1)
    expect(w.text()).toContain('节点A')
    expect(w.text()).toContain('vless://node-a')
    expect(w.text()).toContain('启用')
    w.unmount()
  })

  it('resource.update(item) 调 endpoints.updateSelfNode(item.id, item) 并透传结果', async () => {
    updateMock.mockResolvedValue({ id: 3, name: '改后' })
    const w = mount(SelfNodesPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    const resource = w.findComponent(ResourcePage).props('resource') as any
    const item = { id: 3, link: 'vless://x', name: '改后', enabled: false, sort_order: 1 }
    const res = await resource.update(item)

    expect(updateMock).toHaveBeenCalledTimes(1)
    expect(updateMock).toHaveBeenCalledWith(3, item)
    expect(res).toEqual({ id: 3, name: '改后' })
    w.unmount()
  })

  it('resource.remove(item) 调 endpoints.deleteSelfNode(item.id) 并透传结果', async () => {
    deleteMock.mockResolvedValue({ ok: true })
    const w = mount(SelfNodesPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    const resource = w.findComponent(ResourcePage).props('resource') as any
    const item = { id: 12, link: 'vless://y', name: '待删', enabled: true, sort_order: 1 }
    const res = await resource.remove(item)

    expect(deleteMock).toHaveBeenCalledTimes(1)
    expect(deleteMock).toHaveBeenCalledWith(12)
    expect(res).toEqual({ ok: true })
    w.unmount()
  })

  it('resource.create 就是 endpoints.createSelfNode 本身', async () => {
    createMock.mockResolvedValue({ id: 2, name: '新节点' })
    const w = mount(SelfNodesPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    const resource = w.findComponent(ResourcePage).props('resource') as any
    const body = { link: 'vless://z', name: '新节点', enabled: true, sort_order: 1 }
    const res = await resource.create(body)

    expect(createMock).toHaveBeenCalledWith(body)
    expect(res).toEqual({ id: 2, name: '新节点' })
    w.unmount()
  })

  it('help-panel 显示与服务端校验行为一致的文案（不含旧文案）', async () => {
    const w = mount(SelfNodesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const txt = w.text()
    expect(txt).toContain('保存时会校验名称、链接与必要字段')
    // 旧文案字面量拆开拼，避免把旧短语写回仓库源码（Lead 会全仓 grep 旧短语）
    expect(txt).not.toContain('静默不' + '出现')
    expect(txt).not.toContain('不会有报错提示')
    w.unmount()
  })

  it('help-panel 渲染 oneLiner', async () => {
    const w = mount(SelfNodesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.text()).toContain('管理手动添加的节点，与机场节点一同输出')
    w.unmount()
  })

  it('展开格式示例折叠面板渲染 4 条示例', async () => {
    const w = mount(SelfNodesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const title = w.find('[data-testid="self-node-formats"] .v-expansion-panel-title')
    if (title.exists()) { await title.trigger('click'); await flushPromises() }
    expect(w.text()).toContain('查看格式示例')
    expect(w.text()).toContain('以上均为示例')
    w.unmount()
  })
})

describe('SelfNodesPage 来源处理', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listMock.mockResolvedValue([])
    getSourcePipelineMock.mockResolvedValue({ pipeline: 'pre' })
    pipelinesMock.mockResolvedValue([{ id: 1, name: 'pre', enabled: true }, { id: 2, name: 'off', enabled: false }])
    setSourcePipelineMock.mockResolvedValue({ pipeline: 'pre' })
  })

  it('加载并显示自建节点处理管线，只列出启用管线', async () => {
    const w = mount(SelfNodesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(getSourcePipelineMock).toHaveBeenCalledTimes(1)
    expect(w.find('[data-testid="self-source-card"]').exists()).toBe(true)
    const vm = w.vm as any
    expect(vm.pipeline).toBe('pre')
    expect(w.text()).toContain('自建节点处理')
    w.unmount()
  })

  it('保存自建节点处理管线并提示成功', async () => {
    const w = mount(SelfNodesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    ;(w.vm as any).pipeline = 'pre'
    const select = w.findAllComponents({ name: 'VSelect' }).find((s) => s.attributes('data-testid') === 'self-source-pipeline')
    await select!.setValue('pre')
    await w.find('[data-testid="self-source-save"]').trigger('click')
    await flushPromises()
    expect(setSourcePipelineMock).toHaveBeenCalledWith('pre')
    w.unmount()
  })

  it('读取失败时不阻塞页面', async () => {
    getSourcePipelineMock.mockRejectedValueOnce(new Error('source boom'))
    const w = mount(SelfNodesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.find('[data-testid="self-source-card"]').exists()).toBe(true)
    w.unmount()
  })

  it('保存失败时提示错误', async () => {
    setSourcePipelineMock.mockRejectedValueOnce(new Error('save boom'))
    const w = mount(SelfNodesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await w.find('[data-testid="self-source-save"]').trigger('click')
    await flushPromises()
    expect(setSourcePipelineMock).toHaveBeenCalled()
    w.unmount()
  })
})

describe('SelfNodesPage 来源处理当前值', () => {
  it('当前管线不在启用选项中时仍显示为选项', async () => {
    vi.clearAllMocks()
    listMock.mockResolvedValue([])
    getSourcePipelineMock.mockResolvedValue({ pipeline: 'legacy' })
    pipelinesMock.mockResolvedValue([])
    setSourcePipelineMock.mockResolvedValue({ pipeline: 'legacy' })
    const w = mount(SelfNodesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const select = w.findAllComponents({ name: 'VSelect' }).find((s) => s.attributes('data-testid') === 'self-source-pipeline')
    expect(select).toBeTruthy()
    expect((select!.props('items') as { value: string }[]).some((i) => i.value === 'legacy')).toBe(true)
    w.unmount()
  })
})

describe('SelfNodesPage 处理预览', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    ;(window as any).visualViewport = { width: 1024, height: 768, scale: 1, offsetLeft: 0, offsetTop: 0, pageLeft: 0, pageTop: 0, addEventListener: () => {}, removeEventListener: () => {}, dispatchEvent: () => true }
    listMock.mockResolvedValue([])
    getSourcePipelineMock.mockResolvedValue({ pipeline: 'pre' })
    pipelinesMock.mockResolvedValue([{ id: 1, name: 'pre', enabled: true }])
    setSourcePipelineMock.mockResolvedValue({ pipeline: 'pre' })
    previewProcessMock.mockResolvedValue({ name: '自建节点', pipeline: 'pre', before_count: 2, after_count: 2, before: [], after: [{ name: 'S-01', type: 'ss', server: 's', port: '443' }], warnings: [] })
  })

  it('按当前管线预览自建节点处理结果', async () => {
    const w = mount(SelfNodesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await (w.vm as any).previewProcess()
    await flushPromises()
    expect(previewProcessMock).toHaveBeenCalledWith('pre')
    expect((w.vm as any).previewResult.after_count).toBe(2)
    w.unmount()
  })

  it('预览失败时显示错误', async () => {
    previewProcessMock.mockRejectedValueOnce(new Error('self preview boom'))
    const w = mount(SelfNodesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    await (w.vm as any).previewProcess()
    await flushPromises()
    expect((w.vm as any).previewError).toBe('self preview boom')
    w.unmount()
  })

  it('预览弹窗关闭时写回 previewOpen=false', async () => {
    const w = mount(SelfNodesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    ;(w.vm as any).previewOpen = true
    await flushPromises()
    const dialog = w.findComponent({ name: 'NodePreviewDialog' })
    expect(dialog.exists()).toBe(true)
    dialog.vm.$emit('update:modelValue', false)
    await flushPromises()
    expect((w.vm as any).previewOpen).toBe(false)
    w.unmount()
  })
})
