import 'vuetify/styles'
import '@mdi/font/css/materialdesignicons.css'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import { zhHans } from 'vuetify/locale'

// P3 森林柔和（Everforest 系）双主题（subforgeLight / subforgeDark）。
// 色值逐字取自 docs/design/mockups/palette-p3-forest.html:10-46：
//   浅色 :10-25，深色 :31-46。令牌真值源是 web/src/design/tokens.css，
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
          primary: '#4F6E35', // --sf-accent          palette-p3-forest.html:19
          secondary: '#405B2B', // --sf-accent-hover  palette-p3-forest.html:20
          success: '#2b7b41', // --sf-ok              palette-p3-forest.html:22
          warning: '#976000', // --sf-warn            palette-p3-forest.html:23
          error: '#B3382E', // --sf-danger            palette-p3-forest.html:24
          info: '#2B6198', // --sf-info               palette-p3-forest.html:25
          background: '#F0F2E6', // --sf-bg          palette-p3-forest.html:10
          surface: '#F9FAEF', // --sf-surface        palette-p3-forest.html:12
          'surface-variant': '#FDFEF6', // --sf-surface-raised palette-p3-forest.html:13
          'on-surface': '#242B26', // --sf-text       palette-p3-forest.html:16
          'on-background': '#242B26', // --sf-text    palette-p3-forest.html:16
        },
      },
      subforgeDark: {
        dark: true,
        colors: {
          primary: '#A5C08B', // --sf-accent          palette-p3-forest.html:40
          secondary: '#B7D2A0', // --sf-accent-hover  palette-p3-forest.html:41
          success: '#93CA83', // --sf-ok              palette-p3-forest.html:43
          warning: '#DBBC7F', // --sf-warn            palette-p3-forest.html:44
          error: '#f9837a', // --sf-danger            palette-p3-forest.html:45
          info: '#7CB6C8', // --sf-info               palette-p3-forest.html:46
          background: '#232B30', // --sf-bg          palette-p3-forest.html:31
          surface: '#323C41', // --sf-surface        palette-p3-forest.html:33
          'surface-variant': '#3B474D', // --sf-surface-raised palette-p3-forest.html:34
          'on-surface': '#D8DCC6', // --sf-text       palette-p3-forest.html:37
          'on-background': '#D8DCC6', // --sf-text    palette-p3-forest.html:37
        },
      },
    },
  },
})
