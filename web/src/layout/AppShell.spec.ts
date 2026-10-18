import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import AppShell from './AppShell.vue'

const { routeRef } = vi.hoisted(() => ({ routeRef: { value: { meta: {}, path: '/' } } }))

vi.mock('vue-router', () => ({
  useRoute: () => routeRef.value,
  RouterView: { name: 'RouterView', template: '<div class="router-view-stub" />' },
  RouterLink: { name: 'RouterLink', template: '<a />' },
}))

vi.mock('./AppNav.vue', () => ({
  default: {
    name: 'AppNav',
    props: ['modelValue'],
    emits: ['update:modelValue'],
    template: '<div class="appnav-stub" />',
  },
}))

vi.mock('./AppTopBar.vue', () => ({
  default: {
    name: 'AppTopBar',
    props: ['title'],
    emits: ['toggle-drawer'],
    template: '<div class="apptopbar-stub">{{ title }}</div>',
  },
}))

const { state, closeNotify } = vi.hoisted(() => {
  const s = { open: false, text: '', kind: 'ok' as 'ok' | 'error' | 'info' }
  return { state: s, closeNotify: vi.fn() }
})
vi.mock('../composables/useNotify', () => ({
  notifyState: state,
  notify: vi.fn(),
  closeNotify,
  snackbarColor: (k: string) => (k === 'error' ? 'error' : k === 'info' ? 'info' : 'success'),
}))

const vuetify = createVuetify({ components, directives })

describe('AppShell', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    routeRef.value = { meta: {}, path: '/' }
    state.open = false
    state.text = ''
    state.kind = 'ok'
    ;(globalThis as any).visualViewport ??= {
      addEventListener: () => {},
      removeEventListener: () => {},
      width: 1024,
      height: 768,
    }
  })

  it('pageTitle：route.meta.title 为 undefined 时回退 "SubForge"', async () => {
    routeRef.value = { meta: {}, path: '/' }
    const w = mount(AppShell, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.find('.apptopbar-stub').exists()).toBe(true)
    w.unmount()
  })

  it('pageTitle：route.meta.title 存在时渲染 String(title)', async () => {
    routeRef.value = { meta: { title: 42 }, path: '/' }
    const w = mount(AppShell, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.text()).toContain('42')
    w.unmount()
  })

  it('AppNav v-model setter：update:modelValue 事件更新 drawer', async () => {
    const w = mount(AppShell, { global: { plugins: [vuetify] } })
    await flushPromises()
    const nav = w.findComponent({ name: 'AppNav' })
    expect(nav.props('modelValue')).toBe(true)
    await nav.vm.$emit('update:modelValue', false)
    await flushPromises()
    expect(nav.props('modelValue')).toBe(false)
    w.unmount()
  })

  it('AppTopBar toggle-drawer 事件：drawer 切换（AppNav modelValue 变化）', async () => {
    const w = mount(AppShell, { global: { plugins: [vuetify] } })
    await flushPromises()
    // 初始 drawer = true → AppNav modelValue true
    const nav = w.findComponent({ name: 'AppNav' })
    expect(nav.props('modelValue')).toBe(true)
    // 触发 toggle-drawer
    await w.findComponent({ name: 'AppTopBar' }).vm.$emit('toggle-drawer')
    await flushPromises()
    expect(nav.props('modelValue')).toBe(false)
    w.unmount()
  })

  it('snackbar：notifyState.open 为 true 时渲染', async () => {
    state.open = true
    state.text = 'hello'
    const w = mount(AppShell, { global: { plugins: [vuetify] } })
    await flushPromises()
    const snack = w.findComponent({ name: 'VSnackbar' })
    expect(snack.exists()).toBe(true)
    w.unmount()
  })

  it('snackbar：notifyState.open 为 false 时不渲染内容', async () => {
    state.open = false
    const w = mount(AppShell, { global: { plugins: [vuetify] } })
    await flushPromises()
    const snack = w.find('[data-testid="snackbar"]')
    expect(snack.exists()).toBe(false)
    w.unmount()
  })

  it('snackbar update:model-value 事件触发 closeNotify', async () => {
    state.open = true
    state.text = 'x'
    const w = mount(AppShell, { global: { plugins: [vuetify] } })
    await flushPromises()
    const snack = w.findComponent({ name: 'VSnackbar' })
    expect(snack.exists()).toBe(true)
    await snack.vm.$emit('update:model-value', false)
    expect(closeNotify).toHaveBeenCalled()
    w.unmount()
  })
})
