import { describe, it, expect } from 'vitest'
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
