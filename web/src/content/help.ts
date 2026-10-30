// 文案中枢：所有页面/字段/脚本/管线的说明文案集中在此，组件与页面只引用不重写。
// 见 docs/design/ui-copy-and-config-plan.md §2.2 冻结接口。

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
    oneLiner: '总览订阅、机场、规则与近期任务的运行状态。',
    whenToUse: '打开 Web 界面后的第一站，确认整体健康度或快速跳转到其他页面。',
    steps: [
      { title: '看指标', detail: '顶部卡片显示节点数、原始节点数、分组数、规则数、缓存命中率、来源 URL 总数、日志行数与最近记录条数；卡片不可点击。' },
      { title: '看近期活动', detail: '下方是「最近转换历史」表（内存态，最多 20 条，重启清空）；展开行显示该次的 failures 与 warnings。' },
    ],
    concepts: [
      { term: '缓存命中率', explain: '最近转换请求里命中缓存的占比，低于 90% 说明上游内容在频繁变化。' },
      { term: '最近记录条数', explain: '内存里保留的转换历史条数，上限 20，进程重启后清零。' },
    ],
    faq: [
      { q: '指标为 0 怎么办？', a: '按首次使用路径：先加机场，再配规则，最后建订阅。' },
    ],
    emptyState: { title: '还没有任何数据', detail: '先到「机场」页添加订阅地址，再到「规则」页配置分流，最后在「订阅」页生成令牌。', action: { label: '去添加机场', to: '/airports' } },
    links: [
      { label: '机场', to: '/airports', note: '添加上游订阅地址' },
      { label: '规则', to: '/rules', note: '配置分流规则' },
      { label: '订阅', to: '/subscriptions', note: '生成可访问的订阅 URL' },
    ],
  },
  convert: {
    page: 'convert',
    title: '转换',
    oneLiner: '把一个订阅地址转换成目标客户端配置并预览。',
    whenToUse: '调试规则或机场是否生效，或手动下载一次转换结果。',
    steps: [
      { title: '填入订阅地址', detail: '在「订阅 URL（每行一个）」输入框粘贴上游 URL；也可用「粘贴文本」；可同时填多个，每行一个。' },
      { title: '选择目标格式', detail: 'target 字段决定输出格式（clash.meta/sing-box/surge 等），留空默认 clash.meta（Mihomo/Clash.Meta）。' },
      { title: '点击转换', detail: '查看输出的生效格式、节点数（原始节点数）与缓存命中；错误会直接显示在页面。diff 需手动粘贴两段产物对比。' },
    ],
    concepts: [
      { term: 'target', explain: '输出配置的目标客户端格式，例如 sing-box、clash、surge。' },
      { term: 'diff', explain: '手动粘贴基线 A 与当前 B 两段产物做逐行对比，用于确认改动是否生效。' },
    ],
    faq: [
      { q: 'target 留空默认什么？', a: '默认 clash.meta（Mihomo/Clash.Meta）。注意 /sub 订阅端点不看这个默认值，它会按客户端 User-Agent 自动推断目标格式。' },
      { q: '有一个订阅拉不到会怎样？', a: '部分来源失败不影响其他来源：失败的来源会记入 failures 并显示原因原文，其余来源照常输出节点。只有全部来源都失败才会返回 502。' },
    ],
    emptyState: { title: '还没有转换结果', detail: '填入订阅地址并点击「转换」即可预览输出。', action: { label: '开始转换', to: '/convert' } },
    links: [
      { label: '机场', to: '/airports', note: '复用已保存的机场' },
      { label: '管线', to: '/pipelines', note: '把转换步骤保存为可复用管线' },
    ],
  },
  pipelines: {
    page: 'pipelines',
    title: '管线',
    oneLiner: '把一组转换步骤（filter/operate）保存为可复用的管线。',
    whenToUse: '需要重复执行相同转换流程，或在订阅里引用固定处理链时。',
    steps: [
      { title: '新建管线', detail: '点击「新增」，填写名称。' },
      { title: '添加步骤', detail: '按顺序添加 filter/operate/脚本步骤，每步可上下移动。' },
      { title: '保存', detail: '保存后可在订阅页引用；编译错误会直接显示。' },
    ],
    concepts: [
      { term: 'filter', explain: '链式过滤：每步返回布尔值，决定节点是否保留。' },
      { term: 'operate', explain: '链式改写：每步修改节点字段，返回值被忽略。' },
    ],
    faq: [
      { q: '管线和单次转换什么区别？', a: '管线把转换步骤保存为可复用模板，单次转换是一次性执行。' },
    ],
    emptyState: { title: '还没有管线', detail: '点击「新增」创建第一条管线，或从模板导入。', action: { label: '新增管线', to: '/pipelines' } },
    links: [
      { label: '脚本', to: '/scripts', note: '在管线里引用已保存的脚本' },
      { label: '订阅', to: '/subscriptions', note: '订阅可引用管线' },
    ],
  },
  scripts: {
    page: 'scripts',
    title: '脚本',
    oneLiner: '用 JavaScript（goja）编写 filter/operate 脚本，嵌入转换流程。',
    whenToUse: '内置步骤不够用，需要自定义过滤或改写逻辑时。',
    steps: [
      { title: '新建脚本', detail: '点击「新增」，选择类型 filter 或 operate。' },
      { title: '编写代码', detail: 'filter 写布尔表达式，operate 写赋值语句；可点「插入示例」按钮打开内置示例列表并插入。' },
      { title: '试运行', detail: '用内置节点样例试运行，确认保留/丢弃或改写结果。' },
    ],
    concepts: [
      { term: 'filter 脚本', explain: '脚本体是表达式，返回值转布尔决定节点是否保留。' },
      { term: 'operate 脚本', explain: '脚本体是语句，靠修改 node 字段生效，返回值被忽略。' },
    ],
    emptyState: { title: '还没有脚本', detail: '点击「新建脚本」，或在编辑弹窗里点「插入示例」按钮创建第一条脚本。', action: { label: '新增脚本', to: '/scripts' } },
    links: [
      { label: '管线', to: '/pipelines', note: '在管线步骤里引用脚本' },
      { label: '转换', to: '/convert', note: '试跑完整转换' },
    ],
  },
  airports: {
    page: 'airports',
    title: '机场',
    oneLiner: '管理上游订阅地址（机场），供转换与订阅使用。',
    whenToUse: '新增或维护机场节点来源。',
    steps: [
      { title: '新增机场', detail: '点击「新增」，填名称与订阅地址。' },
      { title: '刷新流量', detail: '点击「刷新」拉取 subscription-userinfo 流量与节点数。' },
    ],
    faq: [
      { q: '余额显示上次的值是不是坏了？', a: '不是。余额刷新失败时只记录 traffic_err，不会清零已存的 upload/download/total/expire，所以显示的是上次成功采集的值；修好上游后重新刷新即可。' },
    ],
    concepts: [
      { term: '机场', explain: '提供订阅地址的上游服务商，SubForge 从其拉取节点。' },
      { term: 'subscription-userinfo', explain: '上游返回的流量/到期信息头，用于聚合显示。' },
    ],
    emptyState: { title: '还没有机场', detail: '点击「新增」添加第一条机场订阅地址。', action: { label: '新增机场', to: '/airports' } },
    links: [
      { label: '订阅', to: '/subscriptions', note: '机场节点会进入订阅输出' },
    ],
  },
  'self-nodes': {
    page: 'self-nodes',
    title: '自建节点',
    oneLiner: '管理手动添加的节点，与机场节点合并输出。',
    whenToUse: '有机场没有的节点需要手动加入时。',
    steps: [
      { title: '新增节点', detail: '点击「新增」，填名称与分享链接（ss:// / vmess:// / vless:// / trojan:// / hysteria2:// 等）。' },
      { title: '启用/停用', detail: '通过开关控制是否参与输出。' },
    ],
    concepts: [
      { term: '自建节点', explain: '不来自机场、由用户手动维护的节点，与机场节点合并后输出。' },
      { term: '合并输出', explain: '自建节点与机场节点合并后一起进入订阅输出，不再单独区分来源。' },
    ],
    faq: [
      { q: '自建节点和机场节点会冲突吗？', a: '不会，二者合并后一起输出；同名时先出现的机场节点保留，自建节点在后。' },
    ],
    emptyState: { title: '还没有自建节点', detail: '点击「新增」添加第一条自建节点。', action: { label: '新增节点', to: '/self-nodes' } },
    links: [
      { label: '机场', to: '/airports', note: '机场节点与自建节点合并输出' },
      { label: '订阅', to: '/subscriptions', note: '自建节点进入订阅' },
    ],
  },
  rules: {
    page: 'rules',
    title: '规则',
    oneLiner: '管理分流规则与规则集，决定节点如何分组。',
    whenToUse: '配置或调整流量分流策略。',
    steps: [
      { title: '新增规则', detail: '点击「新增」，选择规则类型并填参数。' },
      { title: '批量启用', detail: '通过开关控制规则是否生效。' },
    ],
    concepts: [
      { term: '规则', explain: '单条分流条件，匹配后把节点归入指定分组。' },
      { term: '规则集', explain: '一组规则的集合，可批量启用/停用。' },
    ],
    emptyState: { title: '还没有规则', detail: '点击「新增」添加第一条规则。', action: { label: '新增规则', to: '/rules' } },
    links: [
      { label: '规则源', to: '/rule-sources', note: '从外部来源导入规则集' },
    ],
  },
  'rule-sources': {
    page: 'rule-sources',
    title: '规则源',
    oneLiner: '把社区维护的分流规则集拉到本地、定时更新、可回滚；点「新增规则源」即可开始，首次会自动创建清单。',
    whenToUse: '想用 ACL4SSR、blackmatrix7 等社区规则文件、不想手写规则时；规则源支持规则文件与 INI ruleset（input_format=ini）。',
    steps: [
      { title: '新增规则源', detail: '点「新增规则源」填 URL；首次新增会自动创建 manifest 并写入路径，不需要先生成默认清单。' },
      { title: '全部更新', detail: '把每个规则源的最新版本拉到本地，每个源保留最近 5 个版本，可随时回滚。' },
      { title: '去方案绑定', detail: '在「方案」页绑定规则源；产物会内联进订阅。规则页只负责查看/启用规则，不是规则源的引用入口。' },
    ],
    faq: [
      { q: '上游挂了规则集会变空吗？', a: '不会。单个规则源更新失败时该源保留上一版本，产物不受影响；每个源保留最近 5 个版本，失败原因会逐行显示。' },
      { q: '规则源支持 INI 吗？', a: '支持。input_format=ini 或 auto 命中 INI 时，会先展开 ruleset 再按 output.format 落盘；落盘扩展名：clash-meta → .yaml，clash-json / sing-box → .json，mrs → .mrs，text → .txt。' },
      { q: 'INI 转换模板在哪里管理？', a: 'INI 转换模板（不是规则源）由启动参数 -config 指定；已指定时可在「设置」页预览/编辑并热重载，未指定时纯控制台模式不需要模板。' },
    ],
    concepts: [
      { term: '规则源', explain: '外部规则文件的 URL（如 ACL4SSR、blackmatrix7），SubForge 定期拉取并按清单里的 output.format 落盘。' },
      { term: '落盘格式', explain: '规则源文件按清单 output.format 落盘：clash-meta → .yaml，clash-json / sing-box → .json，mrs → .mrs，text → .txt；INI 输入会先展开 ruleset。' },
      { term: '规则源与规则的关系', explain: '规则源页只负责「下载与版本管理」；在「方案」页绑定后，产物规则会内联进订阅。' },
      { term: 'MRS', explain: 'Meta Rule Set，mihomo（Clash.Meta）的二进制规则集格式，体积小、加载快；sing-box 用的是另一种 .srs 格式。' },
    ],
    emptyState: { title: '还没有规则源', detail: '点右上角「新增规则源」即可；首次新增会自动创建清单并写入路径。' },
    links: [
      { label: '设置', to: '/settings', note: '修改清单路径' },
      { label: '方案', to: '/profiles', note: '把规则源绑定到订阅方案' },
    ],
  },
  groups: {
    page: 'groups',
    title: '分组',
    oneLiner: '管理输出分组，决定订阅里节点如何组织。',
    whenToUse: '调整订阅输出的分组结构。',
    steps: [
      { title: '新增分组', detail: '点击「新增」，填名称与类型。' },
      { title: '排序', detail: '调整分组顺序影响输出顺序。' },
    ],
    concepts: [
      { term: '分组', explain: '订阅输出中节点的分类单元，可按规则或手动归类。' },
      { term: '分组类型', explain: 'select 手动选择、url-test 自动测活选最快、fallback 故障切换、load-balance 负载均衡；渲染层还支持 relay/direct/pass/reject。' },
      { term: '成员扩展', explain: '机场的「全部节点」组默认并入方案里所有机场的节点；自建节点由「包含自建节点」单独控制，子集组保持原成员。' },
    ],
    faq: [
      { q: '分组顺序影响什么？', a: '影响订阅输出中分组的排列顺序，靠前的分组在前端展示更醒目。' },
    ],
    emptyState: { title: '还没有分组', detail: '点击「新增」添加第一条分组。', action: { label: '新增分组', to: '/groups' } },
    links: [
      { label: '规则', to: '/rules', note: '规则决定节点归入哪个分组' },
      { label: '订阅', to: '/subscriptions', note: '分组影响订阅输出结构' },
    ],
  },
  subscriptions: {
    page: 'subscriptions',
    title: '订阅',
    oneLiner: '管理订阅令牌与 URL，控制输出与流量聚合。',
    whenToUse: '生成可分享的订阅链接或调整流量聚合策略。',
    steps: [
      { title: '新增令牌', detail: '点击「新增」，填名称。' },
      { title: '复制 URL', detail: '复制生成的订阅地址给客户端使用。' },
    ],
    concepts: [
      { term: '令牌', explain: '订阅访问凭证，每个令牌对应一个独立的订阅 URL；同一地址可长期重复使用，只有删除令牌才会失效。' },
      { term: 'subscription-userinfo', explain: '聚合后的流量/到期信息，写入响应头供客户端显示。' },
    ],
    faq: [
      { q: '生成那么多令牌有什么用？', a: '令牌是订阅地址里的访问凭据：客户端拿这个 URL 拉取你的转换结果，一个令牌对应一个独立地址，用来区分不同设备或用途。同一地址可长期重复使用，不必每次使用都新建令牌；只有泄露或要撤销访问时，才删除并重建。' },
    ],
    emptyState: { title: '还没有订阅', detail: '点击「新增」生成第一条订阅令牌。', action: { label: '新增订阅', to: '/subscriptions' } },
    links: [
      { label: '机场', to: '/airports', note: '机场节点进入订阅' },
      { label: '管线', to: '/pipelines', note: '订阅可引用管线' },
    ],
  },
  tasks: {
    page: 'tasks',
    title: '任务',
    oneLiner: '查看调度器最近执行过的任务记录（内存态）。',
    whenToUse: '需要定期执行转换/刷新等操作时。',
    steps: [
      { title: '看任务记录', detail: '表格显示任务名、状态、开始时间、耗时与错误原文；记录保存在内存里，进程重启清空。' },
      { title: '手动跑一次', detail: '本页没有触发按钮：去「规则源」页点「立即更新」，或去「机场」页点「测活」。' },
    ],
    concepts: [
      { term: 'cron', explain: '定时表达式，格式「分 时 日 月 周」，例如 "0 3 * * *" 表示每天 3 点。' },
      { term: 'cron 默认值', explain: '规则源更新默认 0 4 * * *（rules.update_cron），订阅测活默认 */30 * * * *（subscheck.cron，受 subscheck.enabled 控制）。' },
    ],
    faq: [
      { q: '任务失败会重试吗？', a: '会。失败自动重试 3 次（指数退避），仍失败才告警；也可手动「运行」或等下一次 cron 触发。' },
    ],
    emptyState: { title: '还没有任务', detail: '任务按 cron 到点才执行；也可去「规则源」页点「立即更新」立刻产生一条记录。', action: { label: '去规则源', to: '/rule-sources' } },
    links: [
      { label: '日志', to: '/logs', note: '任务执行记录在日志查看' },
    ],
  },
  settings: {
    page: 'settings',
    title: '设置',
    oneLiner: '配置 5 项运行参数，并查看/编辑转换模板（需 -config）。',
    whenToUse: '调整数据目录、规则清单路径、开关，或编辑 INI 转换模板。',
    steps: [
      { title: '查看设置', detail: '页面列出所有可配置项与当前值/默认值。' },
      { title: '修改并保存', detail: '修改后立即 PUT /api/settings，回显刷新。' },
      { title: '转换模板', detail: '启动时指定 -config 后，可在设置页预览/编辑 INI 模板；保存后约 1 秒热重载。未指定时纯控制台模式不需要模板。' },
    ],
    concepts: [
      { term: '默认值', explain: '未显式配置时使用的值，显示「使用默认值」标记。' },
      { term: '即时生效', explain: '修改后立即 PUT /api/settings 保存，无需重启服务。' },
      { term: '模板热重载', explain: '模板路径与启动时的 -config 相同时，保存后服务轮询到变化并重载模板。' },
    ],
    faq: [
      { q: '改了设置要重启吗？', a: '不用，修改后立即保存并生效。' },
    ],
    emptyState: { title: '暂无设置项', detail: '刷新页面重试。', action: { label: '刷新', to: '/settings' } },
    links: [
      { label: '规则源', to: '/rule-sources', note: '规则清单路径影响此页' },
    ],
  },
  logs: {
    page: 'logs',
    title: '日志',
    oneLiner: '查看运行日志（内存环形缓冲，最多 200 行，可手动刷新或打开自动刷新）。',
    whenToUse: '排查故障或追踪近期操作。',
    steps: [
      { title: '看日志', detail: '本页不区分级别：所有日志按时间倒序显示，error/warn 只是前端关键词高亮，没有单独的级别过滤。' },
      { title: '自动刷新', detail: '打开「自动刷新」后每 3 秒拉取一次（默认关闭）；最多显示最近 200 行，日志行不可点击。' },
    ],
    concepts: [
      { term: '高亮', explain: '前端对包含 error/warn 等关键词的行做颜色高亮，仅用于扫读，不代表日志级别。' },
      { term: '容量', explain: '日志保存在内存环形缓冲里，/api/logs 一次最多返回 200 行，进程重启清空。' },
    ],
    faq: [
      { q: '日志保留多久？', a: '日志保存在内存里，接口一次最多返回最近 200 行，进程重启后清空；不会写日志文件。' },
    ],
    emptyState: { title: '暂无日志', detail: '运行操作后日志会出现在这里。', action: { label: '刷新', to: '/logs' } },
    links: [
      { label: '任务', to: '/tasks', note: '手动或定时任务触发的操作会产生日志' },
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
  { term: '节点', explain: '单个代理服务器条目，含名称/类型/地址/端口等。' },
  { term: '订阅', explain: 'SubForge 生成的可访问 URL，客户端用它拉取配置。' },
  { term: '机场', explain: '提供订阅地址的上游服务商。' },
  { term: '规则', explain: '单条分流条件，匹配后把节点归入指定分组。' },
  { term: '规则集', explain: '一组规则的集合，可批量启用/停用。' },
  { term: '规则源', explain: '外部规则文件的 URL（如 ACL4SSR、blackmatrix7），SubForge 定期拉取并按清单里的 output.format 落盘。' },
  { term: '管线', explain: '有序的转换步骤链（filter → operate），可复用。' },
  { term: '转换', explain: '把上游订阅按规则处理成客户端配置的过程。' },
  { term: '令牌', explain: '订阅访问凭证，每个令牌对应一个独立订阅 URL。' },
  { term: 'subscription-userinfo', explain: '上游返回的流量/到期信息头，用于聚合显示。' },
  { term: 'MRS', explain: 'Meta Rule Set，mihomo（Clash.Meta）的二进制规则集格式，体积小、加载快；sing-box 用的是另一种 .srs 格式。' },
  { term: 'ETag', explain: 'HTTP 缓存标识，SubForge 用它判断规则源是否更新。' },
  { term: '测活', explain: '探测节点是否可达，结果用于过滤或排序。' },
  { term: '沙箱', explain: '脚本运行环境，无网络/文件/进程，5s 超时。' },
]

export function glossaryFor(term: string): string {
  const hit = GLOSSARY.find((g) => g.term === term)
  return hit !== undefined ? hit.explain : ''
}

export const FIELD_HELP: Record<string, { hint: string; example?: string }> = {
  'airport.name': { hint: '机场显示名称，便于识别', example: '香港 IEPL' },
  'airport.subscription_url': { hint: '上游订阅地址，支持 http/https', example: 'https://example.com/sub?token=xxx' },
  'airport.remarks': { hint: '备注信息，可选', example: '主机场' },
  'airport.with_rules': {
    hint: '勾选后，「刷新全部流量」会顺带把每个启用机场的规则保存/更新为版本化规则源（不再写入规则页）；不勾则只刷新节点与流量。想单独更新某个机场，用表格行内的「更新规则源」按钮。',
    example: '',
  },
  'airport.enabled': { hint: '是否参与转换与输出' },
  'airport.traffic_err': { hint: '最近一次余额采集的错误原文；非空时下面的余额是上次成功值，不是 0' },
  'airport.traffic_at': { hint: '最近一次成功采到余额的 Unix 秒；为 0 表示从未采到' },
  'group.name': { hint: '分组名称', example: '香港' },
  'group.type': { hint: '分组类型（select/url-test 等）', example: 'select' },
  'group.regex': { hint: '匹配节点名称的正则', example: '香港|HK' },
  'group.url': { hint: 'url-test 的测活地址', example: 'http://www.gstatic.com/generate_204' },
  'group.interval': { hint: '测活间隔（秒）', example: '300' },
  'rule.rule_type': { hint: '规则类型（DOMAIN/IP/GEOSITE 等）', example: 'DOMAIN-SUFFIX' },
  'rule.rule_param': { hint: '规则参数（域名/IP/关键词）', example: 'google.com' },
  'rule.rule_config': { hint: '目标代理分组的 id（不是 JSON）；0 或不存在则该行目标为 DIRECT', example: '1' },
  'rule.resolve_dns': { hint: '是否解析域名' },
  'rule.enabled': { hint: '是否启用该规则' },
  'token.name': { hint: '令牌名称，便于识别', example: '手机' },
  'self-node.link': { hint: '节点分享链接', example: 'vless://uuid@host:443?...' },
  'self-node.name': { hint: '节点显示名称', example: '自建-香港' },
  'self-node.enabled': { hint: '是否参与输出' },
  'script.name': { hint: '脚本名称，唯一', example: 'hk-filter' },
  'script.kind': { hint: 'filter（返回布尔）或 operate（改写字段）', example: 'filter' },
  'script.code': { hint: 'JavaScript 代码，仅 node 变量可用', example: 'node.name.indexOf("香港") >= 0' },
  'pipeline.name': { hint: '管线名称，唯一', example: 'hk-only' },
  'pipeline.step.kind': { hint: '步骤类型（filter.regex/region/type/script、op.rename/emoji/dedupe/split/sort/script）', example: 'filter.regex' },
  'pipeline.step.param': { hint: '步骤参数', example: 'node.name.indexOf("香港") >= 0' },
  'pipeline.step.on_error': { hint: '该步失败时怎么办：skip 跳过 / abort 中止（默认）/ warn 告警继续' },
  'script.enabled': { hint: '是否启用脚本步骤；关闭时 ref= 脚本步骤编译报错' },
  'subscheck.enabled': { hint: '是否启用订阅测活；关闭后定时测活不再执行' },
  'rules.manifest_path': { hint: '规则源清单 manifest.yaml 的绝对路径' },
  'rules.output_dir': { hint: '规则集版本文件的落盘目录（留空则用 manifest 里的 output.path）' },
}

export function fieldHelp(key: string): { hint: string; example?: string } | undefined {
  return FIELD_HELP[key]
}

export const FAILURE_HELP: { intro: string; cases: FailureCase[] } = {
  intro: '失败不会静默：后端把每一次失败的原因原文留在响应里，界面负责分区展示。部分来源失败不影响其他来源，余额刷新失败不会清零已存值，规则源单源失败会保留上一版本。',
  cases: [
    {
      scene: '转换时某个机场拉取失败',
      behavior: '该来源记入 failures，其余来源照常输出',
      ui: '转换页在 failures 区显示失败来源与原因原文，其他来源的节点正常输出',
    },
    {
      scene: '转换时全部来源失败',
      behavior: '返回 502，正文为「所有来源都失败:」加每条原因',
      ui: '页面保留 502 原文，并引导检查订阅地址是否可访问',
    },
    {
      scene: '来源成功但过滤后无节点',
      behavior: '返回 404「没有可用节点：检查来源内容，或放宽 include/exclude/type 条件」',
      ui: '显示该文案，提示放宽 include/exclude/type 过滤条件',
    },
    {
      scene: '转换响应的 failures 与 warnings 区别',
      behavior: 'failures 是失败来源的原因；warnings 是渲染降级项、模板告警、管线告警与覆盖告警，模板告警排最前',
      ui: '两个数组分区展示原文，不要混在一起',
    },
    {
      scene: '流量余额刷新：单机场失败',
      behavior: '只写 traffic_err，不动 upload/download/total/expire；写 fetch_audit；单个机场失败不炸整批',
      ui: '该机场行显示错误原文，并说明「余额仍是上次成功值，没有被清零」',
    },
    {
      scene: '流量刷新返回',
      behavior: '返回 results（每项含 airport_id/name/ok/usage/node_count/err）与 ok/fail 计数',
      ui: '刷新结果表逐行显示 ok/err，并显示成功/失败计数',
    },
    {
      scene: '余额新鲜度',
      behavior: 'traffic_at 只在成功且拿到 subscription-userinfo 时更新',
      ui: '显示「上次成功：绝对时间（相对时间前）」；traffic_at 为 0 时显示「从未采集」',
    },
    {
      scene: '规则源更新：单源失败',
      behavior: 'UpdateResult.Err 非空，该源保留上一版本；版本保留最近 5 个',
      ui: '逐行显示 err，并说明「该源保留上一版本，产物不受影响」',
    },
    {
      scene: '规则导入拉取失败',
      behavior: '返回 502「导入失败: 」加错误原文',
      ui: '显示 502 原文，提示稍后重试',
    },
    {
      scene: '调度任务失败',
      behavior: '自动重试 3 次（指数退避），仍失败才告警；JobResult.Err 非空',
      ui: '任务页写明「失败自动重试 3 次（指数退避），仍失败才告警」',
    },
    {
      scene: '管线单步失败',
      behavior: 'on_error 段落级默认（空 = abort），单步 on_error= 覆盖；取值 skip/abort/warn，非法值报错',
      ui: '每步显示 on_error 下拉与三值含义，并标注默认 abort',
    },
    {
      scene: '脚本失败',
      behavior: '试运行返回 HTTP 200 且 ok:false 加 error 原文；管线内按该步 on_error 处理',
      ui: '试运行区显示 error 原文；管线内按该步 on_error 决定跳过/中止/告警',
    },
  ],
}

export const ON_ERROR_HELP: { value: 'skip' | 'abort' | 'warn'; label: string; effect: string }[] = [
  { value: 'skip', label: '跳过', effect: '跳过该步，保留已处理结果，产物继续生成' },
  { value: 'abort', label: '中止', effect: '中止整次转换并报错' },
  { value: 'warn', label: '告警', effect: '记录告警，继续执行后续步骤' },
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
  intro: '脚本使用 goja 引擎运行 JavaScript（ES5.1），只能操作 node 变量。filter 脚本写表达式，operate 脚本写赋值语句。',
  sandbox: [
    '无 require / 网络 / 文件 / 进程 / console：只有纯数据对象 node，不能加载外部模块或访问外部资源。',
    '每个节点一个全新的执行环境：节点之间不能共享状态，不能靠全局变量跨节点累加计数。',
    '引擎是 goja（ES5.1+）：支持 var / if / for / function / 数组对象方法；不支持 import / export / async / await。',
    '执行上限 5 秒（可配置）：死循环或长时间计算会被 vm.Interrupt 打断 → 脚本 %q 被中断（超时或死循环）。',
  ],
  variables: [
    { name: 'node.name', type: 'string', writable: true, explain: '节点名，派生自原始配置的 name。' },
    { name: 'node.type', type: 'string', writable: true, explain: '协议类型：ss / ssr / vmess / vless / trojan / hysteria2 / anytls / tuic 等。' },
    { name: 'node.server', type: 'string', writable: true, explain: '服务器地址（域名或 IP）。' },
    { name: 'node.port', type: 'string', writable: true, explain: '端口。是字符串（如 \'443\'），不是数字；写回时也会被转成字符串。' },
    { name: 'node.source', type: 'string', writable: false, explain: '节点来源标识（机场名或导入批次），只读，用于溯源，改了不生效。' },
    { name: 'node.raw', type: 'object', writable: true, explain: '该节点的完整原始配置。协议相关字段全在这里：uuid / id / password / sni / servername / alterId / cipher / ws-opts / tls / network / skip-cert-verify 等。' },
  ],
  returnRules: [
    {
      kind: 'filter',
      body: '表达式（一个值）',
      ret: '取整个表达式的完成值并 .ToBoolean()：真 → 保留该节点，假 → 丢弃。',
      how: '返回值即结果，不能写顶层 return（顶层 return 是语法错误，编译期就报 脚本 %q 编译失败）。',
      good: "node.name.indexOf('香港') >= 0",
      bad: "return node.name.indexOf('香港') >= 0（顶层 return 是语法错误，编译期就报 脚本 %q 编译失败）",
    },
    {
      kind: 'operate',
      body: '语句块',
      ret: '返回值被完全忽略。',
      how: '必须赋值给 node.* 才生效。',
      good: "node.name = '[HK] ' + node.name",
      bad: "node.name.indexOf('HK') >= 0（只算了个布尔值，什么都没改，节点原样输出）",
    },
  ],
  writeback: [
    '顶层字段 node.name / node.type / node.server / node.port 写回后覆盖 node.raw 里的同名字段。',
    '其他协议字段必须写 node.raw.xxx，例如 node.raw.sni = \'example.com\'、node.raw[\'ws-opts\'].path = \'/ws\'。',
    'raw 永远是唯一事实源：写回后由 model.NodeFromRaw 从 raw 重新派生只读视图。',
    '只读字段 node.source 改了不生效。',
  ],
  examples: [
    { name: '按地区保留节点', kind: 'filter' as const, code: "node.name.indexOf('香港') >= 0 || node.name.indexOf('HK') >= 0", why: 'filter 脚本要返回布尔表达式；|| 两侧任一命中即保留。' },
    { name: '给节点名加地区前缀', kind: 'operate' as const, code: "node.name = '[HK] ' + node.name", why: 'operate 靠赋值生效，返回值被忽略。' },
    { name: '统一改端口', kind: 'operate' as const, code: 'node.port = 443', why: '改顶层字段即可；raw.port 会自动同步。' },
    { name: '改写 SNI', kind: 'operate' as const, code: "if (node.raw.sni) { node.raw.sni = 'example.com' }", why: '非顶层字段走 node.raw；先判存在避免给无 SNI 的节点写入。', caution: '仅对已有 sni 字段的节点生效。' },
  ],
}

export const PIPELINE_HELP = {
  intro: '管线是一组有序的转换步骤：先 filter 链过滤节点，再 operate 链改写节点，最后输出。订阅可引用管线。',
  order: [
    'filter 步骤先执行，决定保留哪些节点。',
    'operate 步骤后执行，改写保留节点的字段。',
    'script 步骤可引用已保存的脚本（ref=<脚本名>），也可内联 src。',
  ],
  scriptStep: '管线里的脚本步骤写 ref=<脚本名>；引用的脚本必须存在且启用，否则编译期报错。',
  steps: [
    { kind: 'filter.regex', label: '正则过滤', hint: 'include= 与 exclude= 用正则匹配节点名，逗号分隔', example: 'include=^HK|^JP' },
    { kind: 'filter.region', label: '地区过滤', hint: '按内置地区关键词匹配节点名，include=/exclude= 逗号分隔', example: 'include=香港,日本' },
    { kind: 'filter.type', label: '协议过滤', hint: '按节点协议类型过滤，include=/exclude= 逗号分隔', example: 'include=vmess,trojan' },
    { kind: 'filter.script', label: '脚本过滤', hint: 'JS 表达式，真则保留；用 ref=<脚本名> 或 src=<代码>', example: 'ref=hk-filter' },
    { kind: 'op.rename', label: '正则重命名', hint: 'pattern= 正则，repl= 替换文本（可用 $1 捕获组）', example: 'pattern= - .*$ repl=' },
    { kind: 'op.emoji', label: '加国旗 Emoji', hint: '同 op.rename：pattern= 匹配、repl= 替换为 Emoji', example: 'pattern=香港 repl=🇭🇰' },
    { kind: 'op.dedupe', label: '去重', hint: '按节点指纹去重，无参数', example: 'op.dedupe' },
    { kind: 'op.split', label: '截断节点名', hint: 'sep= 分隔符，按第一个分隔符截断；缺省为 " - "', example: 'sep=|' },
    { kind: 'op.sort', label: '排序', hint: 'by= 排序键：original / name / type / server / port', example: 'by=name' },
    { kind: 'op.script', label: '脚本改写', hint: 'JS 语句，赋值 node.* 生效；用 ref=<脚本名> 或 src=<代码>', example: 'ref=hk-prefix' },
  ],
}
