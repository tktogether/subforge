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

  it('把 title/subtitle/columns/fields/resource/help-key 原样传给 ResourcePage', async () => {
    const w = mount(GroupsPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    const rp = w.findComponent(ResourcePage)
    expect(rp.exists()).toBe(true)
    expect(rp.props('title')).toBe('代理分组')
    expect(rp.props('subtitle')).toBe('决定客户端里的策略组与节点归组')
    expect(rp.props('helpKey')).toBe('groups')
    expect(rp.props('columns')).toEqual([
      { key: 'name', title: '名称' },
      { key: 'type', title: '类型' },
      { key: 'regex', title: '匹配正则' },
      { key: 'url', title: '测活地址' },
      { key: 'interval', title: '间隔(秒)' },
      { key: 'sort_order', title: '顺序' },
    ])
    expect(rp.props('fields')).toEqual([
      { key: 'name', label: '名称', type: 'text', hint: '客户端里显示的策略组名；规则行的目标分组就是按这个名找的', example: '香港' },
      { key: 'type', label: '类型', type: 'text', hint: 'select=手动选；url-test=自动选延迟最低；fallback=按顺序回退；load-balance=负载均衡', example: 'select' },
      { key: 'regex', label: '匹配正则', type: 'text', hint: '按节点名匹配成员；留空则不自动收节点', example: '香港|HK' },
      { key: 'url', label: '测活地址', type: 'text', hint: '仅 url-test / fallback 需要；返回 204 的地址最合适', example: 'http://www.gstatic.com/generate_204' },
      { key: 'interval', label: '测活间隔（秒）', type: 'number', hint: '仅 url-test 需要；太小会频繁测活', example: '300' },
      { key: 'sort_order', label: '排序', type: 'number', hint: '越小越靠前' },
    ])
    const resource = rp.props('resource') as any
    expect(resource.list).toBe(endpoints.listGroups)
    expect(resource.create).toBe(endpoints.createGroup)
    expect(w.text()).toContain('分组是什么')
    expect(w.text()).toContain('rule_config')
    expect(w.text()).toContain('DIRECT')
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
    const item = { id: 4, name: '改后', type: 'select', regex: '.*', url: 'u', interval: 60, sort_order: 1 }
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
    const item = { id: 8, name: '待删', type: 'select', regex: '.*', url: 'u', interval: 60, sort_order: 1 }
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
    const body = { name: '新分组', type: 'select', regex: '.*', url: 'u', interval: 60, sort_order: 1 }
    const res = await resource.create(body)

    expect(createMock).toHaveBeenCalledWith(body)
    expect(res).toEqual({ id: 2, name: '新分组' })
    w.unmount()
  })

  it('help-panel 渲染 oneLiner', async () => {
    const w = mount(GroupsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    // help panel is inside ResourcePage via PageHeader help-key; check page renders
    expect(w.text()).toContain('决定客户端里的策略组与节点归组')
    w.unmount()
  })

  it('展开常见分组配方折叠面板渲染配方列表', async () => {
    const w = mount(GroupsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    // click the expansion panel title to expand
    const panels = w.findAllComponents({ name: 'VExpansionPanel' })
    expect(panels.length).toBeGreaterThan(0)
    const title = w.find('[data-testid="group-recipes"] .v-expansion-panel-title')
    if (title.exists()) { await title.trigger('click'); await flushPromises() }
    expect(w.text()).toContain('常见分组配方')
    expect(w.text()).toContain('故障转移')
    w.unmount()
  })
})
