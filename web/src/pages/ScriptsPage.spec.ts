import { describe, it, expect } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import ScriptsPage from './ScriptsPage.vue'
import UnderConstruction from '../components/UnderConstruction.vue'

const vuetify = createVuetify({ components, directives })

const NOTE = 'goja 脚本子系统已有后端实现（internal/transform/script.go），但尚无管理出口。'
const PLAN = 'docs/design/web-ui-rewrite-plan.md §3.6'

describe('ScriptsPage', () => {
  it('把 page/note/plan 逐字传给 UnderConstruction，并渲染出 page 与 note', async () => {
    const w = mount(ScriptsPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    const uc = w.findComponent(UnderConstruction)
    expect(uc.exists()).toBe(true)
    expect(uc.props('page')).toBe('脚本')
    expect(uc.props('note')).toBe(NOTE)
    expect(uc.props('plan')).toBe(PLAN)

    // 渲染文本里真的出现了 page 与 note
    expect(w.text()).toContain('脚本')
    expect(w.text()).toContain(NOTE)
    expect(w.text()).toContain('脚本 · 迁移中')
    expect(w.find('[data-testid="under-construction"]').exists()).toBe(true)
    w.unmount()
  })
})
