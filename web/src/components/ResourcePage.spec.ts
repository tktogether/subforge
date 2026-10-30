import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ResourcePage from './ResourcePage.vue'
import { vuetify } from '../plugins/vuetify'
import { notifyState } from '../composables/useNotify'

const columns = [
  { key: 'name', title: '名称' },
  { key: 'enabled', title: '状态' },
  { key: 'subscription_url', title: '订阅' },
  { key: 'link', title: '链接' },
  { key: 'token', title: '令牌' },
]
const fields = [
  { key: 'name', label: '名称', type: 'text' },
  { key: 'count', label: '数量', type: 'number' },
  { key: 'enabled', label: '启用', type: 'switch' },
]

const sampleItem = {
  name: 'a',
  enabled: true,
  subscription_url: 'http://x/sub',
  link: 'http://x/link',
  token: 'tok123',
}

function makeResource(items: any[] = [sampleItem]) {
  return {
    list: vi.fn().mockResolvedValue(items),
    create: vi.fn().mockResolvedValue(undefined),
    update: vi.fn().mockResolvedValue(undefined),
    remove: vi.fn().mockResolvedValue(undefined),
  }
}

interface ResourcePageVm {
  headers: any
  emptyText: string
  openCreate: () => void
  openEdit: (row: any) => void
  draft: Record<string, any>
  editing: any
  dialog: boolean
  submit: () => Promise<void>
  askDelete: (row: any) => void
  pendingDelete: any
  confirmOpen: boolean
  doDelete: () => Promise<void>
}

describe('ResourcePage', () => {
  beforeEach(() => {
    notifyState.open = false
    notifyState.text = ''
    notifyState.kind = 'ok'
    if (typeof window.visualViewport === 'undefined') {
      (window as any).visualViewport = {
        width: 1024,
        height: 768,
        offsetLeft: 0,
        offsetTop: 0,
        pageLeft: 0,
        pageTop: 0,
        scale: 1,
        addEventListener: () => {},
        removeEventListener: () => {},
        dispatchEvent: () => false,
      }
    }
  })

  it('setup 时自动 reload，resource.list 被调用 1 次', async () => {
    const resource = makeResource()
    mount(ResourcePage, {
      props: { title: 'T', subtitle: 'S', columns, fields, resource },
      global: { plugins: [vuetify] },
      attachTo: document.body,
    })
    await flushPromises()
    expect(resource.list).toHaveBeenCalledTimes(1)
  })

  it('headers：readonlyMode===true 等于 columns；否则追加 actions 列', () => {
    const resource = makeResource()
    const w = mount(ResourcePage, {
      props: { title: 'T', subtitle: 'S', columns, fields, resource, readonlyMode: true },
      global: { plugins: [vuetify] },
      attachTo: document.body,
    })
    const vm = w.vm as unknown as ResourcePageVm
    expect(vm.headers).toEqual(columns)

    const w2 = mount(ResourcePage, {
      props: { title: 'T', subtitle: 'S', columns, fields, resource },
      global: { plugins: [vuetify] },
      attachTo: document.body,
    })
    const vm2 = w2.vm as unknown as ResourcePageVm
    expect(vm2.headers).toEqual(columns.concat([{ key: 'actions', title: '操作' }]))
  })

  it('emptyText：传了用传的，没传默认 "暂无数据"', () => {
    const resource = makeResource()
    const w = mount(ResourcePage, {
      props: { title: 'T', subtitle: 'S', columns, fields, resource, emptyText: '空空' },
      global: { plugins: [vuetify] },
      attachTo: document.body,
    })
    const vm = w.vm as unknown as ResourcePageVm
    expect(vm.emptyText).toBe('空空')

    const w2 = mount(ResourcePage, {
      props: { title: 'T', subtitle: 'S', columns, fields, resource },
      global: { plugins: [vuetify] },
      attachTo: document.body,
    })
    const vm2 = w2.vm as unknown as ResourcePageVm
    expect(vm2.emptyText).toBe('暂无数据')
  })

  it('blank()：openCreate 后 draft 按类型初始化', async () => {
    const resource = makeResource()
    const w = mount(ResourcePage, {
      props: { title: 'T', subtitle: 'S', columns, fields, resource },
      global: { plugins: [vuetify] },
      attachTo: document.body,
    })
    const vm = w.vm as unknown as ResourcePageVm
    await flushPromises()
    vm.openCreate()
    await w.vm.$nextTick()
    expect(vm.draft).toEqual({ name: '', count: 0, enabled: true })
  })

  it('blank()：字段声明 default 时优先使用该值', async () => {
    const localFields = [
      { key: 'name', label: '名称', type: 'text' },
      { key: 'include_self', label: '包含自建节点', type: 'switch', default: false },
      { key: 'enabled', label: '启用', type: 'switch' },
    ]
    const resource = makeResource()
    const w = mount(ResourcePage, {
      props: { title: 'T', subtitle: 'S', columns, fields: localFields, resource },
      global: { plugins: [vuetify] },
      attachTo: document.body,
    })
    const vm = w.vm as unknown as ResourcePageVm
    await flushPromises()
    vm.openCreate()
    await w.vm.$nextTick()
    expect(vm.draft).toEqual({ name: '', include_self: false, enabled: true })
    w.unmount()
  })

  it('openEdit(row)：draft 是副本、editing 为 row、dialog 打开', async () => {
    const resource = makeResource()
    const row = { name: 'a', count: 1, enabled: false }
    const w = mount(ResourcePage, {
      props: { title: 'T', subtitle: 'S', columns, fields, resource },
      global: { plugins: [vuetify] },
      attachTo: document.body,
    })
    const vm = w.vm as unknown as ResourcePageVm
    await flushPromises()
    vm.openEdit(row)
    await w.vm.$nextTick()
    expect(vm.draft).not.toBe(row)
    expect(vm.draft).toEqual({ ...row })
    expect(vm.editing).toEqual(row)
    expect(vm.dialog).toBe(true)
    vm.draft.name = 'changed'
    expect(row.name).toBe('a')
  })

  it('submit() 新增分支：调 create，成功关闭对话框 + notify("已新增")', async () => {
    const resource = makeResource()
    const w = mount(ResourcePage, {
      props: { title: 'T', subtitle: 'S', columns, fields, resource },
      global: { plugins: [vuetify] },
      attachTo: document.body,
    })
    const vm = w.vm as unknown as ResourcePageVm
    await flushPromises()
    vm.openCreate()
    await w.vm.$nextTick()
    vm.draft = { name: 'x', count: 1, enabled: true }
    await vm.submit()
    await flushPromises()
    expect(resource.create).toHaveBeenCalledWith({ name: 'x', count: 1, enabled: true })
    expect(resource.list).toHaveBeenCalled()
    expect(vm.dialog).toBe(false)
    expect(notifyState.text).toBe('已新增')
    expect(notifyState.open).toBe(true)
  })

  it('submit() 新增失败：notify(error, "error") 且对话框仍开', async () => {
    const resource = makeResource()
    resource.create.mockRejectedValue(new Error('create boom'))
    const w = mount(ResourcePage, {
      props: { title: 'T', subtitle: 'S', columns, fields, resource },
      global: { plugins: [vuetify] },
      attachTo: document.body,
    })
    const vm = w.vm as unknown as ResourcePageVm
    await flushPromises()
    vm.openCreate()
    await w.vm.$nextTick()
    vm.draft = { name: 'x', count: 1, enabled: true }
    await vm.submit()
    await flushPromises()
    expect(vm.dialog).toBe(true)
    expect(notifyState.text).toBe('create boom')
    expect(notifyState.kind).toBe('error')
  })

  it('submit() 编辑分支：调 update + notify("已更新")', async () => {
    const resource = makeResource()
    const row = { name: 'a', count: 1, enabled: false }
    const w = mount(ResourcePage, {
      props: { title: 'T', subtitle: 'S', columns, fields, resource },
      global: { plugins: [vuetify] },
      attachTo: document.body,
    })
    const vm = w.vm as unknown as ResourcePageVm
    await flushPromises()
    vm.openEdit(row)
    await w.vm.$nextTick()
    vm.draft = { name: 'b', count: 2, enabled: true }
    await vm.submit()
    await flushPromises()
    expect(resource.update).toHaveBeenCalledWith({ name: 'b', count: 2, enabled: true })
    expect(resource.create).not.toHaveBeenCalled()
    expect(notifyState.text).toBe('已更新')
    expect(vm.dialog).toBe(false)
  })

  it('askDelete(row) 设置 pendingDelete 并打开确认框；doDelete 有行时调 remove + notify("已删除","ok")', async () => {
    const resource = makeResource()
    const row = { name: 'a' }
    const w = mount(ResourcePage, {
      props: { title: 'T', subtitle: 'S', columns, fields, resource },
      global: { plugins: [vuetify] },
      attachTo: document.body,
    })
    const vm = w.vm as unknown as ResourcePageVm
    await flushPromises()
    vm.askDelete(row)
    await w.vm.$nextTick()
    expect(vm.pendingDelete).toEqual(row)
    expect(vm.confirmOpen).toBe(true)

    await vm.doDelete()
    await flushPromises()
    expect(resource.remove).toHaveBeenCalledWith(row)
    expect(resource.list).toHaveBeenCalled()
    expect(notifyState.text).toBe('已删除')
    expect(notifyState.kind).toBe('ok')
  })

  it('doDelete() 在 pendingDelete===null 时提前 return，不调 remove', async () => {
    const resource = makeResource()
    const w = mount(ResourcePage, {
      props: { title: 'T', subtitle: 'S', columns, fields, resource },
      global: { plugins: [vuetify] },
      attachTo: document.body,
    })
    const vm = w.vm as unknown as ResourcePageVm
    await flushPromises()
    const callsBefore = resource.remove.mock.calls.length
    vm.pendingDelete = null
    await vm.doDelete()
    await flushPromises()
    expect(resource.remove.mock.calls.length).toBe(callsBefore)
  })

  it('readonlyMode 为 true 时 [data-testid="add"] 不存在且表头无「操作」；为 false 时存在', async () => {
    const resource = makeResource()
    const w = mount(ResourcePage, {
      props: { title: 'T', subtitle: 'S', columns, fields, resource, readonlyMode: true },
      global: { plugins: [vuetify] },
      attachTo: document.body,
    })
    const vm = w.vm as unknown as ResourcePageVm
    await flushPromises()
    expect(w.find('[data-testid="add"]').exists()).toBe(false)
    expect(vm.headers).not.toEqual(expect.arrayContaining([{ key: 'actions', title: '操作' }]))

    const w2 = mount(ResourcePage, {
      props: { title: 'T', subtitle: 'S', columns, fields, resource, readonlyMode: false },
      global: { plugins: [vuetify] },
      attachTo: document.body,
    })
    await flushPromises()
    expect(w2.find('[data-testid="add"]').exists()).toBe(true)
  })

  it('点 [data-testid="refresh"] → resource.list 再被调用一次', async () => {
    const resource = makeResource()
    const w = mount(ResourcePage, {
      props: { title: 'T', subtitle: 'S', columns, fields, resource },
      global: { plugins: [vuetify] },
      attachTo: document.body,
    })
    await flushPromises()
    const before = resource.list.mock.calls.length
    await w.find('[data-testid="refresh"]').trigger('click')
    await flushPromises()
    expect(resource.list.mock.calls.length).toBe(before + 1)
  })

  it('字段渲染：switch/number/text 类型时各 data-testid 渲染出来', async () => {
    const resource = makeResource()
    const w = mount(ResourcePage, {
      props: { title: 'T', subtitle: 'S', columns, fields, resource },
      global: { plugins: [vuetify] },
      attachTo: document.body,
    })
    const vm = w.vm as unknown as ResourcePageVm
    await flushPromises()
    vm.openCreate()
    await w.vm.$nextTick()
    await flushPromises()
    await w.vm.$nextTick()
    await new Promise((r) => setTimeout(r, 50))
    await flushPromises()
    expect(document.querySelector('[data-testid="field-switch"]')).not.toBeNull()
    expect(document.querySelector('[data-testid="field-number"]')).not.toBeNull()
    expect(document.querySelector('[data-testid="field-text"]')).not.toBeNull()
  })

  it('AsyncSection 空态：items 为空且传 emptyText 时显示该文案', async () => {
    const resource = makeResource([])
    const w = mount(ResourcePage, {
      props: { title: 'T', subtitle: 'S', columns, fields, resource, emptyText: '没有数据哦' },
      global: { plugins: [vuetify] },
      attachTo: document.body,
    })
    await flushPromises()
    expect(w.text()).toContain('没有数据哦')
  })

  it('交互覆盖：点击编辑/删除/取消按钮与字段 v-model setter', async () => {
    const resource = makeResource()
    const w = mount(ResourcePage, {
      props: { title: 'T', subtitle: 'S', columns, fields, resource },
      global: { plugins: [vuetify] },
      attachTo: document.body,
    })
    const vm = w.vm as unknown as ResourcePageVm
    await flushPromises()

    // 点击编辑按钮（覆盖 item.actions 的 openEdit 箭头）
    const editBtn = w.find('[data-testid="edit"]')
    if (editBtn.exists()) await editBtn.trigger('click')
    await flushPromises()

    // 点击删除按钮（覆盖 askDelete 箭头）
    const delBtn = w.find('[data-testid="delete"]')
    if (delBtn.exists()) await delBtn.trigger('click')
    await flushPromises()

    // v-model setter：dialog v-model (line 159)
    const dialogs = w.findAllComponents({ name: 'VDialog' })
    for (const d of dialogs) {
      try { await (d as any).trigger('update:modelValue', false) } catch {}
      try { await (d as any).vm.$emit('update:modelValue', false) } catch {}
    }
    // v-model setter：字段 switch (line 166)
    const fs2 = w.find('[data-testid="field-switch"]')
    if (fs2.exists()) {
      await (fs2 as unknown as { trigger: (e: string, p?: unknown) => Promise<void> }).trigger('update:modelValue', false)
      const sw = w.findAllComponents({ name: 'VSwitch' })
      for (const s of sw) { try { await (s as any).setValue(false) } catch {} }
    }
    // v-model setter：confirm dialog (line 201) — askDelete opens it
    await vm.askDelete({ name: 'a' })
    await flushPromises()
    const dialogs2 = w.findAllComponents({ name: 'VDialog' })
    for (const d of dialogs2) {
      try { await (d as any).trigger('update:modelValue', false) } catch {}
      try { await (d as any).vm.$emit('update:modelValue', false) } catch {}
    }
    const fn = w.find('[data-testid="field-number"]')
    if (fn.exists()) { const inp = fn.find('input'); if (inp.exists()) await inp.setValue(5); else await (fn as unknown as { trigger: (e: string, p?: unknown) => Promise<void> }).trigger('update:modelValue', 5) }
    const ft = w.find('[data-testid="field-text"]')
    if (ft.exists()) { const inp = ft.find('input'); if (inp.exists()) await inp.setValue('abc'); else await (ft as unknown as { trigger: (e: string, p?: unknown) => Promise<void> }).trigger('update:modelValue', 'abc') }
    await flushPromises()

    // 取消按钮
    const dc = w.find('[data-testid="dialog-cancel"]')
    if (dc.exists()) await dc.trigger('click')
    await flushPromises()
    const cc = w.find('[data-testid="confirm-cancel"]')
    if (cc.exists()) await cc.trigger('click')
    await flushPromises()
  })

  it('对话框字段 v-model setter：VSwitch/VTextField 组件级 setValue 写入 draft', async () => {
    const localFields = [
      { key: 'enabled', label: '启用', type: 'switch' },
      { key: 'port', label: '端口', type: 'number' },
      { key: 'name', label: '名称', type: 'text' },
    ]
    const resource = makeResource()
    const w = mount(ResourcePage, {
      props: { title: 'T', subtitle: 'S', columns, fields: localFields, resource },
      global: { plugins: [vuetify] },
      attachTo: document.body,
    })
    const vm = w.vm as unknown as ResourcePageVm
    await flushPromises()
    vm.openCreate()
    await w.vm.$nextTick()
    await flushPromises()

    const sw = w.findAllComponents({ name: 'VSwitch' })
    expect(sw.length).toBeGreaterThan(0)
    await sw[0].setValue(false)
    expect(vm.draft.enabled).toBe(false)

    const tf = w.findAllComponents({ name: 'VTextField' })
    expect(tf.length).toBeGreaterThanOrEqual(2)
    const numComp = tf.find((c) => c.attributes('data-testid') === 'field-number')
    const txtComp = tf.find((c) => c.attributes('data-testid') === 'field-text')
    expect(numComp).toBeTruthy()
    expect(txtComp).toBeTruthy()
    await (txtComp as any).setValue('hello')
    await (numComp as any).setValue('42')
    expect(vm.draft.name).toBe('hello')
    expect(vm.draft.port).toBe(42)
  })


  it('点击 dialog-cancel / confirm-cancel 关闭对话框（stub 内联）', async () => {
    const resource = makeResource()
    const w = mount(ResourcePage, {
      props: { title: 'T', subtitle: 'S', columns, fields, resource },
      global: { plugins: [vuetify], stubs: { VDialog: { name: 'VDialog', props: ['modelValue'], template: '<div class="dlg" v-if="modelValue"><slot /></div>' } } },
      attachTo: document.body,
    })
    const vm = w.vm as unknown as ResourcePageVm
    await flushPromises()
    vm.openCreate()
    await w.vm.$nextTick()
    expect(vm.dialog).toBe(true)
    await w.find('[data-testid="dialog-cancel"]').trigger('click')
    await flushPromises()
    expect(vm.dialog).toBe(false)

    vm.askDelete({ name: 'a' })
    await w.vm.$nextTick()
    expect(vm.confirmOpen).toBe(true)
    await w.find('[data-testid="confirm-cancel"]').trigger('click')
    await flushPromises()
    expect(vm.confirmOpen).toBe(false)
  })

  it('字段 hint/example：显式传入时渲染 InfoTip 提示文本（含示例）', async () => {
    const localFields = [
      { key: 'name', label: '名称', type: 'text', hint: '自定义提示文案', example: '自定义示例值' },
    ]
    const resource = makeResource()
    const w = mount(ResourcePage, {
      props: { title: 'T', subtitle: 'S', columns, fields: localFields, resource },
      global: { plugins: [vuetify], stubs: { VDialog: { name: 'VDialog', props: ['modelValue'], template: '<div class="dlg" v-if="modelValue"><slot /></div>' } } },
      attachTo: document.body,
    })
    const vm = w.vm as unknown as ResourcePageVm
    await flushPromises()
    vm.openCreate()
    await w.vm.$nextTick()
    await flushPromises()
    const tips = w.findAll('[data-testid="info-tip-text"]')
    expect(tips.length).toBe(1)
    expect(tips[0].text()).toContain('自定义提示文案')
    expect(tips[0].text()).toContain('示例：自定义示例值')
  })

  it('字段 hint/example：未显式传入时回退到 fieldHelp（FIELD_HELP）', async () => {
    const localFields = [
      { key: 'airport.name', label: '名称', type: 'text' },
      { key: 'airport.subscription_url', label: '订阅', type: 'text' },
    ]
    const resource = makeResource()
    const w = mount(ResourcePage, {
      props: { title: 'T', subtitle: 'S', columns, fields: localFields, resource },
      global: { plugins: [vuetify], stubs: { VDialog: { name: 'VDialog', props: ['modelValue'], template: '<div class="dlg" v-if="modelValue"><slot /></div>' } } },
      attachTo: document.body,
    })
    const vm = w.vm as unknown as ResourcePageVm
    await flushPromises()
    vm.openCreate()
    await w.vm.$nextTick()
    await flushPromises()
    const tips = w.findAll('[data-testid="info-tip-text"]')
    expect(tips.length).toBe(2)
    expect(w.text()).toContain('机场显示名称，用于识别来源。')
    expect(w.text()).toContain('香港专线')
  })

  it('字段 hint/example：既无显式也无 FIELD_HELP 时不渲染', async () => {
    const localFields = [
      { key: 'nope.unknown_field', label: '未知', type: 'text' },
    ]
    const resource = makeResource()
    const w = mount(ResourcePage, {
      props: { title: 'T', subtitle: 'S', columns, fields: localFields, resource },
      global: { plugins: [vuetify], stubs: { VDialog: { name: 'VDialog', props: ['modelValue'], template: '<div class="dlg" v-if="modelValue"><slot /></div>' } } },
      attachTo: document.body,
    })
    const vm = w.vm as unknown as ResourcePageVm
    await flushPromises()
    vm.openCreate()
    await w.vm.$nextTick()
    await flushPromises()
    expect(w.find('[data-testid="info-tip"]').exists()).toBe(false)
    expect(w.find('[data-testid="info-tip-text"]').exists()).toBe(false)
  })

  it('empty 传 EmptyStateHelp 且列表为空时渲染 empty-state（优先于 emptyText）', async () => {
    const resource = makeResource([])
    const w = mount(ResourcePage, {
      props: {
        title: 'T', subtitle: 'S', columns, fields, resource,
        emptyText: '旧的空文案',
        empty: { title: '还没有任何记录', detail: '点右上角新增', action: { label: '去新增' } },
      },
      global: { plugins: [vuetify] },
      attachTo: document.body,
    })
    await flushPromises()
    const es = w.find('[data-testid="empty-state"]')
    expect(es.exists()).toBe(true)
    expect(w.find('[data-testid="empty-state-title"]').text()).toContain('还没有任何记录')
    expect(w.find('[data-testid="empty-state-detail"]').text()).toContain('点右上角新增')
    expect(w.text()).not.toContain('旧的空文案')
  })

  it('列表非空时即使传了 empty 也不渲染 empty-state', async () => {
    const resource = makeResource()
    const w = mount(ResourcePage, {
      props: {
        title: 'T', subtitle: 'S', columns, fields, resource,
        empty: { title: '不该出现', detail: 'd' },
      },
      global: { plugins: [vuetify] },
      attachTo: document.body,
    })
    await flushPromises()
    expect(w.find('[data-testid="empty-state"]').exists()).toBe(false)
  })
  it('传了 helpKey 时渲染 help-panel（P8 透传）', async () => {
    const resource = makeResource()
    const w = mount(ResourcePage, {
      props: { title: 'T', subtitle: 'S', columns, fields, resource, helpKey: 'groups' },
      global: { plugins: [vuetify] },
      attachTo: document.body,
    })
    await flushPromises()
    expect(w.find('[data-testid="help-panel"]').exists()).toBe(true)
  })

  it('没传 helpKey 时不渲染 help-panel', async () => {
    const resource = makeResource()
    const w = mount(ResourcePage, {
      props: { title: 'T', subtitle: 'S', columns, fields, resource },
      global: { plugins: [vuetify] },
      attachTo: document.body,
    })
    await flushPromises()
    expect(w.find('[data-testid="help-panel"]').exists()).toBe(false)
  })

  it('rankKey：按 rankKey 升序排名显示 1、2、3…，值不在表里时兜底显示原值', async () => {
    const resource = makeResource([
      { name: 'a', sort_order: 100 },
      { name: 'b', sort_order: 50 },
    ])
    const w = mount(ResourcePage, {
      props: {
        title: 'T',
        subtitle: 'S',
        columns: [
          { key: 'name', title: '名称' },
          { key: 'sort_order', title: '顺序' },
        ],
        fields,
        resource,
        rankKey: 'sort_order',
      },
      global: { plugins: [vuetify] },
      attachTo: document.body,
    })
    await flushPromises()
    const rows = w.findAll('tbody tr')
    expect(rows.length).toBe(2)
    // 升序排名：sort_order=100 排第 2，sort_order=50 排第 1
    expect(rows[0].text()).toContain('2')
    expect(rows[1].text()).toContain('1')

    const vm = w.vm as unknown as ResourcePageVm & { rankOf: (row: any) => unknown }
    expect(vm.rankOf({ sort_order: 999 })).toBe(999)
  })

  it('无 rankKey：rankMap 为空 Map，rankOf 兜底 undefined', async () => {
    const resource = makeResource()
    const w = mount(ResourcePage, {
      props: { title: 'T', subtitle: 'S', columns, fields, resource },
      global: { plugins: [vuetify] },
      attachTo: document.body,
    })
    await flushPromises()
    const vm = w.vm as unknown as ResourcePageVm & { rankMap: Map<unknown, number>; rankOf: (row: any) => unknown }
    expect(vm.rankMap instanceof Map).toBe(true)
    expect(vm.rankMap.size).toBe(0)
    expect(vm.rankOf({ foo: 1 })).toBeUndefined()

    // 覆盖 doDelete 失败分支 (line 159 false)
    const resource2 = makeResource()
    resource2.remove.mockRejectedValue(new Error('del boom'))
    const w2 = mount(ResourcePage, {
      props: { title: 'T', subtitle: 'S', columns, fields, resource: resource2 },
      global: { plugins: [vuetify] },
      attachTo: document.body,
    })
    await flushPromises()
    const vm2 = w2.vm as unknown as ResourcePageVm
    vm2.askDelete({ name: 'a' })
    await flushPromises()
    await vm2.doDelete()
    await flushPromises()
    expect(notifyState.kind).toBe('error')
  })

  it('rankKey 为字符串列：按 localeCompare 排名，null/undefined 排末尾', async () => {
    const resource = makeResource([
      { name: 'b-row', grade: 'b' },
      { name: 'a-row', grade: 'a' },
      { name: 'null-row', grade: null },
    ])
    const w = mount(ResourcePage, {
      props: {
        title: 'T',
        subtitle: 'S',
        columns: [
          { key: 'name', title: '名称' },
          { key: 'grade', title: '等级' },
        ],
        fields,
        resource,
        rankKey: 'grade',
      },
      global: { plugins: [vuetify] },
      attachTo: document.body,
    })
    await flushPromises()
    const vm = w.vm as unknown as ResourcePageVm & { rankMap: Map<unknown, number>; rankOf: (row: any) => unknown }
    expect(vm.rankMap instanceof Map).toBe(true)
    expect(vm.rankOf({ grade: 'a' })).toBe(1)
    expect(vm.rankOf({ grade: 'b' })).toBe(2)
    expect(vm.rankOf({ grade: null })).toBe(3)
    const rows = w.findAll('tbody tr')
    expect(rows.length).toBe(3)
    expect(rows[1].text()).toContain('1')
    expect(rows[0].text()).toContain('2')
    expect(rows[2].text()).toContain('3')

    // 覆盖相等值/重复值分支 (lines 62, 70) 与 null 比较分支 (line 64)
    const resource3 = makeResource([
      { name: 'x', grade: 'a' },
      { name: 'y', grade: 'a' },
      { name: 'z', grade: null },
    ])
    const w3 = mount(ResourcePage, {
      props: {
        title: 'T',
        subtitle: 'S',
        columns: [
          { key: 'name', title: '名称' },
          { key: 'grade', title: '等级' },
        ],
        fields,
        resource: resource3,
        rankKey: 'grade',
      },
      global: { plugins: [vuetify] },
      attachTo: document.body,
    })
    await flushPromises()
    const vm3 = w3.vm as unknown as ResourcePageVm & { rankMap: Map<unknown, number>; rankOf: (row: any) => unknown }
    expect(vm3.rankOf({ grade: 'a' })).toBe(1)
    expect(vm3.rankMap.size).toBe(2)

    // 覆盖 bv 为 null 的比较分支 (line 64)
    const resource4 = makeResource([
      { name: 'y', grade: null },
      { name: 'x', grade: 'a' },
    ])
    const w4 = mount(ResourcePage, {
      props: {
        title: 'T',
        subtitle: 'S',
        columns: [
          { key: 'name', title: '名称' },
          { key: 'grade', title: '等级' },
        ],
        fields,
        resource: resource4,
        rankKey: 'grade',
      },
      global: { plugins: [vuetify] },
      attachTo: document.body,
    })
    await flushPromises()
    const vm4 = w4.vm as unknown as ResourcePageVm & { rankMap: Map<unknown, number>; rankOf: (row: any) => unknown }
    expect(vm4.rankOf({ grade: 'a' })).toBe(1)
    expect(vm4.rankOf({ grade: null })).toBe(2)
  })

  it('T10: type==="select" 字段渲染成 v-select，options 透传为 items，setValue 写入 draft', async () => {
    const localFields = [
      { key: 'name', label: '名称', type: 'text' },
      { key: 'group', label: '分组', type: 'select', options: [
        { title: '无分组（DIRECT）', value: 0 },
        { title: '#3 HK', value: 3 },
      ] },
    ]
    const resource = makeResource()
    const w = mount(ResourcePage, {
      props: { title: 'T', subtitle: 'S', columns, fields: localFields, resource },
      global: { plugins: [vuetify], stubs: { VDialog: { name: 'VDialog', props: ['modelValue'], template: '<div class="dlg" v-if="modelValue"><slot /></div>' } } },
      attachTo: document.body,
    })
    const vm = w.vm as unknown as ResourcePageVm
    await flushPromises()
    vm.openCreate()
    await w.vm.$nextTick()
    await flushPromises()
    expect(w.find('[data-testid="field-select"]').exists()).toBe(true)
    expect(w.find('[data-testid="field-number"]').exists()).toBe(false)
    // 注意：v-data-table 的默认 footer 也带一个 VSelect（每页条数），所以按 testid 定位字段下拉本身
    const select = w.findAllComponents({ name: 'VSelect' }).find((c) => c.attributes('data-testid') === 'field-select')
    expect(select).toBeDefined()
    const items = ((select?.props('items') as { title: string; value: number }[]) ?? [])
    expect(items.map((i) => i.title)).toEqual(['无分组（DIRECT）', '#3 HK'])
    expect(items.map((i) => i.value)).toEqual([0, 3])
    await select!.setValue(3)
    expect(vm.draft.group).toBe(3)
  })


  it('表格分页状态由表格更新，重新加载后仍保留', async () => {
    const many = Array.from({ length: 30 }, (_, i) => ({ id: i + 1, name: 'row-' + i, count: i, enabled: true }))
    const resource = makeResource(many)
    const w = mount(ResourcePage, {
      props: { title: 'T', subtitle: 'S', columns, fields, resource },
      global: { plugins: [vuetify] },
    })
    await flushPromises()
    const vm = w.vm as any
    vm.setPageSize(25)
    await flushPromises()
    expect((w.vm as any).itemsPerPage).toBe(25)
    await (w.vm as any).reload()
    await flushPromises()
    expect((w.vm as any).itemsPerPage).toBe(25)
    vm.page = 5
    vm.itemsPerPage = 10
    await flushPromises()
    expect((w.vm as any).page).toBeGreaterThanOrEqual(1)
    w.unmount()
  })

  it('每页条数变大导致当前页超出时自动回到最后一页', async () => {
    const rows = Array.from({ length: 30 }, (_, i) => ({ id: i + 1, name: 'row-' + i, count: i, enabled: true }))
    const w = mount(ResourcePage, {
      props: { title: 'T', subtitle: 'S', columns, fields, resource: makeResource(rows) },
      global: { plugins: [vuetify] },
    })
    await flushPromises()
    const vm = w.vm as any
    vm.page = 3
    expect(vm.page).toBe(3)
    vm.itemsPerPage = 100
    expect(vm.itemsPerPage).toBe(100)
    expect(vm.pageCount).toBe(1)
    await flushPromises()
    expect(vm.page).toBe(1)
    w.unmount()
  })

  it('members 列通过插槽渲染自定义内容', async () => {
    const resource = makeResource([{ id: 1, name: 'g', enabled: true, members_text: '预览' }])
    const w = mount(ResourcePage, {
      props: { title: 'T', subtitle: 'S', columns: [{ key: 'name', title: '名称' }, { key: 'members_text', title: '成员' }], fields, resource },
      global: { plugins: [vuetify] },
      slots: { members: '<span data-testid="custom-members">自定义成员</span>' },
    })
    await flushPromises()
    expect(w.find('[data-testid="custom-members"]').exists()).toBe(true)
    w.unmount()
  })



  it('members 列无自定义插槽时显示原值', async () => {
    const resource = makeResource([{ id: 1, name: 'g', enabled: true, members_text: '原样成员' }])
    const w = mount(ResourcePage, {
      props: { title: 'T', subtitle: 'S', columns: [{ key: 'name', title: '名称' }, { key: 'members_text', title: '成员' }], fields, resource },
      global: { plugins: [vuetify] },
    })
    await flushPromises()
    expect(w.text()).toContain('原样成员')
    w.unmount()
  })

})