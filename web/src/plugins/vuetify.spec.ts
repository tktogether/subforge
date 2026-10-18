import { describe, it, expect, vi } from 'vitest'
import { createApp } from 'vue'
import { vuetify } from './vuetify'

describe('vuetify 插件', () => {
  it('导出的是 Vue 插件对象（install 为函数）', () => {
    expect(typeof vuetify.install).toBe('function')
  })

  it('默认主题为 subforge 且为亮色', () => {
    expect(vuetify.theme.name.value).toBe('subforge')
    expect(vuetify.theme.themes.value.subforge.dark).toBe(false)
  })

  it('subforge 主题色符合设计约定', () => {
    const colors = vuetify.theme.themes.value.subforge.colors
    expect(colors.primary).toBe('#1976d2')
    expect(colors.secondary).toBe('#00897b')
    expect(colors.background).toBe('#f4f6f8')
    expect(colors.surface).toBe('#ffffff')
  })

  it('注册了 Vuetify 组件与指令（经真实 app 安装后可见）', () => {
    const app = createApp({ render: () => null })
    app.use(vuetify)
    expect(app.component('VBtn')).toBeTruthy()
    expect(app.directive('Ripple')).toBeTruthy()
  })

  it('模块级单例：静态 import 与动态 import 拿到同一实例，重置后仍可重新实例化', async () => {
    const same = (await import('./vuetify')).vuetify
    expect(same).toBe(vuetify)

    vi.resetModules()
    const rebuilt = (await import('./vuetify')).vuetify
    expect(rebuilt).not.toBe(vuetify)
    expect(typeof rebuilt.install).toBe('function')
    expect(rebuilt.theme.name.value).toBe('subforge')
  })
})
