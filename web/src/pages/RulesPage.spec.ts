import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import RulesPage from './RulesPage.vue'
import ResourcePage from '../components/ResourcePage.vue'
import { endpoints } from '../api/endpoints'

vi.mock('../api/endpoints', async () => {
  const actual = await vi.importActual<any>('../api/endpoints')
  return {
    ...actual,
    endpoints: {
      listRules: vi.fn(),
      createRule: vi.fn(),
      updateRule: vi.fn(),
      deleteRule: vi.fn(),
    },
  }
})

const listMock = endpoints.listRules as unknown as ReturnType<typeof vi.fn>
const createMock = endpoints.createRule as unknown as ReturnType<typeof vi.fn>
const updateMock = endpoints.updateRule as unknown as ReturnType<typeof vi.fn>
const deleteMock = endpoints.deleteRule as unknown as ReturnType<typeof vi.fn>
const vuetify = createVuetify({ components, directives })

describe('RulesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listMock.mockResolvedValue([])
  })

  it('把 title/subtitle/columns/fields/resource 原样传给 ResourcePage', async () => {
    const w = mount(RulesPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    const rp = w.findComponent(ResourcePage)
    expect(rp.exists()).toBe(true)
    expect(rp.props('title')).toBe('规则')
    expect(rp.props('subtitle')).toBe('转换规则')
    expect(rp.props('columns')).toEqual([
      { key: 'rule_type', title: '类型' },
      { key: 'rule_param', title: '参数' },
      { key: 'rule_config', title: '配置' },
      { key: 'resolve_dns', title: '解析DNS' },
      { key: 'enabled', title: '启用' },
    ])
    expect(rp.props('fields')).toEqual([
      { key: 'rule_type', label: '类型', type: 'text' },
      { key: 'rule_param', label: '参数', type: 'text' },
      { key: 'rule_config', label: '配置', type: 'number' },
      { key: 'resolve_dns', label: '解析DNS', type: 'switch' },
      { key: 'enabled', label: '启用', type: 'switch' },
    ])
    const resource = rp.props('resource') as any
    expect(resource.list).toBe(endpoints.listRules)
    expect(resource.create).toBe(endpoints.createRule)
    w.unmount()
  })

  it('resource.list 调 endpoints.listRules，并把返回项渲染进表格', async () => {
    listMock.mockResolvedValue([
      { id: 1, rule_type: 'domain', rule_param: 'example.com', rule_config: 1, resolve_dns: true, enabled: true, sort_order: 1 },
    ])

    const w = mount(RulesPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    expect(listMock).toHaveBeenCalledTimes(1)
    expect(w.text()).toContain('domain')
    // enabled=true 模板分支渲染「启用」
    expect(w.text()).toContain('启用')
    w.unmount()
  })

  it('resource.update(item) 调 endpoints.updateRule(item.id, item) 并透传结果', async () => {
    updateMock.mockResolvedValue({ id: 6, rule_type: 'domain' })
    const w = mount(RulesPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    const resource = w.findComponent(ResourcePage).props('resource') as any
    const item = { id: 6, rule_type: 'domain', rule_param: 'p', rule_config: 0, resolve_dns: false, enabled: true }
    const res = await resource.update(item)

    expect(updateMock).toHaveBeenCalledTimes(1)
    expect(updateMock).toHaveBeenCalledWith(6, item)
    expect(res).toEqual({ id: 6, rule_type: 'domain' })
    w.unmount()
  })

  it('resource.remove(item) 调 endpoints.deleteRule(item.id) 并透传结果', async () => {
    deleteMock.mockResolvedValue({ ok: true })
    const w = mount(RulesPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    const resource = w.findComponent(ResourcePage).props('resource') as any
    const item = { id: 11, rule_type: 'domain', rule_param: 'p', rule_config: 0, resolve_dns: false, enabled: true }
    const res = await resource.remove(item)

    expect(deleteMock).toHaveBeenCalledTimes(1)
    expect(deleteMock).toHaveBeenCalledWith(11)
    expect(res).toEqual({ ok: true })
    w.unmount()
  })

  it('resource.create 就是 endpoints.createRule 本身', async () => {
    createMock.mockResolvedValue({ id: 5, rule_type: 'domain' })
    const w = mount(RulesPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    const resource = w.findComponent(ResourcePage).props('resource') as any
    const body = { rule_type: 'domain', rule_param: 'p', rule_config: 0, resolve_dns: false, enabled: true }
    const res = await resource.create(body)

    expect(createMock).toHaveBeenCalledWith(body)
    expect(res).toEqual({ id: 5, rule_type: 'domain' })
    w.unmount()
  })
})
