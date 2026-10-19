import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { nextTick } from 'vue'
import { createSubforgeApp } from './bootstrap'

describe('createSubforgeApp', () => {
  it('返回 Vue 应用实例，并装上 router 与 vuetify', async () => {
    const app = createSubforgeApp()

    expect(typeof app.mount).toBe('function')
    expect(typeof app.use).toBe('function')

    const router = app.config.globalProperties.$router
    expect(router).toBeDefined()
    await router.isReady()
    expect(router.currentRoute.value.name).toBe('overview')
    expect(app.component('VBtn')).toBeTruthy()
  })

  it('mount 到真实 DOM 后渲染出 AppShell 内容', async () => {
    document.body.innerHTML = '<div id="root"></div>'
    const app = createSubforgeApp()

    const router = app.config.globalProperties.$router
    app.mount('#root')
    await nextTick()
    await router.isReady()

    const root = document.querySelector('#root')
    expect(root).not.toBeNull()
    expect(root!.innerHTML.length).toBeGreaterThan(0)
    expect(root!.innerHTML).toContain('SubForge')

    app.unmount()
  })
})

import { vuetify } from './plugins/vuetify'
import { applyFirstPaintTheme } from './bootstrap'

describe('首帧主题（W6A）', () => {
  beforeEach(() => {
    localStorage.removeItem('subforge.theme')
  })
  afterEach(() => {
    localStorage.removeItem('subforge.theme')
    vi.unstubAllGlobals()
  })

  it('localStorage=dark → createSubforgeApp() 后 vuetify.theme.global.name.value === subforgeDark 且 data-theme=dark', () => {
    localStorage.setItem('subforge.theme', 'dark')
    createSubforgeApp()
    expect(vuetify.theme.global.name.value).toBe('subforgeDark')
    expect(document.documentElement.dataset.theme).toBe('dark')
  })

  it('localStorage=light → subforgeLight / light', () => {
    localStorage.setItem('subforge.theme', 'light')
    createSubforgeApp()
    expect(vuetify.theme.global.name.value).toBe('subforgeLight')
    expect(document.documentElement.dataset.theme).toBe('light')
  })

  it('system + matchMedia dark → subforgeDark / dark', () => {
    localStorage.setItem('subforge.theme', 'system')
    vi.stubGlobal('matchMedia', () => ({ matches: true }))
    createSubforgeApp()
    expect(vuetify.theme.global.name.value).toBe('subforgeDark')
    expect(document.documentElement.dataset.theme).toBe('dark')
  })

  it('system + matchMedia light → subforgeLight / light', () => {
    localStorage.setItem('subforge.theme', 'system')
    vi.stubGlobal('matchMedia', () => ({ matches: false }))
    createSubforgeApp()
    expect(vuetify.theme.global.name.value).toBe('subforgeLight')
    expect(document.documentElement.dataset.theme).toBe('light')
  })

  it('applyFirstPaintTheme 单独调用即可把主题名与 data-theme 设好（不依赖 app.use(vuetify)）', () => {
    localStorage.setItem('subforge.theme', 'dark')
    vi.stubGlobal('matchMedia', () => ({ matches: false }))
    applyFirstPaintTheme()
    expect(vuetify.theme.global.name.value).toBe('subforgeDark')
    expect(document.documentElement.dataset.theme).toBe('dark')
  })

  it('主题名在 app.use(vuetify) 之前就绪：install 被调用时主题名已是解析结果', () => {
    localStorage.setItem('subforge.theme', 'dark')
    vi.stubGlobal('matchMedia', () => ({ matches: false }))
    const realInstall = vuetify.install
    const seen: string[] = []
    const spy = vi.spyOn(vuetify, 'install').mockImplementation((app) => {
      seen.push(vuetify.theme.global.name.value)
      realInstall(app)
    })
    try {
      createSubforgeApp()
    } finally {
      spy.mockRestore()
    }
    expect(seen).toEqual(['subforgeDark'])
    expect(document.documentElement.dataset.theme).toBe('dark')
  })
})