import { describe, it, expect } from 'vitest'
import { PAGE_HELP, pageHelp, GLOSSARY, glossaryFor, FIELD_HELP, fieldHelp, SCRIPTS_HELP, PIPELINE_HELP, FAILURE_HELP, ON_ERROR_HELP } from './help'

const keys = ['overview','convert','pipelines','scripts','airports','self-nodes','rules','rule-sources','groups','subscriptions','tasks','settings','logs']

describe('help.ts PAGE_HELP', () => {
  it('恰好 13 个 key', () => {
    expect(Object.keys(PAGE_HELP).sort()).toEqual(keys.slice().sort())
  })
  for (const k of keys) {
    it(k + ' 填满字段', () => {
      const h = PAGE_HELP[k]
      expect(h.oneLiner).not.toBe('')
      expect(h.whenToUse).not.toBe('')
      expect(Array.isArray(h.steps) && h.steps!.length >= 2).toBe(true)
      expect(Array.isArray(h.concepts) && h.concepts!.length >= 2).toBe(true)
      expect(h.emptyState).toBeDefined()
      expect(Array.isArray(h.links)).toBe(true)
    })
  }
  it('pageHelp 未知 key 返回兜底且不抛', () => {
    const h = pageHelp('nope-unknown')
    expect(h.title).toBe('nope-unknown')
    expect(h.oneLiner).toBe('该页面暂无说明')
    expect(h.whenToUse).toBe('')
    expect(h.steps).toEqual([])
    expect(h.concepts).toEqual([])
    expect(h.links).toEqual([])
  })
  it('pageHelp 已知 key 返回对应', () => {
    expect(pageHelp('overview').page).toBe('overview')
  })
})

describe('glossaryFor', () => {
  it('命中返回 explain', () => {
    expect(glossaryFor('节点')).not.toBe('')
  })
  it('未命中返回空串', () => {
    expect(glossaryFor('不存在的术语')).toBe('')
  })
})

describe('fieldHelp', () => {
  it('命中返回对象', () => {
    const r = fieldHelp('airport.name')
    expect(r).toBeDefined()
    expect(r!.hint).not.toBe('')
  })
  it('未命中返回 undefined', () => {
    expect(fieldHelp('nope.field')).toBeUndefined()
  })
})

describe('GLOSSARY / FIELD_HELP / SCRIPTS / PIPELINE', () => {
  it('GLOSSARY 覆盖核心术语', () => {
    const terms = GLOSSARY.map((g) => g.term)
    for (const t of ['节点','订阅','机场','规则','规则集','规则源','管线','转换','令牌','subscription-userinfo','MRS','ETag','测活']) {
      expect(terms).toContain(t)
    }
  })
  it('FIELD_HELP 覆盖 airport/group/rule/token/self-node/script/pipeline', () => {
    expect(Object.keys(FIELD_HELP).length).toBeGreaterThan(0)
    expect(fieldHelp('airport.subscription_url')).toBeDefined()
    expect(fieldHelp('group.regex')).toBeDefined()
    expect(fieldHelp('rule.rule_type')).toBeDefined()
    expect(fieldHelp('token.name')).toBeDefined()
    expect(fieldHelp('self-node.link')).toBeDefined()
    expect(fieldHelp('script.code')).toBeDefined()
    expect(fieldHelp('pipeline.name')).toBeDefined()
  })
  it('SCRIPTS_HELP 变量表逐字覆盖 6 行，node.port 是 string、node.source 只读', () => {
    expect(SCRIPTS_HELP.variables.length).toBe(6)
    const names = SCRIPTS_HELP.variables.map((v) => v.name)
    expect(names).toEqual(['node.name','node.type','node.server','node.port','node.source','node.raw'])
    const port = SCRIPTS_HELP.variables.find((v) => v.name === 'node.port')!
    expect(port.type).toBe('string')
    const source = SCRIPTS_HELP.variables.find((v) => v.name === 'node.source')!
    expect(source.writable).toBe(false)
  })
  it('SCRIPTS_HELP returnRules 有 filter 与 operate 各一条', () => {
    expect(SCRIPTS_HELP.returnRules.length).toBe(2)
    expect(SCRIPTS_HELP.returnRules[0].kind).toBe('filter')
    expect(SCRIPTS_HELP.returnRules[1].kind).toBe('operate')
  })
  it('SCRIPTS_HELP writeback 至少 3 条', () => {
    expect(SCRIPTS_HELP.writeback.length).toBeGreaterThanOrEqual(3)
  })
  it('SCRIPTS_HELP sandbox 至少 4 条', () => {
    expect(SCRIPTS_HELP.sandbox.length).toBeGreaterThanOrEqual(4)
  })
  it('SCRIPTS_HELP 4 个示例代码冻结', () => {
    expect(SCRIPTS_HELP.examples.length).toBe(4)
    expect(SCRIPTS_HELP.examples[0].code).toBe("node.name.indexOf('香港') >= 0 || node.name.indexOf('HK') >= 0")
    expect(SCRIPTS_HELP.examples[1].code).toBe("node.name = '[HK] ' + node.name")
    expect(SCRIPTS_HELP.examples[2].code).toBe('node.port = 443')
    expect(SCRIPTS_HELP.examples[3].code).toBe("if (node.raw.sni) { node.raw.sni = 'example.com' }")
  })
  it('PIPELINE_HELP steps 恰好 10 个真实 kind 且有 scriptStep', () => {
    const kinds = PIPELINE_HELP.steps.map((s) => s.kind)
    expect(kinds).toEqual(['filter.regex','filter.region','filter.type','filter.script','op.rename','op.emoji','op.dedupe','op.split','op.sort','op.script'])
    expect(PIPELINE_HELP.order.length).toBeGreaterThan(0)
    expect(PIPELINE_HELP.scriptStep).not.toBe('')
  })
  it('faq 至少 8 页', () => {
    const withFaq = Object.values(PAGE_HELP).filter((p) => Array.isArray(p.faq) && p.faq!.length > 0)
    expect(withFaq.length).toBeGreaterThanOrEqual(8)
  })
})


describe('FAILURE_HELP / ON_ERROR_HELP', () => {
  it('FAILURE_HELP 有 intro 与至少 11 条 cases', () => {
    expect(FAILURE_HELP.intro).not.toBe('')
    expect(FAILURE_HELP.cases.length).toBeGreaterThanOrEqual(11)
    for (const c of FAILURE_HELP.cases) {
      expect(c.scene).not.toBe('')
      expect(c.behavior).not.toBe('')
      expect(c.ui).not.toBe('')
    }
  })
  it('ON_ERROR_HELP 恰好 3 条 skip/abort/warn', () => {
    expect(ON_ERROR_HELP.length).toBe(3)
    expect(ON_ERROR_HELP.map((o) => o.value)).toEqual(['skip', 'abort', 'warn'])
    expect(ON_ERROR_HELP[0].effect).toBe('跳过该步，保留已处理结果，产物继续生成')
    expect(ON_ERROR_HELP[1].effect).toBe('中止整次转换并报错')
    expect(ON_ERROR_HELP[2].effect).toBe('记录告警，继续执行后续步骤')
  })
  it('FIELD_HELP 增补三个键', () => {
    expect(fieldHelp('airport.traffic_err')?.hint).toContain('错误原文')
    expect(fieldHelp('airport.traffic_at')?.hint).toContain('Unix 秒')
    expect(fieldHelp('pipeline.step.on_error')?.hint).toContain('skip')
  })
  it('airports/convert/rule-sources/tasks faq 含失败相关问答', () => {
    expect(PAGE_HELP.airports.faq?.some((f) => f.q.includes('余额'))).toBe(true)
    expect(PAGE_HELP.convert.faq?.some((f) => f.q.includes('拉不到'))).toBe(true)
    expect(PAGE_HELP['rule-sources'].faq?.some((f) => f.q.includes('变空'))).toBe(true)
    expect(PAGE_HELP.tasks.faq?.some((f) => f.q.includes('重试'))).toBe(true)
  })
})
