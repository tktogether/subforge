import { describe, it, expect } from 'vitest'
import { createMemoryHistory, type RouterHistory } from 'vue-router'
import { navItems, navGroups, routes, createAppRouter } from './index'

describe('navGroups', () => {
  it('返回的组数等于 navItems 中去重后的 group 数', () => {
    const groups = navGroups()
    const distinct = new Set(navItems.map((item) => item.group))
    expect(groups.length).toBe(distinct.size)
  })

  it('拼成 "组名:条目数" 数组等于预期', () => {
    const groups = navGroups()
    const labels = groups.map((group) => `${group.name}:${group.items.length}`)
    expect(labels).toEqual(['转换:4', '来源:2', '规则:3', '输出:2', '运维:3'])
  })

  it('保持首次出现顺序', () => {
    const groups = navGroups()
    expect(groups.map((group) => group.name)).toEqual(['转换', '来源', '规则', '输出', '运维'])
  })

  it('每个 item 都归属到 item.group 对应的那个组', () => {
    const groups = navGroups()
    for (const item of navItems) {
      const group = groups.find((candidate) => candidate.name === item.group)
      expect(group).toBeDefined()
      expect(group!.items).toContain(item)
    }
  })
})

describe('navItems', () => {
  it('每条都有非空 path/name/title/icon/group', () => {
    for (const item of navItems) {
      expect(item.path).toBeTruthy()
      expect(item.name).toBeTruthy()
      expect(item.title).toBeTruthy()
      expect(item.icon).toBeTruthy()
      expect(item.group).toBeTruthy()
    }
  })

  it('path 全部以 "/" 开头且互不重复', () => {
    for (const item of navItems) {
      expect(item.path.startsWith('/')).toBe(true)
    }
    const paths = navItems.map((item) => item.path)
    expect(new Set(paths).size).toBe(paths.length)
  })
})

describe('routes', () => {
  it('path 集合与 navItems 一致，且多出 "/:pathMatch(.*)*"', () => {
    const itemPaths = new Set(navItems.map((item) => item.path))
    const routePaths = routes.map((route) => route.path)
    expect(routePaths).toHaveLength(navItems.length + 1)
    for (const path of itemPaths) {
      expect(routePaths).toContain(path)
    }
    expect(routePaths).toContain('/:pathMatch(.*)*')
  })

  it('每条都有 meta.title 非空字符串', () => {
    for (const route of routes) {
      expect(typeof route.meta?.title).toBe('string')
      expect((route.meta?.title as string).length).toBeGreaterThan(0)
    }
  })
})

describe('createAppRouter', () => {
  it('不传参：await push("/") 后 currentRoute.name 为 overview', async () => {
    const router = createAppRouter()
    await router.push('/')
    expect(router.currentRoute.value.name).toBe('overview')
  })

  it('传入自定义 history：push("/logs") 后 name 为 logs', async () => {
    const history: RouterHistory = createMemoryHistory()
    const router = createAppRouter(history)
    await router.push('/logs')
    expect(router.currentRoute.value.name).toBe('logs')
  })

  it('未知路径：push("/nope") 后 name 为 not-found', async () => {
    const router = createAppRouter(createMemoryHistory())
    await router.push('/nope')
    expect(router.currentRoute.value.name).toBe('not-found')
  })

  it('所有导航路径均可成功导航到对应路由', async () => {
    const router = createAppRouter(createMemoryHistory())
    for (const item of navItems) {
      await router.push(item.path)
      expect(router.currentRoute.value.name).toBe(item.name)
    }
  })
})
