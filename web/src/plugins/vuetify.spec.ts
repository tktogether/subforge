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

  it('subforgeLight 主题色符合高对比浅色令牌', () => {
    const colors = vuetify.theme.themes.value.subforgeLight.colors
    expect(colors.primary).toBe('#174C80') // --sf-accent
    expect(colors.secondary).toBe('#103A63') // --sf-accent-hover
    expect(colors.success).toBe('#15803D') // --sf-ok
    expect(colors.warning).toBe('#9A5B00') // --sf-warn
    expect(colors.error).toBe('#B42318') // --sf-danger
    expect(colors.info).toBe('#0E7490') // --sf-info
    expect(colors.background).toBe('#EEF2F7') // --sf-bg
    expect(colors.surface).toBe('#FFFFFF') // --sf-surface
    expect(colors['surface-variant']).toBe('#F5F8FC') // --sf-surface-raised
    expect(colors['on-surface']).toBe('#0F172A') // --sf-text
    expect(colors['on-background']).toBe('#0F172A') // --sf-text
  })

  it('subforgeDark 主题为深色且符合高对比深色令牌', () => {
    const theme = vuetify.theme.themes.value.subforgeDark
    expect(theme.dark).toBe(true)
    expect(theme.colors.primary).toBe('#8BB8FF') // --sf-accent
    expect(theme.colors.secondary).toBe('#B3D1FF') // --sf-accent-hover
    expect(theme.colors.success).toBe('#5BD68A') // --sf-ok
    expect(theme.colors.warning).toBe('#F5C451') // --sf-warn
    expect(theme.colors.error).toBe('#FF9B90') // --sf-danger
    expect(theme.colors.info).toBe('#6FD8E8') // --sf-info
    expect(theme.colors.background).toBe('#0B1220') // --sf-bg
    expect(theme.colors.surface).toBe('#162033') // --sf-surface
    expect(theme.colors['surface-variant']).toBe('#1D2A40') // --sf-surface-raised
    expect(theme.colors['on-surface']).toBe('#F1F5F9') // --sf-text
    expect(theme.colors['on-background']).toBe('#F1F5F9') // --sf-text
  })

  it('两套主题都完整声明了 P3 语义映射的 11 个键', () => {
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
