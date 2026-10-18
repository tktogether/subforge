import { describe, it, expect, vi } from 'vitest'
import { useCrud } from './useCrud'

describe('useCrud', () => {
  it('load 成功：pending 时 loading 为 true，await 后 items/loading/error 更新', async () => {
    let resolveList!: (v: any[]) => void
    const pending = new Promise<any[]>((res) => {
      resolveList = res
    })
    const resource = {
      list: vi.fn().mockReturnValue(pending),
      create: vi.fn(),
      update: vi.fn(),
      remove: vi.fn(),
    }
    const crud = useCrud<any>(resource)
    const p = crud.load()
    expect(crud.loading.value).toBe(true)
    expect(crud.error.value).toBe('')
    resolveList([{ id: 1 }])
    await p
    expect(crud.items.value).toEqual([{ id: 1 }])
    expect(crud.loading.value).toBe(false)
    expect(crud.error.value).toBe('')
  })

  it('load 失败：error 为错误消息，loading 为 false', async () => {
    const resource = {
      list: vi.fn().mockRejectedValue(new Error('boom')),
      create: vi.fn(),
      update: vi.fn(),
      remove: vi.fn(),
    }
    const crud = useCrud<any>(resource)
    await crud.load()
    expect(crud.error.value).toBe('boom')
    expect(crud.loading.value).toBe(false)
  })

  it('save(draft, null)：调用 create，随后 list，返回 true', async () => {
    const resource = {
      list: vi.fn().mockResolvedValue([{ id: 2 }]),
      create: vi.fn().mockResolvedValue(undefined),
      update: vi.fn(),
      remove: vi.fn(),
    }
    const crud = useCrud<any>(resource)
    const ok = await crud.save({ id: 2 }, null)
    expect(ok).toBe(true)
    expect(resource.create).toHaveBeenCalledWith({ id: 2 })
    expect(resource.list).toHaveBeenCalled()
    expect(crud.items.value).toEqual([{ id: 2 }])
  })

  it('save(draft, existing)：调用 update 不调用 create，随后 list，返回 true', async () => {
    const resource = {
      list: vi.fn().mockResolvedValue([{ id: 3 }]),
      create: vi.fn(),
      update: vi.fn().mockResolvedValue(undefined),
      remove: vi.fn(),
    }
    const crud = useCrud<any>(resource)
    const ok = await crud.save({ id: 3 }, { id: 1 })
    expect(ok).toBe(true)
    expect(resource.update).toHaveBeenCalledWith({ id: 3 })
    expect(resource.create).not.toHaveBeenCalled()
    expect(resource.list).toHaveBeenCalled()
  })

  it('save 失败（create 抛错）：error 为错误消息，返回 false', async () => {
    const resource = {
      list: vi.fn(),
      create: vi.fn().mockRejectedValue(new Error('create failed')),
      update: vi.fn(),
      remove: vi.fn(),
    }
    const crud = useCrud<any>(resource)
    const ok = await crud.save({ id: 4 }, null)
    expect(ok).toBe(false)
    expect(crud.error.value).toBe('create failed')
  })

  it('destroy 成功：调用 remove，随后 list，返回 true', async () => {
    const resource = {
      list: vi.fn().mockResolvedValue([]),
      create: vi.fn(),
      update: vi.fn(),
      remove: vi.fn().mockResolvedValue(undefined),
    }
    const crud = useCrud<any>(resource)
    const ok = await crud.destroy({ id: 5 })
    expect(ok).toBe(true)
    expect(resource.remove).toHaveBeenCalledWith({ id: 5 })
    expect(resource.list).toHaveBeenCalled()
    expect(crud.items.value).toEqual([])
  })

  it('destroy 失败：error 为错误消息，返回 false', async () => {
    const resource = {
      list: vi.fn(),
      create: vi.fn(),
      update: vi.fn(),
      remove: vi.fn().mockRejectedValue(new Error('remove failed')),
    }
    const crud = useCrud<any>(resource)
    const ok = await crud.destroy({ id: 6 })
    expect(ok).toBe(false)
    expect(crud.error.value).toBe('remove failed')
  })
})
