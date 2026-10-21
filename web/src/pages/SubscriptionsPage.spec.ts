import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'
import SubscriptionsPage from './SubscriptionsPage.vue'
import { endpoints, subscribeUrl } from '../api/endpoints'
import { pageHelp } from '../content/help'
import { notify } from '../composables/useNotify'
vi.mock('../api/endpoints', async () => {
  const actual = await vi.importActual<any>('../api/endpoints')
  return {
    endpoints: {
      listTokens: vi.fn(),
      listProfiles: vi.fn(),
      createToken: vi.fn(),
      updateToken: vi.fn(),
      targets: vi.fn(),
      pipelines: vi.fn(),
      configs: vi.fn(),
      deleteToken: vi.fn(),
      getTrafficStrategy: vi.fn(),
      setTrafficStrategy: vi.fn(),
      previewTraffic: vi.fn(),
      refreshAllAirports: vi.fn(),
    },
    subscribeUrl: actual.subscribeUrl,
  }
})

vi.mock('../composables/useNotify', () => ({
  notify: vi.fn(),
}))

import { vuetify } from '../plugins/vuetify'

interface SubscriptionsPageVm {
  selectedToken: string
  selected: { token: string; name: string } | null
  resource: any
  copyUrl: () => Promise<void>
  copied: boolean
  tokens: { token: string; name: string }[]
  errorTokens: string
  tokenDialog: boolean
  editingToken: { token: string; name: string } | null
  tokenForm: { token: string; name: string; target: string; pipeline: string; config: string; include: string; exclude: string; profile_id: number }
  profiles: { id: number; name: string }[]
  targetOptions: string[]
  loadingTargets: boolean
  preset: string
  upload: string
  download: string
  total: string
  expire: string
  savingStrategy: boolean
  previewHeader: string
  previewOk: boolean | null
  previewing: boolean
  refreshError: string
  refreshResult: any
  loadStrategy: () => Promise<void>
  saveStrategy: () => Promise<void>
  previewStrategy: () => Promise<void>
  refreshAll: () => Promise<void>
  onPresetChange: (v: string) => void
  applyQuick: (combo: string) => void
}

describe('SubscriptionsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(endpoints.getTrafficStrategy).mockResolvedValue({ strategy: { preset: 'sum', upload: 'sum', download: 'sum', total: 'sum', expire: 'min' } } as any)
    vi.mocked(endpoints.setTrafficStrategy).mockResolvedValue({ strategy: { preset: 'sum', upload: 'sum', download: 'sum', total: 'sum', expire: 'min' } } as any)
    vi.mocked(endpoints.previewTraffic).mockResolvedValue({ usage: {} as any, header: '', ok: false } as any)
    vi.mocked(endpoints.refreshAllAirports).mockResolvedValue({ results: [], ok: 0, fail: 0 } as any)
    vi.mocked(endpoints.targets).mockResolvedValue(['clash.meta', 'sing-box'])
    vi.mocked(endpoints.listProfiles).mockResolvedValue([])
  })

  function makeClipboard(writeText: ReturnType<typeof vi.fn>) {
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText },
      writable: true,
      configurable: true,
    })
  }

  it('onMounted 触发 resource.list 并调用 endpoints.listTokens，把返回值写入 tokens', async () => {
    const data = [{ token: 't1', name: 'n1' }]
    vi.mocked(endpoints.listTokens).mockResolvedValue(data)
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    expect(endpoints.listTokens).toHaveBeenCalled()
    // selected computed 能在 tokens 里找到该对象
    vm.selectedToken = 't1'
    await nextTick()
    expect(vm.selected).toEqual({ token: 't1', name: 'n1' })
    w.unmount()
  })

  it('resource.create 透传为 endpoints.createToken', () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([])
    const w2 = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    const vm2 = w2.vm as unknown as SubscriptionsPageVm
    expect(vm2.resource.create).toBe(endpoints.createToken)
    w2.unmount()
  })

  it('resource.update(item) resolve 出 item 本身', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([])
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    const item = { token: 'tk', name: 'nm' }
    const res = await vm.resource.update(item)
    expect(res).toEqual(item)
    w.unmount()
  })

  it('resource.remove(item) 调 endpoints.deleteToken(item.token)', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([])
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    const item = { token: 'tok-abc', name: 'n' }
    await vm.resource.remove(item)
    expect(endpoints.deleteToken).toHaveBeenCalledWith('tok-abc')
    w.unmount()
  })

  it('selected computed：找不到或为空时返回 null', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([{ token: 't1', name: 'n1' }])
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    vm.selectedToken = 'nope'
    await nextTick()
    expect(vm.selected).toBeNull()
    vm.selectedToken = ''
    await nextTick()
    expect(vm.selected).toBeNull()
    w.unmount()
  })

  it('copyUrl：selected 为 null 时提前 return，clipboard 未被调用', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([])
    const writeText = vi.fn().mockResolvedValue(undefined)
    makeClipboard(writeText)
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    await vm.copyUrl()
    expect(writeText).not.toHaveBeenCalled()
    w.unmount()
  })

  it('copyUrl：成功时 clipboard 收到 subscribeUrl 字符串，copied=true，notify("已复制","ok")', async () => {
    const origin = window.location.origin
    vi.mocked(endpoints.listTokens).mockResolvedValue([{ token: 't1', name: 'n1' }])
    const writeText = vi.fn().mockResolvedValue(undefined)
    makeClipboard(writeText)
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    vm.selectedToken = 't1'
    await nextTick()
    await vm.copyUrl()
    expect(writeText).toHaveBeenCalledWith(subscribeUrl(origin, 't1'))
    expect(vm.copied).toBe(true)
    expect(notify).toHaveBeenCalledWith('已复制', 'ok')
    w.unmount()
  })

  it('copyUrl：失败时 copied 保持 false，notify("复制失败","error")', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([{ token: 't1', name: 'n1' }])
    const writeText = vi.fn().mockRejectedValue(new Error('fail'))
    makeClipboard(writeText)
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    vm.selectedToken = 't1'
    await nextTick()
    await vm.copyUrl()
    expect(vm.copied).toBe(false)
    expect(notify).toHaveBeenCalledWith('复制失败', 'error')
    w.unmount()
  })

  it('模板：selected 为 null 时显示「请选择一个令牌」', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([])
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    await flushPromises()
    expect(w.text()).toContain('请选择一个令牌')
    w.unmount()
  })

  it('文案：订阅地址说明令牌可长期复用，不必每次新建', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([{ token: 't1', name: 'n1' }])
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    await flushPromises()
    expect(w.find('[data-testid="block-url"]').text()).toContain('长期重复使用')
    expect(w.find('[data-testid="block-url"]').text()).toContain('不必每次使用都新建令牌')
    w.unmount()
  })

  it('模板：选中令牌后渲染 <code> 订阅 URL，按钮文案为「复制」', async () => {
    const origin = window.location.origin
    vi.mocked(endpoints.listTokens).mockResolvedValue([{ token: 't1', name: 'n1' }])
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    vm.selectedToken = 't1'
    await nextTick()
    expect(w.text()).toContain(subscribeUrl(origin, 't1'))
    expect(w.text()).toContain('复制')
    w.unmount()
  })













  it('交互：v-select setValue 覆盖 v-model setter 箭头', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([{ token: 't1', name: 'n1' }])
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    const selects = w.findAllComponents({ name: 'VSelect' })
    expect(selects.length).toBeGreaterThan(0)
    const subSelect = selects[0]
    await subSelect.setValue('t1')
    await nextTick()
    await flushPromises()
    expect(vm.selectedToken).toBe('t1')
    expect(vm.selected).toEqual({ token: 't1', name: 'n1' })
    await nextTick()
    expect(w.html()).toContain(subscribeUrl(window.location.origin, 't1'))
    w.unmount()
  })


  it('页面副标题承载 oneLiner；help-panel 入口行=使用说明且详情默认收起', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([])
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    await flushPromises()
    const panel = w.find('[data-testid="help-panel"]')
    expect(panel.exists()).toBe(true)
    expect(w.find('.page-header__subtitle').text()).toContain(pageHelp('subscriptions').oneLiner)
    expect(panel.find('[data-testid="help-toggle-label"]').text()).toBe('使用说明')
    expect(panel.find('[data-testid="help-body"]').exists()).toBe(false)
    w.unmount()
  })

  it('余额刷新：结果表逐行显示 err，且失败不清零说明存在', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([])
    vi.mocked(endpoints.refreshAllAirports).mockResolvedValue({
      ok: 1,
      fail: 1,
      results: [
        { airport_id: 'a1', name: 'A1', ok: true, node_count: 5, err: '' },
        { airport_id: 'a2', name: 'A2', ok: false, node_count: 0, err: 'boom' },
      ],
    } as any)
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    const vm = w.vm as unknown as { refreshAll: () => Promise<void> }
    await flushPromises()
    await vm.refreshAll()
    await flushPromises()
    expect(w.text()).toContain('刷新失败不会把下游余额清零')
    const errs = w.findAll('[data-testid="refresh-error"]')
    expect(errs.length).toBeGreaterThan(0)
    expect(w.text()).toContain('boom')
    w.unmount()
  })

  it('流量策略：选 weighted 时 4 个字段下拉 disabled 且显示 preset-ignores-fields', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([])
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    await flushPromises()
    const vm = w.vm as unknown as { onPresetChange: (v: string) => void }
    vm.onPresetChange('weighted')
    await nextTick()
    await flushPromises()
    for (const tid of ['upload-field', 'download-field', 'total-field', 'expire-field']) {
      const sel = w.find('[data-testid="' + tid + '"]')
      expect(sel.classes().includes('v-input--disabled')).toBe(true)
    }
    expect(w.find('[data-testid="preset-ignores-fields"]').exists()).toBe(true)
    w.unmount()
  })

  it('流量策略：切回 sum 时字段下拉恢复可编辑', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([])
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    await flushPromises()
    const vm = w.vm as unknown as { onPresetChange: (v: string) => void }
    vm.onPresetChange('weighted')
    await nextTick()
    vm.onPresetChange('sum')
    await nextTick()
    await flushPromises()
    const sel = w.find('[data-testid="upload-field"]')
    expect(sel.attributes('disabled')).toBeUndefined()
    expect(w.find('[data-testid="preset-ignores-fields"]').exists()).toBe(false)
    w.unmount()
  })

  it('流量策略：field-zero-note 文本断言', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([])
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    await flushPromises()
    const note = w.find('[data-testid="field-zero-note"]')
    expect(note.exists()).toBe(true)
    expect(note.text()).toContain('跳过值为 0 的机场')
    w.unmount()
  })

  // ---- 以下为 100% 覆盖率补充用例 ----

  function dialogInput(w: ReturnType<typeof mount>, testid: string) {
    return w.find('[data-testid="' + testid + '"] input')
  }
  function dialogSet(w: ReturnType<typeof mount>, testid: string, value: string): void {
    const input = dialogInput(w, testid)
    expect(input.exists()).toBe(true)
    void input.setValue(value)
  }
  function dialogClick(w: ReturnType<typeof mount>, testid: string): void {
    const el = w.find('[data-testid="' + testid + '"]')
    expect(el.exists()).toBe(true)
    void el.trigger('click')
  }
  function clickButtonByText(w: ReturnType<typeof mount>, text: string): void {
    const btn = w.findAll('button').find((b) => b.text().includes(text))
    expect(btn).toBeTruthy()
    void btn!.trigger('click')
  }

  it('resource.list：listTokens reject 时 errorTokens 写入错误信息', async () => {
    vi.mocked(endpoints.listTokens).mockRejectedValue(new Error('listfail'))
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    expect(vm.errorTokens).toBe('listfail')
    expect(vm.tokens).toEqual([])
    w.unmount()
  })

  it('令牌表：maskToken 长 token 截断显示，空名称走 t.token', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([{ token: 'abcdefghij1234', name: '' }])
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    expect(w.text()).toContain('abcd…1234')
    vm.selectedToken = 'abcdefghij1234'
    await nextTick()
    expect(vm.selected).toEqual({ token: 'abcdefghij1234', name: '' })
    w.unmount()
  })

  it('令牌：点击「编辑」打开对话框（编辑模式）', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([{ token: 'tk1', name: 'n1' }])
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    await w.find('[data-testid="token-edit"]').trigger('click')
    await nextTick()
    await flushPromises()
    expect(vm.tokenDialog).toBe(true)
    expect(vm.editingToken).toEqual({ token: 'tk1', name: 'n1' })
    expect(w.text()).toContain('编辑令牌')
    w.unmount()
  })

  it('令牌：点击「新建令牌」打开对话框（新建模式），取消关闭', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([])
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    await w.find('[data-testid="create-token-btn"]').trigger('click')
    await nextTick()
    await flushPromises()
    expect(vm.tokenDialog).toBe(true)
    expect(vm.editingToken).toBeNull()
    expect(w.text()).toContain('新建令牌')
    clickButtonByText(w, '取消')
    await nextTick()
    await flushPromises()
    expect(vm.tokenDialog).toBe(false)
    w.unmount()
  })

  it('令牌：对话框 VDialog 的 update:modelValue 处理函数（点 stub close 触发）', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([])
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    await w.find('[data-testid="create-token-btn"]').trigger('click')
    await nextTick()
    await flushPromises()
    expect(vm.tokenDialog).toBe(true)
    const closeBtn = w.find('.stub-close')
    expect(closeBtn.exists()).toBe(true)
    await closeBtn.trigger('click')
    await nextTick()
    await flushPromises()
    expect(vm.tokenDialog).toBe(false)
    w.unmount()
  })

  it('令牌：新建保存走 createToken 分支并刷新列表', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([{ token: 'old', name: 'o' }])
    vi.mocked(endpoints.createToken).mockResolvedValue({ token: 'newtok', name: 'newname' })
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    await w.find('[data-testid="create-token-btn"]').trigger('click')
    await nextTick()
    await flushPromises()
    dialogSet(w, 'token-field', 'newtok')
    dialogSet(w, 'name-field', 'newname')
    await nextTick()
    dialogClick(w, 'token-save')
    await flushPromises()
    expect(endpoints.createToken).toHaveBeenCalledWith({ token: 'newtok', name: 'newname', target: '', pipeline: '', config: '', include: '', exclude: '', profile_id: 0 })
    expect(vm.tokenDialog).toBe(false)
    expect(endpoints.listTokens).toHaveBeenCalledTimes(2)
    expect(notify).toHaveBeenCalledWith('已保存', 'ok')
    w.unmount()
  })

  it('令牌：编辑保存走 update 分支，不调 createToken', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([{ token: 'tk1', name: 'n1' }])
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    await w.find('[data-testid="token-edit"]').trigger('click')
    await nextTick()
    await flushPromises()
    dialogClick(w, 'token-save')
    await flushPromises()
    expect(endpoints.createToken).not.toHaveBeenCalled()
    expect(vm.tokenDialog).toBe(false)
    expect(notify).toHaveBeenCalledWith('已保存', 'ok')
    w.unmount()
  })

  it('令牌：createToken reject 时提示错误且对话框保持打开', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([])
    vi.mocked(endpoints.createToken).mockRejectedValue(new Error('savefail'))
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    await w.find('[data-testid="create-token-btn"]').trigger('click')
    await nextTick()
    await flushPromises()
    dialogClick(w, 'token-save')
    await flushPromises()
    expect(notify).toHaveBeenCalledWith('savefail', 'error')
    expect(vm.tokenDialog).toBe(true)
    w.unmount()
  })

  it('令牌：删除调 endpoints.deleteToken；选中项被删时清空 selectedToken', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([{ token: 'tk1', name: 'n1' }])
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    vm.selectedToken = 'tk1'
    await nextTick()
    await w.find('[data-testid="token-delete"]').trigger('click')
    await flushPromises()
    expect(endpoints.deleteToken).toHaveBeenCalledWith('tk1')
    expect(vm.selectedToken).toBe('')
    expect(notify).toHaveBeenCalledWith('已删除', 'ok')
    w.unmount()
  })

  it('令牌：删除非选中令牌不清空 selectedToken', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([
      { token: 'tk1', name: 'n1' },
      { token: 'tk2', name: 'n2' },
    ])
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    vm.selectedToken = 'tk2'
    await nextTick()
    const dels = w.findAll('[data-testid="token-delete"]')
    await dels[0].trigger('click')
    await flushPromises()
    expect(endpoints.deleteToken).toHaveBeenCalledWith('tk1')
    expect(vm.selectedToken).toBe('tk2')
    w.unmount()
  })

  it('令牌：deleteToken reject 时提示错误', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([{ token: 'tk1', name: 'n1' }])
    vi.mocked(endpoints.deleteToken).mockRejectedValue(new Error('delfail'))
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    await flushPromises()
    await w.find('[data-testid="token-delete"]').trigger('click')
    await flushPromises()
    expect(notify).toHaveBeenCalledWith('delfail', 'error')
    w.unmount()
  })

  it('快捷按钮：5 个组合都写入字段并清空 preset', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([])
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    await w.find('[data-testid="quick-default"]').trigger('click')
    expect([vm.upload, vm.download, vm.total, vm.expire]).toEqual(['sum', 'sum', 'sum', 'min'])
    expect(vm.preset).toBe('')
    await w.find('[data-testid="quick-strict"]').trigger('click')
    expect([vm.upload, vm.download, vm.total, vm.expire]).toEqual(['min', 'min', 'min', 'min'])
    await w.find('[data-testid="quick-optimistic"]').trigger('click')
    expect([vm.upload, vm.download, vm.total, vm.expire]).toEqual(['max', 'max', 'max', 'max'])
    await w.find('[data-testid="quick-independent"]').trigger('click')
    expect([vm.upload, vm.download, vm.total, vm.expire]).toEqual(['avg', 'avg', 'avg', 'avg'])
    await w.find('[data-testid="quick-hidden"]').trigger('click')
    expect([vm.upload, vm.download, vm.total, vm.expire]).toEqual(['none', 'none', 'none', 'none'])
    w.unmount()
  })

  it('流量策略：onPresetChange(sum) 写入 4 个字段', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([])
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    vm.onPresetChange('sum')
    expect([vm.upload, vm.download, vm.total, vm.expire]).toEqual(['sum', 'sum', 'sum', 'min'])
    w.unmount()
  })

  it('流量策略：preset 下拉 setValue 触发 v-model setter', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([])
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    const selects = w.findAllComponents({ name: 'VSelect' })
    await selects[1].setValue('min')
    await nextTick()
    expect(vm.preset).toBe('min')
    w.unmount()
  })

  it('流量策略：4 个字段下拉 setValue 触发 v-model setter', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([])
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    const selects = w.findAllComponents({ name: 'VSelect' })
    await selects[2].setValue('max')
    await selects[3].setValue('min')
    await selects[4].setValue('avg')
    await selects[5].setValue('first')
    await nextTick()
    expect([vm.upload, vm.download, vm.total, vm.expire]).toEqual(['max', 'min', 'avg', 'first'])
    w.unmount()
  })

  it('流量策略：loadStrategy 字段缺失时回落默认值', async () => {
    vi.mocked(endpoints.getTrafficStrategy).mockResolvedValue({ strategy: {} } as any)
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    expect(vm.preset).toBe('sum')
    expect(vm.upload).toBe('sum')
    expect(vm.expire).toBe('min')
    w.unmount()
  })

  it('流量策略：loadStrategy reject 时提示错误', async () => {
    vi.mocked(endpoints.getTrafficStrategy).mockRejectedValue(new Error('loadfail'))
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    await flushPromises()
    expect(notify).toHaveBeenCalledWith('loadfail', 'error')
    w.unmount()
  })

  it('流量策略：保存透传 strategyObj 并回填回显', async () => {
    vi.mocked(endpoints.setTrafficStrategy).mockResolvedValue({
      strategy: { preset: 'sum', upload: 'max', download: 'max', total: 'max', expire: 'max' },
    } as any)
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    await vm.saveStrategy()
    await flushPromises()
    expect(endpoints.setTrafficStrategy).toHaveBeenCalledWith({ preset: 'sum', upload: 'sum', download: 'sum', total: 'sum', expire: 'min' })
    expect([vm.upload, vm.download, vm.total, vm.expire]).toEqual(['max', 'max', 'max', 'max'])
    expect(notify).toHaveBeenCalledWith('已保存', 'ok')
    expect(vm.savingStrategy).toBe(false)
    w.unmount()
  })

  it('流量策略：保存回显缺字段时保留当前值', async () => {
    vi.mocked(endpoints.setTrafficStrategy).mockResolvedValue({ strategy: {} } as any)
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    vm.upload = 'min'
    await vm.saveStrategy()
    await flushPromises()
    expect(vm.upload).toBe('min')
    expect(vm.savingStrategy).toBe(false)
    w.unmount()
  })

  it('流量策略：setTrafficStrategy reject 时提示错误并复位 loading', async () => {
    vi.mocked(endpoints.setTrafficStrategy).mockRejectedValue(new Error('savefail'))
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    await vm.saveStrategy()
    await flushPromises()
    expect(notify).toHaveBeenCalledWith('savefail', 'error')
    expect(vm.savingStrategy).toBe(false)
    w.unmount()
  })

  it('流量策略：试算展示 header 与 ok=true', async () => {
    vi.mocked(endpoints.previewTraffic).mockResolvedValue({ usage: {} as any, header: 'X-Sub: yes', ok: true } as any)
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    await vm.previewStrategy()
    await flushPromises()
    expect(vm.previewHeader).toBe('X-Sub: yes')
    expect(vm.previewOk).toBe(true)
    expect(w.find('[data-testid="preview-header"]').exists()).toBe(true)
    w.unmount()
  })

  it('流量策略：试算 reject 时提示错误', async () => {
    vi.mocked(endpoints.previewTraffic).mockRejectedValue(new Error('previewfail'))
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    await vm.previewStrategy()
    await flushPromises()
    expect(notify).toHaveBeenCalledWith('previewfail', 'error')
    expect(vm.previewing).toBe(false)
    w.unmount()
  })

  it('余额刷新：refreshAllAirports reject 时显示 refresh-error', async () => {
    vi.mocked(endpoints.refreshAllAirports).mockRejectedValue(new Error('refreshfail'))
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    await vm.refreshAll()
    await flushPromises()
    expect(vm.refreshError).toBe('refreshfail')
    expect(w.find('[data-testid="refresh-error"]').exists()).toBe(true)
    expect(vm.refreshResult).toBeNull()
    w.unmount()
  })

  it('余额刷新：结果缺 results 时回落 [] 并显示 refresh-empty', async () => {
    vi.mocked(endpoints.refreshAllAirports).mockResolvedValue({ ok: 1, fail: 0 } as any)
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify], stubs: { VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] } } } })
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    await vm.refreshAll()
    await flushPromises()
    expect(vm.refreshResult?.results).toBeUndefined()
    expect(w.find('[data-testid="refresh-empty"]').exists()).toBe(true)
    w.unmount()
  })

  // ---- T19：令牌字段级定制（target/pipeline/config/include/exclude） ----

  function mountSubsPage() {
    return mount(SubscriptionsPage, {
      global: {
        plugins: [vuetify],
        stubs: {
          VDialog: { template: '<div class="v-dialog-stub" v-if="modelValue"><button class="stub-close" @click="$emit(\'update:modelValue\', false)">close</button><slot/></div>', props: ['modelValue'], emits: ['update:modelValue'] },
        },
      },
    })
  }

  function selectByTestId(w: ReturnType<typeof mount>, testid: string) {
    const found = w.findAllComponents({ name: 'VSelect' }).find((c) => c.attributes('data-testid') === testid)
    expect(found).toBeTruthy()
    return found!
  }

  async function openTokenDialog(w: ReturnType<typeof mount>, how: 'edit' | 'create'): Promise<void> {
    const testid = how === 'edit' ? 'token-edit' : 'create-token-btn'
    await w.find('[data-testid="' + testid + '"]').trigger('click')
    await nextTick()
    await flushPromises()
  }

  it('令牌定制：弹窗内有 target 下拉与「高级选项」折叠区（含 4 个字段）', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([{ token: 'tk1', name: 'n1', target: 'clash.meta' }])
    const w = mountSubsPage()
    await flushPromises()
    await openTokenDialog(w, 'edit')
    const target = selectByTestId(w, 'target-select')
    expect(target.props('clearable')).toBe(true)
    expect(target.props('hint')).toBe('留空 = 默认 clash.meta')
    const advanced = w.find('[data-testid="token-advanced"]')
    expect(advanced.exists()).toBe(true)
    expect(advanced.text()).toContain('高级选项')
    const title = w.find('[data-testid="token-advanced"] .v-expansion-panel-title')
    expect(title.exists()).toBe(true)
    await title.trigger('click')
    await flushPromises()
    for (const id of ['token-pipeline', 'token-config', 'include-field', 'exclude-field']) {
      expect(w.find('[data-testid="' + id + '"]').exists()).toBe(true)
    }
    w.unmount()
  })

  it('令牌定制：编辑弹窗回填 5 个定制字段', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([
      { token: 'tk1', name: 'n1', target: 'clash.meta', pipeline: 'p1', config: 'c1', include: '香港', exclude: '过期' },
    ])
    const w = mountSubsPage()
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    await openTokenDialog(w, 'edit')
    expect(vm.tokenForm).toEqual({ token: 'tk1', name: 'n1', target: 'clash.meta', pipeline: 'p1', config: 'c1', include: '香港', exclude: '过期', profile_id: 0 })
    w.unmount()
  })

  it('令牌定制：编辑保存把 6 个字段全量发给 updateToken（空串也发）', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([
      { token: 'tk1', name: 'n1', target: 'clash.meta', pipeline: 'p1', config: 'c1', include: '香港', exclude: '过期' },
    ])
    vi.mocked(endpoints.updateToken).mockResolvedValue({ token: 'tk1', name: 'n1b' })
    const w = mountSubsPage()
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    await openTokenDialog(w, 'edit')
    dialogSet(w, 'name-field', 'n1b')
    dialogSet(w, 'token-pipeline', '')
    dialogSet(w, 'token-config', '')
    dialogSet(w, 'exclude-field', '')
    await nextTick()
    dialogClick(w, 'token-save')
    await flushPromises()
    expect(endpoints.updateToken).toHaveBeenCalledWith('tk1', {
      name: 'n1b', target: 'clash.meta', pipeline: '', config: '', include: '香港', exclude: '', profile_id: 0,
    })
    expect(endpoints.createToken).not.toHaveBeenCalled()
    expect(vm.tokenDialog).toBe(false)
    expect(notify).toHaveBeenCalledWith('已保存', 'ok')
    w.unmount()
  })

  it('令牌定制：target 下拉改值经 v-model 进入表单并随保存发出', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([{ token: 'tk1', name: 'n1' }])
    vi.mocked(endpoints.updateToken).mockResolvedValue({ token: 'tk1', name: 'n1' })
    const w = mountSubsPage()
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    await openTokenDialog(w, 'edit')
    await selectByTestId(w, 'target-select').setValue('sing-box')
    await nextTick()
    expect(vm.tokenForm.target).toBe('sing-box')
    dialogClick(w, 'token-save')
    await flushPromises()
    expect(endpoints.updateToken).toHaveBeenCalledWith('tk1', {
      name: 'n1', target: 'sing-box', pipeline: '', config: '', include: '', exclude: '', profile_id: 0,
    })
    w.unmount()
  })

  it('令牌定制：target 被 clearable 清空（null）时按空串提交，表达「清空定制」', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([{ token: 'tk1', name: 'n1', target: 'clash.meta' }])
    vi.mocked(endpoints.updateToken).mockResolvedValue({ token: 'tk1', name: 'n1' })
    const w = mountSubsPage()
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    await openTokenDialog(w, 'edit')
    selectByTestId(w, 'target-select').vm.$emit('update:modelValue', null)
    await nextTick()
    expect(vm.tokenForm.target).toBeNull()
    dialogClick(w, 'token-save')
    await flushPromises()
    expect(endpoints.updateToken).toHaveBeenCalledWith('tk1', expect.objectContaining({ target: '' }))
    w.unmount()
  })

  it('令牌定制：新建保存全量发送 token+name+5 个定制字段+profile_id（修复新建丢字段）', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([])
    vi.mocked(endpoints.createToken).mockResolvedValue({ token: 'newtok', name: 'newname' })
    const w = mountSubsPage()
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    await openTokenDialog(w, 'create')
    expect(vm.tokenForm).toEqual({ token: '', name: '', target: '', pipeline: '', config: '', include: '', exclude: '', profile_id: 0 })
    dialogSet(w, 'token-field', 'newtok')
    dialogSet(w, 'name-field', 'newname')
    dialogSet(w, 'include-field', '香港')
    await nextTick()
    await selectByTestId(w, 'target-select').setValue('sing-box')
    await nextTick()
    dialogClick(w, 'token-save')
    await flushPromises()
    expect(endpoints.createToken).toHaveBeenCalledWith({
      token: 'newtok', name: 'newname', target: 'sing-box', pipeline: '', config: '', include: '香港', exclude: '', profile_id: 0,
    })
    expect(endpoints.updateToken).not.toHaveBeenCalled()
    w.unmount()
  })

  it('令牌定制：updateToken reject 时提示错误且弹窗保持打开', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([{ token: 'tk1', name: 'n1' }])
    vi.mocked(endpoints.updateToken).mockRejectedValue(new Error('updfail'))
    const w = mountSubsPage()
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    await openTokenDialog(w, 'edit')
    dialogClick(w, 'token-save')
    await flushPromises()
    expect(notify).toHaveBeenCalledWith('updfail', 'error')
    expect(vm.tokenDialog).toBe(true)
    w.unmount()
  })

  it('令牌定制：targets 懒加载一次，弹窗未打开不请求、二次打开不重复请求', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([{ token: 'tk1', name: 'n1' }])
    const w = mountSubsPage()
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    expect(endpoints.targets).not.toHaveBeenCalled()
    await openTokenDialog(w, 'edit')
    expect(endpoints.targets).toHaveBeenCalledTimes(1)
    expect(vm.targetOptions).toEqual(['clash.meta', 'sing-box'])
    expect(vm.loadingTargets).toBe(false)
    await w.find('.stub-close').trigger('click')
    await nextTick()
    await flushPromises()
    await openTokenDialog(w, 'create')
    expect(endpoints.targets).toHaveBeenCalledTimes(1)
    w.unmount()
  })

  it('令牌定制：targets 拉取失败不阻塞弹窗，失败后下次打开重试', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([{ token: 'tk1', name: 'n1' }])
    vi.mocked(endpoints.targets).mockRejectedValue(new Error('tgfail'))
    const w = mountSubsPage()
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    await openTokenDialog(w, 'edit')
    expect(vm.tokenDialog).toBe(true)
    expect(vm.targetOptions).toEqual([])
    expect(vm.loadingTargets).toBe(false)
    expect(notify).not.toHaveBeenCalled()
    await w.find('.stub-close').trigger('click')
    await nextTick()
    await flushPromises()
    await openTokenDialog(w, 'create')
    expect(endpoints.targets).toHaveBeenCalledTimes(2)
    w.unmount()
  })

  it('打开令牌对话框后加载 pipeline/config 候选', async () => {
    ;(endpoints.pipelines as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([{ name: 'clean' }])
    ;(endpoints.configs as unknown as ReturnType<typeof vi.fn>).mockResolvedValue({ configs: ['http://x/a.ini'], default: 'http://x/a.ini' })
    const w = mountSubsPage()
    const vm = w.vm as any
    await flushPromises()
    vm.openCreateToken()
    await flushPromises()
    expect(endpoints.pipelines).toHaveBeenCalled()
    expect(endpoints.configs).toHaveBeenCalled()
    expect(vm.pipelineOptions).toEqual(['clean'])
    expect(vm.configOptions).toEqual(['http://x/a.ini'])
    w.unmount()
  })

  // ---- 方案（Profile）绑定 ----

  it('方案：页面加载拉取 profiles，弹窗下拉给出「未分配 + 各方案」', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([])
    vi.mocked(endpoints.listProfiles).mockResolvedValue([
      { id: 1, name: 'A-方案', description: '', config: '', pipeline: '', enabled: true, sort_order: 1, rule_count: 0, group_count: 0 },
      { id: 2, name: 'B-方案', description: '', config: '', pipeline: '', enabled: true, sort_order: 2, rule_count: 0, group_count: 0 },
    ])
    const w = mountSubsPage()
    await flushPromises()
    expect(endpoints.listProfiles).toHaveBeenCalled()
    await openTokenDialog(w, 'create')
    const sel = selectByTestId(w, 'token-profile')
    expect(sel.props('items')).toEqual([
      { title: '未分配', value: 0 },
      { title: 'A-方案', value: 1 },
      { title: 'B-方案', value: 2 },
    ])
    expect(w.find('[data-testid="token-profile-hint"]').text()).toContain('绑定方案后，订阅只使用该方案的规则与分组')
    w.unmount()
  })

  it('方案：profiles 拉取失败降级为仅未分配，令牌列表不受影响', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([{ token: 'tk1', name: 'n1' }])
    vi.mocked(endpoints.listProfiles).mockRejectedValue(new Error('pfail'))
    const w = mountSubsPage()
    await flushPromises()
    expect(w.find('[data-testid="token-row"]').exists()).toBe(true)
    await openTokenDialog(w, 'create')
    const sel = selectByTestId(w, 'token-profile')
    expect(sel.props('items')).toEqual([{ title: '未分配', value: 0 }])
    w.unmount()
  })

  it('方案：令牌表格显示方案列，未分配显示「未分配」', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([
      { token: 'tk1', name: 'n1', profile_id: 1, profile_name: 'A-方案' },
      { token: 'tk2', name: 'n2' },
    ])
    const w = mountSubsPage()
    await flushPromises()
    const cells = w.findAll('[data-testid="token-profile-name"]')
    expect(cells.length).toBe(2)
    expect(cells[0].text()).toBe('A-方案')
    expect(cells[1].text()).toBe('未分配')
    w.unmount()
  })

  it('方案：新建令牌可绑定方案，body 带 profile_id 与全部定制字段', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([])
    vi.mocked(endpoints.listProfiles).mockResolvedValue([
      { id: 1, name: 'A-方案', description: '', config: '', pipeline: '', enabled: true, sort_order: 1, rule_count: 0, group_count: 0 },
    ])
    vi.mocked(endpoints.createToken).mockResolvedValue({ token: 'newtok', name: 'newname' })
    const w = mountSubsPage()
    await flushPromises()
    await openTokenDialog(w, 'create')
    dialogSet(w, 'token-field', 'newtok')
    dialogSet(w, 'name-field', 'newname')
    dialogSet(w, 'token-config', 'http://x/a.ini')
    await nextTick()
    await selectByTestId(w, 'token-profile').setValue(1)
    await nextTick()
    dialogClick(w, 'token-save')
    await flushPromises()
    expect(endpoints.createToken).toHaveBeenCalledWith({
      token: 'newtok', name: 'newname', target: '', pipeline: '', config: 'http://x/a.ini', include: '', exclude: '', profile_id: 1,
    })
    w.unmount()
  })

  it('方案：编辑令牌回填 profile_id 并随保存发出', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([
      { token: 'tk1', name: 'n1', profile_id: 1, profile_name: 'A-方案' },
    ])
    vi.mocked(endpoints.updateToken).mockResolvedValue({ token: 'tk1', name: 'n1' })
    const w = mountSubsPage()
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    await openTokenDialog(w, 'edit')
    expect(vm.tokenForm.profile_id).toBe(1)
    expect(selectByTestId(w, 'token-profile').props('modelValue')).toBe(1)
    dialogClick(w, 'token-save')
    await flushPromises()
    expect(endpoints.updateToken).toHaveBeenCalledWith('tk1', expect.objectContaining({ profile_id: 1 }))
    w.unmount()
  })

  it('选择字段级预设自动填入 4 个策略字段（T12 补 onPresetChange 展开分支）', async () => {
    const w = mountSubsPage()
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    vm.onPresetChange('min')
    expect(vm.preset).toBe('min')
    expect(vm.upload).toBe('min')
    expect(vm.download).toBe('min')
    expect(vm.total).toBe('min')
    expect(vm.expire).toBe('min')
    w.unmount()
  })
})
