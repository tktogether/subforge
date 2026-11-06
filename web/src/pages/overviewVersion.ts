export interface VersionParts {
  main: string
  meta: string
}

// 版本串形如 "subforge v0.6.0 (commit d16a2a7c, built 2026-11-06T01:42:01Z)"：
// 主值与构建元数据拆开渲染，窄屏卡片里才不会把长串挤成多行。
export function parseVersion(v: string): VersionParts {
  const s = v.trim()
  if (s === '' || /\bdev\b/i.test(s)) {
    return { main: '开发版', meta: '' }
  }
  const m = s.match(/^(?:subforge\s+)?(\S+?)(?:\s+\(commit\s+([^,)]+)(?:,\s*built\s+([^)]+))?\))?$/i)
  if (m === null) {
    return { main: s.replace(/^subforge\s+/i, ''), meta: '' }
  }
  const commit = m[2] === undefined ? '' : 'commit ' + m[2]
  const built = m[3] === undefined ? '' : m[3].replace(/T.*$/, '')
  return { main: m[1], meta: [commit, built].filter((part) => part !== '').join(' · ') }
}
