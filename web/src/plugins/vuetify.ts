import 'vuetify/styles'
import '@mdi/font/css/materialdesignicons.css'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import { zhHans } from 'vuetify/locale'

// D1 Instrument 双主题（subforgeLight / subforgeDark）。
// 色值逐字取自 docs/design/mockups/d1-instrument.html:53-102：
//   浅色 :54-70，深色 :80-96。令牌真值源是 web/src/design/tokens.css，
//   两处的一致性由 web/src/plugins/vuetify.spec.ts 的断言锁定。
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
          primary: '#215da5', // --sf-accent          d1-instrument.html:63
          secondary: '#006794', // --sf-info          d1-instrument.html:70
          success: '#007245', // --sf-ok              d1-instrument.html:67
          warning: '#754c00', // --sf-warn            d1-instrument.html:68
          error: '#a72626', // --sf-danger            d1-instrument.html:69
          info: '#006794', // --sf-info               d1-instrument.html:70
          background: '#f3f5f8', // --sf-bg          d1-instrument.html:54
          surface: '#fafbfc', // --sf-surface        d1-instrument.html:56
          'surface-variant': '#fdfdfe', // --sf-surface-raised d1-instrument.html:57
          'on-surface': '#191f26', // --sf-text       d1-instrument.html:60
          'on-background': '#191f26', // --sf-text    d1-instrument.html:60
        },
      },
      subforgeDark: {
        dark: true,
        colors: {
          primary: '#4b78ba', // --sf-accent          d1-instrument.html:89
          secondary: '#439ccc', // --sf-info          d1-instrument.html:96
          success: '#52a76d', // --sf-ok              d1-instrument.html:93
          warning: '#ce9a43', // --sf-warn            d1-instrument.html:94
          error: '#dc655d', // --sf-danger            d1-instrument.html:95
          info: '#439ccc', // --sf-info               d1-instrument.html:96
          background: '#0d0f12', // --sf-bg          d1-instrument.html:80
          surface: '#1e2124', // --sf-surface        d1-instrument.html:82
          'surface-variant': '#24282b', // --sf-surface-raised d1-instrument.html:83
          'on-surface': '#eaedf1', // --sf-text       d1-instrument.html:86
          'on-background': '#eaedf1', // --sf-text    d1-instrument.html:86
        },
      },
    },
  },
})
