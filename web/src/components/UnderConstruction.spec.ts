import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import UnderConstruction from './UnderConstruction.vue'
import { vuetify } from '../plugins/vuetify'

describe('UnderConstruction', () => {
  it('基础渲染：data-testid 存在且文本含 "机场 · 迁移中"', () => {
    const wrapper = mount(UnderConstruction, {
      props: { page: '机场' },
      global: { plugins: [vuetify] },
    })
    expect(wrapper.find('[data-testid="under-construction"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('机场 · 迁移中')
  })

  it('note 未提供时使用默认文案', () => {
    const wrapper = mount(UnderConstruction, {
      props: { page: '机场' },
      global: { plugins: [vuetify] },
    })
    expect(wrapper.text()).toContain('该页面已纳入 Web UI 重写计划，正按页面规格逐页实现。')
  })

  it('note 提供时显示 note 且不含默认文案', () => {
    const wrapper = mount(UnderConstruction, {
      props: { page: '机场', note: '正在实现中' },
      global: { plugins: [vuetify] },
    })
    expect(wrapper.text()).toContain('正在实现中')
    expect(wrapper.text()).not.toContain('该页面已纳入 Web UI 重写计划，正按页面规格逐页实现。')
  })

  it('plan 未提供时不含 "规格来源："', () => {
    const wrapper = mount(UnderConstruction, {
      props: { page: '机场' },
      global: { plugins: [vuetify] },
    })
    expect(wrapper.text()).not.toContain('规格来源：')
  })

  it('plan 提供时显示 "规格来源：" 与 plan 内容', () => {
    const wrapper = mount(UnderConstruction, {
      props: { page: '机场', plan: 'docs/design/web-ui.md §3' },
      global: { plugins: [vuetify] },
    })
    expect(wrapper.text()).toContain('规格来源：')
    expect(wrapper.text()).toContain('docs/design/web-ui.md §3')
  })

  it('同时提供 note 与 plan 时两者都在文本中', () => {
    const wrapper = mount(UnderConstruction, {
      props: { page: '机场', note: '正在实现中', plan: 'docs/design/web-ui.md §3' },
      global: { plugins: [vuetify] },
    })
    expect(wrapper.text()).toContain('正在实现中')
    expect(wrapper.text()).toContain('规格来源：')
    expect(wrapper.text()).toContain('docs/design/web-ui.md §3')
  })
})
