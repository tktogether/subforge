import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import GroupsPage from './GroupsPage.vue'
import ResourcePage from '../components/ResourcePage.vue'
import { endpoints } from '../api/endpoints'

vi.mock('../api/endpoints', async () => {
  const actual = await vi.importActual<any>('../api/endpoints')
  return {
    ...actual,
    endpoints: {
      listGroups: vi.fn(),
      createGroup: vi.fn(),
      updateGroup: vi.fn(),
      deleteGroup: vi.fn(),
    },
  }
})

const listMock = endpoints.listGroups as unknown as ReturnType<typeof vi.fn>
const createMock = endpoints.createGroup as unknown as ReturnType<typeof vi.fn>
const updateMock = endpoints.updateGroup as unknown as ReturnType<typeof vi.fn>
const deleteMock = endpoints.deleteGroup as unknown as ReturnType<typeof vi.fn>
const vuetify = createVuetify({ components, directives })

describe('GroupsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listMock.mockResolvedValue([])
  })

  it('把 title/subtitle/columns/fields/resource 原样传给 ResourcePage', async () => {
    const w = mount(GroupsPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    const rp = w.findComponent(ResourcePage)
    expect(rp.exists()).toBe(true)
    expect(rp.props('title')).toBe('分组')
    expect(rp.props('subtitle')).toBe('代理分组')
    expect(rp.props('columns')).toEqual([
      { key: 'name', title: '名称' },
      { key: 'type', title: '类型' },
      { key: 'regex', title: '正则' },
      { key: 'url', title: 'URL' },
      { key: 'interval', title: '间隔' },
    ])
    expect(rp.props('fields')).toEqual([
      { key: 'name', label: '名称', type: 'text' },
      { key: 'type', label: '类型', type: 'text' },
      { key: 'regex', label: '正则', type: 'text' },
      { key: 'url', label: 'URL', type: 'text' },
      { key: 'interval', label: '间隔', type: 'number' },
    ])
    const resource = rp.props('resource') as any
    expect(resource.list).toBe(endpoints.listGroups)
    expect(resource.create).toBe(endpoints.createGroup)
    w.unmount()
  })

  it('resource.list 调 endpoints.listGroups，并把返回项渲染进表格', async () => {
    listMock.mockResolvedValue([
      { id: 1, name: '分组A', type: 'select', regex: '.*', url: 'https://g.example', interval: 300, sort_order: 1 },
    ])

    const w = mount(GroupsPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    expect(listMock).toHaveBeenCalledTimes(1)
    expect(w.text()).toContain('分组A')
    expect(w.text()).toContain('select')
    w.unmount()
  })

  it('resource.update(item) 调 endpoints.updateGroup(item.id, item) 并透传结果', async () => {
    updateMock.mockResolvedValue({ id: 4, name: '改后' })
    const w = mount(GroupsPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    const resource = w.findComponent(ResourcePage).props('resource') as any
    const item = { id: 4, name: '改后', type: 'select', regex: '.*', url: 'u', interval: 60 }
    const res = await resource.update(item)

    expect(updateMock).toHaveBeenCalledTimes(1)
    expect(updateMock).toHaveBeenCalledWith(4, item)
    expect(res).toEqual({ id: 4, name: '改后' })
    w.unmount()
  })

  it('resource.remove(item) 调 endpoints.deleteGroup(item.id) 并透传结果', async () => {
    deleteMock.mockResolvedValue({ ok: true })
    const w = mount(GroupsPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    const resource = w.findComponent(ResourcePage).props('resource') as any
    const item = { id: 8, name: '待删', type: 'select', regex: '.*', url: 'u', interval: 60 }
    const res = await resource.remove(item)

    expect(deleteMock).toHaveBeenCalledTimes(1)
    expect(deleteMock).toHaveBeenCalledWith(8)
    expect(res).toEqual({ ok: true })
    w.unmount()
  })

  it('resource.create 就是 endpoints.createGroup 本身', async () => {
    createMock.mockResolvedValue({ id: 2, name: '新分组' })
    const w = mount(GroupsPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    const resource = w.findComponent(ResourcePage).props('resource') as any
    const body = { name: '新分组', type: 'select', regex: '.*', url: 'u', interval: 60 }
    const res = await resource.create(body)

    expect(createMock).toHaveBeenCalledWith(body)
    expect(res).toEqual({ id: 2, name: '新分组' })
    w.unmount()
  })
})
