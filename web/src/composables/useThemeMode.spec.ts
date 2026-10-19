import { describe, it, expect, vi, afterEach } from 'vitest'
import {
  THEME_STORAGE_KEY,
  isThemeMode,
  readStoredMode,
  systemPrefersDark,
  resolveDark,
  useThemeMode,
} from './useThemeMode'

function createMemoryStorage(initial: Record<string, string> = {}): Storage {
  const data = new Map<string, string>(Object.entries(initial))
  const setItem = vi.fn((key: string, value: string) => {
    data.set(key, String(value))
  })
  const storage: Storage = {
    get length() {
      return data.size
    },
    key(index: number) {
      return [...data.keys()][index] ?? null
    },
    getItem(key: string) {
      return data.has(key) ? (data.get(key) as string) : null
    },
    setItem,
    removeItem(key: string) {
      data.delete(key)
    },
    clear() {
      data.clear()
    },
  }
  // expose setItem spy via a property for assertions
  ;(storage as Storage & { __setItemSpy?: typeof setItem }).__setItemSpy = setItem
  return storage
}

describe('isThemeMode', () => {
  it('合法模式返回 true', () => {
    expect(isThemeMode('light')).toBe(true)
    expect(isThemeMode('dark')).toBe(true)
    expect(isThemeMode('system')).toBe(true)
  })

  it('非法/空值返回 false', () => {
    expect(isThemeMode('auto')).toBe(false)
    expect(isThemeMode('')).toBe(false)
    expect(isThemeMode('blue')).toBe(false)
  })
})

describe('readStoredMode', () => {
  it('storage 里是合法值时返回它', () => {
    const storage = createMemoryStorage({ [THEME_STORAGE_KEY]: 'dark' })
    expect(readStoredMode(storage)).toBe('dark')
    const storage2 = createMemoryStorage({ [THEME_STORAGE_KEY]: 'light' })
    expect(readStoredMode(storage2)).toBe('light')
    const storage3 = createMemoryStorage({ [THEME_STORAGE_KEY]: 'system' })
    expect(readStoredMode(storage3)).toBe('system')
  })

  it('getItem 返回 null 时返回 system', () => {
    const storage = createMemoryStorage()
    expect(readStoredMode(storage)).toBe('system')
  })

  it('返回非法值时返回 system', () => {
    const storage = createMemoryStorage({ [THEME_STORAGE_KEY]: 'blue' })
    expect(readStoredMode(storage)).toBe('system')
  })
})

describe('systemPrefersDark', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('matchMedia 返回 matches: true → true', () => {
    vi.stubGlobal('matchMedia', () => ({ matches: true }))
    expect(systemPrefersDark()).toBe(true)
  })

  it('matchMedia 返回 matches: false → false', () => {
    vi.stubGlobal('matchMedia', () => ({ matches: false }))
    expect(systemPrefersDark()).toBe(false)
  })

  it('window.matchMedia 为 undefined → false', () => {
    vi.stubGlobal('matchMedia', undefined)
    expect(systemPrefersDark()).toBe(false)
  })
})

describe('resolveDark', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('dark → true', () => {
    expect(resolveDark('dark')).toBe(true)
  })

  it('light → false', () => {
    expect(resolveDark('light')).toBe(false)
  })

  it('system 跟随 matchMedia.matches', () => {
    vi.stubGlobal('matchMedia', () => ({ matches: true }))
    expect(resolveDark('system')).toBe(true)
    vi.unstubAllGlobals()
    vi.stubGlobal('matchMedia', () => ({ matches: false }))
    expect(resolveDark('system')).toBe(false)
  })
})

describe('useThemeMode', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('初始 mode 取 storage 里的值，isDark 由 resolveDark 决定', () => {
    vi.stubGlobal('matchMedia', () => ({ matches: false }))
    const storage = createMemoryStorage({ [THEME_STORAGE_KEY]: 'dark' })
    const api = useThemeMode(storage)
    expect(api.mode.value).toBe('dark')
    expect(api.isDark.value).toBe(true)
  })

  it('初始 mode 为 system 时 isDark 跟随 matchMedia', () => {
    vi.stubGlobal('matchMedia', () => ({ matches: true }))
    const storage = createMemoryStorage()
    const api = useThemeMode(storage)
    expect(api.mode.value).toBe('system')
    expect(api.isDark.value).toBe(true)
  })

  it('setMode("dark")：mode/isDark/storage.setItem', () => {
    vi.stubGlobal('matchMedia', () => ({ matches: false }))
    const storage = createMemoryStorage()
    const api = useThemeMode(storage)
    api.setMode('dark')
    expect(api.mode.value).toBe('dark')
    expect(api.isDark.value).toBe(true)
    const spy = (storage as Storage & { __setItemSpy?: ReturnType<typeof vi.fn> }).__setItemSpy
    expect(spy).toHaveBeenCalledWith(THEME_STORAGE_KEY, 'dark')
  })

  it('cycle() 回绕：light → dark → system → light', () => {
    vi.stubGlobal('matchMedia', () => ({ matches: false }))
    const storage = createMemoryStorage({ [THEME_STORAGE_KEY]: 'light' })
    const api = useThemeMode(storage)
    expect(api.mode.value).toBe('light')
    api.cycle()
    expect(api.mode.value).toBe('dark')
    api.cycle()
    expect(api.mode.value).toBe('system')
    api.cycle()
    expect(api.mode.value).toBe('light')
  })

  it('同一 storage 的两个实例共享状态：写侧 setMode，读侧同步（AppShell 读 / SettingsPage 写）', () => {
    vi.stubGlobal('matchMedia', () => ({ matches: false }))
    const storage = createMemoryStorage({ [THEME_STORAGE_KEY]: 'light' })
    const reader = useThemeMode(storage)
    const writer = useThemeMode(storage)
    expect(reader.isDark.value).toBe(false)
    writer.setMode('dark')
    expect(reader.mode.value).toBe('dark')
    expect(reader.isDark.value).toBe(true)
    reader.cycle()
    expect(writer.mode.value).toBe('system')
  })

  it('不同 storage 的实例互相隔离', () => {
    vi.stubGlobal('matchMedia', () => ({ matches: false }))
    const a = useThemeMode(createMemoryStorage({ [THEME_STORAGE_KEY]: 'light' }))
    const b = useThemeMode(createMemoryStorage({ [THEME_STORAGE_KEY]: 'dark' }))
    a.setMode('light')
    expect(b.mode.value).toBe('dark')
    expect(b.isDark.value).toBe(true)
  })
})

import { LIGHT_THEME_NAME, DARK_THEME_NAME, themeNameFor, documentThemeFor, resolveTheme, applyTheme } from './useThemeMode'
import type { ThemeMode } from './useThemeMode'

describe('首帧主题解析（W6A 单一真值源）', () => {
  it('themeNameFor：dark → subforgeDark，light → subforgeLight', () => {
    expect(themeNameFor(true)).toBe(DARK_THEME_NAME)
    expect(themeNameFor(false)).toBe(LIGHT_THEME_NAME)
    expect(themeNameFor(true)).toBe('subforgeDark')
    expect(themeNameFor(false)).toBe('subforgeLight')
  })

  it('documentThemeFor：dark → "dark"，light → "light"', () => {
    expect(documentThemeFor(true)).toBe('dark')
    expect(documentThemeFor(false)).toBe('light')
  })

  it('resolveTheme：dark/light 各自产出 name 与 dataTheme', () => {
    expect(resolveTheme(true)).toEqual({ name: 'subforgeDark', dataTheme: 'dark' })
    expect(resolveTheme(false)).toEqual({ name: 'subforgeLight', dataTheme: 'light' })
  })

  it('三种模式：light / dark / system 都走同一解析路径（mode → resolveDark → themeNameFor/documentThemeFor）', () => {
    // light
    expect(themeNameFor(resolveDark('light'))).toBe(LIGHT_THEME_NAME)
    expect(documentThemeFor(resolveDark('light'))).toBe('light')
    // dark
    expect(themeNameFor(resolveDark('dark'))).toBe(DARK_THEME_NAME)
    expect(documentThemeFor(resolveDark('dark'))).toBe('dark')
    // system 跟随 matchMedia
    vi.stubGlobal('matchMedia', () => ({ matches: true }))
    expect(themeNameFor(resolveDark('system'))).toBe(DARK_THEME_NAME)
    expect(documentThemeFor(resolveDark('system'))).toBe('dark')
    vi.unstubAllGlobals()
    vi.stubGlobal('matchMedia', () => ({ matches: false }))
    expect(themeNameFor(resolveDark('system'))).toBe(LIGHT_THEME_NAME)
    expect(documentThemeFor(resolveDark('system'))).toBe('light')
    vi.unstubAllGlobals()
  })
})

describe('applyTheme（首帧与运行时切换共用）', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('dark：theme.change("subforgeDark") + data-theme=dark', () => {
    const theme = { change: vi.fn() }
    const root = { documentElement: { dataset: {} as Record<string, string> } }
    applyTheme(true, theme, root as unknown as Document)
    expect(theme.change).toHaveBeenCalledWith('subforgeDark')
    expect((root.documentElement.dataset as Record<string, string>).theme).toBe('dark')
  })

  it('light：theme.change("subforgeLight") + data-theme=light', () => {
    const theme = { change: vi.fn() }
    const root = { documentElement: { dataset: {} as Record<string, string> } }
    applyTheme(false, theme, root as unknown as Document)
    expect(theme.change).toHaveBeenCalledWith('subforgeLight')
    expect((root.documentElement.dataset as Record<string, string>).theme).toBe('light')
  })

  it('三种模式 × 两种 media：解析结果与 data-theme 同步', () => {
    const cases: Array<[ThemeMode, boolean, string, string]> = [
      ['light', false, 'subforgeLight', 'light'],
      ['dark', false, 'subforgeDark', 'dark'],
    ]
    for (const [mode, media, expectedName, expectedData] of cases) {
      vi.stubGlobal('matchMedia', () => ({ matches: media }))
      const theme = { change: vi.fn() }
      const root = { documentElement: { dataset: {} as Record<string, string> } }
      applyTheme(resolveDark(mode), theme, root as unknown as Document)
      expect(theme.change).toHaveBeenLastCalledWith(expectedName)
      expect((root.documentElement.dataset as Record<string, string>).theme).toBe(expectedData)
      vi.unstubAllGlobals()
    }
    // system + matchMedia dark
    vi.stubGlobal('matchMedia', () => ({ matches: true }))
    {
      const theme = { change: vi.fn() }
      const root = { documentElement: { dataset: {} as Record<string, string> } }
      applyTheme(resolveDark('system'), theme, root as unknown as Document)
      expect(theme.change).toHaveBeenCalledWith('subforgeDark')
      expect((root.documentElement.dataset as Record<string, string>).theme).toBe('dark')
    }
    vi.unstubAllGlobals()
    vi.stubGlobal('matchMedia', () => ({ matches: false }))
    {
      const theme = { change: vi.fn() }
      const root = { documentElement: { dataset: {} as Record<string, string> } }
      applyTheme(resolveDark('system'), theme, root as unknown as Document)
      expect(theme.change).toHaveBeenCalledWith('subforgeLight')
      expect((root.documentElement.dataset as Record<string, string>).theme).toBe('light')
    }
    vi.unstubAllGlobals()
  })
})
