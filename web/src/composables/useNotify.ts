import { reactive } from 'vue'

export type NotifyKind = 'ok' | 'error' | 'info'

export interface NotifyState {
  open: boolean
  text: string
  kind: NotifyKind
}

export const notifyState = reactive<NotifyState>({ open: false, text: '', kind: 'ok' })

export function notify(text: string, kind: NotifyKind = 'ok'): void {
  notifyState.text = text
  notifyState.kind = kind
  notifyState.open = true
}

export function closeNotify(): void {
  notifyState.open = false
}

export function snackbarColor(kind: NotifyKind): string {
  if (kind === 'error') {
    return 'error'
  }
  if (kind === 'info') {
    return 'info'
  }
  return 'success'
}
