import { defineComponent } from 'vue'
import { describe, it, expect, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import { createRouter, createMemoryHistory } from 'vue-router'
import { routes, navGroups } from '../router'
import AppNav from './AppNav.vue'

const vuetify = createVuetify({ components, directives })

function mountNav(modelValue: boolean, router: any) {
  const Wrapper = defineComponent({
    components: { AppNav },
    template: '<v-app><AppNav :model-value="mv" /></v-app>',
    data() {
      return { mv: modelValue }
    },
  })
  const w = mount(Wrapper, { global: { plugins: [vuetify, router] } })
  const appNav = w.findComponent(AppNav)
  return { w, appNav }
}

describe('AppNav', () => {
  let router: ReturnType<typeof createRouter>

  beforeEach(async () => {
    router = createRouter({ history: createMemoryHistory(), routes })
    await router.push('/convert')
    await router.isReady()
  })

  it('渲染 v-navigation-drawer width=248，且 navGroups 的每个 group.name 都出现', async () => {
    const { w, appNav } = mountNav(true, router)
    await flushPromises()

    const drawer = appNav.findComponent({ name: 'VNavigationDrawer' })
    expect(drawer.exists()).toBe(true)
    expect(drawer.props('width')).toBe('248')
    expect(drawer.props('modelValue')).toBe(true)

    const groups = navGroups()
    for (const g of groups) {
      expect(w.text()).toContain(g.name)
    }
    expect(w.text()).toContain('SubForge')
    expect(appNav.find('img[src="/favicon.svg"]').exists()).toBe(true)
    w.unmount()
  })

  it('每个 item 对应的 v-list-item 的 to 等于 item.path；当前路由条目 active=true，其余 active=false', async () => {
    const { w, appNav } = mountNav(true, router)
    await flushPromises()

    const groups = navGroups()
    const items = groups.flatMap((g) => g.items)

    const listItems = appNav.findAllComponents({ name: 'VListItem' })
    const navListItems = listItems.filter((li) => li.props('to') !== undefined)
    expect(navListItems.length).toBe(items.length)

    for (const item of items) {
      const li = navListItems.find((x) => x.props('title') === item.title)
      expect(li).toBeTruthy()
      expect(li!.props('to')).toBe(item.path)
      if (item.path === '/convert') {
        expect(li!.props('active')).toBe(true)
      } else {
        expect(li!.props('active')).toBe(false)
      }
    }
    w.unmount()
  })

  it('VNavigationDrawer 触发 update:model-value 时 onUpdate 把值 emit 出去', async () => {
    const { w, appNav } = mountNav(true, router)
    await flushPromises()

    const drawer = appNav.findComponent({ name: 'VNavigationDrawer' })
    await drawer.vm.$emit('update:modelValue', false)
    await flushPromises()

    expect(appNav.emitted('update:modelValue')).toBeTruthy()
    expect(appNav.emitted('update:modelValue')![0]).toEqual([false])
    w.unmount()
  })
})
