import { createApp, type App as VueApp } from 'vue'
import App from './App.vue'
import { vuetify } from './plugins/vuetify'
import { createAppRouter } from './router'
import { applyTheme, readStoredMode, resolveDark } from './composables/useThemeMode'

// 首帧主题（W6A）：此前 defaultTheme 写死 subforgeLight，主题切换发生在 AppShell 挂载后的
// watcher 里，深色用户的第一帧是浅色（FOUC 闪白）。这里抢在 app.use(vuetify) 之前把主题名与
// <html data-theme> 设成解析结果，使深色用户第一帧即深色。解析逻辑在 useThemeMode.ts，
// AppShell 的运行时切换调同一个 applyTheme，两条路径不会漂移。
export function applyFirstPaintTheme(): void {
  applyTheme(resolveDark(readStoredMode(window.localStorage)), vuetify.theme)
}

export function createSubforgeApp(): VueApp<Element> {
  const app = createApp(App)
  app.use(createAppRouter())
  applyFirstPaintTheme()
  app.use(vuetify)
  return app
}
