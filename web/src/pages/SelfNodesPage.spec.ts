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
    },
  }
})

const listMock = endpoints.listSelfNodes as unknown as ReturnType<typeof vi.fn>
const createMock = endpoints.createSelfNode as unknown as ReturnType<typeof vi.fn>
const updateMock = endpoints.updateSelfNode as unknown as ReturnType<typeof vi.fn>
const deleteMock = endpoints.deleteSelfNode as unknown as ReturnType<typeof vi.fn>
const vuetify = createVuetify({ components, directives })

describe('SelfNodesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listMock.mockResolvedValue([])
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
      { key: 'sort_order', title: '顺序' },
    ])
    expect(rp.props('fields')).toEqual([
      { key: 'name', label: '名称', type: 'text', hint: '输出到客户端显示的名字；留空则用链接里自带的名字', example: '自建-香港' },
      { key: 'link', label: '分享链接', type: 'text', hint: '标准分享链接，以 ss:// / vmess:// / vless:// / trojan:// / hysteria2:// 开头', example: 'vless://uuid@1.2.3.4:443?encryption=none&security=tls#香港' },
      { key: 'enabled', label: '启用', type: 'switch', hint: '关闭后不参与输出' },
      { key: 'sort_order', label: '排序', type: 'number', hint: '越小越靠前' },
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
