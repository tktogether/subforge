import { ref, type Ref } from 'vue'

export type ThemeMode = 'light' | 'dark' | 'system'

export const THEME_STORAGE_KEY = 'subforge.theme'

export const THEME_MODES: ThemeMode[] = ['light', 'dark', 'system']

export function isThemeMode(value: string): value is ThemeMode {
  return THEME_MODES.indexOf(value as ThemeMode) >= 0
}

export function readStoredMode(storage: Storage): ThemeMode {
  const raw = storage.getItem(THEME_STORAGE_KEY)
  if (raw !== null && isThemeMode(raw)) {
    return raw
  }
  return 'dark'
}

export function systemPrefersDark(): boolean {
  if (typeof window.matchMedia !== 'function') {
    return false
  }
  return window.matchMedia('(prefers-color-scheme: dark)').matches
}

export function resolveDark(mode: ThemeMode): boolean {
  if (mode === 'dark') {
    return true
  }
  if (mode === 'light') {
    return false
  }
  return systemPrefersDark()
}

export interface ThemeModeApi {
  mode: Ref<ThemeMode>
  isDark: Ref<boolean>
  setMode: (next: ThemeMode) => void
  cycle: () => void
}

interface ThemeState {
  mode: Ref<ThemeMode>
  isDark: Ref<boolean>
}

/**
 * 同一个 storage 实例共享一份状态：AppShell（读）与 SettingsPage（写）各自调用
 * useThemeMode() 时拿到的是同一对 ref，设置页 setMode 才会即时驱动外壳换主题。
 * 按 storage 缓存（WeakMap）：浏览器里共用 window.localStorage，测试里各自 new 的
 * MemoryStorage 仍然互相隔离。
 */
const themeStates = new WeakMap<Storage, ThemeState>()

function themeStateFor(storage: Storage): ThemeState {
  const persisted = readStoredMode(storage)
  const cached = themeStates.get(storage)
  if (cached === undefined) {
    const mode = ref<ThemeMode>(persisted)
    const state: ThemeState = { mode, isDark: ref<boolean>(resolveDark(persisted)) }
    themeStates.set(storage, state)
    return state
  }
  // 每次取用时与持久层对齐：storage 是真源，refs 只是同一 storage 上所有调用者共享的视图。
  if (cached.mode.value !== persisted) {
    cached.mode.value = persisted
    cached.isDark.value = resolveDark(persisted)
  }
  return cached
}

export function useThemeMode(storage: Storage = window.localStorage): ThemeModeApi {
  const state = themeStateFor(storage)

  function apply(next: ThemeMode): void {
    state.mode.value = next
    state.isDark.value = resolveDark(next)
    storage.setItem(THEME_STORAGE_KEY, next)
  }

  function cycle(): void {
    const index = THEME_MODES.indexOf(state.mode.value)
    apply(THEME_MODES[(index + 1) % THEME_MODES.length])
  }

  return { mode: state.mode, isDark: state.isDark, setMode: apply, cycle }
}

// ===== 首帧主题：把解析结果落到 <html data-theme>，防止 FOUC =====
export type DocumentTheme = 'light' | 'dark'

export function documentThemeFor(dark: boolean): DocumentTheme {
  return dark ? 'dark' : 'light'
}

export function applyTheme(dark: boolean, root?: Document): void {
  const doc = root ?? (typeof document === 'undefined' ? undefined : document)
  if (doc !== undefined) {
    doc.documentElement.dataset.theme = documentThemeFor(dark)
  }
}

// 跟随系统模式下的实时切换：系统明暗变化时更新共享 isDark（AppShell 负责落到 data-theme）。
export function watchSystemTheme(storage: Storage = window.localStorage): void {
  if (typeof window.matchMedia !== 'function') {
    return
  }
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    const state = themeStates.get(storage)
    if (state !== undefined && state.mode.value === 'system') {
      state.isDark.value = systemPrefersDark()
    }
  })
}
