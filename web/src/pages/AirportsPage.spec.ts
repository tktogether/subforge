import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import AirportsPage from './AirportsPage.vue'
import ResourcePage from '../components/ResourcePage.vue'
import { endpoints } from '../api/endpoints'

vi.mock('../api/endpoints', async () => {
  const actual = await vi.importActual<any>('../api/endpoints')
  return {
    ...actual,
    endpoints: {
      listAirports: vi.fn(),
      createAirport: vi.fn(),
      updateAirport: vi.fn(),
      deleteAirport: vi.fn(),
    },
  }
})

const listMock = endpoints.listAirports as unknown as ReturnType<typeof vi.fn>
const createMock = endpoints.createAirport as unknown as ReturnType<typeof vi.fn>
const updateMock = endpoints.updateAirport as unknown as ReturnType<typeof vi.fn>
const deleteMock = endpoints.deleteAirport as unknown as ReturnType<typeof vi.fn>
const vuetify = createVuetify({ components, directives })

describe('AirportsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listMock.mockResolvedValue([])
  })

  it('把 title/subtitle/columns/fields/resource 原样传给 ResourcePage', async () => {
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    const rp = w.findComponent(ResourcePage)
    expect(rp.exists()).toBe(true)
    expect(rp.props('title')).toBe('机场')
    expect(rp.props('subtitle')).toBe('订阅源机场')
    expect(rp.props('columns')).toEqual([
      { key: 'name', title: '名称' },
      { key: 'subscription_url', title: '订阅地址' },
      { key: 'remarks', title: '备注' },
      { key: 'enabled', title: '启用' },
    ])
    expect(rp.props('fields')).toEqual([
      { key: 'name', label: '名称', type: 'text' },
      { key: 'subscription_url', label: '订阅地址', type: 'text' },
      { key: 'remarks', label: '备注', type: 'text' },
      { key: 'enabled', label: '启用', type: 'switch' },
    ])
    const resource = rp.props('resource') as any
    expect(resource.list).toBe(endpoints.listAirports)
    expect(resource.create).toBe(endpoints.createAirport)
    w.unmount()
  })

  it('resource.list 调 endpoints.listAirports，并把返回项渲染进表格', async () => {
    listMock.mockResolvedValue([
      { id: 1, name: '机场A', subscription_url: 'https://a.example/sub', remarks: 'r1', enabled: true, sort_order: 1 },
    ])

    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    expect(listMock).toHaveBeenCalledTimes(1)
    expect(w.text()).toContain('机场A')
    // enabled 模板分支：true -> 启用
    expect(w.text()).toContain('启用')
    w.unmount()
  })

  it('resource.update(item) 调 endpoints.updateAirport(item.id, item) 并透传结果', async () => {
    updateMock.mockResolvedValue({ id: 7, name: '改后' })
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    const resource = w.findComponent(ResourcePage).props('resource') as any
    const item = { id: 7, name: '改后', subscription_url: 'u', remarks: 'r', enabled: false }
    const res = await resource.update(item)

    expect(updateMock).toHaveBeenCalledTimes(1)
    expect(updateMock).toHaveBeenCalledWith(7, item)
    expect(res).toEqual({ id: 7, name: '改后' })
    w.unmount()
  })

  it('resource.remove(item) 调 endpoints.deleteAirport(item.id) 并透传结果', async () => {
    deleteMock.mockResolvedValue({ ok: true })
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    const resource = w.findComponent(ResourcePage).props('resource') as any
    const item = { id: 9, name: '待删', subscription_url: 'u', remarks: 'r', enabled: true }
    const res = await resource.remove(item)

    expect(deleteMock).toHaveBeenCalledTimes(1)
    expect(deleteMock).toHaveBeenCalledWith(9)
    expect(res).toEqual({ ok: true })
    w.unmount()
  })

  it('resource.create 就是 endpoints.createAirport 本身（直接调用会走该 mock）', async () => {
    createMock.mockResolvedValue({ id: 3, name: '新机场' })
    const w = mount(AirportsPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    const resource = w.findComponent(ResourcePage).props('resource') as any
    const body = { name: '新机场', subscription_url: 'u', remarks: '', enabled: true }
    const res = await resource.create(body)

    expect(createMock).toHaveBeenCalledWith(body)
    expect(res).toEqual({ id: 3, name: '新机场' })
    w.unmount()
  })
})
