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
})
