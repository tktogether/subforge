import { ref, type Ref } from 'vue'

export function errorMessage(err: unknown): string {
  if (err instanceof Error) {
    return err.message
  }
  return String(err)
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
