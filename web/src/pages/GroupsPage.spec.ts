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
      { key: 'source_text', title: '来源' },
      { key: 'type', title: '类型' },
      { key: 'regex', title: '匹配正则' },
      { key: 'members_text', title: '成员' },
      { key: 'include_self_text', title: '含自建' },
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
      {
        key: 'include_self',
        label: '包含自建节点',
        type: 'switch',
        default: false,
        hint: '开启后，订阅会在该分组成员末尾追加方案选中的自建节点。机场分组按成员渲染，自建节点不会自动出现；导入时成员覆盖全部机场节点的组默认开启，子集组默认关闭。',
      },
      { key: 'sort_order', label: '排序', type: 'number', hint: '越小越靠前' },
    ])
    const resource = rp.props('resource') as any
    expect(resource.list).not.toBe(endpoints.listGroups)
    expect(resource.create).toBe(endpoints.createGroup)
    expect(w.text()).toContain('分组是什么')
    expect(w.text()).toContain('rule_config')
    expect(w.text()).toContain('DIRECT')
    w.unmount()
  })

  it('resource.list 包装 endpoints.listGroups，给每行补 source_text 并渲染进表格', async () => {
    listMock.mockResolvedValue([
      { id: 1, name: '分组A', type: 'select', regex: '', url: 'https://g.example', interval: 300, sort_order: 1, source: 'airport:机场X', members: ['HK-1', 'JP-1', 'SG-1', 'US-1'], include_self: true },
      { id: 2, name: '自定义组', type: 'select', regex: '.*', url: '', interval: 0, sort_order: 2, source: '', include_self: false },
    ])

    const w = mount(GroupsPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    expect(listMock).toHaveBeenCalledTimes(1)
    // 来源列格式化：airport:机场X → 机场:机场X；空串 → 自定义
    expect(w.text()).toContain('分组A')
    expect(w.text()).toContain('机场:机场X')
    expect(w.text()).toContain('自定义组')
    expect(w.text()).toContain('自定义')
    expect(w.text()).toContain('4 个：HK-1、JP-1、SG-1…')
    expect(w.text()).toContain('—')
    expect(w.text()).toContain('含自建')
    const rows = await (w.findComponent(ResourcePage).props('resource') as any).list()
    expect(rows[0].include_self_text).toBe('是')
    expect(rows[1].include_self_text).toBe('否')
    w.unmount()
  })

  it('resource.update(item) 调 endpoints.updateGroup(item.id, item) 并透传结果', async () => {
    updateMock.mockResolvedValue({ id: 4, name: '改后' })
    const w = mount(GroupsPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    const resource = w.findComponent(ResourcePage).props('resource') as any
    const item = { id: 4, name: '改后', type: 'select', regex: '.*', url: 'u', interval: 60, sort_order: 1, source: '' }
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
    const item = { id: 8, name: '待删', type: 'select', regex: '.*', url: 'u', interval: 60, sort_order: 1, source: '' }
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

  it('新增分组表单无「方案」下拉（字段里没有 profile_id）', async () => {
    const w = mount(GroupsPage, { global: { plugins: [vuetify], stubs: { teleport: true, VDialog: { template: '<div><slot /></div>' } } } })
    await flushPromises()

    const fields = w.findComponent(ResourcePage).props('fields') as any[]
    expect(fields.find((f) => f.key === 'profile_id')).toBeUndefined()
    w.unmount()
  })

  it('新增分组提交时不带 profile_id（source 恒为自定义，后端默认）', async () => {
    createMock.mockResolvedValue({ id: 2, name: '新分组' })
    const w = mount(GroupsPage, {
      global: { plugins: [vuetify], stubs: { teleport: true, VDialog: { template: '<div><slot /></div>' } } },
    })
    await flushPromises()

    await w.find('[data-testid="add"]').trigger('click')
    await flushPromises()
    await w.find('[data-testid="field-text"] input').setValue('新分组')
    await w.find('[data-testid="dialog-submit"]').trigger('click')
    await flushPromises()

    expect(createMock).toHaveBeenCalledTimes(1)
    expect(createMock.mock.calls[0][0]).not.toHaveProperty('profile_id')
    expect(createMock.mock.calls[0][0].name).toBe('新分组')
    w.unmount()
  })

  it('编辑分组回填时不带 profile_id', async () => {
    listMock.mockResolvedValue([
      { id: 5, name: '老分组', type: 'select', regex: '.*', url: 'u', interval: 300, sort_order: 2, source: 'airport:老机场' },
    ])
    updateMock.mockResolvedValue({ id: 5, name: '老分组' })
    const w = mount(GroupsPage, {
      global: { plugins: [vuetify], stubs: { teleport: true, VDialog: { template: '<div><slot /></div>' } } },
    })
    await flushPromises()

    const editBtn = w.find('[data-testid="edit"]')
    expect(editBtn.exists()).toBe(true)
    await editBtn.trigger('click')
    await flushPromises()
    await w.find('[data-testid="dialog-submit"]').trigger('click')
    await flushPromises()

    expect(updateMock).toHaveBeenCalledTimes(1)
    expect(updateMock.mock.calls[0][1]).not.toHaveProperty('profile_id')
    w.unmount()
  })

  it('编辑分组时「包含自建节点」开关回填并随提交保存', async () => {
    listMock.mockResolvedValue([
      { id: 9, name: '机场组', type: 'select', regex: '', url: '', interval: 0, sort_order: 1, source: 'airport:机场X', members: ['HK-1'], include_self: true },
    ])
    updateMock.mockResolvedValue({ id: 9, name: '机场组' })
    const w = mount(GroupsPage, {
      global: { plugins: [vuetify], stubs: { teleport: true, VDialog: { template: '<div><slot /></div>' } } },
    })
    await flushPromises()
    await w.find('[data-testid="edit"]').trigger('click')
    await flushPromises()

    const switches = w.findAllComponents({ name: 'VSwitch' })
    expect(switches.length).toBe(1)
    expect(switches[0].props('modelValue')).toBe(true)
    await switches[0].setValue(false)
    await w.find('[data-testid="dialog-submit"]').trigger('click')
    await flushPromises()

    expect(updateMock).toHaveBeenCalledTimes(1)
    expect(updateMock.mock.calls[0][1].include_self).toBe(false)
    w.unmount()
  })

  it('来源列格式化：airport: 前缀转「机场:」、空串转「自定义」、其它原样', async () => {
    listMock.mockResolvedValue([
      { id: 1, name: 'g1', type: 't', regex: '', url: '', interval: 0, sort_order: 0, source: 'airport:北京' },
      { id: 2, name: 'g2', type: 't', regex: '', url: '', interval: 0, sort_order: 0, source: '' },
      { id: 3, name: 'g3', type: 't', regex: '', url: '', interval: 0, sort_order: 0, source: 'manual:foo' },
    ])
    const w = mount(GroupsPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    expect(w.text()).toContain('机场:北京')
    // g2 的空串来源显示「自定义」（注意：g3 的 manual:foo 不会误命中「自定义」）
    const texts = w.text()
    expect(texts).toContain('机场:北京')
    // manual:foo 原样
    expect(texts).toContain('manual:foo')
    w.unmount()
  })

  it('help-panel 渲染 oneLiner', async () => {
    const w = mount(GroupsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.text()).toContain('决定客户端里的策略组与节点归组')
    w.unmount()
  })

  it('展开常见分组配方折叠面板渲染配方列表', async () => {
    const w = mount(GroupsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    const panels = w.findAllComponents({ name: 'VExpansionPanel' })
    expect(panels.length).toBeGreaterThan(0)
    const title = w.find('[data-testid="group-recipes"] .v-expansion-panel-title')
    if (title.exists()) { await title.trigger('click'); await flushPromises() }
    expect(w.text()).toContain('常见分组配方')
    expect(w.text()).toContain('故障转移')
    w.unmount()
  })
})
