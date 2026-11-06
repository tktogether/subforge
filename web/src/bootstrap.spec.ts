import { describe, it, expect, afterEach } from 'vitest'
import { createSubforgeApp, applyFirstPaintTheme } from './bootstrap'
import { THEME_STORAGE_KEY } from './composables/useThemeMode'
import { TOKEN_STORAGE_KEY } from './api/token'

afterEach(() => {
  window.localStorage.clear()
  delete document.documentElement.dataset.theme
})

describe('applyFirstPaintTheme', () => {
  it('localStorage=light → data-theme=light', () => {
    window.localStorage.setItem(THEME_STORAGE_KEY, 'light')
    applyFirstPaintTheme()
    expect(document.documentElement.dataset.theme).toBe('light')
  })

  it('localStorage=dark → data-theme=dark', () => {
    window.localStorage.setItem(THEME_STORAGE_KEY, 'dark')
    applyFirstPaintTheme()
    expect(document.documentElement.dataset.theme).toBe('dark')
  })

  it('未设置时默认深色', () => {
    applyFirstPaintTheme()
    expect(document.documentElement.dataset.theme).toBe('dark')
  })
})

describe('createSubforgeApp', () => {
  it('装配 router，落在概览路由，并完成首帧主题与令牌接线', async () => {
    window.localStorage.setItem(THEME_STORAGE_KEY, 'light')
    window.localStorage.setItem(TOKEN_STORAGE_KEY, 'test-token')

    const app = createSubforgeApp()
    expect(typeof app.mount).toBe('function')

    const router = app.config.globalProperties.$router
    expect(router).toBeDefined()
    await router.isReady()
    expect(router.currentRoute.value.name).toBe('overview')
    expect(document.documentElement.dataset.theme).toBe('light')
  })
})
