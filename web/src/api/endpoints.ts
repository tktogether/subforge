import { api } from './client'

export interface Airport {
  id: number
  name: string
  subscription_url: string
  remarks: string
  enabled: boolean
  sort_order: number
}

export interface ProxyGroup {
  id: number
  name: string
  type: string
  regex: string
  url: string
  interval: number
  sort_order: number
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
}

export interface SubToken {
  token: string
  name: string
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
  output: string
  warnings: string[] | null
  failures: string[] | null
  nodes: number
  groups: number
  rules: number
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

export const endpoints = {
  stats: (): Promise<Stats> => api.get<Stats>('/api/stats'),
  history: (): Promise<HistoryEntry[]> => api.get<HistoryEntry[]>('/api/history'),
  pipelines: (config: string): Promise<string[]> => api.get<string[]>('/api/pipelines', { query: { config } }),
  targets: (): Promise<string[]> => api.get<string[]>('/api/targets'),
  rulesets: (config: string): Promise<RulesetsResponse> => api.get<RulesetsResponse>('/api/rulesets', { query: { config } }),
  logs: (since: number): Promise<LogsResponse> => api.get<LogsResponse>('/api/logs', { query: { since } }),
  convert: (body: ConvertRequest): Promise<ConvertResponse> => api.post<ConvertResponse>('/api/convert', body),
  diff: (a: string, b: string): Promise<DiffResponse> => api.post<DiffResponse>('/api/diff', { a, b }),

  listAirports: (): Promise<Airport[]> => api.get<{ airports: Airport[] }>('/api/airports').then((r) => r.airports),
  createAirport: (body: AirportInput): Promise<Airport> => api.post<{ airport: Airport }>('/api/airports', body).then((r) => r.airport),
  updateAirport: (id: number, body: AirportInput): Promise<Airport> => api.put<{ airport: Airport }>('/api/airports/' + id, body).then((r) => r.airport),
  deleteAirport: (id: number): Promise<OkResponse> => api.del<OkResponse>('/api/airports/' + id),

  listGroups: (): Promise<ProxyGroup[]> => api.get<{ groups: ProxyGroup[] }>('/api/groups').then((r) => r.groups),
  createGroup: (body: GroupInput): Promise<ProxyGroup> => api.post<{ group: ProxyGroup }>('/api/groups', body).then((r) => r.group),
  updateGroup: (id: number, body: GroupInput): Promise<ProxyGroup> => api.put<{ group: ProxyGroup }>('/api/groups/' + id, body).then((r) => r.group),
  deleteGroup: (id: number): Promise<OkResponse> => api.del<OkResponse>('/api/groups/' + id),

  listRules: (): Promise<Rule[]> => api.get<{ rules: Rule[] }>('/api/rules').then((r) => r.rules),
  createRule: (body: RuleInput): Promise<Rule> => api.post<{ rule: Rule }>('/api/rules', body).then((r) => r.rule),
  updateRule: (id: number, body: RuleInput): Promise<Rule> => api.put<{ rule: Rule }>('/api/rules/' + id, body).then((r) => r.rule),
  deleteRule: (id: number): Promise<OkResponse> => api.del<OkResponse>('/api/rules/' + id),

  listTokens: (): Promise<SubToken[]> => api.get<{ tokens: SubToken[] }>('/api/tokens').then((r) => r.tokens),
  createToken: (body: { token: string; name: string }): Promise<SubToken> => api.post<{ token: SubToken }>('/api/tokens', body).then((r) => r.token),
  deleteToken: (token: string): Promise<OkResponse> => api.del<OkResponse>('/api/tokens/' + encodeURIComponent(token)),

  listSelfNodes: (): Promise<SelfNode[]> => api.get<{ self_nodes: SelfNode[] }>('/api/self-nodes').then((r) => r.self_nodes),
  createSelfNode: (body: SelfNodeInput): Promise<SelfNode> => api.post<{ self_node: SelfNode }>('/api/self-nodes', body).then((r) => r.self_node),
  updateSelfNode: (id: number, body: SelfNodeInput): Promise<OkResponse> => api.put<OkResponse>('/api/self-nodes/' + id, body),
  deleteSelfNode: (id: number): Promise<OkResponse> => api.del<OkResponse>('/api/self-nodes/' + id),
}

export function subscribeUrl(origin: string, token: string): string {
  return origin + '/subscribe?token=' + encodeURIComponent(token)
}
