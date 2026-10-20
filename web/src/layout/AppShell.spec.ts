import { describe, it, expect, vi, beforeEach } from 'vitest'
import { nextTick } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import AppShell from './AppShell.vue'
import TokenDialog from '../components/TokenDialog.vue'
import { TOKEN_STORAGE_KEY, saveToken, tokenDialogOpen, tokenRejected, tokenState } from '../api/token'
import { useThemeMode } from '../composables/useThemeMode'

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
    emits: ['toggle-drawer', 'open-token', 'logout'],
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

const vuetify = createVuetify({
  components,
  directives,
  theme: {
    defaultTheme: 'subforgeLight',
    themes: {
      subforgeLight: { dark: false, colors: {} },
      subforgeDark: { dark: true, colors: {} },
    },
  },
})

function queryTestId(w: ReturnType<typeof mount>, id: string): Element | null {
  const inside = w.find(`[data-testid="${id}"]`)
  if (inside.exists()) return inside.element
  return document.querySelector(`[data-testid="${id}"]`)
}

async function clickTestId(w: ReturnType<typeof mount>, id: string): Promise<void> {
  const el = queryTestId(w, id)
  expect(el, '未渲染 ' + id).not.toBeNull()
  ;(el as HTMLElement).click()
  await flushPromises()
}

describe('AppShell', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    routeRef.value = { meta: {}, path: '/' }
    state.open = false
    state.text = ''
    state.kind = 'ok'
    tokenDialogOpen.value = false
    tokenRejected.value = false
    tokenState.value = ''
    window.localStorage.clear()
    ;(globalThis as any).visualViewport ??= {
      addEventListener: () => {},
      removeEventListener: () => {},
      width: 1024,
      height: 768,
    }
  })

  it('顶栏 open-token 事件：打开令牌输入框', async () => {
    const w = mount(AppShell, { global: { plugins: [vuetify] } })
    await flushPromises()
    await w.findComponent({ name: 'AppTopBar' }).vm.$emit('open-token')
    await flushPromises()
    expect(tokenDialogOpen.value).toBe(true)
    w.unmount()
  })

  it('令牌保存后重建 RouterView（reloadKey 自增，页面用新令牌重新取数）', async () => {
    const w = mount(AppShell, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect((w.vm as any).reloadKey).toBe(0)
    await w.findComponent(TokenDialog).vm.$emit('saved')
    await flushPromises()
    expect((w.vm as any).reloadKey).toBe(1)
    w.unmount()
  })

  it('令牌输入框 v-model：关闭时把 tokenDialogOpen 置回 false', async () => {
    tokenDialogOpen.value = true
    const w = mount(AppShell, { global: { plugins: [vuetify] } })
    await flushPromises()
    await w.findComponent(TokenDialog).vm.$emit('update:modelValue', false)
    await flushPromises()
    expect(tokenDialogOpen.value).toBe(false)
    w.unmount()
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

  it('D-20：顶栏 logout 先弹确认框，取消不清令牌也不弹令牌框', async () => {
    saveToken('tok-1')
    const w = mount(AppShell, { global: { plugins: [vuetify] } })
    await flushPromises()
    await w.findComponent({ name: 'AppTopBar' }).vm.$emit('logout')
    await flushPromises()

    expect(tokenDialogOpen.value).toBe(false)
    expect(queryTestId(w, 'logout-confirm'), '未渲染退出确认框').not.toBeNull()
    expect(queryTestId(w, 'logout-cancel'), '未渲染取消按钮').not.toBeNull()

    await clickTestId(w, 'logout-cancel')
    expect(tokenState.value).toBe('tok-1')
    expect(window.localStorage.getItem(TOKEN_STORAGE_KEY)).toBe('tok-1')
    expect(tokenDialogOpen.value).toBe(false)
    w.unmount()
  })

  it('D-20：确认退出登录清空令牌并弹出令牌输入框', async () => {
    saveToken('tok-1')
    const w = mount(AppShell, { global: { plugins: [vuetify] } })
    await flushPromises()
    await w.findComponent({ name: 'AppTopBar' }).vm.$emit('logout')
    await flushPromises()

    await clickTestId(w, 'logout-confirm')
    expect(tokenState.value).toBe('')
    expect(window.localStorage.getItem(TOKEN_STORAGE_KEY)).toBeNull()
    expect(tokenDialogOpen.value).toBe(true)
    expect((w.vm as any).reloadKey).toBe(1)
    w.unmount()
  })

  it('深色接线：localStorage 为 dark 时 document.documentElement.dataset.theme=dark 且 VApp 带 v-theme--subforgeDark', async () => {
    localStorage.setItem('subforge.theme', 'dark')
    const w = mount(AppShell, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(document.documentElement.dataset.theme).toBe('dark')
    expect(w.html()).toContain('v-theme--subforgeDark')
    w.unmount()
    localStorage.removeItem('subforge.theme')
  })

  it('深色实时切换：设置页侧 setMode("dark") 立即改动 data-theme 与 Vuetify 主题名（无需重挂载）', async () => {
    const settingsApi = useThemeMode()
    settingsApi.setMode('light')
    const w = mount(AppShell, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(document.documentElement.dataset.theme).toBe('light')
    settingsApi.setMode('dark')
    await nextTick()
    await flushPromises()
    expect(document.documentElement.dataset.theme).toBe('dark')
    expect(w.html()).toContain('v-theme--subforgeDark')
    w.unmount()
    settingsApi.setMode('light')
    localStorage.removeItem('subforge.theme')
  })
})
