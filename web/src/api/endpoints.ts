import { api } from './client'

export interface Airport {
  id: number
  name: string
  subscription_url: string
  remarks: string
  enabled: boolean
  sort_order: number
  // 流量字段（可选，后端 internal/store/store.go Airport 返回）
  upload?: number // 字节
  download?: number // 字节
  total?: number // 字节
  expire?: number // unix 秒
  node_count?: number
  traffic_at?: number // unix 秒
  traffic_err?: string
}

export interface ProxyGroup {
  id: number
  name: string
  type: string
  regex: string
  url: string
  interval: number
  sort_order: number
  source?: string
  // 机场自带分组的成员；手工分组通常为空，按 regex 展开。
  members?: string[]
  // 订阅渲染时是否在成员末尾追加方案选中的自建节点。
  include_self?: boolean
  // 订阅渲染时是否把方案里所有机场的节点并入（机场「全部节点」组默认开启）。
  include_all_sources?: boolean
}

export interface SelfNode {
  id: number
  link: string
  name: string
  enabled: boolean
  sort_order: number
}

export interface Rule {
  id: number
  rule_type: string
  rule_param: string
  rule_config: number
  resolve_dns: boolean
  enabled: boolean
  sort_order: number
  source?: string
  raw?: string
  origin_key?: string
  group_name?: string
}

export interface RulePage {
  rules: Rule[]
  total: number
  limit: number
  offset: number
}

export interface RuleSourceCount {
  source: string
  count: number
}

export interface RuleSourceConflictSample {
  type: string
  param: string
  first: string
  second: string
}

export interface RuleSourceConflicts {
  total: number
  samples: RuleSourceConflictSample[]
}

export interface SubToken {
  token: string
  name: string
  // 字段级定制 5 列：后端 internal/store/store.go:245-254 的 json tag 逐字为
  // target/pipeline/config/include/exclude/created_at，且 SELECT 用 COALESCE 兜底，
  // 列表与详情必定返回这 6 个字段。这里声明为可选，是为了不被迫改写既有 spec 里的
  // 老 fixture（必填会让 SubscriptionsPage.spec.ts:87 / :355 的 toEqual 精确匹配断言失败）。
  target?: string
  pipeline?: string
  config?: string
  include?: string
  exclude?: string
  created_at?: string // RFC3339 字符串；后端 store.SubToken.CreatedAt json tag 为 created_at
  profile_id?: number
  profile_name?: string
}

// PUT /api/tokens/{token} 的 body：与后端 *string 语义对齐——字段缺失=不改动，空串=清空。
// 见 internal/api/subscription.go:415-426（6 个 *string 字段）。
export interface SubTokenUpdate {
  name?: string
  target?: string
  pipeline?: string
  config?: string
  include?: string
  exclude?: string
  profile_id?: number
}

export interface HistoryEntry {
  id: number
  at: string
  target: string
  format: string
  nodes: number
  rawNodes: number
  groups: number
  rules: number
  failures: string[] | null
  warnings: string[] | null
  cached: number
  urlCount: number
}

export interface LogEntry {
  id: number
  ts: string
  line: string
}

export interface StatsHistory {
  total: number
  cachedHits: number
  urlTotal: number
  cacheHitRate: number
}

export interface StatsLast {
  nodes: number
  rawNodes: number
  groups: number
  rules: number
  cached: number
  urlCount: number
  cacheHitRate: number
}

export interface Stats {
  history: StatsHistory
  logs: number
  last: StatsLast | null
}

export interface RulesetsResponse {
  rulesets: Record<string, string>
  rules: string[]
}

export interface LogsResponse {
  logs: LogEntry[]
}

export interface DiffLine {
  type: string
  text: string
}

export interface DiffResponse {
  lines: DiffLine[]
}

export interface ConvertRequest {
  target: string
  urls: string[]
  text: string
  types: string[]
  include: string[]
  exclude: string[]
  pipeline: string
  config: string
  sort: string
  list: boolean
  override: string
}

export interface ConvertResponse {
  body: string
  format: string
  nodes: number
  rawNodes: number
  failures: string[] | null
  warnings: string[] | null
  cached: number
}

export interface AirportInput {
  name: string
  subscription_url: string
  remarks: string
  enabled: boolean
}

export interface GroupInput {
  name: string
  type: string
  regex: string
  url: string
  interval: number
  include_self?: boolean
  include_all_sources?: boolean
}

export interface SelfNodeInput {
  link: string
  name: string
  enabled: boolean
}

export interface RuleInput {
  rule_type: string
  rule_param: string
  rule_config: number
  resolve_dns: boolean
  enabled: boolean
}

export interface OkResponse {
  ok: boolean
}

export interface Script {
  id: number
  name: string
  kind: string
  code: string
  enabled: boolean
}

export interface ScriptInput {
  name: string
  kind: string
  code: string
  enabled: boolean
}

export interface ScriptTestResult {
  ok: boolean
  kept: boolean
  before: Record<string, unknown>
  after: Record<string, unknown>
  duration_ms: number
  error: string
}

export interface TaskEntry {
  JobName: string
  Success: boolean
  Err: string
  StartedAt: string
  Duration: number
}

export interface RuleProvider {
  id: number
  name: string
  raw_json: string
  source: string
  enabled: boolean
  sort_order: number
  created_at: string
  updated_at: string
}

export interface SuggestionAnomaly {
  Type: string
  Message: string
  Source: string
  Timestamp: string
  Raw: string
}

export interface Suggestion {
  Anomaly: SuggestionAnomaly
  Action: string
  Rule: string
  Reason: string
  AutoApply: boolean
}

export interface SettingItem {
  key: string
  value: string
  kind: string
  label: string
  description: string
  placeholder?: string
  is_default?: boolean
}

export interface RuleSourceStatus {
  name: string
  url: string
  format: string
  dir: string
  versions: string[]
  latest: string
  /** airport = 机场订阅生成；remote = 远程清单 */
  origin?: 'airport' | 'remote' | string
  /** 界面展示名；机场源已去掉内部前缀 */
  label?: string
  rule_count?: number
  /** 当前版本文件修改时间（Unix 秒） */
  updated_at?: number
  /** 机场源对应的机场已删除 */
  airport_missing?: boolean
}
export type RuleSource = RuleSourceStatus

export interface RuleSourceRulesPage {
  name: string
  total: number
  offset: number
  limit: number
  rules: string[]
  warnings?: string[]
}

export interface RuleSourceImportResult {
  added: number
  skipped: number
  overwrote: number
  warnings?: string[]
}

export interface RuleSourceUpdateResult {
  source: string
  version: string
  path: string
  from_cache: boolean
  err: string
}

export interface TrafficUsage { upload: number; download: number; total: number; expire: number }
export interface TrafficStrategy {
  preset?: string
  upload?: string; download?: string; total?: string; expire?: string
}
export interface TrafficRefreshResult {
  airport_id: number; name: string; ok: boolean
  usage: TrafficUsage; node_count: number; err: string
  /** 成功但有话要说时的说明（目前只有「上游不提供流量信息」一种）。不是错误。 */
  note?: string
}
export interface ImportRulesOutcome {
  airport_id: number; name: string
  rules_added: number; rules_skipped: number; rules_enabled?: number; providers_added: number
  groups_created?: number
  rule_source_name?: string; rule_source_version?: string; rule_source_rules?: number
  warnings: string[]; err: string
}
export interface TrafficRefreshResponse {
  results: TrafficRefreshResult[]; ok: number; fail: number
  imports?: ImportRulesOutcome[]
}
export interface ImportRulesResult {
  rules_added: number; rules_skipped: number; rules_enabled?: number; providers_added: number; warnings: string[]; groups_created?: number
  rule_source_name?: string; rule_source_version?: string; rule_source_rules?: number
}
export interface SubscheckResult {
  name?: string; url: string; ok: boolean; status_code: number; latency_ms: number; bytes: number; err: string
}
export interface AlertChannel {
  kind: string; enabled: boolean
  bot_token?: string; chat_id?: string; webhook?: string
  host?: string; port?: number; username?: string; password?: string
  from?: string; to?: string; configured?: Record<string, boolean>
}
export interface AlertTestResult { ok: boolean; error: string; channel: string }
export interface RuleSourcesResponse {
  manifest_path: string
  default_manifest_path?: string
  output_dir?: string
  exists?: boolean
  sources: RuleSourceStatus[]
  hint?: string
  manifest_template?: string
}
export interface TemplateFile { path: string; content: string }
export interface TrafficPreview { usage: TrafficUsage; header: string; ok: boolean }


export interface Pipeline {
  id: number
  name: string
  description: string
  steps_json: string
  enabled: boolean
  sort_order: number
  source: 'ui' | 'template'
  read_only: boolean
}

export interface PipelineStep {
  op: string
  args?: Record<string, string>
  on_error?: string
  line?: number
}

export interface PipelineInput {
  name: string
  description?: string
  steps?: PipelineStep[]
  steps_json?: string
  enabled?: boolean
}

export interface Profile {
  id: number
  name: string
  description: string
  config: string
  pipeline: string
  target: string
  include: string
  exclude: string
  enabled: boolean
  sort_order: number
  rule_layers: string[]
  // 规则源（D-33）：方案绑定的规则源名；未绑定 = []（后端返回空数组，不是 null）。
  rule_sources: string[]
  // 节点来源：null = 未限制（全部启用来源）；[] = 明确不使用该类来源。
  airport_ids: number[] | null
  self_node_ids: number[] | null
  group_layers: string[]
  rule_count: number
  group_count: number
  created_at?: string
  updated_at?: string
}

export interface ProfileInput {
  name: string
  description?: string
  config?: string
  pipeline?: string
  target?: string
  include?: string
  exclude?: string
  enabled?: boolean
  sort_order?: number
  rule_layers?: string[]
  // 规则源（D-33）：绑定到方案的规则源名；缺省 = 不改（PUT），[] = 清空。
  rule_sources?: string[]
  // 节点来源：all_sources=true 恢复“全部启用来源”；否则按 airport_ids/self_node_ids 限制。
  all_sources?: boolean
  airport_ids?: number[]
  self_node_ids?: number[]
  group_layers?: string[]
}

export const endpoints = {
  stats: (): Promise<Stats> => api.get<Stats>('/api/stats'),
  history: (): Promise<HistoryEntry[]> => api.get<HistoryEntry[]>('/api/history'),
  pipelines: (config: string): Promise<Pipeline[]> => api.get<Pipeline[]>('/api/pipelines', { query: { config } }),
  createPipeline: (body: PipelineInput): Promise<Pipeline> => api.post<{ pipeline: Pipeline }>('/api/pipelines', body).then((r) => r.pipeline),
  updatePipeline: (id: number, body: PipelineInput): Promise<Pipeline> => api.put<{ pipeline: Pipeline }>('/api/pipelines/' + id, body).then((r) => r.pipeline),
  deletePipeline: (id: number): Promise<OkResponse> => api.del<OkResponse>('/api/pipelines/' + id),
  targets: (): Promise<string[]> => api.get<string[]>('/api/targets'),
  // GET /api/configs → { configs: string[], default: string }（T5 已交付，见 internal/api/configs_api.go）
  configs: (): Promise<{ configs: string[]; default: string }> =>
    api.get<{ configs: string[]; default: string }>('/api/configs'),
  rulesets: (config: string): Promise<RulesetsResponse> => api.get<RulesetsResponse>('/api/rulesets', { query: { config } }),
  logs: (since: number): Promise<LogsResponse> => api.get<LogsResponse>('/api/logs', { query: { since } }),
  convert: (body: ConvertRequest): Promise<ConvertResponse> => api.post<ConvertResponse>('/api/convert', body),
  diff: (a: string, b: string): Promise<DiffResponse> => api.post<DiffResponse>('/api/diff', { a, b }),

  listAirports: (): Promise<Airport[]> => api.get<{ airports: Airport[] }>('/api/airports').then((r) => r.airports),
  createAirport: (body: AirportInput): Promise<Airport> => api.post<{ airport: Airport }>('/api/airports', body).then((r) => r.airport),
  updateAirport: (id: number, body: AirportInput): Promise<Airport> => api.put<{ airport: Airport }>('/api/airports/' + id, body).then((r) => r.airport),
  deleteAirport: (id: number): Promise<OkResponse> => api.del<OkResponse>('/api/airports/' + id),

  // GET /api/groups?source=&source_prefix=1：不带参数=全部；source=空串=只看自定义层（§6.5 编码约定：URLSearchParams 保留 ?source=）。
  listGroups: (filter?: { source?: string; sourcePrefix?: boolean }): Promise<ProxyGroup[]> =>
    api.get<{ groups: ProxyGroup[] }>('/api/groups', filter && filter.source !== undefined ? { query: { source: filter.source, source_prefix: filter.sourcePrefix ? 1 : 0 } } : undefined).then((r) => r.groups),
  createGroup: (body: GroupInput): Promise<ProxyGroup> => api.post<{ group: ProxyGroup }>('/api/groups', body).then((r) => r.group),
  updateGroup: (id: number, body: GroupInput): Promise<ProxyGroup> => api.put<{ group: ProxyGroup }>('/api/groups/' + id, body).then((r) => r.group),
  deleteGroup: (id: number): Promise<OkResponse> => api.del<OkResponse>('/api/groups/' + id),

  // GET /api/rules?source=&source_prefix=1：不带参数=全部；source=空串=只看自定义层（§6.5 编码约定）。
  listRules: (filter?: { source?: string; sourcePrefix?: boolean }): Promise<Rule[]> =>
    api.get<{ rules: Rule[] }>('/api/rules', filter && filter.source !== undefined ? { query: { source: filter.source, source_prefix: filter.sourcePrefix ? 1 : 0 } } : undefined).then((r) => r.rules),
  listRulesPage: (query: { source?: string | null; enabled?: boolean; q?: string; limit: number; offset: number }): Promise<RulePage> =>
    api.get<RulePage>('/api/rules', {
      query: {
        source: query.source === null || query.source === undefined ? undefined : query.source,
        enabled: query.enabled === undefined ? undefined : query.enabled ? 1 : 0,
        q: query.q === undefined || query.q === '' ? undefined : query.q,
        limit: query.limit,
        offset: query.offset,
      },
    }),
  listRuleSourceCounts: (): Promise<RuleSourceCount[]> =>
    api.get<{ sources: RuleSourceCount[] }>('/api/rules/sources').then((r) => r.sources),
  checkRuleSourceConflicts: (names: string[]): Promise<RuleSourceConflicts> =>
    api.get<RuleSourceConflicts>('/api/rule-sources/conflicts', { query: { names: names.join(',') } }),
  createRule: (body: RuleInput): Promise<Rule> => api.post<{ rule: Rule }>('/api/rules', body).then((r) => r.rule),
  updateRule: (id: number, body: RuleInput): Promise<Rule> => api.put<{ rule: Rule }>('/api/rules/' + id, body).then((r) => r.rule),
  deleteRule: (id: number): Promise<OkResponse> => api.del<OkResponse>('/api/rules/' + id),

  listTokens: (): Promise<SubToken[]> => api.get<{ tokens: SubToken[] }>('/api/tokens').then((r) => r.tokens),
  createToken: (body: { token: string; name: string; target?: string; pipeline?: string; config?: string; include?: string; exclude?: string; profile_id?: number }): Promise<SubToken> => api.post<{ token: SubToken }>('/api/tokens', body).then((r) => r.token),
  updateToken: (token: string, body: SubTokenUpdate): Promise<SubToken> =>
    api.put<{ token: SubToken }>('/api/tokens/' + encodeURIComponent(token), body).then((r) => r.token),
  deleteToken: (token: string): Promise<OkResponse> => api.del<OkResponse>('/api/tokens/' + encodeURIComponent(token)),

  listSelfNodes: (): Promise<SelfNode[]> => api.get<{ self_nodes: SelfNode[] }>('/api/self-nodes').then((r) => r.self_nodes),
  createSelfNode: (body: SelfNodeInput): Promise<SelfNode> => api.post<{ self_node: SelfNode }>('/api/self-nodes', body).then((r) => r.self_node),
  updateSelfNode: (id: number, body: SelfNodeInput): Promise<OkResponse> => api.put<OkResponse>('/api/self-nodes/' + id, body),
  deleteSelfNode: (id: number): Promise<OkResponse> => api.del<OkResponse>('/api/self-nodes/' + id),

  listScripts: (): Promise<Script[]> => api.get<{ scripts: Script[] }>('/api/scripts').then((r) => r.scripts),
  createScript: (body: ScriptInput): Promise<Script> => api.post<{ script: Script }>('/api/scripts', body).then((r) => r.script),
  getScript: (id: number): Promise<Script> => api.get<{ script: Script }>('/api/scripts/' + id).then((r) => r.script),
  updateScript: (id: number, body: ScriptInput): Promise<Script> => api.put<{ script: Script }>('/api/scripts/' + id, body).then((r) => r.script),
  deleteScript: (id: number): Promise<OkResponse> => api.del<OkResponse>('/api/scripts/' + id),
  testScript: (body: { code: string; kind: string; node?: Record<string, unknown>; timeout?: number }): Promise<ScriptTestResult> =>
    api.post<ScriptTestResult>('/api/scripts/test', body),

  listTasks: (): Promise<TaskEntry[]> => api.get<{ tasks: TaskEntry[] }>('/api/tasks').then((r) => r.tasks),

  listRuleProviders: (source?: string, enabled?: boolean): Promise<RuleProvider[]> =>
    api.get<{ rule_providers: RuleProvider[] }>('/api/rule-providers', { query: { source, enabled } }).then((r) => r.rule_providers),
  updateRuleProvider: (id: number, enabled: boolean): Promise<OkResponse> =>
    api.put<OkResponse>('/api/rule-providers/' + id + '/enabled', { enabled }),
  // POST /api/rules/bulk（后端 internal/api/rules_bulk.go）：
  // 按 ids 整批启停/删除，或按 source（source_prefix 可选）整批启停/删除（v2：ids 与 source 互斥，后端校验）。
  bulkRules: (body: { ids?: number[]; source?: string; sourcePrefix?: boolean; enabled?: boolean; delete?: boolean }): Promise<{ updated?: number; deleted?: number }> =>
    api.post<{ updated?: number; deleted?: number }>('/api/rules/bulk', body),
  listSuggestions: (): Promise<Suggestion[]> => api.get<{ suggestions: Suggestion[] }>('/api/suggestions').then((r) => r.suggestions),
  confirmSuggestion: (index: number): Promise<OkResponse> => api.post<OkResponse>('/api/suggestions/' + index + '/confirm', {}),
  rejectSuggestion: (index: number): Promise<OkResponse> => api.post<OkResponse>('/api/suggestions/' + index + '/reject', {}),
  applySuggestions: (): Promise<{ applied: number }> => api.post<{ applied: number }>('/api/suggestions/apply', {}),
  listRuleSources: (): Promise<RuleSourcesResponse> =>
    api.get<RuleSourcesResponse>('/api/rule-sources'),
  listRuleSourceRules: (name: string, opts?: { q?: string; offset?: number; limit?: number }): Promise<RuleSourceRulesPage> =>
    api.get<RuleSourceRulesPage>('/api/rule-sources/rules', { query: { name, q: opts?.q, offset: opts?.offset, limit: opts?.limit } }),
  importRuleSourceRules: (body: { name: string; rules: string[]; conflict?: 'skip' | 'overwrite' }): Promise<RuleSourceImportResult> =>
    api.post<RuleSourceImportResult>('/api/rule-sources/import-rules', body),
  updateRuleSources: (): Promise<{ results: RuleSourceUpdateResult[] }> =>
    api.post<{ results: RuleSourceUpdateResult[] }>('/api/rule-sources/update', {}),
  rollbackRuleSource: (name: string): Promise<{ path: string }> =>
    api.post<{ path: string }>('/api/rule-sources/rollback', { name }),
  generateMRS: (name: string, behavior: string): Promise<{ path: string; bytes: number }> =>
    api.post<{ path: string; bytes: number }>('/api/rule-sources/mrs', { name, behavior }),
  addRuleSource: (body: { name?: string; url: string; input_format?: string }): Promise<{ path: string; created: boolean }> =>
    api.post<{ path: string; created: boolean }>('/api/rule-sources', body),
  removeRuleSource: (name: string): Promise<OkResponse> =>
    api.post<OkResponse>('/api/rule-sources/remove', { name }),
  updateRuleSourceOne: (name: string): Promise<{ results: RuleSourceUpdateResult[] }> =>
    api.post<{ results: RuleSourceUpdateResult[] }>('/api/rule-sources/update-one', { name }),

  listSettings: (): Promise<{ settings: SettingItem[] }> => api.get<{ settings: SettingItem[] }>('/api/settings'),
  updateSettings: (values: Record<string, string>): Promise<{ settings: SettingItem[] }> => api.put<{ settings: SettingItem[] }>('/api/settings', { settings: values }),

  refreshAllAirports: (withRules = false): Promise<TrafficRefreshResponse> =>
    api.post<TrafficRefreshResponse>('/api/airports/refresh', {}, withRules ? { query: { with_rules: 1 } } : undefined),
  refreshAirport: (id: number | string): Promise<{ result: TrafficRefreshResult }> =>
    api.post<{ result: TrafficRefreshResult }>('/api/airports/' + id + '/refresh', {}),
  importAirportRules: (id: number | string, body?: { create_groups?: boolean }): Promise<ImportRulesResult> =>
    api.post<ImportRulesResult>('/api/airports/' + id + '/import-rules', body ?? {}),
  checkAirports: (ids?: number[]): Promise<{ results: SubscheckResult[] }> =>
    api.post<{ results: SubscheckResult[] }>('/api/airports/check', ids === undefined ? {} : { ids }),
  lastAirportCheck: (): Promise<{ results: SubscheckResult[]; at?: number }> =>
    api.get<{ results: SubscheckResult[]; at?: number }>('/api/airports/check/last'),

  getTrafficStrategy: (): Promise<{ strategy: TrafficStrategy }> =>
    api.get<{ strategy: TrafficStrategy }>('/api/traffic/strategy'),
  setTrafficStrategy: (s: TrafficStrategy): Promise<{ strategy: TrafficStrategy }> =>
    api.put<{ strategy: TrafficStrategy }>('/api/traffic/strategy', s),
  previewTraffic: (strategy?: TrafficStrategy): Promise<TrafficPreview> =>
    api.get<TrafficPreview>('/api/traffic/preview', strategy !== undefined ? { query: { strategy: JSON.stringify(strategy) } } : undefined),

  getTemplate: (): Promise<TemplateFile> => api.get<TemplateFile>('/api/template'),
  putTemplate: (content: string): Promise<{ ok: boolean }> =>
    api.put<{ ok: boolean }>('/api/template', { content }),
  getOverride: (): Promise<TemplateFile> => api.get<TemplateFile>('/api/override'),
  putOverride: (content: string): Promise<{ ok: boolean }> =>
    api.put<{ ok: boolean }>('/api/override', { content }),

  listAlertChannels: (): Promise<{ channels: AlertChannel[] }> =>
    api.get<{ channels: AlertChannel[] }>('/api/alert-channels'),
  putAlertChannels: (channels: AlertChannel[]): Promise<{ channels: AlertChannel[] }> =>
    api.put<{ channels: AlertChannel[] }>('/api/alert-channels', { channels }),
  testAlertChannel: (index: number): Promise<AlertTestResult> =>
    api.post<AlertTestResult>('/api/alert-channels/test', { index }),

  listProfiles: (): Promise<Profile[]> => api.get<{ profiles: Profile[] }>('/api/profiles').then((r) => r.profiles),
  createProfile: (body: ProfileInput): Promise<Profile> => api.post<{ profile: Profile }>('/api/profiles', body).then((r) => r.profile),
  updateProfile: (id: number, body: ProfileInput): Promise<Profile> => api.put<{ profile: Profile }>('/api/profiles/' + id, body).then((r) => r.profile),
  deleteProfile: (id: number): Promise<OkResponse> => api.del<OkResponse>('/api/profiles/' + id),
}

export function subscribeUrl(origin: string, token: string): string {
  return origin + '/subscribe?token=' + encodeURIComponent(token)
}
