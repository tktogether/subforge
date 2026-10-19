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

  it('subforgeLight 主题色符合 P3 浅色令牌（palette-p3-forest.html:10-25）', () => {
    const colors = vuetify.theme.themes.value.subforgeLight.colors
    expect(colors.primary).toBe('#4F6E35') // --sf-accent
    expect(colors.secondary).toBe('#405B2B') // --sf-accent-hover
    expect(colors.success).toBe('#2b7b41') // --sf-ok
    expect(colors.warning).toBe('#976000') // --sf-warn
    expect(colors.error).toBe('#B3382E') // --sf-danger
    expect(colors.info).toBe('#2B6198') // --sf-info
    expect(colors.background).toBe('#F0F2E6') // --sf-bg
    expect(colors.surface).toBe('#F9FAEF') // --sf-surface
    expect(colors['surface-variant']).toBe('#FDFEF6') // --sf-surface-raised
    expect(colors['on-surface']).toBe('#242B26') // --sf-text
    expect(colors['on-background']).toBe('#242B26') // --sf-text
  })

  it('subforgeDark 主题为深色且取 P3 深色令牌（palette-p3-forest.html:31-46）', () => {
    const theme = vuetify.theme.themes.value.subforgeDark
    expect(theme.dark).toBe(true)
    expect(theme.colors.primary).toBe('#A5C08B') // --sf-accent
    expect(theme.colors.secondary).toBe('#B7D2A0') // --sf-accent-hover
    expect(theme.colors.success).toBe('#93CA83') // --sf-ok
    expect(theme.colors.warning).toBe('#DBBC7F') // --sf-warn
    expect(theme.colors.error).toBe('#f9837a') // --sf-danger
    expect(theme.colors.info).toBe('#7CB6C8') // --sf-info
    expect(theme.colors.background).toBe('#232B30') // --sf-bg
    expect(theme.colors.surface).toBe('#323C41') // --sf-surface
    expect(theme.colors['surface-variant']).toBe('#3B474D') // --sf-surface-raised
    expect(theme.colors['on-surface']).toBe('#D8DCC6') // --sf-text
    expect(theme.colors['on-background']).toBe('#D8DCC6') // --sf-text
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
