// 文案中枢：所有页面/字段/脚本/管线的说明文案集中在此，组件与页面只引用不重写。
// 语气要求：面向首次使用的用户，说明事实与操作，不使用口语、比喻或测试数据。

export interface HelpLink { label: string; to: string; note?: string }
export interface HelpStep { title: string; detail: string }
export interface HelpConcept { term: string; explain: string }
export interface HelpFaq { q: string; a: string }
export interface EmptyStateHelp { title: string; detail: string; action?: { label: string; to?: string } }
export interface FailureCase { scene: string; behavior: string; ui: string }

export interface PageHelp {
  page: string
  title: string
  oneLiner: string
  whenToUse: string
  steps?: HelpStep[]
  concepts?: HelpConcept[]
  faq?: HelpFaq[]
  emptyState?: EmptyStateHelp
  links?: HelpLink[]
}

export const PAGE_HELP: Record<string, PageHelp> = {
  overview: {
    page: 'overview',
    title: '概览',
    oneLiner: '查看订阅转换、节点来源与规则的运行概况。',
    whenToUse: '用于确认服务当前状态，或从概览进入需要管理的页面。',
    steps: [
      { title: '查看指标', detail: '顶部指标显示最近一次转换的节点数、分组数、规则数与缓存命中情况。' },
      { title: '查看转换历史', detail: '下方表格列出最近的转换记录；展开一行可查看失败与告警原因。' },
    ],
    concepts: [
      { term: '缓存命中率', explain: '最近一次转换中，直接使用本地缓存、未向上游重新请求的来源比例。' },
      { term: '历史记录', explain: '转换历史保存在内存中，最多保留 20 条，服务重启后清空。' },
    ],
    faq: [
      { q: '指标为空或为 0 时怎么办？', a: '请先添加机场订阅，再配置规则与分组，最后在订阅页创建令牌。' },
    ],
    emptyState: { title: '暂无转换数据', detail: '请先在机场页添加订阅地址，再配置规则与分组，最后在订阅页创建令牌。', action: { label: '添加机场', to: '/airports' } },
    links: [
      { label: '机场', to: '/airports', note: '添加上游订阅地址' },
      { label: '规则', to: '/rules', note: '配置分流规则' },
      { label: '订阅', to: '/subscriptions', note: '生成订阅地址' },
    ],
  },
  convert: {
    page: 'convert',
    title: '转换',
    oneLiner: '将订阅内容转换为指定客户端格式并预览。',
    whenToUse: '用于检查节点、规则或模板是否按预期生效。',
    steps: [
      { title: '填写来源', detail: '在订阅地址中逐行填写上游地址，也可直接粘贴订阅文本；两种来源可以同时使用。' },
      { title: '选择目标格式', detail: '选择客户端格式；留空时使用 clash.meta。' },
      { title: '执行转换', detail: '点击转换查看产物、节点数量与缓存情况。' },
    ],
    concepts: [
      { term: '目标格式', explain: '输出配置对应的客户端类型，例如 Clash、sing-box、Surge。' },
      { term: '差异对比', explain: '粘贴两份产物，按行查看新增与删除的内容。' },
    ],
    faq: [
      { q: '部分订阅失败时如何处理？', a: '其他来源仍会正常输出，失败原因显示在失败列表中；只有全部来源失败时才会终止转换。' },
    ],
    emptyState: { title: '暂无转换结果', detail: '填写订阅地址或粘贴订阅文本后点击转换。', action: { label: '开始转换', to: '/convert' } },
    links: [
      { label: '机场', to: '/airports', note: '复用已保存的机场' },
      { label: '管线', to: '/pipelines', note: '引用可复用的处理步骤' },
    ],
  },
  pipelines: {
    page: 'pipelines',
    title: '管线',
    oneLiner: '将一组节点处理步骤保存为可重复使用的管线。',
    whenToUse: '当多个订阅需要相同的过滤、重命名或排序步骤时使用。',
    steps: [
      { title: '新建管线', detail: '点击新建管线并填写名称。' },
      { title: '添加步骤', detail: '按执行顺序添加过滤或改写步骤，可上移、下移或删除。' },
      { title: '保存', detail: '保存后可在订阅方案或转换页引用。' },
    ],
    concepts: [
      { term: '过滤步骤', explain: '判断节点是否保留，例如按名称、地区或协议过滤。' },
      { term: '改写步骤', explain: '修改节点字段，例如重命名、添加前缀、去重或排序。' },
    ],
    faq: [
      { q: '管线与单次转换有什么区别？', a: '管线保存为可重复使用的处理步骤；单次转换只对当前请求生效。' },
    ],
    emptyState: { title: '暂无管线', detail: '点击新建管线，添加第一个处理步骤。', action: { label: '新建管线', to: '/pipelines' } },
    links: [
      { label: '脚本', to: '/scripts', note: '在管线中引用自定义脚本' },
      { label: '方案', to: '/profiles', note: '在方案中引用管线' },
    ],
  },
  scripts: {
    page: 'scripts',
    title: '脚本',
    oneLiner: '使用 JavaScript 编写节点过滤或改写步骤。',
    whenToUse: '当内置过滤与改写步骤无法满足需求时使用。',
    steps: [
      { title: '新建脚本', detail: '选择过滤或改写类型，并填写名称。' },
      { title: '编写代码', detail: '过滤脚本返回布尔值；改写脚本修改 node 字段。' },
      { title: '试运行', detail: '使用示例节点验证脚本结果，再在管线或转换中引用。' },
    ],
    concepts: [
      { term: '过滤脚本', explain: '返回真时保留节点，返回假时丢弃节点。' },
      { term: '改写脚本', explain: '修改 node 的字段后生效；返回值不参与处理。' },
    ],
    emptyState: { title: '暂无脚本', detail: '点击新建脚本，或在编辑窗口中插入示例代码。', action: { label: '新建脚本', to: '/scripts' } },
    links: [
      { label: '管线', to: '/pipelines', note: '在管线步骤中引用脚本' },
      { label: '转换', to: '/convert', note: '试运行完整转换' },
    ],
  },
  airports: {
    page: 'airports',
    title: '机场',
    oneLiner: '管理机场订阅地址，为转换和订阅提供节点。',
    whenToUse: '新增或维护节点来源，查看流量与可用性。',
    steps: [
      { title: '新增机场', detail: '点击新增，填写名称与订阅地址。' },
      { title: '刷新流量', detail: '读取上游返回的流量与到期信息。' },
      { title: '可用性检测', detail: '检测订阅地址是否可以正常访问。' },
    ],
    faq: [
      { q: '刷新失败后为什么余额没有清零？', a: '刷新失败时只记录错误，保留上一次成功获取的余额；上游恢复后重新刷新即可更新。' },
    ],
    concepts: [
      { term: '机场', explain: '提供订阅地址的服务商，SubForge 从该地址获取节点。' },
      { term: '流量信息', explain: '上游响应头中的已用流量、总流量与到期时间，用于订阅中的余额显示。' },
    ],
    emptyState: { title: '暂无机场', detail: '点击新增，添加第一个机场订阅地址。', action: { label: '新增机场', to: '/airports' } },
    links: [
      { label: '订阅', to: '/subscriptions', note: '机场节点会进入订阅输出' },
    ],
  },
  'self-nodes': {
    page: 'self-nodes',
    title: '自建节点',
    oneLiner: '管理手动添加的节点，与机场节点一同输出。',
    whenToUse: '需要加入机场订阅中没有的服务器时使用。',
    steps: [
      { title: '新增节点', detail: '填写名称与标准分享链接（ss://、vmess://、vless://、trojan://、hysteria2:// 等）。' },
      { title: '启用或停用', detail: '通过开关控制节点是否参与输出。' },
    ],
    concepts: [
      { term: '自建节点', explain: '不来自机场、由用户手动维护的节点，与机场节点合并后输出。' },
      { term: '分享链接', explain: '客户端中复制的节点链接；保存前服务端会校验格式与必要字段。' },
    ],
    faq: [
      { q: '自建节点与机场节点同名时如何处理？', a: '按来源顺序保留先出现的节点，自建节点排在机场节点之后。' },
    ],
    emptyState: { title: '暂无自建节点', detail: '点击新增，添加第一个节点分享链接。', action: { label: '新增节点', to: '/self-nodes' } },
    links: [
      { label: '机场', to: '/airports', note: '机场节点与自建节点合并输出' },
      { label: '订阅', to: '/subscriptions', note: '自建节点进入订阅输出' },
    ],
  },
  rules: {
    page: 'rules',
    title: '规则',
    oneLiner: '管理分流规则与规则集，并将规则指向目标分组。',
    whenToUse: '调整流量的匹配顺序与目标出站时使用。',
    steps: [
      { title: '新增规则', detail: '选择规则类型并填写匹配参数。' },
      { title: '选择目标分组', detail: '目标分组决定匹配流量使用哪个出站；选择无分组时直连。' },
      { title: '启用规则', detail: '只有已启用的规则会进入订阅输出。' },
    ],
    concepts: [
      { term: '规则行', explain: '由类型、参数与目标分组组成；开启解析 DNS 时会在规则末尾附加 no-resolve。' },
      { term: '规则集', explain: '一组可复用的规则定义，可按启用状态整体管理。' },
    ],
    faq: [
      { q: '规则没有生效时如何排查？', a: '确认规则已启用、目标分组存在，并检查同一匹配条件是否有更靠前的规则。' },
    ],
    emptyState: { title: '暂无规则', detail: '点击新增，添加第一条分流规则。', action: { label: '新增规则', to: '/rules' } },
    links: [
      { label: '规则源', to: '/rule-sources', note: '从外部来源获取规则集' },
    ],
  },
  'rule-sources': {
    page: 'rule-sources',
    title: '规则源',
    oneLiner: '从外部地址获取规则文件，保存为可回滚的本地版本。',
    whenToUse: '使用社区维护的规则文件，或需要在多个方案间复用规则时。',
    steps: [
      { title: '新增规则源', detail: '填写名称与地址；首次添加会自动创建规则源清单。' },
      { title: '更新', detail: '获取最新版本；每个规则源保留最近 5 个版本，可回滚。' },
      { title: '在方案中绑定', detail: '到方案页绑定规则源；订阅生成时会内联其规则。' },
    ],
    faq: [
      { q: '更新失败会影响已有规则吗？', a: '不会。更新失败时保留上一版本，已有订阅产物不受影响。' },
      { q: '规则源支持哪些输入格式？', a: '支持 Clash YAML、文本规则与 INI ruleset。输出格式由规则源清单的 output.format 决定：clash-meta 输出 .yaml，clash-json 与 sing-box 输出 .json，mrs 输出 .mrs，text 输出 .txt。' },
      { q: 'INI 转换模板在哪里管理？', a: '转换模板由启动参数 -config 指定；指定后可在设置页预览和编辑。' },
    ],
    concepts: [
      { term: '规则源', explain: '外部规则文件的地址；SubForge 定期获取并按清单配置保存为本地版本。' },
      { term: '版本与回滚', explain: '每次更新生成一个新版本，每个规则源保留最近 5 个版本；回滚只切换当前版本，不删除历史文件。' },
      { term: '与规则的关系', explain: '规则源负责获取与版本管理；在方案中绑定后，规则才会进入订阅输出。' },
      { term: 'MRS', explain: 'mihomo 使用的二进制规则集格式，文件更小、加载更快；sing-box 使用另一种 .srs 格式。' },
    ],
    emptyState: { title: '暂无规则源', detail: '点击新增规则源，填写外部规则文件地址。' },
    links: [
      { label: '设置', to: '/settings', note: '修改规则源清单路径' },
      { label: '方案', to: '/profiles', note: '将规则源绑定到订阅方案' },
    ],
  },
  groups: {
    page: 'groups',
    title: '分组',
    oneLiner: '管理输出到客户端的策略组，以及每个组的成员范围。',
    whenToUse: '调整客户端中节点如何分组时使用。',
    steps: [
      { title: '新增分组', detail: '填写名称与类型；手工分组可用匹配正则选择成员。' },
      { title: '设置成员扩展', detail: '机场的「全部节点」组可扩展到方案中的所有机场；自建节点单独控制。' },
      { title: '排序', detail: '调整分组在客户端中的显示顺序。' },
    ],
    concepts: [
      { term: '分组类型', explain: 'select 手动选择、url-test 自动选择延迟最低、fallback 故障切换、load-balance 负载均衡。' },
      { term: '成员扩展', explain: '机场「全部节点」组导入时会自动开启「包含全部来源节点」，订阅生成时并入方案中所有机场的节点；「包含自建节点」单独控制自建节点。子集组保持原成员。' },
    ],
    faq: [
      { q: '分组顺序影响什么？', a: '影响订阅输出中分组的排列顺序。' },
      { q: '机场分组的成员可以修改吗？', a: '成员由机场导入维护；可以在分组页调整类型、检测地址与成员扩展开关。' },
    ],
    emptyState: { title: '暂无分组', detail: '点击新增，添加第一个分组。', action: { label: '新增分组', to: '/groups' } },
    links: [
      { label: '规则', to: '/rules', note: '规则决定流量进入哪个分组' },
      { label: '订阅', to: '/subscriptions', note: '分组影响订阅输出结构' },
    ],
  },
  profiles: {
    page: 'profiles',
    title: '方案',
    oneLiner: '定义订阅使用的节点来源、输出选项、规则层与规则源。',
    whenToUse: '需要为不同设备或用途生成不同订阅内容时使用。',
    steps: [
      { title: '新建方案', detail: '填写名称，选择节点来源与输出选项。' },
      { title: '绑定规则源', detail: '按优先级添加规则源；订阅生成时会按顺序内联规则。' },
      { title: '绑定令牌', detail: '在订阅页创建令牌并选择该方案。' },
    ],
    concepts: [
      { term: '方案', explain: '方案决定订阅使用哪些机场和自建节点、哪个模板与管线，以及规则与分组来自哪些层。' },
      { term: '层顺序', explain: '规则层按顺序拼接，先出现的规则优先；分组层同名时先出现的分组优先。' },
      { term: '规则源优先级', explain: '多个规则源包含同一匹配条件时，按绑定顺序由先出现的源生效。' },
      { term: '节点来源', explain: '默认使用全部启用的机场与自建节点；也可以只选择指定来源。' },
    ],
    faq: [
      { q: '删除方案会影响规则或分组吗？', a: '不会。规则与分组是全局数据，不会随方案删除。' },
      { q: '令牌没有绑定方案时使用什么？', a: '使用全部启用的来源与默认层顺序。' },
    ],
    emptyState: { title: '暂无方案', detail: '点击新增，创建第一个订阅方案。', action: { label: '新建方案', to: '/profiles' } },
    links: [
      { label: '规则源', to: '/rule-sources', note: '方案可绑定规则源' },
      { label: '订阅', to: '/subscriptions', note: '令牌绑定方案后生成独立订阅' },
    ],
  },
  subscriptions: {
    page: 'subscriptions',
    title: '订阅',
    oneLiner: '管理订阅令牌与订阅地址，并设置流量信息聚合方式。',
    whenToUse: '为客户端生成订阅地址，或调整余额显示方式时使用。',
    steps: [
      { title: '新建令牌', detail: '填写名称；令牌可以绑定一个方案。' },
      { title: '复制订阅地址', detail: '选择令牌后复制生成的地址，填入客户端。' },
      { title: '设置流量聚合', detail: '选择多个机场时余额的合并方式。' },
    ],
    concepts: [
      { term: '令牌', explain: '订阅地址中的访问凭据；每个令牌对应一个独立地址，可长期使用。' },
      { term: '方案', explain: '令牌绑定方案后，订阅按该方案选择来源、模板与规则源。' },
      { term: '流量聚合', explain: '将多个机场的已用流量、总流量与到期时间合并为客户端显示的一行信息。' },
    ],
    faq: [
      { q: '一个令牌可以用于多台设备吗？', a: '可以。同一地址可以长期重复使用；如需分别撤销访问，请为不同用途创建独立令牌。' },
    ],
    emptyState: { title: '暂无订阅', detail: '点击新建令牌，生成第一个订阅地址。', action: { label: '新建令牌', to: '/subscriptions' } },
    links: [
      { label: '机场', to: '/airports', note: '机场节点进入订阅输出' },
      { label: '方案', to: '/profiles', note: '方案决定订阅使用的来源与规则' },
    ],
  },
  tasks: {
    page: 'tasks',
    title: '任务',
    oneLiner: '查看调度器最近执行的任务记录。',
    whenToUse: '确认定时更新或可用性检测是否正常执行时使用。',
    steps: [
      { title: '查看结果', detail: '表格显示任务名称、状态、开始时间、耗时与错误信息；记录保存在内存中。' },
      { title: '手动触发', detail: '在规则源页点击立即更新，或在机场页执行可用性检测。' },
    ],
    concepts: [
      { term: 'cron', explain: '定时表达式，格式为「分 时 日 月 周」，例如 0 3 * * * 表示每天 03:00。' },
      { term: '重试', explain: '任务失败后自动重试 3 次；仍失败时记录为失败并触发告警。' },
    ],
    faq: [
      { q: '任务失败会重试吗？', a: '会。任务失败后自动重试 3 次；仍失败时记录为失败并触发告警。' },
      { q: '任务记录会保留多久？', a: '任务记录保存在内存中，服务重启后清空。' },
    ],
    emptyState: { title: '暂无任务记录', detail: '任务按计划执行；也可以在规则源页立即更新一次。', action: { label: '前往规则源', to: '/rule-sources' } },
    links: [
      { label: '日志', to: '/logs', note: '查看任务执行日志' },
    ],
  },
  settings: {
    page: 'settings',
    title: '设置',
    oneLiner: '调整服务运行参数，并查看或编辑转换模板。',
    whenToUse: '修改规则源路径、功能开关或 INI 转换模板时使用。',
    steps: [
      { title: '修改设置', detail: '在对应输入框中填写新值；清空表示恢复默认值。' },
      { title: '保存', detail: '点击保存后立即生效，无需重启服务。' },
      { title: '编辑转换模板', detail: '启动时指定 -config 后，可在本页预览和编辑 INI 模板。' },
    ],
    concepts: [
      { term: '默认值', explain: '未显式配置时使用的值。' },
      { term: '即时生效', explain: '保存后立即更新运行参数。' },
      { term: '模板热重载', explain: '保存模板后，服务会自动重新加载模板文件。' },
    ],
    faq: [
      { q: '修改设置需要重启吗？', a: '不需要。保存后立即生效。' },
    ],
    emptyState: { title: '暂无可配置项', detail: '请刷新页面重试。', action: { label: '刷新', to: '/settings' } },
    links: [
      { label: '规则源', to: '/rule-sources', note: '规则源清单路径在此页配置' },
    ],
  },
  logs: {
    page: 'logs',
    title: '日志',
    oneLiner: '查看服务最近的运行日志。',
    whenToUse: '排查请求失败或确认操作是否执行时使用。',
    steps: [
      { title: '查看日志', detail: '每行显示时间、级别与内容；长内容默认显示前三行，点击「查看完整内容」打开详情。' },
      { title: '筛选', detail: '可以按级别筛选，或输入关键字搜索日志内容。' },
      { title: '自动刷新', detail: '开启后每 3 秒获取一次新增日志；最多显示最近 200 行。' },
    ],
    concepts: [
      { term: '级别', explain: '错误、警告与信息由前端按关键词归类，仅用于浏览，不代表服务端日志级别。' },
      { term: '容量', explain: '日志保存在内存中，最多保留 200 行，服务重启后清空。' },
    ],
    faq: [
      { q: '日志会写入文件吗？', a: '不会。日志保存在内存中，接口最多返回最近 200 行。' },
      { q: '长日志为什么只显示部分内容？', a: '为避免页面被单条日志撑开，默认显示前三行；点击「查看完整内容」可查看和复制全文。' },
    ],
    emptyState: { title: '暂无日志', detail: '执行操作或请求订阅后，日志会显示在这里。', action: { label: '刷新', to: '/logs' } },
    links: [
      { label: '任务', to: '/tasks', note: '查看定时任务执行记录' },
    ],
  },
}

export function pageHelp(key: string): PageHelp {
  const found = PAGE_HELP[key]
  if (found !== undefined) {
    return found
  }
  return {
    page: key,
    title: key,
    oneLiner: '该页面暂无说明',
    whenToUse: '',
    steps: [],
    concepts: [],
    faq: [],
    emptyState: undefined,
    links: [],
  }
}

export const GLOSSARY: HelpConcept[] = [
  { term: '节点', explain: '单个代理服务器条目，包含名称、类型、地址与端口。' },
  { term: '订阅', explain: 'SubForge 生成的访问地址，客户端使用该地址获取配置。' },
  { term: '机场', explain: '提供订阅地址的服务商。' },
  { term: '规则', explain: '单条分流条件，匹配后把流量指向指定分组。' },
  { term: '规则集', explain: '一组可复用的规则定义。' },
  { term: '规则源', explain: '外部规则文件地址；SubForge 定期获取并保存为本地版本。' },
  { term: '管线', explain: '有序的节点处理步骤，可重复使用。' },
  { term: '转换', explain: '将上游订阅转换为客户端配置的过程。' },
  { term: '令牌', explain: '订阅地址中的访问凭据。' },
  { term: 'subscription-userinfo', explain: '上游返回的流量与到期信息响应头。' },
  { term: 'MRS', explain: 'mihomo 使用的二进制规则集格式；sing-box 使用 .srs 格式。' },
  { term: 'ETag', explain: 'HTTP 缓存标识，用于判断远程内容是否更新。' },
  { term: '可用性检测', explain: '请求订阅地址，确认其是否可以正常访问。' },
  { term: '沙箱', explain: '脚本运行环境，不提供网络、文件与进程访问，并有执行时间上限。' },
]

export function glossaryFor(term: string): string {
  const hit = GLOSSARY.find((g) => g.term === term)
  return hit !== undefined ? hit.explain : ''
}

export const FIELD_HELP: Record<string, { hint: string; example?: string }> = {
  'airport.name': { hint: '机场显示名称，用于识别来源。', example: '香港专线' },
  'airport.subscription_url': { hint: '机场提供的订阅地址，支持 http 与 https。', example: 'https://example.com/sub?token=xxx' },
  'airport.remarks': { hint: '可选备注，不影响转换结果。', example: '主用机场' },
  'airport.with_rules': {
    hint: '开启后，「刷新全部流量」会同时更新各机场的规则源；关闭时只刷新节点与流量。单个机场可在表格中使用「更新规则源」。',
    example: '',
  },
  'airport.enabled': { hint: '是否参与转换与订阅输出。' },
  'airport.traffic_err': { hint: '最近一次采集失败的原因；失败时余额保留上一次成功值。' },
  'airport.traffic_at': { hint: '最近一次成功获取余额的时间。' },
  'group.name': { hint: '输出到客户端的策略组名称。', example: '香港' },
  'group.type': { hint: '策略组类型。', example: 'select' },
  'group.regex': { hint: '按节点名称匹配成员的正则表达式。', example: '香港|HK' },
  'group.url': { hint: '自动测速使用的检测地址。', example: 'http://www.gstatic.com/generate_204' },
  'group.interval': { hint: '自动测速的间隔，单位为秒。', example: '300' },
  'group.include_all_sources': { hint: '将方案中所有机场的节点加入该分组。机场「全部节点」组导入时默认开启。' },
  'group.include_self': { hint: '将方案中选中的自建节点加入该分组。' },
  'rule.rule_type': { hint: '规则的匹配类型。', example: 'DOMAIN-SUFFIX' },
  'rule.rule_param': { hint: '规则的匹配内容。', example: 'example.com' },
  'rule.rule_config': { hint: '匹配后使用的目标分组；选择无分组时直连。' },
  'rule.resolve_dns': { hint: '是否在规则末尾附加 no-resolve。' },
  'rule.enabled': { hint: '只有已启用的规则会进入订阅输出。' },
  'token.name': { hint: '令牌名称，用于区分用途。', example: '手机' },
  'self-node.link': { hint: '标准节点分享链接。', example: 'vless://uuid@host:443?encryption=none&security=tls#香港' },
  'self-node.name': { hint: '输出到客户端的节点名称；留空时使用链接中的名称。', example: '自建-香港' },
  'self-node.enabled': { hint: '是否参与订阅输出。' },
  'script.name': { hint: '脚本名称，需唯一。', example: 'hk-filter' },
  'script.kind': { hint: '过滤脚本返回布尔值；改写脚本修改 node 字段。', example: 'filter' },
  'script.code': { hint: 'JavaScript 代码，仅提供 node 变量。', example: "node.name.indexOf('香港') >= 0" },
  'pipeline.name': { hint: '管线名称，需唯一。', example: 'hk-only' },
  'pipeline.step.kind': { hint: '处理步骤类型。', example: 'filter.regex' },
  'pipeline.step.param': { hint: '步骤参数。', example: "include=^HK|^JP" },
  'pipeline.step.on_error': { hint: '步骤失败时的处理方式：跳过、中止或记录告警。' },
  'script.enabled': { hint: '关闭后，引用该脚本的管线步骤会编译失败。' },
  'subscheck.enabled': { hint: '是否执行订阅可用性检测。' },
  'rules.manifest_path': { hint: '规则源清单文件的绝对路径。' },
  'rules.output_dir': { hint: '规则源版本文件的保存目录；留空时使用清单中的输出路径。' },
}

export function fieldHelp(key: string): { hint: string; example?: string } | undefined {
  return FIELD_HELP[key]
}

export const FAILURE_HELP: { intro: string; cases: FailureCase[] } = {
  intro: '失败原因会保留在响应中，界面按区域展示。部分来源失败不影响其他来源；余额刷新失败不会清零已保存的值；规则源更新失败会保留上一版本。',
  cases: [
    {
      scene: '转换时某个机场拉取失败',
      behavior: '该来源记入 failures，其余来源继续输出',
      ui: '转换页在 failures 区显示失败来源与原因，其他来源的节点正常输出',
    },
    {
      scene: '转换时全部来源失败',
      behavior: '返回 502，正文包含每条失败原因',
      ui: '页面保留 502 原文，并提示检查订阅地址是否可访问',
    },
    {
      scene: '来源成功但过滤后没有节点',
      behavior: '返回 404「没有可用节点：检查来源内容，或放宽 include/exclude/type 条件」',
      ui: '显示该提示，并建议放宽过滤条件',
    },
    {
      scene: 'failures 与 warnings 的区别',
      behavior: 'failures 是失败来源的原因；warnings 是渲染降级、模板、管线与覆盖提示',
      ui: '两个列表分区展示',
    },
    {
      scene: '流量余额刷新：单个机场失败',
      behavior: '只记录错误，不修改已保存的流量与到期值',
      ui: '该机场行显示错误原因，并说明余额仍为上次成功值',
    },
    {
      scene: '流量刷新返回',
      behavior: '返回每个机场的结果与成功、失败计数',
      ui: '刷新结果表逐行显示成功或错误，并显示计数',
    },
    {
      scene: '余额更新时间',
      behavior: '仅在成功获取流量信息时更新',
      ui: '显示上次成功时间；从未成功时显示「从未采集」',
    },
    {
      scene: '规则源更新：单个源失败',
      behavior: '该源保留上一版本；每个源保留最近 5 个版本',
      ui: '逐行显示错误，并说明该源保留上一版本',
    },
    {
      scene: '规则导入拉取失败',
      behavior: '返回 502 与错误原因',
      ui: '显示错误原文，并提示稍后重试',
    },
    {
      scene: '调度任务失败',
      behavior: '自动重试 3 次，仍失败时记录并告警',
      ui: '任务页显示重试规则与失败原因',
    },
    {
      scene: '管线单步失败',
      behavior: '由该步的失败处理方式决定跳过、中止或继续',
      ui: '每步显示失败处理选项与默认值',
    },
    {
      scene: '脚本失败',
      behavior: '试运行返回 ok:false 与错误原因；管线内按该步设置处理',
      ui: '试运行区域显示错误原因',
    },
  ],
}

export const ON_ERROR_HELP: { value: 'skip' | 'abort' | 'warn'; label: string; effect: string }[] = [
  { value: 'skip', label: '跳过', effect: '跳过该步骤，保留已处理结果并继续生成产物' },
  { value: 'abort', label: '中止', effect: '终止本次转换并返回错误' },
  { value: 'warn', label: '告警', effect: '记录告警并继续执行后续步骤' },
]

export interface ScriptVariable { name: string; type: string; writable: boolean; explain: string }
export interface ScriptReturnRule { kind: 'filter' | 'operate'; body: string; ret: string; how: string; good: string; bad: string }
export const SCRIPTS_HELP: {
  intro: string
  sandbox: string[]
  variables: ScriptVariable[]
  returnRules: ScriptReturnRule[]
  writeback: string[]
  examples: { name: string; kind: 'filter' | 'operate'; code: string; why: string; caution?: string }[]
} = {
  intro: '脚本使用 JavaScript（ES5.1）运行，只能访问 node 变量。过滤脚本返回布尔值，改写脚本修改 node 字段。',
  sandbox: [
    '脚本不能访问网络、文件或进程，只能使用 node 数据对象。',
    '每个节点在独立环境中执行，节点之间不能共享状态。',
    '支持 var、if、for、function 与数组、对象方法；不支持 import、export、async 或 await。',
    '单次执行最长 5 秒；超时后脚本会被终止。',
  ],
  variables: [
    { name: 'node.name', type: 'string', writable: true, explain: '节点名称。' },
    { name: 'node.type', type: 'string', writable: true, explain: '协议类型，例如 ss、vmess、vless、trojan、hysteria2。' },
    { name: 'node.server', type: 'string', writable: true, explain: '服务器地址。' },
    { name: 'node.port', type: 'string', writable: true, explain: '端口，使用字符串形式。' },
    { name: 'node.source', type: 'string', writable: false, explain: '节点来源标识，只读。' },
    { name: 'node.raw', type: 'object', writable: true, explain: '节点的完整原始配置，协议字段保存在这里。' },
  ],
  returnRules: [
    {
      kind: 'filter',
      body: '表达式',
      ret: '表达式结果转换为布尔值：真表示保留，假表示丢弃。',
      how: '返回值即为结果，不能使用顶层 return。',
      good: "node.name.indexOf('香港') >= 0",
      bad: "return node.name.indexOf('香港') >= 0",
    },
    {
      kind: 'operate',
      body: '语句',
      ret: '返回值不参与处理。',
      how: '必须修改 node 字段才会生效。',
      good: "node.name = '[HK] ' + node.name",
      bad: "node.name.indexOf('HK') >= 0",
    },
  ],
  writeback: [
    '修改 node.name、node.type、node.server 或 node.port 后，会同步到原始配置。',
    '其他协议字段通过 node.raw 修改，例如 node.raw.sni 或 node.raw[\'ws-opts\'].path。',
    'node.source 为只读字段，修改后不会生效。',
  ],
  examples: [
    { name: '按地区保留节点', kind: 'filter' as const, code: "node.name.indexOf('香港') >= 0 || node.name.indexOf('HK') >= 0", why: '过滤脚本返回布尔表达式；任一条件成立即保留节点。' },
    { name: '给节点名加地区前缀', kind: 'operate' as const, code: "node.name = '[HK] ' + node.name", why: '改写脚本通过赋值生效。' },
    { name: '统一修改端口', kind: 'operate' as const, code: 'node.port = 443', why: '修改顶层字段后会自动同步到原始配置。' },
    { name: '修改 SNI', kind: 'operate' as const, code: "if (node.raw.sni) { node.raw.sni = 'example.com' }", why: '协议字段通过 node.raw 修改；先判断字段存在。', caution: '仅对已有 sni 字段的节点生效。' },
  ],
}

export const PIPELINE_HELP = {
  intro: '管线由有序的处理步骤组成：先执行过滤步骤，再执行改写步骤，最后输出。订阅方案与转换页可以引用管线。',
  order: [
    '过滤步骤先执行，决定保留哪些节点。',
    '改写步骤后执行，修改保留下来的节点。',
    '脚本步骤可以引用已保存的脚本，也可以直接填写代码。',
  ],
  scriptStep: '脚本步骤需要填写已保存且已启用的脚本名称；引用不存在的脚本会导致编译失败。',
  steps: [
    { kind: 'filter.regex', label: '正则过滤', hint: '按节点名称匹配；include 与 exclude 使用正则表达式，逗号分隔。', example: 'include=^HK|^JP' },
    { kind: 'filter.region', label: '地区过滤', hint: '按内置地区关键词匹配节点名称。', example: 'include=香港,日本' },
    { kind: 'filter.type', label: '协议过滤', hint: '按节点协议类型过滤。', example: 'include=vmess,trojan' },
    { kind: 'filter.script', label: '脚本过滤', hint: '使用脚本判断节点是否保留。', example: 'ref=hk-filter' },
    { kind: 'op.rename', label: '正则重命名', hint: '使用 pattern 匹配并替换节点名称，可使用 $1 等捕获组。', example: 'pattern= - .*$ repl=' },
    { kind: 'op.emoji', label: '添加地区 Emoji', hint: '按节点名称匹配并替换为 Emoji。', example: 'pattern=香港 repl=🇭🇰' },
    { kind: 'op.dedupe', label: '去重', hint: '按节点指纹去重，不需要参数。', example: 'op.dedupe' },
    { kind: 'op.split', label: '截断节点名', hint: '按第一个分隔符截断节点名称；默认分隔符为 " - "。', example: 'sep=|' },
    { kind: 'op.sort', label: '排序', hint: '按指定字段排序。', example: 'by=name' },
    { kind: 'op.script', label: '脚本改写', hint: '使用脚本修改节点字段。', example: 'ref=hk-prefix' },
  ],
}
