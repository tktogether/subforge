import { ref, type Ref } from 'vue'
import { errorMessage } from './useAsync'

export interface CrudResource<T> {
  list: () => Promise<T[]>
  create: (item: T) => Promise<unknown>
  update: (item: T) => Promise<unknown>
  remove: (item: T) => Promise<unknown>
}

export interface CrudState<T> {
  items: Ref<T[]>
  error: Ref<string>
  loading: Ref<boolean>
  load: () => Promise<void>
  save: (draft: T, existing: T | null) => Promise<boolean>
  destroy: (item: T) => Promise<boolean>
}

export function useCrud<T>(resource: CrudResource<T>): CrudState<T> {
  const items = ref<T[]>([]) as Ref<T[]>
  const error = ref('')
  const loading = ref(false)

  async function load(): Promise<void> {
    loading.value = true
    error.value = ''
    try {
      items.value = await resource.list()
    } catch (err) {
      error.value = errorMessage(err)
    } finally {
      loading.value = false
    }
  }

  async function save(draft: T, existing: T | null): Promise<boolean> {
    try {
      if (existing === null) {
        await resource.create(draft)
      } else {
        await resource.update(draft)
      }
      await load()
      return true
    } catch (err) {
      error.value = errorMessage(err)
      return false
    }
  }

  async function destroy(item: T): Promise<boolean> {
    try {
      await resource.remove(item)
      await load()
      return true
    } catch (err) {
      error.value = errorMessage(err)
      return false
    }
  }

  return { items, error, loading, load, save, destroy }
}
