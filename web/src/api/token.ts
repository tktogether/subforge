import { ref } from 'vue'
import { setTokenProvider, setUnauthorizedHandler } from './client'

// 访问令牌存在 localStorage：控制台的每个请求都要带它，刷新页面后不该要求重新输入。
export const TOKEN_STORAGE_KEY = 'subforge.token'

// tokenState 是当前生效的令牌；tokenDialogOpen 控制输入框是否显示；
// tokenRejected 记录最近一次请求是否被服务端拒绝（401），用于在输入框里给出提示。
export const tokenState = ref('')
export const tokenDialogOpen = ref(false)
export const tokenRejected = ref(false)

export function readStoredToken(): string {
  return window.localStorage.getItem(TOKEN_STORAGE_KEY) ?? ''
}

// saveToken 同时更新内存状态与 localStorage；空串表示清除令牌。
export function saveToken(token: string): void {
  const trimmed = token.trim()
  tokenState.value = trimmed
  tokenRejected.value = false
  if (trimmed === '') {
    window.localStorage.removeItem(TOKEN_STORAGE_KEY)
    return
  }
  window.localStorage.setItem(TOKEN_STORAGE_KEY, trimmed)
}

export function openTokenDialog(): void {
  tokenDialogOpen.value = true
}

export function closeTokenDialog(): void {
  tokenDialogOpen.value = false
}

// logout 清掉当前生效的令牌（内存 + localStorage）并复位 401 标记。
// 退出登录相当于 saveToken(''): 清空后所有请求都不再带 X-Token，
// 顶栏令牌按钮弹框也会跟着失效。
export function logout(): void {
  saveToken('')
}

// initToken 把令牌接到请求层：每个请求带 X-Token，收到 401 时自动弹出输入框。
//
// 服务端没设令牌时不会出现 401，输入框也就不会自己跳出来——用户仍可从顶栏手动打开。
export function initToken(): void {
  setTokenProvider(() => tokenState.value)
  setUnauthorizedHandler(() => {
    tokenRejected.value = true
    tokenDialogOpen.value = true
  })
  tokenState.value = readStoredToken()
}
