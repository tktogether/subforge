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

  it('分支1 重试：点击 async-retry 触发 retry 事件', async () => {
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

  it('hint 非空 + error 非空时渲染 async-hint', () => {
    const wrapper = mount(AsyncSection, {
      props: { loading: false, error: '炸了', empty: false, emptyText: '空', hint: '检查机场地址是否可达' },
      global: { plugins: [vuetify] },
    })
    const h = wrapper.find('[data-testid="async-hint"]')
    expect(h.exists()).toBe(true)
    expect(h.text()).toContain('检查机场地址是否可达')
  })

  it('hint 非空但 error 为空时不渲染 async-hint', () => {
    const wrapper = mount(AsyncSection, {
      props: { loading: false, error: '', empty: true, emptyText: '空', hint: '提示' },
      global: { plugins: [vuetify] },
    })
    expect(wrapper.find('[data-testid="async-hint"]').exists()).toBe(false)
  })

  it('empty 给 EmptyStateHelp 对象时渲染 empty-state', () => {
    const wrapper = mount(AsyncSection, {
      props: {
        loading: false, error: '', empty: { title: '还没有机场', detail: '点新增添加' }, emptyText: '空',
      },
      global: { plugins: [vuetify] },
    })
    const e = wrapper.find('[data-testid="empty-state"]')
    expect(e.exists()).toBe(true)
    expect(wrapper.find('[data-testid="empty-state-title"]').text()).toContain('还没有机场')
    expect(wrapper.find('[data-testid="empty-state-detail"]').text()).toContain('点新增添加')
  })

  it('empty 的 action 有 to 时渲染 empty-state-action 为链接', () => {
    const wrapper = mount(AsyncSection, {
      props: {
        loading: false, error: '',
        empty: { title: 't', detail: 'd', action: { label: '去添加', to: '/airports' } },
        emptyText: '空',
      },
      global: { plugins: [vuetify] },
    })
    const a = wrapper.find('[data-testid="empty-state-action"]')
    expect(a.exists()).toBe(true)
    expect(a.text()).toContain('去添加')
  })

  it('D-22：errorDetail 非空时渲染可折叠的详情块，主文案区不重复原始报文', () => {
    const wrapper = mount(AsyncSection, {
      props: {
        loading: false,
        error: '服务端内部错误（请求 ID r-1）',
        empty: false,
        emptyText: '空',
        errorDetail: 'boom: 内部细节',
      },
      global: { plugins: [vuetify] },
    })
    const err = wrapper.find('[data-testid="async-error"]')
    expect(err.text()).toContain('服务端内部错误（请求 ID r-1）')
    expect(err.text()).not.toContain('boom: 内部细节')
    const det = wrapper.find('[data-testid="async-error-detail"]')
    expect(det.exists()).toBe(true)
    expect(det.text()).toContain('boom: 内部细节')
    expect(det.text()).toContain('查看错误详情')
  })

  it('D-22：errorDetail 为空时不渲染详情块', () => {
    const wrapper = mount(AsyncSection, {
      props: { loading: false, error: '炸了', empty: false, emptyText: '空', errorDetail: '' },
      global: { plugins: [vuetify] },
    })
    expect(wrapper.find('[data-testid="async-error-detail"]').exists()).toBe(false)
  })

  it('empty 的 action 无 to 时渲染 empty-state-action 为按钮', () => {
    const wrapper = mount(AsyncSection, {
      props: {
        loading: false, error: '',
        empty: { title: 't', detail: 'd', action: { label: '刷新' } },
        emptyText: '空',
      },
      global: { plugins: [vuetify] },
    })
    const a = wrapper.find('[data-testid="empty-state-action"]')
    expect(a.exists()).toBe(true)
    expect(a.text()).toContain('刷新')
    expect(a.attributes('href')).toBeFalsy()
  })
})
