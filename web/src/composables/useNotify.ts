import { reactive } from 'vue'

export type NotifyKind = 'ok' | 'error' | 'info'

export interface NotifyState {
  open: boolean
  text: string
  kind: NotifyKind
  // seq 让「连续弹出同一条通知」也能重置计时并触发过渡。
  seq: number
}

export const notifyState = reactive<NotifyState>({ open: false, text: '', kind: 'ok', seq: 0 })

export function notify(text: string, kind: NotifyKind = 'ok'): void {
  notifyState.text = text
  notifyState.kind = kind
  notifyState.open = true
  notifyState.seq = notifyState.seq + 1
}

export function closeNotify(): void {
  notifyState.open = false
}
