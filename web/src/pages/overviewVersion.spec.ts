import { describe, it, expect } from 'vitest'
import { parseVersion } from './overviewVersion'

describe('parseVersion', () => {
  it('完整构建串拆出主版本与 commit/构建日期', () => {
    expect(parseVersion('subforge v0.6.0 (commit d16a2a7c, built 2026-11-06T01:42:01Z)')).toEqual({
      main: 'v0.6.0',
      meta: 'commit d16a2a7c · 2026-11-06',
    })
  })

  it('只有 commit 没有构建时间', () => {
    expect(parseVersion('subforge v0.6.0 (commit d16a2a7c)')).toEqual({
      main: 'v0.6.0',
      meta: 'commit d16a2a7c',
    })
  })

  it('只有版本号', () => {
    expect(parseVersion('v0.6.0')).toEqual({ main: 'v0.6.0', meta: '' })
    expect(parseVersion('subforge v0.6.0')).toEqual({ main: 'v0.6.0', meta: '' })
  })

  it('dev 构建显示为开发版且无元数据', () => {
    expect(parseVersion('subforge dev (commit none, built unknown)')).toEqual({ main: '开发版', meta: '' })
    expect(parseVersion('dev')).toEqual({ main: '开发版', meta: '' })
  })

  it('空串显示为开发版', () => {
    expect(parseVersion('   ')).toEqual({ main: '开发版', meta: '' })
  })

  it('无法解析的串原样去掉前缀', () => {
    expect(parseVersion('subforge custom-build !!!')).toEqual({ main: 'custom-build !!!', meta: '' })
  })
})
