import { ref, type Ref } from 'vue'
import { HttpError } from '../api/client'

export function errorMessage(err: unknown): string {
  if (err instanceof Error) {
    return err.message
  }
  return String(err)
}

// errorDetail 取出 HttpError 的可折叠原始报文（例如服务端内部错误）供 AsyncSection
// 的折叠块展示。其他错误没有详情，返回空串，界面就不显示折叠块。
export function errorDetail(err: unknown): string {
  if (err instanceof HttpError) {
    return err.detail
  }
  return ''
}

export interface AsyncState<T> {
  data: Ref<T | null>
  error: Ref<string>
  loading: Ref<boolean>
  load: () => Promise<void>
}

export function useAsync<T>(loader: () => Promise<T>): AsyncState<T> {
  const data = ref<T | null>(null) as Ref<T | null>
  const error = ref('')
  const loading = ref(false)

  async function load(): Promise<void> {
    loading.value = true
    error.value = ''
    try {
      data.value = await loader()
    } catch (err) {
      error.value = errorMessage(err)
    } finally {
      loading.value = false
    }
  }

  return { data, error, loading, load }
}
