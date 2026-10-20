import { describe, it, expect, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import TokenDialog from './TokenDialog.vue'
import { TOKEN_STORAGE_KEY, tokenRejected, tokenState } from '../api/token'

const vuetify = createVuetify({ components, directives })

// VDialog 的内容会 teleport 到 document.body，所以按仓库既有做法：挂到 body 上，
// 再用 document.body.querySelectorAll 找按钮，而不是用 wrapper.find。
function mountDialog(open = false) {
  return mount(
    {
      components: { TokenDialog },
      template: '<TokenDialog v-model="open" @saved="onSaved" />',
      data() {
        return { open }
      },
      methods: { onSaved() {} },
    },
    { global: { plugins: [vuetify] }, attachTo: document.body },
  )
}

function dialogOf(w: ReturnType<typeof mountDialog>): any {
  return w.findComponent(TokenDialog).vm as any
}

async function openDialog(w: ReturnType<typeof mountDialog>): Promise<void> {
  w.findComponent({ name: 'VDialog' }).vm.$emit('update:modelValue', true)
  await flushPromises()
}

function button(label: string): HTMLButtonElement {
  const found = Array.from(document.body.querySelectorAll('button')).find(
    (b) => (b.textContent ?? '').trim() === label,
  )
  if (found === undefined) {
    throw new Error('找不到按钮：' + label)
  }
  return found
}

function input(): HTMLInputElement {
  const found = document.body.querySelector('[data-testid="token-input"] input')
  if (found === null) {
    throw new Error('找不到令牌输入框')
  }
  return found as HTMLInputElement
}

describe('TokenDialog', () => {
  beforeEach(() => {
    window.localStorage.clear()
    tokenState.value = ''
    tokenRejected.value = false
    document.body.innerHTML = ''
    // VDialog 的定位策略会读 visualViewport，jsdom 没有实现（与 AppShell.spec 同款桩）。
    ;(globalThis as any).visualViewport ??= {
      addEventListener: () => {},
      removeEventListener: () => {},
      width: 1024,
      height: 768,
    }
  })

  it('打开时把输入框预填成当前令牌，并默认隐藏明文', async () => {
    tokenState.value = 'existing'
    const w = mountDialog()
    await openDialog(w)
    expect(dialogOf(w).draft).toBe('existing')
    expect(dialogOf(w).reveal).toBe(false)
    expect(input().type).toBe('password')
  })

  it('关闭状态下不预填（避免把上一次的草稿带进来）', () => {
    tokenState.value = 'existing'
    const w = mountDialog(false)
    expect(dialogOf(w).draft).toBe('')
  })

  it('点击保存：写入 localStorage、发出 saved、关闭对话框', async () => {
    const w = mountDialog()
    await openDialog(w)
    await w.findComponent({ name: 'VTextField' }).vm.$emit('update:modelValue', '  tok-9  ')
    button('保存').click()
    await flushPromises()
    expect(window.localStorage.getItem(TOKEN_STORAGE_KEY)).toBe('tok-9')
    expect(tokenState.value).toBe('tok-9')
    expect(w.findComponent(TokenDialog).emitted('saved')).toBeTruthy()
    expect(dialogOf(w).open).toBe(false)
  })

  it('点击清除：只清空输入框，不写存储（仍需点保存才生效）', async () => {
    tokenState.value = 'existing'
    const w = mountDialog()
    await openDialog(w)
    button('清除').click()
    await flushPromises()
    expect(dialogOf(w).draft).toBe('')
    expect(window.localStorage.getItem(TOKEN_STORAGE_KEY)).toBeNull()
    expect(tokenState.value).toBe('existing')
  })

  it('点击取消：只关闭对话框，不改动已保存的令牌', async () => {
    tokenState.value = 'existing'
    const w = mountDialog()
    await openDialog(w)
    await w.findComponent({ name: 'VTextField' }).vm.$emit('update:modelValue', 'typed-but-cancelled')
    button('取消').click()
    await flushPromises()
    expect(dialogOf(w).open).toBe(false)
    expect(tokenState.value).toBe('existing')
    expect(window.localStorage.getItem(TOKEN_STORAGE_KEY)).toBeNull()
  })

  it('点击眼睛图标在密码与明文之间切换', async () => {
    const w = mountDialog()
    await openDialog(w)
    await w.findComponent({ name: 'VTextField' }).vm.$emit('click:appendInner')
    await flushPromises()
    expect(dialogOf(w).reveal).toBe(true)
    expect(input().type).toBe('text')
  })

  it('tokenRejected 为 true 时渲染 401 提示', async () => {
    tokenRejected.value = true
    const w = mountDialog()
    await openDialog(w)
    expect(document.body.innerHTML).toContain('服务端拒绝了上一次请求')
  })

  it('tokenRejected 为 false 时不渲染 401 提示', async () => {
    tokenRejected.value = false
    const w = mountDialog()
    await openDialog(w)
    expect(document.body.innerHTML).not.toContain('服务端拒绝了上一次请求')
  })
})
