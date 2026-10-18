import 'vuetify/styles'
import '@mdi/font/css/materialdesignicons.css'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'

export const vuetify = createVuetify({
  components,
  directives,
  theme: {
    defaultTheme: 'subforge',
    themes: {
      subforge: {
        dark: false,
        colors: {
          primary: '#1976d2',
          secondary: '#00897b',
          background: '#f4f6f8',
          surface: '#ffffff',
        },
      },
    },
  },
})
