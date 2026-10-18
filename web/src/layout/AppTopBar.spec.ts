import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import AppTopBar from './AppTopBar.vue'

const vuetify = createVuetify({ components, directives })

function mountTopBar(props: { title: string }) {
  return mount({
    components: { AppTopBar },
    template: '<v-layout><AppTopBar :title="title" /></v-layout>',
    data() { return { title: props.title } },
  }, { global: { plugins: [vuetify] } })
}

describe('AppTopBar', () => {
  it('渲染 title 与 SubForge chip', () => {
    const w = mountTopBar({ title: '机场' })
    const topbar = w.find('[data-testid="topbar"]')
    expect(topbar.exists()).toBe(true)
    expect(w.text()).toContain('机场')
    expect(w.text()).toContain('SubForge')
  })

  it('点击 drawer-toggle 触发 toggle-drawer 事件', async () => {
    const w = mountTopBar({ title: '机场' })
    const cmp = w.findComponent(AppTopBar)
    await w.find('[data-testid="drawer-toggle"]').trigger('click')
    const emitted = cmp.emitted('toggle-drawer')
    expect(emitted).toBeTruthy()
    expect(emitted!.length).toBe(1)
  })

  it('title 为空串也能渲染', () => {
    const w = mountTopBar({ title: '' })
    expect(w.find('[data-testid="topbar"]').exists()).toBe(true)
  })
})
