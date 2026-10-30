<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import PageHeader from '../components/PageHeader.vue'
import AsyncSection from '../components/AsyncSection.vue'
import InfoTip from '../components/InfoTip.vue'
import { pageHelp } from '../content/help'
import { endpoints, type Profile } from '../api/endpoints'
import { notify } from '../composables/useNotify'
import { errorMessage } from '../composables/useAsync'

const help = pageHelp('profiles')

// 模板列的显示上限：超过就截断，完整内容放在 title 里。
const TEMPLATE_MAX = 60

// 层键：规则层与分组层共用同一组键；数组顺序 = 优先级从高到低。
type LayerKey = 'custom' | 'airport' | 'template'

interface LayerRow {
  key: LayerKey
  enabled: boolean
}

// 层编辑器配置：规则层锁定 template 恒在末位，分组层可自由排序。
interface LayerEditorConfig {
  testid: string
  // 编辑器前缀（D-43）：行级 testid 拼上它，区分规则层/分组层，避免页面上出现重复 testid。
  prefix: 'rule' | 'group'
  caption: string
  hint: string
  rows: LayerRow[]
  note: string
  move?: (index: number, delta: number) => void
  lockTemplate: boolean
}

const RULE_LAYER_KEYS: LayerKey[] = ['custom', 'template']
const GROUP_LAYER_KEYS: LayerKey[] = ['custom', 'airport', 'template']
const LAYER_LABELS: Record<string, string> = {
  custom: '自定义',
  airport: '机场导入',
  template: '模板',
}

// 超过这个量级说明 sort_order 是服务端回退的 UnixNano（D-41），前端显示「未设置」。
const SORT_ORDER_UNSET_LIMIT = 1e12

const EMPTY_PROFILE: Profile = {
  id: 0,
  name: '',
  description: '',
  config: '',
  pipeline: '',
  target: '',
  include: '',
  exclude: '',
  enabled: true,
  sort_order: 0,
  rule_layers: [...RULE_LAYER_KEYS],
  rule_sources: [],
  airport_ids: null,
  self_node_ids: null,
  group_layers: [...GROUP_LAYER_KEYS],
  rule_count: 0,
  group_count: 0,
}

const profiles = ref<Profile[]>([])
const loading = ref(false)
const error = ref('')

// 规则源选项（D-33）：方案可绑定规则源；选项来自 GET /api/rule-sources 的 sources[].name。
const ruleSourceOptions = ref<{ title: string; value: string; origin?: string; ruleCount?: number }[]>([])

// 节点来源选项：机场/自建节点。加载失败只降级为空选项，不阻断方案编辑。
const airportOptions = ref<{ title: string; value: number }[]>([])
const selfNodeOptions = ref<{ title: string; value: number }[]>([])
const targetOptions = ref<string[]>([])

async function loadTargetOptions(): Promise<void> {
  try {
    targetOptions.value = await endpoints.targets()
  } catch {
    targetOptions.value = []
  }
}

async function loadSourceOptions(): Promise<void> {
  const [airports, selfNodes] = await Promise.allSettled([endpoints.listAirports(), endpoints.listSelfNodes()])
  airportOptions.value = airports.status === 'fulfilled'
    ? airports.value.map((a) => ({ title: a.name + (a.enabled ? '' : '（已停用）'), value: a.id }))
    : []
  selfNodeOptions.value = selfNodes.status === 'fulfilled'
    ? selfNodes.value.map((n) => ({ title: n.name + (n.enabled ? '' : '（已停用）'), value: n.id }))
    : []
}

// editingId === 0 表示「新建」，否则为被编辑方案的 id。
const editingId = ref(0)
const dialog = ref(false)
const draft = ref({
  name: '',
  description: '',
  config: '',
  pipeline: '',
  target: '',
  include: '',
  exclude: '',
  enabled: true,
  sort_order: 0,
  // 规则源（D-33）：方案绑定的规则源名列表；空 = 未绑定。
  rule_sources: [] as string[],
  // 未限制来源 = 使用全部启用机场/自建节点；打开后才按下面的 id 多选限制。
  limit_sources: false,
  airport_ids: [] as number[],
  self_node_ids: [] as number[],
})

// 两个层编辑器的行状态（顺序即优先级）。
const ruleLayers = ref<LayerRow[]>(defaultLayers(RULE_LAYER_KEYS))
const groupLayers = ref<LayerRow[]>(defaultLayers(GROUP_LAYER_KEYS))

const confirmOpen = ref(false)
const deleteTarget = ref<Profile>(EMPTY_PROFILE)
const conflictDialogOpen = ref(false)

const headers = [
  { key: 'name', title: '名称' },
  { key: 'config', title: '模板' },
  { key: 'pipeline', title: '管线' },
  { key: 'rule_layers', title: '规则层' },
  { key: 'group_layers', title: '分组层' },
  { key: 'rule_sources', title: '规则源', sortable: false },
  { key: 'enabled', title: '启用' },
  { key: 'sort_order', title: '顺序' },
  { key: 'actions', title: '操作' },
]

// 规则层只有自定义与模板；机场规则是规则源，在下方显式绑定。
// 分组层保留机场层：机场自带分组仍由导入路径维护。
function defaultLayers(keys: LayerKey[]): LayerRow[] {
  return keys.map((key) => ({ key, enabled: true }))
}

// 后端把空数组/空串当默认值；前端同样把缺失或空数组还原成默认层。
// 顺序即优先级：保持后端给的顺序，未出现的层补在末尾并默认不启用。
function layersToRows(layers: string[] | undefined, keys: LayerKey[]): LayerRow[] {
  const raw = Array.isArray(layers) && layers.length > 0 ? layers : keys
  const known = raw.filter((k): k is LayerKey => (keys as string[]).includes(k))
  const missing = keys.filter((k) => !known.includes(k))
  return [...known, ...missing].map((key) => ({ key, enabled: known.includes(key) }))
}

// 规则层的 template 恒在末位（后端保存时也会强制移到最后）。
function ruleRowsFrom(layers: string[] | undefined): LayerRow[] {
  const rows = layersToRows(layers, RULE_LAYER_KEYS)
  const index = rows.findIndex((r) => r.key === 'template')
  if (index >= 0 && index !== rows.length - 1) {
    const [row] = rows.splice(index, 1)
    rows.push(row)
  }
  return rows
}

// 只保留启用的层，顺序即提交顺序。
function layerKeys(rows: LayerRow[]): LayerKey[] {
  return rows.filter((r) => r.enabled).map((r) => r.key)
}

// 优先级文案：启用中按启用顺序编号，未启用显示「未启用」。
function layerPriority(rows: LayerRow[], key: LayerKey): string {
  const index = rows.filter((r) => r.enabled).findIndex((r) => r.key === key)
  return index < 0 ? '未启用' : '优先级 ' + (index + 1)
}

// 分组层移动：同名分组先出现者胜出，顺序完全由用户决定。
function moveGroupLayer(index: number, delta: number): void {
  const rows = groupLayers.value
  const target = index + delta
  if (target < 0 || target >= rows.length) return
  const next = rows.slice()
  const [row] = next.splice(index, 1)
  next.splice(target, 0, row)
  groupLayers.value = next
}

const layerEditors = computed<LayerEditorConfig[]>(() => [
  {
    testid: 'profile-rule-layers',
    prefix: 'rule',
    caption: '规则层',
    hint: '规则层 = 自定义规则 + 模板规则；机场规则在下方「规则源」里显式绑定。先出现者优先，模板规则始终最后（MATCH/FINAL）。',
    rows: ruleLayers.value,
    note: '模板规则始终最后（MATCH/FINAL）',
    lockTemplate: true,
  },
  {
    testid: 'profile-group-layers',
    prefix: 'group',
    caption: '分组层',
    hint: '分组层 = 同名分组先出现者胜出；模板层 = INI 模板里的分组。',
    rows: groupLayers.value,
    note: '',
    move: moveGroupLayer,
    lockTemplate: false,
  },
])

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    profiles.value = await endpoints.listProfiles()
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

// 规则源清单（D-33）：加载失败只降级成空选项，不影响方案页主功能（仍可编辑/保存其它字段）。
async function loadRuleSourceOptions(): Promise<void> {
  try {
    const res = await endpoints.listRuleSources()
    // 只展示已有产物的源：无产物的源绑定后订阅会静默回退，保存时也会被后端拒绝。
    ruleSourceOptions.value = (res.sources ?? [])
      .filter((s) => (s.latest ?? '') !== '')
      .map((s) => ({
        title: s.origin === 'airport' ? '机场规则 · ' + (s.label ?? s.name) : (s.label ?? s.name),
        value: s.name,
        origin: s.origin,
        ruleCount: s.rule_count ?? 0,
      }))
  } catch {
    // 规则源清单加载失败不阻断方案编辑：保留空选项，用户仍可保存其它字段
    ruleSourceOptions.value = []
  }
}

// ---- 规则源有序多选 ----
const sourcePickerOpen = ref(false)
const sourceSearch = ref('')
const sourcePicked = ref<string[]>([])

const filteredSourceOptions = computed(() => {
  const selected = new Set(draft.value.rule_sources)
  const q = sourceSearch.value.trim().toLowerCase()
  return ruleSourceOptions.value.filter((o) => {
    if (selected.has(o.value)) return false
    if (q === '') return true
    return o.title.toLowerCase().includes(q) || o.value.toLowerCase().includes(q)
  })
})

function sourceOption(name: string): { title: string; value: string; origin?: string; ruleCount?: number } | undefined {
  return ruleSourceOptions.value.find((o) => o.value === name)
}

function sourceTitle(name: string): string {
  return sourceOption(name)?.title ?? name
}

function sourceMeta(name: string): string {
  const o = sourceOption(name)
  if (o === undefined) return '已移除或暂无产物'
  return (o.origin === 'airport' ? '机场' : '远程') + ' · ' + (o.ruleCount ?? 0) + ' 条'
}

function openSourcePicker(): void {
  sourceSearch.value = ''
  sourcePicked.value = []
  sourcePickerOpen.value = true
}

function toggleSourcePick(value: string): void {
  const i = sourcePicked.value.indexOf(value)
  if (i >= 0) {
    sourcePicked.value.splice(i, 1)
  } else {
    sourcePicked.value.push(value)
  }
}

function addPickedSources(): void {
  for (const value of sourcePicked.value) {
    if (!draft.value.rule_sources.includes(value)) {
      draft.value.rule_sources.push(value)
    }
  }
  sourcePicked.value = []
  sourcePickerOpen.value = false
}

function removeRuleSource(value: string): void {
  draft.value.rule_sources = draft.value.rule_sources.filter((v) => v !== value)
}

function moveRuleSource(index: number, delta: number): void {
  const target = index + delta
  const list = draft.value.rule_sources
  if (target < 0 || target >= list.length) return
  const next = list.slice()
  const [row] = next.splice(index, 1)
  next.splice(target, 0, row)
  draft.value.rule_sources = next
}

// ---- 跨源重复规则提示 ----
const sourceConflictTotal = ref(0)
const sourceConflictSamples = ref<{ type: string; param: string; first: string; second: string }[]>([])
let conflictTimer: ReturnType<typeof setTimeout> | undefined

async function checkSourceConflicts(): Promise<void> {
  if (!dialog.value || draft.value.rule_sources.length < 2) {
    sourceConflictTotal.value = 0
    sourceConflictSamples.value = []
    return
  }
  try {
    const r = await endpoints.checkRuleSourceConflicts(draft.value.rule_sources)
    sourceConflictTotal.value = r.total
    sourceConflictSamples.value = r.samples
  } catch {
    sourceConflictTotal.value = 0
    sourceConflictSamples.value = []
  }
}

function scheduleConflictCheck(): void {
  if (conflictTimer !== undefined) {
    clearTimeout(conflictTimer)
  }
  conflictTimer = setTimeout(() => {
    void checkSourceConflicts()
  }, 250)
}

watch(() => [...draft.value.rule_sources], scheduleConflictCheck)

// truncateTemplate 把过长的模板文本截断到 60 字。
function truncateTemplate(text: string): string {
  if (text.length <= TEMPLATE_MAX) {
    return text
  }
  return text.slice(0, TEMPLATE_MAX) + '…'
}

function templateText(config: string): string {
  return config === '' ? '默认' : truncateTemplate(config)
}

function pipelineText(pipeline: string): string {
  return pipeline === '' ? '默认' : pipeline
}

// 列表里的层展示：缺省显示「默认」，未知层键原样透出。
function layerText(layers: string[] | undefined): string {
  if (!Array.isArray(layers) || layers.length === 0) return '默认'
  return layers.map((k) => LAYER_LABELS[k] ?? k).join(' → ')
}

// 顺序列：服务端回退的 UnixNano（19 位）显示「未设置」。
function sortOrderText(value: number): string {
  return value > SORT_ORDER_UNSET_LIMIT ? '未设置' : String(value)
}

function openCreate(): void {
  editingId.value = 0
  draft.value = {
    name: '',
    description: '',
    config: '',
    pipeline: '',
    target: '',
    include: '',
    exclude: '',
    enabled: true,
    sort_order: 0,
    rule_sources: [],
    limit_sources: false,
    airport_ids: [],
    self_node_ids: [],
  }
  ruleLayers.value = defaultLayers(RULE_LAYER_KEYS)
  groupLayers.value = defaultLayers(GROUP_LAYER_KEYS)
  // 打开弹窗时重新拉一次规则源与节点来源清单：清单可能已被其它页面改动。
  void loadRuleSourceOptions()
  void loadSourceOptions()
  void loadTargetOptions()
  dialog.value = true
  scheduleConflictCheck()
}

function openEdit(row: Profile): void {
  editingId.value = row.id
  draft.value = {
    name: row.name,
    description: row.description,
    config: row.config,
    pipeline: row.pipeline,
    target: row.target ?? '',
    include: row.include ?? '',
    exclude: row.exclude ?? '',
    enabled: row.enabled,
    sort_order: row.sort_order,
    rule_sources: [...(row.rule_sources ?? [])],
    limit_sources: row.airport_ids !== null || row.self_node_ids !== null,
    airport_ids: [...(row.airport_ids ?? [])],
    self_node_ids: [...(row.self_node_ids ?? [])],
  }
  ruleLayers.value = ruleRowsFrom(row.rule_layers)
  groupLayers.value = layersToRows(row.group_layers, GROUP_LAYER_KEYS)
  // 打开弹窗时重新拉一次规则源与节点来源清单：清单可能已被其它页面改动。
  void loadRuleSourceOptions()
  void loadSourceOptions()
  void loadTargetOptions()
  dialog.value = true
  scheduleConflictCheck()
}

async function save(): Promise<void> {
  const rule_layers = layerKeys(ruleLayers.value)
  const group_layers = layerKeys(groupLayers.value)
  if (rule_layers.length === 0 || group_layers.length === 0) {
    notify('至少启用一层', 'error')
    return
  }
  const body = {
    name: draft.value.name,
    description: draft.value.description,
    config: draft.value.config,
    pipeline: draft.value.pipeline,
    target: draft.value.target,
    include: draft.value.include,
    exclude: draft.value.exclude,
    enabled: draft.value.enabled,
    sort_order: draft.value.sort_order,
    rule_layers,
    rule_sources: [...draft.value.rule_sources],
    group_layers,
    all_sources: !draft.value.limit_sources,
    ...(draft.value.limit_sources
      ? { airport_ids: [...draft.value.airport_ids], self_node_ids: [...draft.value.self_node_ids] }
      : {}),
  }
  try {
    if (editingId.value === 0) {
      await endpoints.createProfile(body)
    } else {
      await endpoints.updateProfile(editingId.value, body)
    }
    notify('已保存')
    dialog.value = false
    await load()
  } catch (err) {
    // 后端的冲突文案（例如「方案名称 “X” 已存在」）原样透出。
    notify(errorMessage(err), 'error')
  }
}

function askDelete(row: Profile): void {
  deleteTarget.value = row
  confirmOpen.value = true
}

function cancelDelete(): void {
  confirmOpen.value = false
}

async function confirmDelete(): Promise<void> {
  const target = deleteTarget.value
  confirmOpen.value = false
  try {
    await endpoints.deleteProfile(target.id)
    notify('已删除')
    await load()
  } catch (err) {
    notify(errorMessage(err), 'error')
  }
}

load()
loadRuleSourceOptions()
</script>

<template>
  <div class="profiles-page">
    <PageHeader :title="help.title" :subtitle="help.oneLiner" help-key="profiles">
      <template #actions>
        <v-btn color="primary" prepend-icon="mdi-plus" data-testid="profile-new" @click="openCreate">
          新增
        </v-btn>
        <v-btn variant="text" prepend-icon="mdi-refresh" data-testid="profile-refresh" @click="load">
          刷新
        </v-btn>
      </template>
    </PageHeader>

    <AsyncSection
      :loading="loading"
      :error="error"
      :empty="profiles.length === 0"
      empty-text="还没有订阅方案"
      @retry="load"
    >
      <v-data-table
        :items="profiles"
        :headers="headers"
        density="comfortable"
        hover
        data-testid="profile-table"
        class="profiles-table"
      >
        <template #item.name="{ item }">
          <span data-testid="profile-row">{{ item.name }}</span>
        </template>
        <template #item.config="{ item }">
          <span class="profiles-truncate" :title="item.config">{{ templateText(item.config) }}</span>
        </template>
        <template #item.pipeline="{ item }">
          <span>{{ pipelineText(item.pipeline) }}</span>
        </template>
        <template #item.rule_layers="{ item }">
          <span data-testid="profile-rule-layers-text">{{ layerText(item.rule_layers) }}</span>
        </template>
        <template #item.group_layers="{ item }">
          <span data-testid="profile-group-layers-text">{{ layerText(item.group_layers) }}</span>
        </template>
        <template #item.rule_sources="{ item }">
          <span :data-testid="'profile-rule-sources-text-' + item.id">
            {{ (item.rule_sources ?? []).join('、') || '默认' }}
          </span>
        </template>
        <template #item.enabled="{ item }">
          <span class="sf-status">
            <span class="sf-status__dot" :class="item.enabled ? 'sf-status__dot--ok' : 'sf-status__dot--muted'"></span>
            <span>{{ item.enabled ? '启用' : '停用' }}</span>
          </span>
        </template>
        <template #item.sort_order="{ item }">
          <span data-testid="profile-sort-order">{{ sortOrderText(item.sort_order) }}</span>
        </template>
        <template #item.actions="{ item }">
          <v-btn
            icon="mdi-pencil"
            size="small"
            variant="text"
            data-testid="profile-edit"
            @click="openEdit(item)"
          />
          <v-btn
            icon="mdi-delete"
            size="small"
            variant="text"
            color="error"
            data-testid="profile-delete"
            @click="askDelete(item)"
          />
        </template>
      </v-data-table>
    </AsyncSection>

    <v-dialog v-model="dialog" max-width="560">
      <v-card>
        <v-card-title>{{ editingId === 0 ? '新建方案' : '编辑方案' }}</v-card-title>
        <v-card-text class="profile-dialog__body">
          <v-text-field
            v-model="draft.name"
            label="名称"
            variant="outlined"
            density="comfortable"
            class="mb-2"
            data-testid="profile-name"
          />
          <v-text-field
            v-model="draft.description"
            label="说明"
            variant="outlined"
            density="comfortable"
            class="mb-2"
            data-testid="profile-description"
          />
          <v-text-field
            v-model="draft.config"
            label="模板"
            hint="留空时使用默认模板"
            variant="outlined"
            density="comfortable"
            hide-details
            class="mb-3"
            data-testid="profile-config"
          />
          <v-text-field
            v-model="draft.pipeline"
            label="管线"
            hint="留空时不使用管线"
            variant="outlined"
            density="comfortable"
            hide-details
            class="mb-3"
            data-testid="profile-pipeline"
          />
          <div class="sf-form-label">
            <span>目标格式</span>
            <InfoTip text="留空时使用 clash.meta。" label="目标格式" />
          </div>
          <v-select
            v-model="draft.target"
            :items="targetOptions"
            variant="outlined"
            density="comfortable"
            clearable
            hide-details
            class="mb-3"
            data-testid="profile-target"
          />
          <div class="sf-form-label">
            <span>包含节点</span>
            <InfoTip text="按节点名称匹配的逗号分隔正则；留空表示不筛选。" example="香港,日本" label="包含节点" />
          </div>
          <v-text-field
            v-model="draft.include"
            variant="outlined"
            density="comfortable"
            hide-details
            class="mb-3"
            data-testid="profile-include"
          />
          <div class="sf-form-label">
            <span>排除节点</span>
            <InfoTip text="按节点名称匹配的逗号分隔正则；留空表示不排除。" example="过期,剩余" label="排除节点" />
          </div>
          <v-text-field
            v-model="draft.exclude"
            variant="outlined"
            density="comfortable"
            hide-details
            class="mb-3"
            data-testid="profile-exclude"
          />
          <div class="mb-3">
            <div class="sf-form-label">
              <span>规则源（顺序即优先级）</span>
              <InfoTip text="同一匹配条件出现在多个规则源时，按列表顺序由先出现的源生效。机场规则源显示为「机场规则 · 名称」。" label="规则源优先级" />
            </div>
            <div class="sf-source-list" data-testid="profile-rule-sources">
              <div
                v-for="(name, index) in draft.rule_sources"
                :key="name"
                class="sf-source-row"
                :data-testid="'profile-rule-source-row-' + name"
              >
                <span class="sf-source-priority">{{ index + 1 }}</span>
                <span class="sf-source-name">{{ sourceTitle(name) }}</span>
                <span class="sf-source-meta">{{ sourceMeta(name) }}</span>
                <v-btn
                  icon="mdi-chevron-up"
                  size="x-small"
                  variant="text"
                  :disabled="index === 0"
                  :data-testid="'profile-rule-source-up-' + name"
                  @click="moveRuleSource(index, -1)"
                />
                <v-btn
                  icon="mdi-chevron-down"
                  size="x-small"
                  variant="text"
                  :disabled="index === draft.rule_sources.length - 1"
                  :data-testid="'profile-rule-source-down-' + name"
                  @click="moveRuleSource(index, 1)"
                />
                <v-btn
                  icon="mdi-close"
                  size="x-small"
                  variant="text"
                  :data-testid="'profile-rule-source-remove-' + name"
                  @click="removeRuleSource(name)"
                />
              </div>
              <div v-if="draft.rule_sources.length === 0" class="sf-source-empty text-medium-emphasis" data-testid="profile-rule-sources-empty">
                未绑定规则源
              </div>
            </div>
            <v-btn
              size="small"
              variant="tonal"
              prepend-icon="mdi-plus"
              class="mt-2"
              data-testid="profile-rule-source-add"
              @click="openSourcePicker"
            >
              添加规则源
            </v-btn>
            <v-alert
              v-if="sourceConflictTotal > 0"
              type="warning"
              variant="tonal"
              density="compact"
              class="mt-2"
              data-testid="profile-source-conflicts"
            >
              发现 {{ sourceConflictTotal }} 条跨源重复规则，当前按列表顺序由先出现的源生效。
              <v-btn
                v-if="sourceConflictSamples.length > 0"
                variant="text"
                size="small"
                class="ml-2"
                data-testid="profile-source-conflicts-detail"
                @click="conflictDialogOpen = true"
              >查看示例</v-btn>
            </v-alert>
          </div>
          <v-divider class="mb-3" />
          <div class="text-subtitle-2 mb-1">节点来源</div>
          <div class="d-flex align-center ga-1 mb-2">
            <v-switch
              v-model="draft.limit_sources"
              label="限制机场与自建节点来源"
              color="primary"
              density="compact"
              hide-details
              data-testid="profile-limit-sources"
            />
            <InfoTip text="关闭时使用全部启用的来源；开启后只使用下方选择的来源，均不选择表示不使用该类来源。" label="节点来源" />
          </div>
          <v-select
            v-model="draft.airport_ids"
            :items="airportOptions"
            :disabled="!draft.limit_sources"
            label="机场来源"
            multiple
            chips
            closable-chips
            variant="outlined"
            density="comfortable"
            data-testid="profile-airports"
            class="mb-2"
          />
          <v-select
            v-model="draft.self_node_ids"
            :items="selfNodeOptions"
            :disabled="!draft.limit_sources"
            label="自建节点来源"
            multiple
            chips
            closable-chips
            variant="outlined"
            density="comfortable"
            data-testid="profile-self-nodes"
            class="mb-2"
          />
          <v-switch
            v-model="draft.enabled"
            label="启用"
            color="primary"
            hide-details
            class="mb-2"
            data-testid="profile-enabled"
          />
          <v-text-field
            v-model.number="draft.sort_order"
            label="顺序"
            type="number"
            variant="outlined"
            density="comfortable"
            class="mb-2"
            data-testid="profile-sort"
          />

          <div v-for="editor in layerEditors" :key="editor.testid" class="mb-3">
            <div class="sf-form-label">
              <span>{{ editor.caption }}</span>
              <InfoTip :text="editor.hint" :label="editor.caption" />
            </div>
            <div class="sf-layer-editor" :data-testid="editor.testid">
            <div
              v-for="(row, index) in editor.rows"
              :key="row.key"
              class="sf-layer-row"
              :data-testid="editor.prefix + '-layer-row-' + row.key"
            >
              <v-checkbox
                v-model="row.enabled"
                hide-details
                density="compact"
                :data-testid="editor.prefix + '-layer-toggle-' + row.key"
              />
              <span class="sf-layer-name">{{ LAYER_LABELS[row.key] }}</span>
              <span class="sf-layer-priority">{{ layerPriority(editor.rows, row.key) }}</span>
              <v-btn
                v-if="editor.move"
                icon="mdi-chevron-up"
                size="x-small"
                variant="text"
                :disabled="index === 0 || (editor.lockTemplate && row.key === 'template')"
                :data-testid="editor.prefix + '-layer-up-' + row.key"
                @click="editor.move(index, -1)"
              />
              <v-btn
                v-if="editor.move"
                icon="mdi-chevron-down"
                size="x-small"
                variant="text"
                :disabled="index === editor.rows.length - 1 || (editor.lockTemplate && row.key === 'template')"
                :data-testid="editor.prefix + '-layer-down-' + row.key"
                @click="editor.move(index, 1)"
              />
              <span v-if="editor.note !== '' && row.key === 'template'" class="sf-layer-note">
                {{ editor.note }}
              </span>
            </div>
            </div>
          </div>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" data-testid="profile-cancel" @click="dialog = false">取消</v-btn>
          <v-btn color="primary" data-testid="profile-save" @click="save">保存</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog v-model="sourcePickerOpen" max-width="560">
    <v-card data-testid="profile-source-picker">
      <v-card-title>添加规则源</v-card-title>
      <v-card-text>
        <v-text-field v-model="sourceSearch" label="搜索规则源" density="compact" hide-details data-testid="profile-source-search" class="mb-2" />
        <div class="sf-source-picker-list">
          <v-list v-if="filteredSourceOptions.length > 0">
            <v-list-item v-for="opt in filteredSourceOptions" :key="opt.value" data-testid="profile-source-option">
              <template #prepend>
                <v-checkbox
                  :model-value="sourcePicked.includes(opt.value)"
                  hide-details
                  density="compact"
                  :data-testid="'profile-source-check-' + opt.value"
                  @update:model-value="toggleSourcePick(opt.value)"
                />
              </template>
              <v-list-item-title>{{ opt.title }}</v-list-item-title>
              <v-list-item-subtitle>{{ (opt.origin === 'airport' ? '机场' : '远程') + ' · ' + (opt.ruleCount ?? 0) + ' 条' }}</v-list-item-subtitle>
            </v-list-item>
          </v-list>
          <div v-else class="text-medium-emphasis py-4 text-center" data-testid="profile-source-empty">没有可添加的规则源</div>
        </div>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" data-testid="profile-source-cancel" @click="sourcePickerOpen = false">取消</v-btn>
        <v-btn color="primary" :disabled="sourcePicked.length === 0" data-testid="profile-source-confirm" @click="addPickedSources">添加</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-dialog v-model="conflictDialogOpen" max-width="640" scrollable data-testid="profile-source-conflicts-dialog">
    <v-card>
      <v-card-title>跨源重复规则示例</v-card-title>
      <v-card-text>
        <p class="text-body-2 text-medium-emphasis mb-3">
          以下匹配条件同时存在于两个规则源。订阅生成时按规则源顺序取先出现的定义。
        </p>
        <v-table density="compact">
          <thead>
            <tr><th>类型</th><th>参数</th><th>先出现</th><th>后出现</th></tr>
          </thead>
          <tbody>
            <tr v-for="(s, i) in sourceConflictSamples" :key="i">
              <td>{{ s.type }}</td>
              <td>{{ s.param }}</td>
              <td>{{ s.first }}</td>
              <td>{{ s.second }}</td>
            </tr>
          </tbody>
        </v-table>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" data-testid="profile-source-conflicts-close" @click="conflictDialogOpen = false">关闭</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-dialog v-model="confirmOpen" max-width="420">
      <v-card>
        <v-card-title>确认删除</v-card-title>
        <v-card-text>删除方案「{{ deleteTarget.name }}」后不可恢复，确定继续？</v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" data-testid="profile-delete-cancel" @click="cancelDelete">取消</v-btn>
          <v-btn color="error" data-testid="profile-delete-confirm" @click="confirmDelete">删除</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>

<style scoped>
/* 方案弹窗字段变多（来源/层顺序）后需要内部滚动，避免小屏溢出。 */
.profile-dialog__body {
  max-height: 68vh;
  overflow-y: auto;
  scrollbar-gutter: stable;
}
.profiles-table {
  border: 1px solid var(--sf-border);
  border-radius: var(--sf-radius);
}
.profiles-truncate {
  display: inline-block;
  max-width: 380px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.sf-form-label {
  display: flex;
  align-items: center;
  gap: 2px;
  min-height: 24px;
  margin-bottom: 4px;
  font-size: 13px;
  color: var(--sf-text-muted);
}
.sf-layer-editor {
  border: 1px solid var(--sf-border);
  border-radius: var(--sf-radius);
  padding: 4px 8px;
}
.sf-layer-row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 32px;
}
.sf-layer-name {
  min-width: 72px;
}
.sf-layer-priority {
  font-size: 12px;
  color: var(--sf-text-muted, rgba(0, 0, 0, 0.6));
  min-width: 72px;
}
.sf-layer-note {
  font-size: 12px;
  color: var(--sf-text-muted, rgba(0, 0, 0, 0.6));
}
.sf-source-list {
  border: 1px solid var(--sf-border, rgba(0, 0, 0, 0.12));
  border-radius: 8px;
  overflow: hidden;
}
.sf-source-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  border-bottom: 1px solid var(--sf-border, rgba(0, 0, 0, 0.08));
}
.sf-source-row:last-child {
  border-bottom: 0;
}
.sf-source-priority {
  width: 20px;
  text-align: center;
  color: var(--sf-text-muted, rgba(0, 0, 0, 0.6));
  font-size: 12px;
}
.sf-source-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.sf-source-meta {
  color: var(--sf-text-muted, rgba(0, 0, 0, 0.6));
  font-size: 12px;
  white-space: nowrap;
}
.sf-source-empty {
  padding: 10px;
  font-size: 13px;
}
.sf-source-picker-list {
  max-height: 40vh;
  overflow-y: auto;
}
</style>
