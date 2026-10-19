import { describe, it, expect } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import { createRouter, createMemoryHistory } from 'vue-router'
import { routes } from './router'
import App from './App.vue'
import AppShell from './layout/AppShell.vue'

const vuetify = createVuetify({
  components,
  directives,
  theme: {
    defaultTheme: 'subforgeLight',
    themes: {
      subforgeLight: { dark: false, colors: {} },
      subforgeDark: { dark: true, colors: {} },
    },
  },
})

describe('App', () => {
  it('渲染 AppShell，并包含导航与 SubForge 文本', async () => {
    const router = createRouter({ history: createMemoryHistory(), routes })
    await router.push('/convert')
    await router.isReady()

    const w = mount(App, { global: { plugins: [vuetify, router] } })
    await flushPromises()

    const shell = w.findComponent(AppShell)
    expect(shell.exists()).toBe(true)
    // AppShell 渲染的导航 drawer 与 SubForge 文本
    expect(w.find('[data-testid="nav"]').exists()).toBe(true)
    expect(w.text()).toContain('SubForge')
    w.unmount()
  })
})
