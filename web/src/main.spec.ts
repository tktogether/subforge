import { beforeEach, describe, expect, it, vi } from 'vitest'

// main.ts 是"模块顶层副作用"型入口：import 就会执行 createSubforgeApp().mount('#app')。
// 这里把 bootstrap 换成假对象，避免真的挂载整个 Vue 应用树。
// vi.mock 会被提升到文件顶部，工厂函数在动态 import('./main') 解析到 './bootstrap' 时才执行，
// 因此可以安全引用模块作用域里的 mount。
const mount = vi.fn()

vi.mock('./bootstrap', () => ({
  createSubforgeApp: () => ({ mount }),
}))

describe('main', () => {
  beforeEach(() => {
    // main.ts 无导出，只能靠重新执行模块来做用例隔离。
    vi.resetModules()
    mount.mockClear()
    document.body.innerHTML = '<div id="app"></div>'
  })

  it('启动时创建应用并挂载到 #app，且只挂载一次', async () => {
    // 动态 import 才能拿到被 mock 的 ./bootstrap，并触发 main.ts 的顶层副作用。
    await import('./main')

    expect(mount).toHaveBeenCalledTimes(1)
    expect(mount).toHaveBeenCalledWith('#app')
  })

  it('入口按顺序导入设计令牌 ./design/tokens.css 与样式层 ./styles/index.css（bootstrap 之后，令牌在前）', async () => {
    const { readFileSync } = await import('node:fs')
    const { resolve } = await import('node:path')
    // vitest 运行目录就是 web/（vitest.config.ts 所在目录）；jsdom 环境下 import.meta.url
    // 不是 file: 协议，readFileSync(URL) 会抛 "The URL must be of scheme file"，故用 cwd 定位。
    const source = readFileSync(resolve(process.cwd(), 'src/main.ts'), 'utf8')
    const lines = source.split('\n').map((line) => line.trim())
    const bootstrapIndex = lines.findIndex((line) => line.startsWith('import { createSubforgeApp }'))
    const tokensIndex = lines.indexOf("import './design/tokens.css'")
    const stylesIndex = lines.indexOf("import './styles/index.css'")
    // W1 的令牌必须被引用；W5 起样式层也接线（此前 W1 期间禁止 main.ts 引用 styles）
    expect(tokensIndex).toBeGreaterThan(bootstrapIndex)
    expect(stylesIndex).toBeGreaterThan(bootstrapIndex)
    // 样式层大面积使用 var(--sf-*)，必须排在令牌之后，否则级联顺序无保障
    expect(stylesIndex).toBeGreaterThan(tokensIndex)
  })
})
