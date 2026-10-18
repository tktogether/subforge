import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import SelfNodesPage from './SelfNodesPage.vue'
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

  it('把 title/subtitle/columns/fields/resource 原样传给 ResourcePage', async () => {
    const w = mount(SelfNodesPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    const rp = w.findComponent(ResourcePage)
    expect(rp.exists()).toBe(true)
    expect(rp.props('title')).toBe('自建节点')
    expect(rp.props('subtitle')).toBe('自建代理节点')
    expect(rp.props('columns')).toEqual([
      { key: 'link', title: '链接' },
      { key: 'name', title: '名称' },
      { key: 'enabled', title: '启用' },
    ])
    expect(rp.props('fields')).toEqual([
      { key: 'link', label: '链接', type: 'text' },
      { key: 'name', label: '名称', type: 'text' },
      { key: 'enabled', label: '启用', type: 'switch' },
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
    // enabled=true 模板分支渲染「启用」
    expect(w.text()).toContain('启用')
    w.unmount()
  })

  it('resource.update(item) 调 endpoints.updateSelfNode(item.id, item) 并透传结果', async () => {
    updateMock.mockResolvedValue({ id: 3, name: '改后' })
    const w = mount(SelfNodesPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    const resource = w.findComponent(ResourcePage).props('resource') as any
    const item = { id: 3, link: 'vless://x', name: '改后', enabled: false }
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
    const item = { id: 12, link: 'vless://y', name: '待删', enabled: true }
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
    const body = { link: 'vless://z', name: '新节点', enabled: true }
    const res = await resource.create(body)

    expect(createMock).toHaveBeenCalledWith(body)
    expect(res).toEqual({ id: 2, name: '新节点' })
    w.unmount()
  })
})
