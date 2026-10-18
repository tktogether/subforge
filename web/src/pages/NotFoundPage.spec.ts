import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import { createRouter, createMemoryHistory, type Router } from 'vue-router'
import NotFoundPage from './NotFoundPage.vue'

const vuetify = createVuetify({ components, directives })

function makeRouter(): Router {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', name: 'overview', component: { template: '<div>overview</div>' } },
      { path: '/:pathMatch(.*)*', name: 'not-found', component: NotFoundPage },
    ],
  })
}

describe('NotFoundPage', () => {
  let router: Router

  beforeEach(() => {
    router = makeRouter()
  })

  it('渲染 404 卡片与文案，且 RouterLink 指向 "/"（渲染为 href="/" 的链接）', async () => {
    await router.push('/missing')
    await router.isReady()

    const w = mount(NotFoundPage, { global: { plugins: [vuetify, router] } })
    await flushPromises()

    expect(w.find('[data-testid="not-found"]').exists()).toBe(true)
    expect(w.text()).toContain('页面不存在')
    expect(w.text()).toContain('请从左侧导航选择模块。')
    expect(w.text()).toContain('返回概览')

    const link = w.find('a')
    expect(link.exists()).toBe(true)
    expect(link.attributes('href')).toBe('/')
    expect(link.text()).toContain('返回概览')
    w.unmount()
  })

  it('点击「返回概览」触发 router.push("/") 并跳转到概览路由', async () => {
    await router.push('/missing')
    await router.isReady()

    const pushSpy = vi.spyOn(router, 'push')
    const w = mount(NotFoundPage, { global: { plugins: [vuetify, router] } })
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/missing')

    await w.find('a').trigger('click')
    await flushPromises()

    expect(pushSpy).toHaveBeenCalled()
    expect(pushSpy.mock.calls[0][0]).toBe('/')
    expect(router.currentRoute.value.path).toBe('/')
    expect(router.currentRoute.value.name).toBe('overview')
    w.unmount()
  })
})
