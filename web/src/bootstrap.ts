import { createApp, type App as VueApp } from 'vue'
import App from './App.vue'
import { vuetify } from './plugins/vuetify'
import { createAppRouter } from './router'

export function createSubforgeApp(): VueApp<Element> {
  const app = createApp(App)
  app.use(createAppRouter())
  app.use(vuetify)
  return app
}
