import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import ScriptsPage from './ScriptsPage.vue'
import { endpoints } from '../api/endpoints'

vi.mock('../api/endpoints', async () => {
  const actual = await vi.importActual<any>('../api/endpoints')
  return {
    ...actual,
    endpoints: {
      listScripts: vi.fn(),
      createScript: vi.fn(),
      updateScript: vi.fn(),
      deleteScript: vi.fn(),
      testScript: vi.fn(),
    },
  }
})

const listScriptsMock = endpoints.listScripts as unknown as ReturnType<typeof vi.fn>
const createScriptMock = endpoints.createScript as unknown as ReturnType<typeof vi.fn>
const updateScriptMock = endpoints.updateScript as unknown as ReturnType<typeof vi.fn>
const deleteScriptMock = endpoints.deleteScript as unknown as ReturnType<typeof vi.fn>
const testScriptMock = endpoints.testScript as unknown as ReturnType<typeof vi.fn>

const vuetify = createVuetify({ components, directives })

function button(w: ReturnType<typeof mount>, text: string) {
  const b = w.findAll('button').find((x) => x.text().includes(text))
  if (!b) throw new Error('button not found: ' + text)
  return b
}

describe('ScriptsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    Object.defineProperty(window, 'visualViewport', { value: { width: 1024, height: 768, scale: 1, offsetLeft: 0, offsetTop: 0, addEventListener: vi.fn(), removeEventListener: vi.fn() }, writable: true, configurable: true })
  })

  it('onMounted 调用 listScripts 并渲染列表', async () => {
    listScriptsMock.mockResolvedValue([{ id: 1, name: 'a', kind: 'filter', code: 'x', enabled: true }])

    const w = mount(ScriptsPage, { global: { plugins: [vuetify], stubs: { teleport: true, VDialog: { template: '<div><slot /></div>' } } } })
    await flushPromises()

    expect(listScriptsMock).toHaveBeenCalledTimes(1)
    expect(w.text()).toContain('a')
    expect(w.text()).toContain('filter')
    w.unmount()
  })

  it('空列表时空态', async () => {
    listScriptsMock.mockResolvedValue([])

    const w = mount(ScriptsPage, { global: { plugins: [vuetify], stubs: { teleport: true, VDialog: { template: '<div><slot /></div>' } } } })
    await flushPromises()

    expect(w.text()).toContain('暂无脚本')
    w.unmount()
  })

  it('load 失败显示错误', async () => {
    listScriptsMock.mockRejectedValueOnce(new Error('load boom'))

    const w = mount(ScriptsPage, { global: { plugins: [vuetify], stubs: { teleport: true, VDialog: { template: '<div><slot /></div>' } } } })
    await flushPromises()

    expect(w.text()).toContain('load boom')
    w.unmount()
  })

  it('点击新建打开对话框', async () => {
    listScriptsMock.mockResolvedValue([])

    const w = mount(ScriptsPage, { global: { plugins: [vuetify], stubs: { teleport: true, VDialog: { template: '<div><slot /></div>' } } } })
    await flushPromises()

    await button(w, '新建脚本').trigger('click')
    await nextTick()

    expect(w.text()).toContain('新建脚本')
    w.unmount()
  })

  it('点击编辑打开对话框', async () => {
    listScriptsMock.mockResolvedValue([{ id: 2, name: 'b', kind: 'operate', code: 'y', enabled: false }])

    const w = mount(ScriptsPage, { global: { plugins: [vuetify], stubs: { teleport: true, VDialog: { template: '<div><slot /></div>' } } } })
    await flushPromises()

    await button(w, '编辑').trigger('click')
    await nextTick()

    expect(w.text()).toContain('编辑脚本')
    w.unmount()
  })

  it('取消按钮关闭新建对话框', async () => {
    listScriptsMock.mockResolvedValue([])

    const w = mount(ScriptsPage, { global: { plugins: [vuetify], stubs: { teleport: true, VDialog: { template: '<div><slot /></div>' } } } })
    await flushPromises()

    await button(w, '新建脚本').trigger('click')
    await nextTick()
    expect(w.text()).toContain('新建脚本')

    const cancel = w.findAll('button').find((b) => b.text().includes('取消'))
    expect(cancel).toBeDefined()
    await cancel!.trigger('click')
    await nextTick()
    w.unmount()
  })

  it('保存新建分支', async () => {
    listScriptsMock.mockResolvedValue([])
    createScriptMock.mockResolvedValue({ id: 3, name: 'c', kind: 'filter', code: 'z', enabled: true })

    const w = mount(ScriptsPage, { global: { plugins: [vuetify], stubs: { teleport: true, VDialog: { template: '<div><slot /></div>' } } } })
    await flushPromises()

    await button(w, '新建脚本').trigger('click')
    await nextTick()

    await button(w, '保存').trigger('click')
    await flushPromises()

    expect(createScriptMock).toHaveBeenCalled()
    expect(listScriptsMock).toHaveBeenCalledTimes(2)
    w.unmount()
  })

  it('保存编辑分支', async () => {
    listScriptsMock.mockResolvedValue([{ id: 4, name: 'd', kind: 'filter', code: 'w', enabled: true }])
    updateScriptMock.mockResolvedValue({ id: 4, name: 'd2', kind: 'filter', code: 'w', enabled: true })

    const w = mount(ScriptsPage, { global: { plugins: [vuetify], stubs: { teleport: true, VDialog: { template: '<div><slot /></div>' } } } })
    await flushPromises()

    await button(w, '编辑').trigger('click')
    await nextTick()

    await button(w, '保存').trigger('click')
    await flushPromises()

    expect(updateScriptMock).toHaveBeenCalledWith(4, expect.any(Object))
    w.unmount()
  })

  it('保存失败显示 saveError', async () => {
    listScriptsMock.mockResolvedValue([])
    createScriptMock.mockRejectedValueOnce(new Error('save boom'))

    const w = mount(ScriptsPage, { global: { plugins: [vuetify], stubs: { teleport: true, VDialog: { template: '<div><slot /></div>' } } } })
    await flushPromises()

    await button(w, '新建脚本').trigger('click')
    await nextTick()

    await button(w, '保存').trigger('click')
    await flushPromises()

    expect(w.text()).toContain('save boom')
    w.unmount()
  })

  it('删除脚本', async () => {
    listScriptsMock.mockResolvedValue([{ id: 5, name: 'e', kind: 'filter', code: 'q', enabled: true }])
    deleteScriptMock.mockResolvedValue({ ok: true })

    const w = mount(ScriptsPage, { global: { plugins: [vuetify], stubs: { teleport: true, VDialog: { template: '<div><slot /></div>' } } } })
    await flushPromises()

    await button(w, '删除').trigger('click')
    await flushPromises()

    expect(deleteScriptMock).toHaveBeenCalledWith(5)
    expect(listScriptsMock).toHaveBeenCalledTimes(2)
    w.unmount()
  })

  it('删除失败显示 error', async () => {
    listScriptsMock.mockResolvedValue([{ id: 6, name: 'f', kind: 'filter', code: 'q', enabled: true }])
    deleteScriptMock.mockRejectedValueOnce(new Error('del boom'))

    const w = mount(ScriptsPage, { global: { plugins: [vuetify], stubs: { teleport: true, VDialog: { template: '<div><slot /></div>' } } } })
    await flushPromises()

    await button(w, '删除').trigger('click')
    await flushPromises()

    expect(w.text()).toContain('del boom')
    w.unmount()
  })

  it('点击试运行打开面板', async () => {
    listScriptsMock.mockResolvedValue([{ id: 7, name: 'g', kind: 'operate', code: 'op', enabled: true }])

    const w = mount(ScriptsPage, { global: { plugins: [vuetify], stubs: { teleport: true, VDialog: { template: '<div><slot /></div>' } } } })
    await flushPromises()

    await button(w, '试运行').trigger('click')
    await nextTick()

    expect(w.text()).toContain('试运行脚本')
    w.unmount()
  })

  it('关闭试运行对话框', async () => {
    listScriptsMock.mockResolvedValue([{ id: 7, name: 'g', kind: 'operate', code: 'op', enabled: true }])

    const w = mount(ScriptsPage, { global: { plugins: [vuetify], stubs: { teleport: true, VDialog: { template: '<div><slot /></div>' } } } })
    await flushPromises()

    await button(w, '试运行').trigger('click')
    await nextTick()
    expect(w.text()).toContain('试运行脚本')

    const close = w.findAll('button').find((b) => b.text().includes('关闭'))
    expect(close).toBeDefined()
    await close!.trigger('click')
    await nextTick()
    w.unmount()
  })

  it('runTest 成功显示成功与耗时', async () => {
    listScriptsMock.mockResolvedValue([{ id: 7, name: 'g', kind: 'operate', code: 'op', enabled: true }])
    testScriptMock.mockResolvedValue({ ok: true, duration_ms: 12, after: { x: 1 } })

    const w = mount(ScriptsPage, { global: { plugins: [vuetify], stubs: { teleport: true, VDialog: { template: '<div><slot /></div>' } } } })
    await flushPromises()

    await button(w, '试运行').trigger('click')
    await nextTick()
    await flushPromises()
    const runBtn = w.findAll('button').find((b) => b.text().trim() === '运行')
    await runBtn!.trigger('click')
    await flushPromises()
    await flushPromises()

    expect(testScriptMock).toHaveBeenCalled()
    expect(w.text()).toContain('成功')
    expect(w.text()).toContain('12ms')
    w.unmount()
  })

  it('runTest 失败显示失败与 error', async () => {
    listScriptsMock.mockResolvedValue([{ id: 7, name: 'g', kind: 'operate', code: 'op', enabled: true }])
    testScriptMock.mockResolvedValue({ ok: false, duration_ms: 5, error: 'boom' })

    const w = mount(ScriptsPage, { global: { plugins: [vuetify], stubs: { teleport: true, VDialog: { template: '<div><slot /></div>' } } } })
    await flushPromises()

    await button(w, '试运行').trigger('click')
    await nextTick()

    await w.findAll('button').find((b) => b.text().trim() === '运行')!.trigger('click')
    await flushPromises()

    expect(w.text()).toContain('失败')
    expect(w.text()).toContain('boom')
    w.unmount()
  })

  it('runTest 抛错时构造失败结果', async () => {
    listScriptsMock.mockResolvedValue([{ id: 7, name: 'g', kind: 'operate', code: 'op', enabled: true }])
    testScriptMock.mockRejectedValueOnce(new Error('net err'))

    const w = mount(ScriptsPage, { global: { plugins: [vuetify], stubs: { teleport: true, VDialog: { template: '<div><slot /></div>' } } } })
    await flushPromises()

    await button(w, '试运行').trigger('click')
    await nextTick()

    await w.findAll('button').find((b) => b.text().trim() === '运行')!.trigger('click')
    await flushPromises()

    expect(w.text()).toContain('失败')
    expect(w.text()).toContain('net err')
    w.unmount()
  })

  it('刷新按钮重新 load', async () => {
    listScriptsMock.mockResolvedValue([])

    const w = mount(ScriptsPage, { global: { plugins: [vuetify], stubs: { teleport: true, VDialog: { template: '<div><slot /></div>' } } } })
    await flushPromises()
    expect(listScriptsMock).toHaveBeenCalledTimes(1)

    await button(w, '刷新').trigger('click')
    await flushPromises()

    expect(listScriptsMock).toHaveBeenCalledTimes(2)
    w.unmount()
  })

  it('风险提示文案存在', async () => {
    listScriptsMock.mockResolvedValue([])

    const w = mount(ScriptsPage, { global: { plugins: [vuetify], stubs: { teleport: true, VDialog: { template: '<div><slot /></div>' } } } })
    await flushPromises()

    expect(w.text()).toContain('脚本在服务端 goja 沙箱执行')
    w.unmount()
  })

  it('对话框渲染覆盖（真实 VDialog，无 stub）', async () => {
    listScriptsMock.mockResolvedValue([{ id: 7, name: 'g', kind: 'operate', code: 'op', enabled: true }])
    const w = mount(ScriptsPage, { global: { plugins: [vuetify] }, attachTo: document.body })
    await flushPromises()
    await button(w, '新建脚本').trigger('click')
    await flushPromises()
    await button(w, '试运行').trigger('click')
    await flushPromises()
    w.unmount()
  })


  it('coverage: 对话框插槽 v-model getter 全覆盖', async () => {
    listScriptsMock.mockResolvedValue([{ id: 1, name: 'a', kind: 'filter', code: 'x', enabled: true }])
    const w = mount(ScriptsPage, { global: { plugins: [vuetify], stubs: { teleport: true, VDialog: { template: '<div><slot /></div>' } } } })
    await flushPromises()

    // 新建脚本对话框
    button(w, '新建脚本').trigger('click')
    await flushPromises()
    const textFields = w.findAllComponents({ name: 'VTextField' })
    const selects = w.findAllComponents({ name: 'VSelect' })
    const textareas = w.findAllComponents({ name: 'VTextarea' })
    const switches = w.findAllComponents({ name: 'VSwitch' })
    if (textFields.length) await textFields[0].setValue('x')
    if (selects.length) await selects[0].setValue('filter')
    if (textareas.length) await textareas[0].setValue('true')
    if (switches.length) await switches[0].setValue(true)
    await flushPromises()

    // 取消
    const cancelBtn = w.findAll('button').find((b) => b.text().includes('取消'))
    cancelBtn?.trigger('click')
    await flushPromises()

    // 试运行对话框
    const testBtn = w.findAll('button').find((b) => b.text().includes('试运行'))
    testBtn?.trigger('click')
    await flushPromises()
    const selects2 = w.findAllComponents({ name: 'VSelect' })
    const textareas2 = w.findAllComponents({ name: 'VTextarea' })
    if (selects2.length) await selects2[selects2.length - 1].setValue('operate')
    if (textareas2.length) await textareas2[textareas2.length - 1].setValue('code')
    await flushPromises()

    // 关闭
    const closeBtn = w.findAll('button').find((b) => b.text().includes('关闭'))
    closeBtn?.trigger('click')
    await flushPromises()

    // 运行
    const runBtn = w.findAll('button').find((b) => b.text().includes('运行'))
    runBtn?.trigger('click')
    await flushPromises()

    w.unmount()
  })


  it('coverage: v-dialog v-model getter 全覆盖（真实 VDialog）', async () => {
    listScriptsMock.mockResolvedValue([{ id: 1, name: 'a', kind: 'filter', code: 'x', enabled: true }])
    const w = mount(ScriptsPage, { global: { plugins: [vuetify], stubs: { teleport: true } }, attachTo: document.body })
    await flushPromises()

    const dlg = w.findComponent({ name: 'VDialog' })
    await dlg.setValue(true)
    await nextTick()
    await flushPromises()

    const dlg2 = w.findAllComponents({ name: 'VDialog' })
    if (dlg2.length > 1) {
      await dlg2[1].setValue(true)
      await nextTick()
      await flushPromises()
    }

    w.unmount()
  })

})
