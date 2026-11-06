import { reactive } from 'vue'

export interface ConfirmOptions {
  title: string
  body?: string
  confirmText?: string
  cancelText?: string
  danger?: boolean
}

interface ConfirmState {
  open: boolean
  options: ConfirmOptions | null
  resolve: ((value: boolean) => void) | null
}

export const confirmState = reactive<ConfirmState>({ open: false, options: null, resolve: null })

// confirmAction 弹出确认框并返回 Promise：true=确认，false=取消/关闭。
export function confirmAction(options: ConfirmOptions): Promise<boolean> {
  return new Promise((resolve) => {
    confirmState.options = options
    confirmState.open = true
    confirmState.resolve = resolve
  })
}

export function settleConfirm(value: boolean): void {
  confirmState.open = false
  confirmState.resolve?.(value)
  confirmState.resolve = null
}
