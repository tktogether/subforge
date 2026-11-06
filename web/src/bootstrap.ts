import { createApp, type App as VueApp } from 'vue'
import App from './App.vue'
import { createAppRouter } from './router'
import { applyTheme, readStoredMode, resolveDark } from './composables/useThemeMode'
import { initToken } from './api/token'

// 首帧主题：抢在挂载前把 <html data-theme> 设成解析结果，深色用户第一帧即深色（防 FOUC）。
// 解析逻辑在 useThemeMode.ts，运行时切换走 AppShell 里的同一个 applyTheme，两条路径不漂移。
export function applyFirstPaintTheme(): void {
  applyTheme(resolveDark(readStoredMode(window.localStorage)))
}

export function createSubforgeApp(): VueApp<Element> {
  const app = createApp(App)
  app.use(createAppRouter())
  applyFirstPaintTheme()
  // 令牌接线必须在挂载前完成：首屏页面一挂载就会发请求，晚一步就会先吃一个 401。
  initToken()
  return app
}
