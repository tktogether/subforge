import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import AsyncSection from './AsyncSection.vue'
import { vuetify } from '../plugins/vuetify'

describe('AsyncSection', () => {
  it('分支1：error 非空时显示 async-error，含错误文本；loading/empty/slot 均不存在', () => {
    const wrapper = mount(AsyncSection, {
      props: { loading: false, error: '炸了', empty: false, emptyText: '空' },
      global: { plugins: [vuetify] },
    })
    const err = wrapper.find('[data-testid="async-error"]')
    expect(err.exists()).toBe(true)
    expect(err.text()).toContain('炸了')
    expect(wrapper.find('[data-testid="async-loading"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="async-empty"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="slot-content"]').exists()).toBe(false)
  })

  it('分支1 重试：点击 async-retry 触发 retry 事件，emitted 长度为 1', async () => {
    const wrapper = mount(AsyncSection, {
      props: { loading: false, error: '炸了', empty: false, emptyText: '空' },
      global: { plugins: [vuetify] },
    })
    await wrapper.find('[data-testid="async-retry"]').trigger('click')
    const emitted = wrapper.emitted('retry')
    expect(emitted).toBeTruthy()
    expect(emitted!.length).toBe(1)
  })

  it('分支2：error 为空 + loading true 时显示 async-loading', () => {
    const wrapper = mount(AsyncSection, {
      props: { loading: true, error: '', empty: false, emptyText: '空' },
      global: { plugins: [vuetify] },
    })
    expect(wrapper.find('[data-testid="async-loading"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="async-error"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="async-empty"]').exists()).toBe(false)
  })

  it('分支3：error 为空 + loading false + empty true 时显示 async-empty，含 emptyText', () => {
    const wrapper = mount(AsyncSection, {
      props: { loading: false, error: '', empty: true, emptyText: '还没有数据' },
      global: { plugins: [vuetify] },
    })
    const empty = wrapper.find('[data-testid="async-empty"]')
    expect(empty.exists()).toBe(true)
    expect(empty.text()).toContain('还没有数据')
  })

  it('分支4：全部 false 时渲染默认 slot 内容', () => {
    const wrapper = mount(AsyncSection, {
      props: { loading: false, error: '', empty: false, emptyText: '空' },
      global: { plugins: [vuetify] },
      slots: { default: '<div data-testid="slot-content">正文</div>' },
    })
    const slot = wrapper.find('[data-testid="slot-content"]')
    expect(slot.exists()).toBe(true)
    expect(slot.text()).toContain('正文')
    expect(wrapper.find('[data-testid="async-error"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="async-loading"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="async-empty"]').exists()).toBe(false)
  })
})
