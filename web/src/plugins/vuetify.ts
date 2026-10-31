import 'vuetify/styles'
import '@mdi/font/css/materialdesignicons.css'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import { zhHans } from 'vuetify/locale'

// 高对比中性蓝双主题（subforgeLight / subforgeDark）。
// 令牌真值源是 web/src/design/tokens.css，两处的一致性由
// web/src/plugins/vuetify.spec.ts 的断言锁定。
// 注意：这里必须写字面 hex 而不是 var(--sf-*)。Vuetify 生成 --v-theme-* 时会对每个
// 色值调用 parseColor()（web/node_modules/vuetify/lib/composables/theme.js:124，
// 实现见 web/node_modules/vuetify/lib/util/colorUtils.js:81-91），传 var(--sf-accent)
// 会落到 parseInt(hex,16)=NaN 分支并打印 "is not a valid hex(a) color" 警告。
export const vuetify = createVuetify({
  components,
  directives,
  // 界面全中文，Vuetify 内建文案（v-data-table 的 "Items per page" / "1-3 of 3"）也必须中文化，
  // 否则表格页脚是整页唯一的英文残留。zhHans 来自 vuetify/locale（package exports "./locale"）。
  locale: {
    locale: 'zhHans',
    fallback: 'en',
    messages: { zhHans },
  },
  theme: {
    defaultTheme: 'subforgeLight',
    themes: {
      subforgeLight: {
        dark: false,
        colors: {
          primary: '#174C80', // --sf-accent
          secondary: '#103A63', // --sf-accent-hover
          success: '#15803D', // --sf-ok
          warning: '#9A5B00', // --sf-warn
          error: '#B42318', // --sf-danger
          info: '#0E7490', // --sf-info
          background: '#EEF2F7', // --sf-bg
          surface: '#FFFFFF', // --sf-surface
          'surface-variant': '#F5F8FC', // --sf-surface-raised
          'on-surface': '#0F172A', // --sf-text
          'on-background': '#0F172A', // --sf-text
        },
      },
      subforgeDark: {
        dark: true,
        colors: {
          primary: '#8BB8FF', // --sf-accent
          secondary: '#B3D1FF', // --sf-accent-hover
          success: '#5BD68A', // --sf-ok
          warning: '#F5C451', // --sf-warn
          error: '#FF9B90', // --sf-danger
          info: '#6FD8E8', // --sf-info
          background: '#0B1220', // --sf-bg
          surface: '#162033', // --sf-surface
          'surface-variant': '#1D2A40', // --sf-surface-raised
          'on-surface': '#F1F5F9', // --sf-text
          'on-background': '#F1F5F9', // --sf-text
        },
      },
    },
  },
})
