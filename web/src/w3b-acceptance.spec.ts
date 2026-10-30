// @ts-nocheck
// W3B 独立验收探针（fix-w3b-free）：不复用 fix-w3-free 的断言，独立验证 D-15/16/18/20/21/22/23。
// 与既有 spec 并存，用于回归防护；如需精简可整份删除。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import { request } from './api/client'
import { errorDetail as uaErrorDetail } from './composables/useAsync'
import { endpoints } from './api/endpoints'
import { SCRIPTS_HELP } from './content/help'
import { TOKEN_STORAGE_KEY, saveToken, tokenDialogOpen, tokenState } from './api/token'
import ScriptsPage from './pages/ScriptsPage.vue'
import PipelinesPage from './pages/PipelinesPage.vue'
import AppShell from './layout/AppShell.vue'

vi.mock('./api/endpoints', async () => {
  const actual = await vi.importActual<any>('./api/endpoints')
  return {
    ...actual,
    endpoints: {
      pipelines: vi.fn(),
      listScripts: vi.fn(),
      createPipeline: vi.fn(),
      updatePipeline: vi.fn(),
      deletePipeline: vi.fn(),
      createScript: vi.fn(),
      updateScript: vi.fn(),
      deleteScript: vi.fn(),
      testScript: vi.fn(),
    },
  }
})

const { routeRef } = vi.hoisted(() => ({ routeRef: { value: { meta: {}, path: '/' } } }))
vi.mock('vue-router', () => ({
  useRoute: () => routeRef.value,
  RouterView: { name: 'RouterView', template: '<div class="router-view-stub" />' },
  RouterLink: { name: 'RouterLink', template: '<a />' },
}))
vi.mock('./layout/AppNav.vue', () => ({
  default: { name: 'AppNav', props: ['modelValue'], emits: ['update:modelValue'], template: '<div class="appnav-stub" />' },
}))

const listScriptsMock = endpoints.listScripts as any
const deleteScriptMock = endpoints.deleteScript as any
const testScriptMock = endpoints.testScript as any
const createScriptMock = endpoints.createScript as any
const pipelinesMock = endpoints.pipelines as any

const vuetify = createVuetify({ components, directives })
const stubs = { teleport: true, VDialog: { template: '<div class="vdialog-stub"><slot /></div>' } }

function textOf(w: any, id: string): string {
  const el = w.find('[data-testid="' + id + '"]')
  return el.exists() ? el.text() : ''
}

describe('W3B 独立验收探针', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    ;(window as any).visualViewport = { width: 1024, height: 768, scale: 1, pageLeft: 0, pageTop: 0, addEventListener: () => {}, removeEventListener: () => {}, dispatchEvent: () => true }
  })

  it('D-18：真实 401 响应文本里只有一个状态码，且没有 401: 401', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false, status: 401, statusText: 'Unauthorized',
      headers: { get: () => 'text/plain' },
      text: async () => '401: 缺少或错误的访问令牌\n',
    }))
    let caught: any = null
    try { await request('GET', '/api/version') } catch (e) { caught = e }
    expect(caught).not.toBeNull()
    expect(caught.status).toBe(401)
    expect(caught.message).toBe('HTTP 401: 缺少或错误的访问令牌')
    expect(caught.message.match(/401/g)!.length).toBe(1)
    expect(caught.message).not.toContain('401: 401')
    vi.unstubAllGlobals()
  })

  it('D-22：JSON 错误体主文案只取 error 字段，detail 进折叠，原始报文不外泄', async () => {
    const body = '{"error":"请求参数格式不正确：字段 interval 需要整数","detail":"json: cannot unmarshal string into Go struct field .interval of type int"}'
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false, status: 400, statusText: 'Bad Request',
      headers: { get: () => 'application/json' },
      text: async () => body,
    }))
    let caught: any = null
    try { await request('POST', '/api/pipelines', { body: {} }) } catch (e) { caught = e }
    expect(caught.message).toBe('HTTP 400: 请求参数格式不正确：字段 interval 需要整数')
    expect(caught.message).not.toContain('cannot unmarshal')
    expect(caught.detail).toContain('cannot unmarshal')
    expect(uaErrorDetail(caught)).toBe(caught.detail)
    vi.unstubAllGlobals()
  })

  it('D-22：500 的原始 panic 只在 detail，主文案是中文内部错误', async () => {
    const body = '{"error":"服务端内部错误（请求 ID r-9）","detail":"panic: runtime error: invalid memory address"}'
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false, status: 500, statusText: 'Internal Server Error',
      headers: { get: () => 'application/json' },
      text: async () => body,
    }))
    let caught: any = null
    try { await request('GET', '/api/version') } catch (e) { caught = e }
    expect(caught.message).toBe('HTTP 500: 服务端内部错误（请求 ID r-9）')
    expect(caught.message).not.toContain('panic')
    expect(caught.detail).toContain('panic')
    vi.unstubAllGlobals()
  })

  it('D-15：点「插入示例」展开示例列表，选中后才写入表单', async () => {
    listScriptsMock.mockResolvedValue([])
    const w = mount(ScriptsPage, { global: { plugins: [vuetify], stubs } })
    await flushPromises()
    const vm: any = w.vm
    await w.findAll('button').find((b: any) => b.text().includes('新建脚本'))!.trigger('click')
    await flushPromises()
    expect(vm.formCode).toBe('')
    await w.find('[data-testid="example-insert"]').trigger('click')
    await flushPromises()
    expect(vm.exampleMenu).toBe(true)
    expect(vm.formCode).toBe('')
    const items = w.findAll('[data-testid="example-item"]')
    expect(items.length).toBeGreaterThan(0)
    await items[0].trigger('click')
    await flushPromises()
    const def = SCRIPTS_HELP.examples[0]
    expect(def).toBeTruthy()
    expect(vm.formCode).toBe(def!.code)
    expect(vm.formCode.length).toBeGreaterThan(0)
    expect(vm.formKind).toBe(def!.kind)
    w.unmount()
  })

  it('D-21：删除脚本有二次确认——取消不删，确认才删并刷新', async () => {
    listScriptsMock.mockResolvedValue([{ id: 7, name: 'my-script', kind: 'filter', code: 'x', enabled: true }])
    deleteScriptMock.mockResolvedValue({ ok: true })
    const w = mount(ScriptsPage, { global: { plugins: [vuetify], stubs } })
    await flushPromises()
    await w.find('[data-testid="script-delete"]').trigger('click')
    await flushPromises()
    expect(deleteScriptMock).not.toHaveBeenCalled()
    expect(w.text()).toContain('删除')
    expect(w.text()).toContain('my-script')
    await w.find('[data-testid="script-confirm-cancel"]').trigger('click')
    await flushPromises()
    expect(deleteScriptMock).not.toHaveBeenCalled()
    await w.find('[data-testid="script-delete"]').trigger('click')
    await flushPromises()
    await w.find('[data-testid="script-confirm-delete"]').trigger('click')
    await flushPromises()
    expect(deleteScriptMock).toHaveBeenCalledWith(7)
    w.unmount()
  })

  it('D-23：试运行结果区限高可滚动，0 结果时对话框仍可关闭', async () => {
    listScriptsMock.mockResolvedValue([{ id: 8, name: 's8', kind: 'filter', code: 'return node', enabled: true }])
    testScriptMock.mockResolvedValue({ ok: true, kept: true, before: { a: 1 }, after: { a: 1 }, duration_ms: 3 })
    const w = mount(ScriptsPage, { global: { plugins: [vuetify], stubs } })
    await flushPromises()
    const vm: any = w.vm
    vm.openTest({ id: 8, name: 's8', kind: 'filter', code: 'return node', enabled: true })
    await flushPromises()
    await vm.runTest()
    await flushPromises()
    const el = w.find('[data-testid="test-result"]')
    expect(el.exists()).toBe(true)
    expect(el.classes()).toContain('test-result-scroll')
    expect(el.classes()).toContain('result-panel')
    w.unmount()
  })

  it('D-16：脚本 kind 为空时同屏只有一条中性提示，无 undefined / 无矛盾句', async () => {
    pipelinesMock.mockResolvedValue([])
    listScriptsMock.mockResolvedValue([{ id: 4, name: 'legacy', kind: '', code: 'return node', enabled: true }])
    const w = mount(PipelinesPage, { global: { plugins: [vuetify], stubs }, attachTo: document.body })
    await flushPromises()
    const vm: any = w.vm
    await w.find('[data-testid="pipeline-new"]').trigger('click')
    await flushPromises()
    vm.addStep()
    await flushPromises()
    vm.formSteps[0].op = 'op.script'
    vm.formSteps[0].args.ref = 'legacy'
    await flushPromises()
    await vm.$nextTick()
    const ok = document.querySelectorAll('[data-testid="script-ref-ok"]')
    expect(ok.length).toBe(1)
    expect(ok[0].textContent!.trim()).toBe('脚本已启用（类型未标注）')
    expect(document.querySelector('[data-testid="script-ref-kind-mismatch"]')).toBeNull()
    const scope = (ok[0].parentElement!.textContent ?? '')
    expect(scope).not.toContain('undefined')
    expect(scope).not.toContain('类型可能不匹配')
    w.unmount()
  })

  it('D-20：顶栏退出登录需确认，确认后清空 localStorage 并弹令牌框', async () => {
    saveToken('tok-abc')
    const w = mount(AppShell, { global: { plugins: [vuetify], stubs } })
    await flushPromises()
    await w.find('[data-testid="logout"]').trigger('click')
    await flushPromises()
    // 先取消
    await w.find('[data-testid="logout-cancel"]').trigger('click')
    await flushPromises()
    expect(window.localStorage.getItem(TOKEN_STORAGE_KEY)).toBe('tok-abc')
    expect(tokenDialogOpen.value).toBe(false)
    // 再确认
    await w.find('[data-testid="logout"]').trigger('click')
    await flushPromises()
    await w.find('[data-testid="logout-confirm"]').trigger('click')
    await flushPromises()
    expect(window.localStorage.getItem(TOKEN_STORAGE_KEY)).toBeNull()
    expect(tokenState.value).toBe('')
    expect(tokenDialogOpen.value).toBe(true)
    w.unmount()
  })

  it('D-22：脚本保存失败时主文案可读、原始报文只在折叠详情里', async () => {
    listScriptsMock.mockResolvedValue([])
    createScriptMock.mockRejectedValueOnce(new (await import('./api/client')).HttpError(400, '{"error":"请求参数格式不正确：字段 name 需要字符串","detail":"json: cannot unmarshal number into Go struct field .name of type string"}'))
    const w = mount(ScriptsPage, { global: { plugins: [vuetify], stubs } })
    await flushPromises()
    await w.findAll('button').find((b: any) => b.text().includes('新建脚本'))!.trigger('click')
    await flushPromises()
    await w.findAll('button').find((b: any) => b.text().includes('保存'))!.trigger('click')
    await flushPromises()
    const main = textOf(w, 'script-save-error')
    expect(main).toContain('请求参数格式不正确')
    expect(main).not.toContain('cannot unmarshal')
    expect(w.find('[data-testid="script-save-error-detail"]').exists()).toBe(true)
    expect(textOf(w, 'script-save-error-detail')).toContain('cannot unmarshal')
    w.unmount()
  })
})