import { describe, expect, it, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import AppTopBar from './AppTopBar.vue'
import { tokenState } from '../api/token'

const vuetify = createVuetify({ components, directives })

function mountTopBar(props: { title: string }) {
  return mount({
    components: { AppTopBar },
    template: '<v-layout><AppTopBar :title="title" /></v-layout>',
    data() { return { title: props.title } },
  }, { global: { plugins: [vuetify] } })
}

describe('AppTopBar', () => {
  beforeEach(() => {
    tokenState.value = ''
  })

  it('渲染 title 与 SubForge wordmark', () => {
    const w = mountTopBar({ title: '机场' })
    const topbar = w.find('[data-testid="topbar"]')
    expect(topbar.exists()).toBe(true)
    expect(w.text()).toContain('机场')
    expect(w.text()).toContain('SubForge')
    expect(w.find('img[src="/favicon.svg"]').exists()).toBe(true)
  })

  it('点击 drawer-toggle 触发 toggle-drawer 事件', async () => {
    const w = mountTopBar({ title: '机场' })
    const cmp = w.findComponent(AppTopBar)
    await w.find('[data-testid="drawer-toggle"]').trigger('click')
    const emitted = cmp.emitted('toggle-drawer')
    expect(emitted).toBeTruthy()
    expect(emitted!.length).toBe(1)
  })

  it('点击令牌按钮触发 open-token 事件', async () => {
    const w = mountTopBar({ title: '机场' })
    const cmp = w.findComponent(AppTopBar)
    await w.find('[data-testid="token-button"]').trigger('click')
    expect(cmp.emitted('open-token')).toBeTruthy()
  })

  it('未设置令牌时按钮提示「设置访问令牌」', () => {
    tokenState.value = ''
    const w = mountTopBar({ title: '机场' })
    expect(w.find('[data-testid="token-button"]').attributes('title')).toBe('设置访问令牌')
  })

  it('已设置令牌时按钮提示「已设置访问令牌，点击修改」', () => {
    tokenState.value = 'tok'
    const w = mountTopBar({ title: '机场' })
    expect(w.find('[data-testid="token-button"]').attributes('title')).toBe('已设置访问令牌，点击修改')
  })

  it('D-20：渲染「退出登录」入口并带可读提示', () => {
    const w = mountTopBar({ title: '机场' })
    const btn = w.find('[data-testid="logout"]')
    expect(btn.exists()).toBe(true)
    expect(btn.attributes('title')).toBe('退出登录')
    expect(btn.attributes('aria-label')).toBe('退出登录')
  })

  it('D-20：点击「退出登录」触发 logout 事件', async () => {
    const w = mountTopBar({ title: '机场' })
    const cmp = w.findComponent(AppTopBar)
    await w.find('[data-testid="logout"]').trigger('click')
    const emitted = cmp.emitted('logout')
    expect(emitted).toBeTruthy()
    expect(emitted!.length).toBe(1)
  })

  it('title 为空串也能渲染', () => {
    const w = mountTopBar({ title: '' })
    expect(w.find('[data-testid="topbar"]').exists()).toBe(true)
  })


  it('页面模式入口常驻顶栏', () => {
    const w = mountTopBar({ title: '机场' })
    const btn = w.find('[data-testid="theme-button"]')
    expect(btn.exists()).toBe(true)
    expect(btn.attributes('title')).toBe('页面模式')
    expect(btn.attributes('aria-label')).toBe('页面模式')
  })



  it('页面模式菜单可切换主题', async () => {
    localStorage.removeItem('subforge.theme')
    const w = mount({
      components: { VLayout: components.VLayout, AppTopBar },
      template: '<v-layout><AppTopBar title="机场" /></v-layout>',
    }, {
      global: {
        plugins: [vuetify],
        stubs: { VMenu: { template: '<div><slot name="activator" :props="{}" /><slot /></div>' } },
      },
    })
    const dark = w.find('[data-testid="theme-dark"]')
    expect(dark.exists()).toBe(true)
    await dark.trigger('click')
    expect(localStorage.getItem('subforge.theme')).toBe('dark')
    w.unmount()
  })

})
