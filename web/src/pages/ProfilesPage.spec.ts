import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import ProfilesPage from './ProfilesPage.vue'
import { endpoints } from '../api/endpoints'
import { notifyState } from '../composables/useNotify'

// 端点层用 mock 隔离，页面 spec 只关心页面自己的行为。
vi.mock('../api/endpoints', () => ({
  endpoints: {
    listProfiles: vi.fn(),
    createProfile: vi.fn(),
    updateProfile: vi.fn(),
    deleteProfile: vi.fn(),
  },
}))

const vuetify = createVuetify({ components, directives })

interface Draft {
  name: string
  description: string
  config: string
  pipeline: string
  enabled: boolean
  sort_order: number
}

interface ProfilesPageVm {
  dialog: boolean
  confirmOpen: boolean
  draft: Draft
}

function makeProfile(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    name: '订阅A',
    description: '说明A',
    config: '',
    pipeline: '',
    enabled: true,
    sort_order: 3,
    rule_count: 2,
    group_count: 1,
    ...overrides,
  }
}

// v-dialog 的内容默认惰性渲染，测试里换成内联 stub 让弹窗内容直接可查。
function mountPage() {
  return mount(ProfilesPage, {
    global: {
      plugins: [vuetify],
      stubs: {
        VDialog: {
          name: 'VDialog',
          props: ['modelValue'],
          template: '<div class="dlg" v-if="modelValue"><slot /></div>',
        },
      },
    },
  })
}

describe('ProfilesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    notifyState.open = false
    notifyState.text = ''
    notifyState.kind = 'ok'
    ;(endpoints.listProfiles as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([])
  })

  it('渲染方案表格：模板/管线空值显示默认、模板超长截断、启用停用与计数', async () => {
    const longConfig = 'x'.repeat(80)
    ;(endpoints.listProfiles as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([
      makeProfile(),
      makeProfile({ id: 2, name: '订阅B', config: longConfig, pipeline: 'pipeB', enabled: false, sort_order: 7, rule_count: 5, group_count: 4 }),
    ])
    const w = mountPage()
    await flushPromises()

    expect(w.find('[data-testid="profile-table"]').exists()).toBe(true)
    expect(w.findAll('[data-testid="profile-row"]')).toHaveLength(2)

    const cells = w.findAll('.profiles-truncate')
    expect(cells[0].text()).toBe('默认')
    expect(cells[0].attributes('title')).toBe('')
    expect(cells[1].text()).toBe(longConfig.slice(0, 60) + '…')
    expect(cells[1].attributes('title')).toBe(longConfig)

    expect(w.text()).toContain('默认')
    expect(w.text()).toContain('启用')
    expect(w.text()).toContain('停用')
    expect(w.text()).toContain('pipeB')
    expect(w.text()).toContain('7')
  })

  it('帮助文案说明未分配、绑定隔离、删除前置条件与模板默认', async () => {
    const w = mountPage()
    await flushPromises()
    const help = w.find('[data-testid="profiles-help"]')
    expect(help.exists()).toBe(true)
    expect(help.text()).toContain('未分配')
    expect(help.text()).toContain('profile_id = 0')
    expect(help.text()).toContain('绑定方案后')
    expect(help.text()).toContain('删除方案前要先把规则/分组移走或删除')
    expect(help.text()).toContain('模板留空')
  })

  it('空态：没有方案时显示还没有订阅方案且不渲染表格', async () => {
    const w = mountPage()
    await flushPromises()
    expect(w.text()).toContain('还没有订阅方案')
    expect(w.find('[data-testid="profile-table"]').exists()).toBe(false)
  })

  it('新建方案：填字段后保存调用 createProfile 并提示已保存', async () => {
    ;(endpoints.createProfile as unknown as ReturnType<typeof vi.fn>).mockResolvedValue(makeProfile({ id: 5 }))
    const w = mountPage()
    await flushPromises()
    const vm = w.vm as unknown as ProfilesPageVm

    await w.find('[data-testid="profile-new"]').trigger('click')
    await flushPromises()
    expect(vm.dialog).toBe(true)
    expect(w.text()).toContain('新建方案')

    await w.find('[data-testid="profile-name"]').find('input').setValue('新方案')
    await w.find('[data-testid="profile-description"]').find('input').setValue('新说明')
    await w.find('[data-testid="profile-config"]').find('input').setValue('cfg')
    await w.find('[data-testid="profile-pipeline"]').find('input').setValue('pipe')
    const sw = w.findAllComponents({ name: 'VSwitch' })
    await sw[0].setValue(false)
    await w.find('[data-testid="profile-sort"]').find('input').setValue('9')
    await flushPromises()

    await w.find('[data-testid="profile-save"]').trigger('click')
    await flushPromises()

    expect(endpoints.createProfile).toHaveBeenCalledWith({
      name: '新方案',
      description: '新说明',
      config: 'cfg',
      pipeline: 'pipe',
      enabled: false,
      sort_order: 9,
    })
    expect(notifyState.kind).toBe('ok')
    expect(notifyState.text).toBe('已保存')
    expect(vm.dialog).toBe(false)
  })

  it('编辑方案：回填已有字段，保存调用 updateProfile', async () => {
    ;(endpoints.listProfiles as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([makeProfile({ config: 'old-cfg', pipeline: 'old-pipe' })])
    ;(endpoints.updateProfile as unknown as ReturnType<typeof vi.fn>).mockResolvedValue(makeProfile())
    const w = mountPage()
    await flushPromises()

    await w.find('[data-testid="profile-edit"]').trigger('click')
    await flushPromises()
    expect(w.text()).toContain('编辑方案')
    const vm = w.vm as unknown as ProfilesPageVm
    expect(vm.draft.name).toBe('订阅A')
    expect(vm.draft.description).toBe('说明A')
    expect(vm.draft.config).toBe('old-cfg')
    expect(vm.draft.pipeline).toBe('old-pipe')
    expect(vm.draft.enabled).toBe(true)
    expect(vm.draft.sort_order).toBe(3)

    await w.find('[data-testid="profile-save"]').trigger('click')
    await flushPromises()

    expect(endpoints.updateProfile).toHaveBeenCalledWith(1, expect.objectContaining({ name: '订阅A', config: 'old-cfg' }))
    expect(notifyState.text).toBe('已保存')
  })

  it('取消按钮关闭新建弹窗', async () => {
    const w = mountPage()
    await flushPromises()
    const vm = w.vm as unknown as ProfilesPageVm
    await w.find('[data-testid="profile-new"]').trigger('click')
    await flushPromises()
    expect(vm.dialog).toBe(true)
    await w.find('[data-testid="profile-cancel"]').trigger('click')
    await flushPromises()
    expect(vm.dialog).toBe(false)
    expect(endpoints.createProfile).not.toHaveBeenCalled()
  })

  it('删除确认：先弹确认框，取消不删除，确认后删除并提示已删除', async () => {
    ;(endpoints.listProfiles as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([makeProfile()])
    ;(endpoints.deleteProfile as unknown as ReturnType<typeof vi.fn>).mockResolvedValue({ ok: true })
    const w = mountPage()
    await flushPromises()
    const vm = w.vm as unknown as ProfilesPageVm

    await w.find('[data-testid="profile-delete"]').trigger('click')
    await flushPromises()
    expect(vm.confirmOpen).toBe(true)
    expect(w.text()).toContain('确定继续')

    await w.find('[data-testid="profile-delete-cancel"]').trigger('click')
    await flushPromises()
    expect(vm.confirmOpen).toBe(false)
    expect(endpoints.deleteProfile).not.toHaveBeenCalled()

    await w.find('[data-testid="profile-delete"]').trigger('click')
    await flushPromises()
    await w.find('[data-testid="profile-delete-confirm"]').trigger('click')
    await flushPromises()
    expect(endpoints.deleteProfile).toHaveBeenCalledWith(1)
    expect(notifyState.kind).toBe('ok')
    expect(notifyState.text).toBe('已删除')
  })

  it('删除冲突 409：把后端错误文案以 error 形式提示', async () => {
    ;(endpoints.listProfiles as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([makeProfile()])
    ;(endpoints.deleteProfile as unknown as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('HTTP 409: 方案名称 “订阅A” 已存在'))
    const w = mountPage()
    await flushPromises()

    await w.find('[data-testid="profile-delete"]').trigger('click')
    await flushPromises()
    await w.find('[data-testid="profile-delete-confirm"]').trigger('click')
    await flushPromises()

    expect(notifyState.kind).toBe('error')
    expect(notifyState.text).toContain('已存在')
  })

  it('加载失败：显示错误并可重试', async () => {
    const list = endpoints.listProfiles as unknown as ReturnType<typeof vi.fn>
    list.mockRejectedValueOnce(new Error('方案列表加载失败')).mockResolvedValueOnce([makeProfile()])
    const w = mountPage()
    await flushPromises()

    expect(w.find('[data-testid="async-error"]').exists()).toBe(true)
    expect(w.text()).toContain('方案列表加载失败')

    await w.find('[data-testid="async-retry"]').trigger('click')
    await flushPromises()
    expect(list).toHaveBeenCalledTimes(2)
    expect(w.find('[data-testid="profile-row"]').exists()).toBe(true)
  })

  it('保存失败：提示错误文案', async () => {
    ;(endpoints.createProfile as unknown as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('保存失败'))
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="profile-new"]').trigger('click')
    await flushPromises()
    await w.find('[data-testid="profile-save"]').trigger('click')
    await flushPromises()

    expect(notifyState.kind).toBe('error')
    expect(notifyState.text).toContain('保存失败')
  })

  it('刷新按钮重新拉取方案列表', async () => {
    const list = endpoints.listProfiles as unknown as ReturnType<typeof vi.fn>
    const w = mountPage()
    await flushPromises()
    expect(list).toHaveBeenCalledTimes(1)
    await w.find('[data-testid="profile-refresh"]').trigger('click')
    await flushPromises()
    expect(list).toHaveBeenCalledTimes(2)
  })
  it('编辑方案保存失败提示错误', async () => {
    ;(endpoints.listProfiles as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([makeProfile()])
    ;(endpoints.updateProfile as unknown as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('更新失败'))
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="profile-edit"]').trigger('click')
    await flushPromises()
    await w.find('[data-testid="profile-save"]').trigger('click')
    await flushPromises()
    expect(notifyState.kind).toBe('error')
    expect(notifyState.text).toContain('更新失败')
  })

  it('删除成功后刷新列表', async () => {
    ;(endpoints.listProfiles as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([makeProfile()])
    ;(endpoints.deleteProfile as unknown as ReturnType<typeof vi.fn>).mockResolvedValue({ ok: true })
    const w = mountPage()
    await flushPromises()
    await w.find('[data-testid="profile-delete"]').trigger('click')
    await flushPromises()
    await w.find('[data-testid="profile-delete-confirm"]').trigger('click')
    await flushPromises()
    expect(endpoints.deleteProfile).toHaveBeenCalledWith(1)
    expect(endpoints.listProfiles).toHaveBeenCalledTimes(2)
  })

  it('关闭弹窗以覆盖 v-dialog v-model setter', async () => {
    const list = endpoints.listProfiles as unknown as ReturnType<typeof vi.fn>
    list.mockResolvedValue([makeProfile()])
    const w = mountPage()
    await flushPromises()
    // 打开新建弹窗，关闭以覆盖 v-model setter
    await w.find('[data-testid="profile-new"]').trigger('click')
    await flushPromises()
    for (const d of w.findAllComponents({ name: 'VDialog' })) {
      await (d.vm as { $emit: (e: string, ...a: unknown[]) => void }).$emit('update:modelValue', false)
    }
    await flushPromises()
    // 打开编辑弹窗，关闭以覆盖 v-model setter
    await w.find('[data-testid="profile-edit"]').trigger('click')
    await flushPromises()
    for (const d of w.findAllComponents({ name: 'VDialog' })) {
      await (d.vm as { $emit: (e: string, ...a: unknown[]) => void }).$emit('update:modelValue', false)
    }
    await flushPromises()
    // 打开删除确认弹窗，关闭以覆盖 v-model setter
    await w.find('[data-testid="profile-delete"]').trigger('click')
    await flushPromises()
    const dialogs = w.findAllComponents({ name: 'VDialog' })
    for (const d of dialogs) {
      await (d.vm as { $emit: (e: string, ...a: unknown[]) => void }).$emit('update:modelValue', false)
    }
    await flushPromises()
    expect(dialogs.length).toBeGreaterThan(0)
  })
})