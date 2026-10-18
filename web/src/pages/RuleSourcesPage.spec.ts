import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import RuleSourcesPage from './RuleSourcesPage.vue'
import { endpoints } from '../api/endpoints'

vi.mock('../api/endpoints', () => ({
  endpoints: {
    rulesets: vi.fn(),
  },
}))

const vuetify = createVuetify({ components, directives })

describe('RuleSourcesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('成功加载：调用 rulesets("")，渲染规则集映射与规则列表', async () => {
    vi.mocked(endpoints.rulesets).mockResolvedValue({
      rulesets: { 'rs1': '/path/to/rs1' },
      rules: ['rule-a', 'rule-b'],
    })
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(endpoints.rulesets).toHaveBeenCalledTimes(1)
    expect(endpoints.rulesets).toHaveBeenCalledWith('')
    expect(w.find('[data-testid="async-error"]').exists()).toBe(false)
    expect(w.find('[data-testid="async-empty"]').exists()).toBe(false)
    expect(w.text()).toContain('规则集映射')
    expect(w.text()).toContain('规则列表')
    expect(w.text()).toContain('rs1')
    expect(w.text()).toContain('rule-a')
  })

  it('空数据：显示空态文案', async () => {
    vi.mocked(endpoints.rulesets).mockResolvedValue({
      rulesets: {},
      rules: [],
    })
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.find('[data-testid="async-empty"]').exists()).toBe(true)
    expect(w.text()).toContain('暂无规则源')
  })

  it('加载失败：显示错误，点击重试重新加载', async () => {
    vi.mocked(endpoints.rulesets).mockRejectedValueOnce(new Error('boom'))
    const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.find('[data-testid="async-error"]').exists()).toBe(true)
    expect(w.text()).toContain('boom')
    vi.mocked(endpoints.rulesets).mockResolvedValue({ rulesets: {}, rules: [] })
    await w.find('[data-testid="async-retry"]').trigger('click')
    await flushPromises()
    expect(endpoints.rulesets).toHaveBeenCalledTimes(2)
  })
})
