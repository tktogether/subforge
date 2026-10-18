import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import PageHeader from './PageHeader.vue'
import { vuetify } from '../plugins/vuetify'

describe('PageHeader', () => {
  it('基础渲染：title 渲染到 h1，subtitle 渲染到 p', () => {
    const wrapper = mount(PageHeader, {
      props: { title: '机场', subtitle: '管理上游机场' },
      global: { plugins: [vuetify] },
    })
    expect(wrapper.find('h1').text()).toBe('机场')
    expect(wrapper.find('p').text()).toBe('管理上游机场')
  })

  it('actions slot 提供时：渲染 slot 内容', () => {
    const wrapper = mount(PageHeader, {
      props: { title: '机场', subtitle: '管理上游机场' },
      global: { plugins: [vuetify] },
      slots: { actions: '<button data-testid="act">A</button>' },
    })
    const act = wrapper.find('[data-testid="act"]')
    expect(act.exists()).toBe(true)
    expect(act.text()).toBe('A')
  })

  it('actions slot 不提供时：h1 仍存在且没有 button', () => {
    const wrapper = mount(PageHeader, {
      props: { title: '机场', subtitle: '管理上游机场' },
      global: { plugins: [vuetify] },
    })
    expect(wrapper.find('h1').exists()).toBe(true)
    expect(wrapper.findAll('button').length).toBe(0)
  })

  it('标题为空串也能渲染', () => {
    const wrapper = mount(PageHeader, {
      props: { title: '', subtitle: '' },
      global: { plugins: [vuetify] },
    })
    expect(wrapper.find('h1').text()).toBe('')
  })

  it('subtitle 放在带 text-body-2 类的 p 里', () => {
    const wrapper = mount(PageHeader, {
      props: { title: '机场', subtitle: '管理上游机场' },
      global: { plugins: [vuetify] },
    })
    const p = wrapper.find('p')
    expect(p.classes()).toContain('text-body-2')
  })
})
