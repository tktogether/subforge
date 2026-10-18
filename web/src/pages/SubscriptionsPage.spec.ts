import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'
import SubscriptionsPage from './SubscriptionsPage.vue'
import { endpoints, subscribeUrl } from '../api/endpoints'
import { notify } from '../composables/useNotify'
vi.mock('../api/endpoints', async () => {
  const actual = await vi.importActual<any>('../api/endpoints')
  return {
    endpoints: {
      listTokens: vi.fn(),
      createToken: vi.fn(),
      deleteToken: vi.fn(),
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
}

describe('SubscriptionsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
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
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify] } })
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
    const w2 = mount(SubscriptionsPage, { global: { plugins: [vuetify] } })
    const vm2 = w2.vm as unknown as SubscriptionsPageVm
    expect(vm2.resource.create).toBe(endpoints.createToken)
    w2.unmount()
  })

  it('resource.update(item) resolve 出 item 本身', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([])
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    const item = { token: 'tk', name: 'nm' }
    const res = await vm.resource.update(item)
    expect(res).toEqual(item)
    w.unmount()
  })

  it('resource.remove(item) 调 endpoints.deleteToken(item.token)', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([])
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    const item = { token: 'tok-abc', name: 'n' }
    await vm.resource.remove(item)
    expect(endpoints.deleteToken).toHaveBeenCalledWith('tok-abc')
    w.unmount()
  })

  it('selected computed：找不到或为空时返回 null', async () => {
    vi.mocked(endpoints.listTokens).mockResolvedValue([{ token: 't1', name: 'n1' }])
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify] } })
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
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify] } })
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
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify] } })
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
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify] } })
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
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.text()).toContain('请选择一个令牌')
    w.unmount()
  })

  it('模板：选中令牌后渲染 <code> 订阅 URL，按钮文案为「复制」', async () => {
    const origin = window.location.origin
    vi.mocked(endpoints.listTokens).mockResolvedValue([{ token: 't1', name: 'n1' }])
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify] } })
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
    const w = mount(SubscriptionsPage, { global: { plugins: [vuetify] } })
    const vm = w.vm as unknown as SubscriptionsPageVm
    await flushPromises()
    const selects = w.findAllComponents({ name: 'VSelect' })
    expect(selects.length).toBeGreaterThan(0)
    const subSelect = selects[selects.length - 1]
    await subSelect.setValue('t1')
    await nextTick()
    await flushPromises()
    expect(vm.selectedToken).toBe('t1')
    expect(vm.selected).toEqual({ token: 't1', name: 'n1' })
    await nextTick()
    expect(w.html()).toContain(subscribeUrl(window.location.origin, 't1'))
    w.unmount()
  })

})
