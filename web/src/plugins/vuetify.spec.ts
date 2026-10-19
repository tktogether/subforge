import { describe, it, expect, vi } from 'vitest'
import { createApp } from 'vue'
import { vuetify } from './vuetify'

describe('vuetify 插件', () => {
  it('导出的是 Vue 插件对象（install 为函数）', () => {
    expect(typeof vuetify.install).toBe('function')
  })

  it('默认主题为 subforgeLight 且为亮色', () => {
    expect(vuetify.theme.name.value).toBe('subforgeLight')
    expect(vuetify.theme.themes.value.subforgeLight.dark).toBe(false)
  })

  it('subforgeLight 主题色符合 D1 浅色令牌（d1-instrument.html:54-70）', () => {
    const colors = vuetify.theme.themes.value.subforgeLight.colors
    expect(colors.primary).toBe('#215da5') // --sf-accent
    expect(colors.secondary).toBe('#006794') // --sf-info
    expect(colors.success).toBe('#007245') // --sf-ok
    expect(colors.warning).toBe('#754c00') // --sf-warn
    expect(colors.error).toBe('#a72626') // --sf-danger
    expect(colors.info).toBe('#006794') // --sf-info
    expect(colors.background).toBe('#f3f5f8') // --sf-bg
    expect(colors.surface).toBe('#fafbfc') // --sf-surface
    expect(colors['surface-variant']).toBe('#fdfdfe') // --sf-surface-raised
    expect(colors['on-surface']).toBe('#191f26') // --sf-text
    expect(colors['on-background']).toBe('#191f26') // --sf-text
  })

  it('subforgeDark 主题为深色且取 D1 深色令牌（d1-instrument.html:80-96）', () => {
    const theme = vuetify.theme.themes.value.subforgeDark
    expect(theme.dark).toBe(true)
    expect(theme.colors.primary).toBe('#4b78ba') // --sf-accent
    expect(theme.colors.secondary).toBe('#439ccc') // --sf-info
    expect(theme.colors.success).toBe('#52a76d') // --sf-ok
    expect(theme.colors.warning).toBe('#ce9a43') // --sf-warn
    expect(theme.colors.error).toBe('#dc655d') // --sf-danger
    expect(theme.colors.info).toBe('#439ccc') // --sf-info
    expect(theme.colors.background).toBe('#0d0f12') // --sf-bg
    expect(theme.colors.surface).toBe('#1e2124') // --sf-surface
    expect(theme.colors['surface-variant']).toBe('#24282b') // --sf-surface-raised
    expect(theme.colors['on-surface']).toBe('#eaedf1') // --sf-text
    expect(theme.colors['on-background']).toBe('#eaedf1') // --sf-text
  })

  it('两套主题都完整声明了 D1 语义映射的 11 个键', () => {
    const keys = [
      'primary',
      'secondary',
      'success',
      'warning',
      'error',
      'info',
      'background',
      'surface',
      'surface-variant',
      'on-surface',
      'on-background',
    ]
    for (const name of ['subforgeLight', 'subforgeDark'] as const) {
      const colors = vuetify.theme.themes.value[name].colors
      expect(Object.keys(colors).filter((k) => keys.includes(k)).sort()).toEqual([...keys].sort())
    }
  })

  it('内建文案中文化（v-data-table 页脚不再出现英文）', () => {
    expect(vuetify.locale.current.value).toBe('zhHans')
    const messages = vuetify.locale.messages.value.zhHans as {
      dataFooter: { itemsPerPageText: string; pageText: string }
    }
    // 取自 vuetify/lib/locale/zh-Hans.js:29-36，逐字比对，防止 v-data-table 页脚回落英文。
    expect(messages.dataFooter.itemsPerPageText).toBe('每页数目：')
    expect(messages.dataFooter.pageText).toBe('{0}-{1} 共 {2}')
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
    expect(rebuilt.theme.name.value).toBe('subforgeLight')
  })
})
