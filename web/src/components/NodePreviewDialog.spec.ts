import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import NodePreviewDialog from './NodePreviewDialog.vue'

const vuetify = createVuetify({ components, directives })
const stubs = { VDialog: { props: ['modelValue'], template: '<div v-if="modelValue" data-testid="dialog"><slot /></div>' } }

function mountDialog(props: Record<string, unknown>) {
  return mount(NodePreviewDialog, {
    props: { modelValue: true, title: '处理预览', loading: false, error: '', result: null, ...props },
    global: { plugins: [vuetify], stubs },
  })
}

describe('NodePreviewDialog', () => {
  it('loading 时显示加载提示', () => {
    const w = mountDialog({ loading: true })
    expect(w.find('[data-testid="preview-loading"]').exists()).toBe(true)
    w.unmount()
  })

  it('error 时显示错误', () => {
    const w = mountDialog({ error: 'boom' })
    expect(w.find('[data-testid="preview-error"]').text()).toBe('boom')
    w.unmount()
  })

  it('展示处理前后节点、计数和告警', () => {
    const w = mountDialog({
      result: {
        name: '机场', pipeline: 'pre', before_count: 2, after_count: 1,
        before: [{ name: 'A', type: 'ss', server: 'a', port: '1' }, { name: 'B', type: 'ss', server: 'b', port: '2' }],
        after: [{ name: 'P-A', type: 'ss', server: 'a', port: '1' }],
        warnings: ['跳过一步'],
      },
    })
    expect(w.find('[data-testid="preview-summary"]').text()).toContain('处理前')
    expect(w.find('[data-testid="preview-before-table"]').text()).toContain('B')
    expect(w.find('[data-testid="preview-after-table"]').text()).toContain('P-A')
    expect(w.find('[data-testid="preview-warnings"]').text()).toContain('跳过一步')
    w.unmount()
  })

  it('处理前后为空时显示空提示', () => {
    const w = mountDialog({
      result: { name: 'x', before_count: 0, after_count: 0, before: [], after: [], warnings: [] },
    })
    expect(w.text()).toContain('没有节点')
    expect(w.text()).toContain('处理后没有节点')
    w.unmount()
  })

  it('关闭按钮 emit update:modelValue=false', async () => {
    const w = mountDialog({ result: { name: 'x', before_count: 0, after_count: 0, before: [], after: [], warnings: [] } })
    await w.find('[data-testid="preview-close"]').trigger('click')
    expect(w.emitted('update:modelValue')).toBeTruthy()
    expect(w.emitted('update:modelValue')![0]).toEqual([false])
    w.unmount()
  })
})
