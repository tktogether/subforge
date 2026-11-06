import { describe, it, expect } from 'vitest'
import { stepArgRef } from './pipelineStepArgs'

describe('stepArgRef', () => {
  it('字符串原样返回', () => {
    expect(stepArgRef('abc')).toBe('abc')
  })

  it('null → 空串且不抛异常', () => {
    expect(() => stepArgRef(null)).not.toThrow()
    expect(stepArgRef(null)).toBe('')
  })

  it('undefined → 空串且不抛异常', () => {
    expect(() => stepArgRef(undefined)).not.toThrow()
    expect(stepArgRef(undefined)).toBe('')
  })

  it('对象含 value 为字符串 → 取 value', () => {
    expect(stepArgRef({ value: 'x' })).toBe('x')
  })

  it('对象含 value 为 null → 空串', () => {
    expect(stepArgRef({ value: null })).toBe('')
  })

  it('对象含 value 为数字 → String(value)', () => {
    expect(stepArgRef({ value: 123 })).toBe('123')
  })

  it('裸数字 → 空串（不是对象形态，不取 .value）', () => {
    expect(() => stepArgRef(123)).not.toThrow()
    expect(stepArgRef(123)).toBe('')
  })

  it('其它异常输入不抛异常', () => {
    expect(() => stepArgRef({})).not.toThrow()
    expect(stepArgRef({})).toBe('')
    expect(() => stepArgRef({ text: 'x' })).not.toThrow()
    expect(stepArgRef({ text: 'x' })).toBe('')
    expect(() => stepArgRef([])).not.toThrow()
    expect(stepArgRef([])).toBe('')
    expect(() => stepArgRef(true)).not.toThrow()
    expect(stepArgRef(true)).toBe('')
  })
})
