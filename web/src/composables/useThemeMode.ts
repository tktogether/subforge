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
  return 'system'
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

export function useThemeMode(storage: Storage = window.localStorage): ThemeModeApi {
  const mode = ref<ThemeMode>(readStoredMode(storage))
  const isDark = ref<boolean>(resolveDark(mode.value))

  function apply(next: ThemeMode): void {
    mode.value = next
    isDark.value = resolveDark(next)
    storage.setItem(THEME_STORAGE_KEY, next)
  }

  function cycle(): void {
    const index = THEME_MODES.indexOf(mode.value)
    apply(THEME_MODES[(index + 1) % THEME_MODES.length])
  }

  return { mode, isDark, setMode: apply, cycle }
}
