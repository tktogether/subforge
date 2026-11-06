import { describe, it, expect, vi, afterEach } from 'vitest'
import {
  THEME_STORAGE_KEY,
  isThemeMode,
  readStoredMode,
  systemPrefersDark,
  resolveDark,
  useThemeMode,
  applyTheme,
  documentThemeFor,
  watchSystemTheme,
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

  it('getItem 返回 null 时返回 dark（管制台默认深色）', () => {
    const storage = createMemoryStorage()
    expect(readStoredMode(storage)).toBe('dark')
  })

  it('返回非法值时返回 dark', () => {
    const storage = createMemoryStorage({ [THEME_STORAGE_KEY]: 'blue' })
    expect(readStoredMode(storage)).toBe('dark')
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
    const storage = createMemoryStorage({ [THEME_STORAGE_KEY]: 'system' })
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

describe('applyTheme（首帧与运行时切换共用）', () => {
  it('documentThemeFor：dark → "dark"，light → "light"', () => {
    expect(documentThemeFor(true)).toBe('dark')
    expect(documentThemeFor(false)).toBe('light')
  })

  it('dark：data-theme=dark；light：data-theme=light', () => {
    const root = { documentElement: { dataset: {} as Record<string, string> } }
    applyTheme(true, root as unknown as Document)
    expect((root.documentElement.dataset as Record<string, string>).theme).toBe('dark')
    applyTheme(false, root as unknown as Document)
    expect((root.documentElement.dataset as Record<string, string>).theme).toBe('light')
  })

  it('三种模式 × 两种 media：解析结果与 data-theme 同步', () => {
    const cases: Array<['light' | 'dark', boolean, string]> = [
      ['light', false, 'light'],
      ['dark', false, 'dark'],
    ]
    for (const [mode, media, expected] of cases) {
      vi.stubGlobal('matchMedia', () => ({ matches: media }))
      const root = { documentElement: { dataset: {} as Record<string, string> } }
      applyTheme(resolveDark(mode), root as unknown as Document)
      expect((root.documentElement.dataset as Record<string, string>).theme).toBe(expected)
      vi.unstubAllGlobals()
    }
    // system + matchMedia dark
    vi.stubGlobal('matchMedia', () => ({ matches: true }))
    {
      const root = { documentElement: { dataset: {} as Record<string, string> } }
      applyTheme(resolveDark('system'), root as unknown as Document)
      expect((root.documentElement.dataset as Record<string, string>).theme).toBe('dark')
    }
    vi.unstubAllGlobals()
    vi.stubGlobal('matchMedia', () => ({ matches: false }))
    {
      const root = { documentElement: { dataset: {} as Record<string, string> } }
      applyTheme(resolveDark('system'), root as unknown as Document)
      expect((root.documentElement.dataset as Record<string, string>).theme).toBe('light')
    }
    vi.unstubAllGlobals()
  })
})

describe('themeStateFor 与持久层对齐', () => {
  it('storage 被外部改写后，再次取用会刷新共享 refs', () => {
    vi.stubGlobal('matchMedia', () => ({ matches: false }))
    const storage = createMemoryStorage({ [THEME_STORAGE_KEY]: 'light' })
    const api = useThemeMode(storage)
    expect(api.mode.value).toBe('light')

    storage.setItem(THEME_STORAGE_KEY, 'dark')
    const again = useThemeMode(storage)

    expect(again.mode.value).toBe('dark')
    expect(again.isDark.value).toBe(true)
    expect(api.mode.value).toBe('dark')
  })
})

describe('watchSystemTheme（跟随系统实时切换）', () => {
  function stubMatchMedia(initial: boolean) {
    const listeners: Array<() => void> = []
    let matches = initial
    vi.stubGlobal('matchMedia', () => ({
      get matches() {
        return matches
      },
      addEventListener: (_type: string, callback: () => void) => {
        listeners.push(callback)
      },
    }))
    return {
      listeners,
      flip(next: boolean) {
        matches = next
      },
    }
  }

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('system 模式下系统切深色，共享 isDark 跟随', () => {
    const media = stubMatchMedia(false)
    const storage = createMemoryStorage({ [THEME_STORAGE_KEY]: 'system' })
    const api = useThemeMode(storage)
    expect(api.isDark.value).toBe(false)

    watchSystemTheme(storage)
    media.flip(true)
    media.listeners.forEach((callback) => callback())

    expect(api.isDark.value).toBe(true)
  })

  it('非 system 模式忽略系统变化', () => {
    const media = stubMatchMedia(false)
    const storage = createMemoryStorage({ [THEME_STORAGE_KEY]: 'dark' })
    const api = useThemeMode(storage)

    watchSystemTheme(storage)
    media.flip(false)
    media.listeners.forEach((callback) => callback())

    expect(api.isDark.value).toBe(true)
  })

  it('storage 尚未建立状态时系统变化不抛错', () => {
    const media = stubMatchMedia(true)
    const storage = createMemoryStorage()

    watchSystemTheme(storage)
    expect(() => media.listeners.forEach((callback) => callback())).not.toThrow()
  })

  it('matchMedia 缺失时直接返回', () => {
    vi.stubGlobal('matchMedia', undefined)
    expect(() => watchSystemTheme(createMemoryStorage())).not.toThrow()
  })
})
