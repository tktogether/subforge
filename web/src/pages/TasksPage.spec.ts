import { describe, it, expect } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import TasksPage from './TasksPage.vue'
import UnderConstruction from '../components/UnderConstruction.vue'

const vuetify = createVuetify({ components, directives })

const NOTE = '定时任务与告警已有后端实现（internal/scheduler），任务历史第一版不落库。'
const PLAN = 'docs/design/web-ui-rewrite-plan.md §4'

describe('TasksPage', () => {
  it('把 page/note/plan 逐字传给 UnderConstruction，并渲染出 page 与 note', async () => {
    const w = mount(TasksPage, { global: { plugins: [vuetify] } })
    await flushPromises()

    const uc = w.findComponent(UnderConstruction)
    expect(uc.exists()).toBe(true)
    expect(uc.props('page')).toBe('任务')
    expect(uc.props('note')).toBe(NOTE)
    expect(uc.props('plan')).toBe(PLAN)

    expect(w.text()).toContain('任务')
    expect(w.text()).toContain(NOTE)
    expect(w.text()).toContain('任务 · 迁移中')
    expect(w.find('[data-testid="under-construction"]').exists()).toBe(true)
    w.unmount()
  })
})
