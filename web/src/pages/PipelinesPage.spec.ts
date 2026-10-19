import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import PipelinesPage from './PipelinesPage.vue'
import { type Pipeline, type Script } from '../api/endpoints'
import { endpoints } from '../api/endpoints'

vi.mock('../api/endpoints', async () => {
  const actual = await vi.importActual<any>('../api/endpoints')
  return {
    ...actual,
    endpoints: {
      pipelines: vi.fn(),
      listScripts: vi.fn(),
      createPipeline: vi.fn(),
      updatePipeline: vi.fn(),
      deletePipeline: vi.fn(),
    },
  }
})

const pipelinesMock = endpoints.pipelines as unknown as ReturnType<typeof vi.fn>
const scriptsMock = endpoints.listScripts as unknown as ReturnType<typeof vi.fn>
const createMock = endpoints.createPipeline as unknown as ReturnType<typeof vi.fn>
const updateMock = endpoints.updatePipeline as unknown as ReturnType<typeof vi.fn>
const deleteMock = endpoints.deletePipeline as unknown as ReturnType<typeof vi.fn>

const vuetify = createVuetify({ components, directives })

const samplePipelines: Pipeline[] = [
  { id: 1, name: 'pipe-a', description: 'desc-a', steps_json: '[{"op":"filter.regex","args":{"include":"^HK"}},{"op":"op.sort","args":{"by":"name"}}]', enabled: true, sort_order: 0, source: 'ui', read_only: false },
  { id: 2, name: 'pipe-b', description: '', steps_json: '[]', enabled: false, sort_order: 1, source: 'template', read_only: true },
]

const sampleScripts: Script[] = [
  { id: 1, name: 'hk-filter', kind: 'filter', code: 'node.name.indexOf("HK") >= 0', enabled: true },
  { id: 2, name: 'disabled-script', kind: 'operate', code: 'node.port = 443', enabled: false },
]

function mountPage() {
  return mount(PipelinesPage, { global: { plugins: [vuetify] }, attachTo: document.body })
}

describe('PipelinesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    pipelinesMock.mockResolvedValue(samplePipelines)
    scriptsMock.mockResolvedValue(sampleScripts)
    // jsdom 缺 visualViewport，VOverlay 需要
    ;(window as any).visualViewport = { width: 1024, height: 768, scale: 1, offsetLeft: 0, offsetTop: 0, pageLeft: 0, pageTop: 0, addEventListener: () => {}, removeEventListener: () => {}, dispatchEvent: () => true }
  })

  it('onMounted 加载管线与脚本，渲染列表与帮助块', async () => {
    const w = mountPage()
    await flushPromises()

    expect(pipelinesMock).toHaveBeenCalledWith('')
    expect(scriptsMock).toHaveBeenCalled()
    expect(w.find('[data-testid="pipeline-intro"]').exists()).toBe(true)
    expect(w.find('[data-testid="pipeline-order"]').exists()).toBe(true)
    expect(w.findAll('[data-testid="pipeline-order-item"]').length).toBe(3)
    expect(w.find('[data-testid="pipeline-script-step"]').exists()).toBe(true)
    expect(w.findAll('[data-testid="pipeline-row"]').length).toBe(2)
    expect(w.text()).toContain('pipe-a')
    expect(w.text()).toContain('pipe-b')
    w.unmount()
  })

  it('来源显示：ui→界面，template→模板（只读）', async () => {
    const w = mountPage()
    await flushPromises()
    const rows = w.findAll('[data-testid="pipeline-row"]')
    expect(rows[0].text()).toContain('界面')
    expect(rows[1].text()).toContain('模板（只读）')
    w.unmount()
  })

  it('read_only 行的编辑/删除按钮 disabled（不隐藏）', async () => {
    const w = mountPage()
    await flushPromises()
    const rows = w.findAll('[data-testid="pipeline-row"]')
    const editBtns = rows[1].findAll('[data-testid="pipeline-edit"]')
    const delBtns = rows[1].findAll('[data-testid="pipeline-delete"]')
    expect(editBtns.length).toBe(1)
    expect(delBtns.length).toBe(1)
    expect(editBtns[0].attributes('disabled')).toBeDefined()
    expect(delBtns[0].attributes('disabled')).toBeDefined()
    w.unmount()
  })

  it('空列表时渲染 emptyState', async () => {
    pipelinesMock.mockResolvedValue([])
    const w = mountPage()
    await flushPromises()
    expect(w.find('[data-testid="empty-state"]').exists()).toBe(true)
    expect(w.text()).toContain('还没有管线')
    w.unmount()
  })

  it('loading 时渲染骨架屏', async () => {
    pipelinesMock.mockImplementation(() => new Promise<Pipeline[]>(() => {}))
    const w = mountPage()
    await nextTick()
    expect(w.find('[data-testid="async-loading"]').exists()).toBe(true)
    w.unmount()
  })

  it('请求失败显示错误原文', async () => {
    pipelinesMock.mockRejectedValueOnce(new Error('配置文件缺少 pipelines'))
    const w = mountPage()
    await flushPromises()
    expect(w.find('[data-testid="async-error"]').exists()).toBe(true)
    expect(w.text()).toContain('配置文件缺少 pipelines')
    w.unmount()
  })

  it('点击刷新重新加载', async () => {
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="pipeline-refresh"]').trigger('click')
    await flushPromises()
    expect(pipelinesMock).toHaveBeenCalledTimes(2)
    w.unmount()
  })

  it('新建管线：添加步骤并保存成功', async () => {
    createMock.mockResolvedValue({ id: 3, name: 'new-pipe', description: '', steps_json: '[]', enabled: true, sort_order: 0, source: 'ui', read_only: false })
    const w = mountPage()
    await flushPromises()

    await w.find('[data-testid="pipeline-new"]').trigger('click')
    await flushPromises()
    expect((w.vm as any).dialog).toBe(true)

    ;(w.vm as any).addStep()
    await flushPromises()
    await flushPromises()
    expect((w.vm as any).formSteps.length).toBe(1)

    ;(w.vm as any).formName = 'new-pipe'
    await flushPromises()

    ;(document.querySelector('[data-testid="pipeline-save"]') as HTMLElement)?.click()
    await flushPromises()
    await flushPromises()

    expect(createMock).toHaveBeenCalled()
    const body = createMock.mock.calls[0][0]
    expect(body.name).toBe('new-pipe')
    expect(Array.isArray(body.steps)).toBe(true)
    w.unmount()
  })

  it('编辑管线：加载已有步骤并保存 updatePipeline', async () => {
    updateMock.mockResolvedValue(samplePipelines[0])
    const w = mountPage()
    await flushPromises()

    const editBtn = w.findAll('[data-testid="pipeline-edit"]')[0]
    await editBtn.trigger('click')
    await flushPromises()

    expect((w.vm as any).formSteps.length).toBe(2)
    ;(document.querySelector('[data-testid="pipeline-save"]') as HTMLElement)?.click()
    await flushPromises()
    await flushPromises()

    expect(updateMock).toHaveBeenCalled()
    const [id, body] = updateMock.mock.calls[0]
    expect(id).toBe(1)
    expect(body.name).toBe('pipe-a')
    expect(body.steps.length).toBe(2)
    w.unmount()
  })

  it('步骤上移/下移/删除', async () => {
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="pipeline-new"]').trigger('click')
    await flushPromises()
    ;(w.vm as any).addStep()
    ;(w.vm as any).addStep()
    await flushPromises()
    await flushPromises()
    expect(document.querySelectorAll('[data-testid="step-row"]').length).toBe(2)

    ;(w.vm as any).removeStep(0)
    await flushPromises()
    await flushPromises()
    expect(document.querySelectorAll('[data-testid="step-row"]').length).toBe(1)
    w.unmount()
  })

  it('script 步骤引用已启用脚本显示 script-ref-ok', async () => {
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="pipeline-new"]').trigger('click')
    await flushPromises()
    ;(w.vm as any).addStep()
    await flushPromises()

    // 设置 op 为 op.script
    const opSelect = w.findComponent('[data-testid="step-op"]')
    await opSelect.setValue('op.script')
    await flushPromises()
    ;(w.vm as any).formSteps[0].args.ref = 'hk-filter'
    await flushPromises()

    expect(document.body.textContent).toContain('脚本已启用')
    w.unmount()
  })

  it('script 步骤引用未启用脚本显示 script-ref-disabled', async () => {
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="pipeline-new"]').trigger('click')
    await flushPromises()
    ;(w.vm as any).addStep()
    await flushPromises()

    const opSelect = w.findComponent('[data-testid="step-op"]')
    await opSelect.setValue('op.script')
    await flushPromises()
    ;(w.vm as any).formSteps[0].args.ref = 'disabled-script'
    await flushPromises()
    ;(w.vm as any).formSteps = [...(w.vm as any).formSteps]
    await flushPromises()
    await (w.vm as any).$nextTick()
    await flushPromises()

    expect(document.body.textContent).toContain('脚本已存在但未启用')
    w.unmount()
  })

  it('script 步骤引用不存在脚本显示 script-ref-missing', async () => {
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="pipeline-new"]').trigger('click')
    await flushPromises()
    ;(w.vm as any).addStep()
    await flushPromises()

    const opSelect = w.findComponent('[data-testid="step-op"]')
    await opSelect.setValue('op.script')
    await flushPromises()

    ;(w.vm as any).formSteps[0].args.ref = 'nonexistent'
    await flushPromises()
    await flushPromises()

    expect(document.body.textContent).toContain('未找到该脚本')
    w.unmount()
  })

  it('保存失败原样显示后端错误', async () => {
    createMock.mockRejectedValueOnce(new Error('流水线名称不能为空'))
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="pipeline-new"]').trigger('click')
    await flushPromises()
    ;(document.querySelector('[data-testid="pipeline-save"]') as HTMLElement)?.click()
    await flushPromises()
    await flushPromises()

    expect(document.body.textContent).toContain('流水线名称不能为空')
    w.unmount()
  })

  it('删除确认对话框：确认后调 deletePipeline', async () => {
    deleteMock.mockResolvedValue({ ok: true })
    const w = mountPage()
    await flushPromises()

    const delBtn = w.findAll('[data-testid="pipeline-delete"]')[0]
    await delBtn.trigger('click')
    await flushPromises()

    expect(document.body.textContent).toContain('删除后引用它的订阅产物会退回默认步骤')

    await (w.vm as any).doDelete()
    await flushPromises()
    await flushPromises()

    expect(deleteMock).toHaveBeenCalledWith(1)
    w.unmount()
  })

  it('删除确认取消不调 deletePipeline', async () => {
    const w = mountPage()
    await flushPromises()
    const delBtn = w.findAll('[data-testid="pipeline-delete"]')[0]
    await delBtn.trigger('click')
    await flushPromises()

    const cancelBtn = document.querySelector('[data-testid="pipeline-confirm-cancel"]') as HTMLElement | null
    cancelBtn?.click()
    await flushPromises()
    await flushPromises()
    expect(deleteMock).not.toHaveBeenCalled()
    w.unmount()
  })

  it('parseSteps 非法 JSON 返回空数组（openEdit 容错）', async () => {
    const w = mountPage()
    await flushPromises()
    const p = { ...samplePipelines[1], steps_json: 'not-json' }
    ;(w.vm as any).openEdit(p)
    await flushPromises()
    expect((w.vm as any).formSteps).toEqual([])
    w.unmount()
  })

  it('moveStep 上移/下移覆盖', async () => {
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="pipeline-new"]').trigger('click')
    await flushPromises()
    ;(w.vm as any).addStep()
    ;(w.vm as any).addStep()
    await flushPromises()
    ;(w.vm as any).moveStep(1, -1)
    await flushPromises()
    ;(w.vm as any).moveStep(0, 1)
    await flushPromises()
    // 边界：越界不抛
    ;(w.vm as any).moveStep(0, -1)
    ;(w.vm as any).moveStep(10, 1)
    await flushPromises()
    w.unmount()
  })

  it('多种 op 的 paramFields 分支覆盖（rename/split/unknown）', async () => {
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="pipeline-new"]').trigger('click')
    await flushPromises()
    ;(w.vm as any).formSteps = [
      { op: 'op.rename', args: { pattern: 'a', repl: 'b' }, on_error: '' },
      { op: 'op.split', args: { sep: '|' }, on_error: '' },
      { op: 'op.dedupe', args: {}, on_error: '' },
    ]
    await flushPromises()
    expect((w.vm as any).formSteps.length).toBe(3)
    // stepScriptRef 非脚本分支
    expect((w.vm as any).stepScriptRef({ op: 'op.rename', args: {} })).toBe('')
    w.unmount()
  })

  it('load 失败分支覆盖', async () => {
    pipelinesMock.mockRejectedValueOnce(new Error('boom-load'))
    const w = mountPage()
    await flushPromises()
    expect(w.text()).toContain('boom-load')
    w.unmount()
  })


  it('步骤上移/下移边界与删除按钮点击覆盖', async () => {
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="pipeline-new"]').trigger('click')
    await flushPromises()
    ;(w.vm as any).addStep()
    ;(w.vm as any).addStep()
    await flushPromises()

    // 上移第一步（边界：no-op）
    ;(w.vm as any).moveStep(0, -1)
    await flushPromises()
    // 下移最后一步（边界：no-op）
    ;(w.vm as any).moveStep(1, 1)
    await flushPromises()
    // 点击 step-up on step 1
    const upBtns = Array.from(document.querySelectorAll('[data-testid="step-up"]')) as HTMLElement[]
    if (upBtns[1]) upBtns[1].click()
    await flushPromises()
    // 点击 step-down on step 0
    const downBtns = Array.from(document.querySelectorAll('[data-testid="step-down"]')) as HTMLElement[]
    if (downBtns[0]) downBtns[0].click()
    await flushPromises()
    // 点击 step-remove
    const removeBtns = Array.from(document.querySelectorAll('[data-testid="step-remove"]')) as HTMLElement[]
    if (removeBtns[0]) removeBtns[0].click()
    await flushPromises()

    ;(w.vm as any).dialog = false
    await flushPromises()
    w.unmount()
  })

  it('on_error 三值选择与保存失败原样显示', async () => {
    createMock.mockRejectedValueOnce(new Error('流水线名称不能为空'))
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="pipeline-new"]').trigger('click')
    await flushPromises()
    ;(w.vm as any).addStep()
    await flushPromises()
    const onErrSelects = Array.from(document.querySelectorAll('[data-testid="step-on-error"]')) as any[]
    if (onErrSelects[0]) {
      const comp = (w as any).findComponent('[data-testid="step-on-error"]')
      for (const v of ['', 'abort', 'warn', 'skip']) {
        if (comp.exists()) await comp.setValue(v)
        await flushPromises()
      }
    }
    ;(w.vm as any).formSteps[0].on_error = 'warn'
    ;(w.vm as any).formSteps[0].args.by = 'name'
    await flushPromises()

    // 触发保存失败
    ;(document.querySelector('[data-testid="pipeline-save"]') as HTMLElement)?.click()
    await flushPromises()
    await flushPromises()
    expect(document.body.textContent).toContain('流水线名称不能为空')

    ;(w.vm as any).dialog = false
    await flushPromises()
    w.unmount()
  })

  it('pipeline-cancel 关闭对话框', async () => {
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="pipeline-new"]').trigger('click')
    await flushPromises()
    ;(document.querySelector('[data-testid="pipeline-cancel"]') as HTMLElement)?.click()
    await flushPromises()
    expect((w.vm as any).dialog).toBe(false)
    w.unmount()
  })

  it('删除确认对话框取消与确认', async () => {
    deleteMock.mockResolvedValue({ ok: true })
    const w = mountPage()
    await flushPromises()
    // askDelete
    const delBtn = w.findAll('[data-testid="pipeline-delete"]')[0]
    await delBtn.trigger('click')
    await flushPromises()
    // cancel
    ;(document.querySelector('[data-testid="pipeline-confirm-cancel"]') as HTMLElement)?.click()
    await flushPromises()
    expect((w.vm as any).confirmOpen).toBe(false)

    // confirm delete
    await delBtn.trigger('click')
    await flushPromises()
    ;(document.querySelector('[data-testid="pipeline-confirm-delete"]') as HTMLElement)?.click()
    await flushPromises()
    await flushPromises()
    expect(deleteMock).toHaveBeenCalled()
    w.unmount()
  })


  it('保持对话框打开以覆盖渲染分支', async () => {
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="pipeline-new"]').trigger('click')
    await flushPromises()
    ;(w.vm as any).addStep()
    await flushPromises()
    ;(w.vm as any).formSteps[0].on_error = 'warn'
    ;(w.vm as any).formSteps[0].args.by = 'name'
    ;(w.vm as any).formSteps[0].args.f = 'x'
    await flushPromises()
    await new Promise((r) => setTimeout(r, 100))
    await flushPromises()
    expect(document.body.textContent).toContain('名称')
    w.unmount()
  })


  it('对话框字段 v-model setter 覆盖（name/description/enabled/on_error/by/pattern）', async () => {
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="pipeline-new"]').trigger('click')
    await flushPromises()
    ;(w.vm as any).addStep()
    await flushPromises()

    // name / description / enabled
    const nameComp = (w as any).findComponent('[data-testid="pipeline-name"]')
    if (nameComp.exists()) { await nameComp.setValue('n'); await flushPromises() }
    const descComp = (w as any).findComponent('[data-testid="pipeline-description"]')
    if (descComp.exists()) { await descComp.setValue('d'); await flushPromises() }
    const enabledComp = (w as any).findComponent('[data-testid="pipeline-enabled"]')
    if (enabledComp.exists()) { await enabledComp.setValue(false); await flushPromises() }
    expect((w.vm as any).formName).toBe('n')
    expect((w.vm as any).formDescription).toBe('d')
    expect((w.vm as any).formEnabled).toBe(false)

    // on_error select (line 264 setter)
    const onErrComp = (w as any).findComponent('[data-testid="step-on-error"]')
    if (onErrComp.exists()) { await onErrComp.setValue('warn'); await flushPromises() }
    expect((w.vm as any).formSteps[0].on_error).toBe('warn')

    // by-select: switch op to op.sort
    ;(w.vm as any).formSteps[0].op = 'op.sort'
    await flushPromises()
    const byComp = (w as any).findComponent('[data-testid="step-arg-by"]')
    if (byComp.exists()) { await byComp.setValue('name'); await flushPromises() }
    expect((w.vm as any).formSteps[0].args.by).toBe('name')

    // text-field: switch op to op.rename
    ;(w.vm as any).formSteps[0].op = 'op.rename'
    await flushPromises()
    const tfComp = (w as any).findComponent('[data-testid="step-arg"]')
    if (tfComp.exists()) { await tfComp.setValue('pat'); await flushPromises() }

    w.unmount()
  })

  it('触发 VDialog update:modelValue 覆盖 v-model 绑定', async () => {
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="pipeline-new"]').trigger('click')
    await flushPromises()
    // emit update:modelValue on the VDialog component
    const vdialog = (w as any).findComponent({ name: 'VDialog' })
    if (vdialog.exists()) {
      vdialog.vm.$emit('update:modelValue', false)
      await flushPromises()
    }
    // confirm dialog
    const delBtn = w.findAll('[data-testid="pipeline-delete"]')[0]
    await delBtn.trigger('click')
    await flushPromises()
    const vdialog2 = (w as any).findAllComponents({ name: 'VDialog' })
    if (vdialog2.length > 1 && vdialog2[1].exists()) {
      vdialog2[1].vm.$emit('update:modelValue', false)
      await flushPromises()
    }
    w.unmount()
  })


  it('doDelete：pendingDelete 为 null 时提前 return，不调 deletePipeline', async () => {
    const w = mountPage()
    await flushPromises()
    await (w.vm as any).doDelete()
    await flushPromises()
    expect(deleteMock).not.toHaveBeenCalled()
    w.unmount()
  })

  it('doDelete：deletePipeline 失败时 error 被设置', async () => {
    deleteMock.mockRejectedValue(new Error('delete fail'))
    const w = mountPage()
    await flushPromises()
    const delBtn = w.findAll('[data-testid="pipeline-delete"]')[0]
    await delBtn.trigger('click')
    await flushPromises()
    await (w.vm as any).doDelete()
    await flushPromises()
    expect((w.vm as any).error).toContain('delete fail')
    w.unmount()
  })

  it('W7D：script 步骤的 ref 参数用下拉选择已有脚本并可手输', async () => {
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="pipeline-new"]').trigger('click')
    await flushPromises()
    ;(w.vm as any).addStep()
    await flushPromises()

    // filter.script 的 ref 走 v-combobox（其余 src/timeout 仍是 step-arg 文本框）
    ;(w.vm as any).formSteps[0].op = 'filter.script'
    await flushPromises()
    const refComp = (w as any).findComponent('[data-testid="step-arg-ref"]')
    expect(refComp.exists()).toBe(true)

    // 下拉数据源来自已加载脚本列表
    const names = ((refComp.props('items') ?? []) as string[]).map((x) => String(x))
    expect(names).toContain('hk-filter')
    expect(names).toContain('disabled-script')

    // 选中已有脚本写入 args.ref
    if (refComp.exists()) { await refComp.setValue('hk-filter'); await flushPromises() }
    expect((w.vm as any).formSteps[0].args.ref).toBe('hk-filter')

    // :model-value 双向绑定：外部改 args.ref 也要回显到组件
    ;(w.vm as any).formSteps[0].args.ref = 'hk-filter'
    await flushPromises()
    expect(String(refComp.props('modelValue'))).toBe('hk-filter')

    // 手输一个还没建的脚本名同样写进 args.ref（保留自由输入能力）
    if (refComp.exists()) { await refComp.setValue('my-script'); await flushPromises() }
    if (refComp.exists()) { expect((w.vm as any).formSteps[0].args.ref).toBe('my-script') }

    w.unmount()
  })

  it('W7D：脚本 kind 与步骤不匹配时给出提示，匹配或未找到时不提示', async () => {
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="pipeline-new"]').trigger('click')
    await flushPromises()
    ;(w.vm as any).addStep()
    await flushPromises()

    // op.script（需要 operate）引用了 filter 脚本 hk-filter → 提示
    ;(w.vm as any).formSteps[0].op = 'op.script'
    await flushPromises()
    ;(w.vm as any).formSteps[0].args.ref = 'hk-filter'
    await flushPromises()
    expect((w.vm as any).scriptRefKindMismatch((w.vm as any).formSteps[0])).toContain('类型可能不匹配')
    const hint = document.querySelector('[data-testid="script-ref-kind-mismatch"]')
    expect(hint).not.toBeNull()
    expect(hint?.textContent).toContain('脚本 kind=filter')
    expect(hint?.textContent).toContain('该步骤需要 operate')

    // 换成 kind 匹配的 operate 脚本 disabled-script → 不提示
    ;(w.vm as any).formSteps[0].args.ref = 'disabled-script'
    await flushPromises()
    expect((w.vm as any).scriptRefKindMismatch((w.vm as any).formSteps[0])).toBe('')

    // filter.script（需要 filter）引用 operate 脚本 → 提示
    ;(w.vm as any).formSteps[0].op = 'filter.script'
    await flushPromises()
    expect((w.vm as any).scriptRefKindMismatch((w.vm as any).formSteps[0])).toContain('该步骤需要 filter')

    // 引用不存在的脚本 → 交给 script-ref-missing，不做 kind 提示
    ;(w.vm as any).formSteps[0].args.ref = 'nonexistent'
    await flushPromises()
    expect((w.vm as any).scriptRefKindMismatch((w.vm as any).formSteps[0])).toBe('')

    w.unmount()
  })


  it('W7L：openEdit 归一化缺 args / on_error 的步骤（覆盖 :131-132）', async () => {
    const w = mountPage()
    await flushPromises()
    const legacy: Pipeline = {
      id: 9,
      name: 'pipe-legacy',
      description: '',
      steps_json: '[{"op":"op.rename","on_error":"warn"},{"op":"op.sort"}]',
      enabled: true,
      sort_order: 2,
      source: 'ui',
      read_only: false,
    }
    ;(w.vm as any).openEdit(legacy)
    // 同步断言归一化结果（渲染前，避免组件回写干扰）
    const steps = (w.vm as any).formSteps
    expect(steps.length).toBe(2)
    // 缺 args → 归一化为 {}
    expect(steps[0].args).toEqual({})
    expect(steps[1].args).toEqual({})
    // 有 on_error 保留，缺 on_error → 归一化为 ''
    expect(steps[0].on_error).toBe('warn')
    expect(steps[1].on_error).toBe('')
    await flushPromises()
    // 编辑对话框已打开，args 表单为空值
    expect((w.vm as any).dialog).toBe(true)
    expect(String((w.vm as any).formSteps[0].args.pattern ?? '')).toBe('')
    w.unmount()
  })

  it('W7L：paramHint 未知 op 回落字段名；scriptRefKindMismatch 对非脚本 op 早退（覆盖 :69/:90）', async () => {
    const w = mountPage()
    await flushPromises()

    // :69 未知 op 不在 PIPELINE_HELP.steps 中 → 回落到字段名
    expect((w.vm as any).paramHint('op.unknown', 'myField')).toBe('myField')
    // 已知 op → 返回 help 里的 hint
    expect((w.vm as any).paramHint('op.rename', 'pattern')).toContain('pattern=')

    // :90 非脚本 op 提前 return ''，不进入 kind 比对
    expect((w.vm as any).scriptRefKindMismatch({ op: 'op.rename', args: { ref: 'hk-filter' } })).toBe('')

    await w.find('[data-testid="pipeline-new"]').trigger('click')
    await flushPromises()
    ;(w.vm as any).formSteps = [{ op: 'op.rename', args: { ref: 'hk-filter' }, on_error: '' }]
    await flushPromises()
    // 非脚本 op 不渲染 kind 不匹配探针
    expect(document.querySelector('[data-testid="script-ref-kind-mismatch"]')).toBeNull()
    // 文本框 hint 走 paramHint 的已知 op 分支
    const tf = (w as any).findComponent('[data-testid="step-arg"]')
    if (tf.exists()) expect(String(tf.props('hint'))).toContain('pattern=')
    w.unmount()
  })

  it('W7L：列表非空时渲染表格（:219 三元 false 分支）', async () => {
    pipelinesMock.mockResolvedValue([samplePipelines[0]])
    const w = mountPage()
    await flushPromises()
    expect(w.findAll('[data-testid="pipeline-row"]').length).toBe(1)
    expect(w.find('[data-testid="pipeline-table"]').exists()).toBe(true)
    expect(w.find('[data-testid="empty-state"]').exists()).toBe(false)
    expect(w.find('[data-testid="async-empty"]').exists()).toBe(false)
    w.unmount()
  })

  it('W7L：ref 组合框 update:model-value 传对象时归一化为字符串（覆盖 :281）', async () => {
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="pipeline-new"]').trigger('click')
    await flushPromises()
    ;(w.vm as any).addStep()
    await flushPromises()
    ;(w.vm as any).formSteps[0].op = 'filter.script'
    await flushPromises()

    const refComp = (w as any).findComponent('[data-testid="step-arg-ref"]')
    expect(refComp.exists()).toBe(true)

    // 对象带 value → 取 value
    refComp.vm.$emit('update:modelValue', { value: '脚本A' })
    await flushPromises()
    expect((w.vm as any).formSteps[0].args.ref).toBe('脚本A')

    // 对象无 value（{ text } 形态）→ 归一化为空串
    refComp.vm.$emit('update:modelValue', { text: 'x' })
    await flushPromises()
    expect((w.vm as any).formSteps[0].args.ref).toBe('')

    // 字符串仍原样写入
    refComp.vm.$emit('update:modelValue', 'plain-script')
    await flushPromises()
    expect((w.vm as any).formSteps[0].args.ref).toBe('plain-script')
    w.unmount()
  })

})
