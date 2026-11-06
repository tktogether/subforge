<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import SfPageHead from '../components/ui/SfPageHead.vue'
import SfField from '../components/ui/SfField.vue'
import SfSwitch from '../components/ui/SfSwitch.vue'
import SfDialog from '../components/ui/SfDialog.vue'
import SfNote from '../components/ui/SfNote.vue'
import SfEmpty from '../components/ui/SfEmpty.vue'
import SfInfoTip from '../components/ui/SfInfoTip.vue'
import { endpoints, type FinalGroupConfig, type FinalGroupPreview, type Profile } from '../api/endpoints'
import { notify } from '../composables/useNotify'
import { errorMessage } from '../composables/useAsync'
import { confirmAction } from '../composables/useConfirm'
import {
  IconPlus,
  IconRefresh,
  IconPencil,
  IconTrash,
  IconChevronUp,
  IconChevronDown,
  IconX,
  IconScanEye,
} from '@tabler/icons-vue'

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

const FINAL_GROUP_DEFAULT_NAME = '🚀 节点选择'

const profiles = ref<Profile[]>([])
const profilePage = ref(1)
const profilePageSize = ref(10)
const loading = ref(false)
const error = ref('')

// 规则源选项（D-33）：方案可绑定规则源；选项来自 GET /api/rule-sources 的 sources[].name。
const ruleSourceOptions = ref<{ title: string; value: string; origin?: string; ruleCount?: number }[]>([])

// 节点来源选项：机场/自建节点。加载失败只降级为空选项，不阻断方案编辑。
const airportOptions = ref<{ title: string; value: number }[]>([])
const selfNodeOptions = ref<{ title: string; value: number }[]>([])
const targetOptions = ref<string[]>([])
const pipelineOptions = ref<{ title: string; value: string }[]>([])
const pipelineItems = computed(() => {
  const items = [...pipelineOptions.value]
  const current = draft.value.pipeline.trim()
  if (current !== '' && !items.some((i) => i.value === current)) {
    items.unshift({ title: current, value: current })
  }
  return items
})

async function loadPipelineOptions(): Promise<void> {
  try {
    const list = await endpoints.pipelines('')
    pipelineOptions.value = list.filter((p) => p.enabled).map((p) => ({ title: p.name, value: p.name }))
  } catch {
    pipelineOptions.value = []
  }
}

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
  // 规则源（D-33）：方案绑定的规则源名列表；空 = 未绑定。
  rule_sources: [] as string[],
  // 未限制来源 = 使用全部启用机场/自建节点；打开后才按下面的 id 多选限制。
  limit_sources: false,
  airport_ids: [] as number[],
  self_node_ids: [] as number[],
  // 统一最终选择组（客制化）：自动/自定义名称、DIRECT、来源覆盖。
  final_group_enabled: false,
  final_group_mode: 'auto' as 'auto' | 'custom',
  final_group_name: FINAL_GROUP_DEFAULT_NAME,
  final_group_include_direct: true,
  final_group_keep_source_terminals: false,
  final_group_overrides: {} as Record<string, string[]>,
})

// 统一选择组预览：报告、来源分组选项与加载状态。
const finalGroupModeOptions = [
  { title: '自动指定（标准名）', value: 'auto' },
  { title: '自定义名称', value: 'custom' },
]
const finalGroupPreviewOpen = ref(false)
const finalGroupPreviewLoading = ref(false)
const finalGroupPreview = ref<FinalGroupPreview | null>(null)
const finalGroupGroupOptions = ref<{ title: string; value: string; source: string }[]>([])

// 自动模式固定使用标准名；切回自动时把名称复位，避免沿用上一次的自定义名。
watch(() => draft.value.final_group_mode, (mode) => {
  if (mode === 'auto') draft.value.final_group_name = FINAL_GROUP_DEFAULT_NAME
})

function draftFinalGroup(): FinalGroupConfig {
  return {
    enabled: draft.value.final_group_enabled,
    mode: draft.value.final_group_mode,
    name: draft.value.final_group_mode === 'auto' ? FINAL_GROUP_DEFAULT_NAME : draft.value.final_group_name,
    include_direct: draft.value.final_group_include_direct,
    keep_source_terminals: draft.value.final_group_keep_source_terminals,
    overrides: { ...draft.value.final_group_overrides },
  }
}

function overrideKey(source: string): string {
  return 'group:' + source
}

function overrideValue(source: string): string {
  return draft.value.final_group_overrides[overrideKey(source)]?.[0] ?? ''
}

function setOverride(source: string, value: string): void {
  const next = { ...draft.value.final_group_overrides }
  if (!value) {
    delete next[overrideKey(source)]
  } else {
    next[overrideKey(source)] = [value]
  }
  draft.value.final_group_overrides = next
}

function ruleOverrideText(name: string): string {
  return (draft.value.final_group_overrides['rule:' + name] ?? []).join('、')
}

function setRuleOverrideText(name: string, raw: string): void {
  const names = raw.split(/[,，、]/).map((v) => v.trim()).filter(Boolean)
  const next = { ...draft.value.final_group_overrides }
  if (names.length === 0) {
    delete next['rule:' + name]
  } else {
    next['rule:' + name] = [...new Set(names)]
  }
  draft.value.final_group_overrides = next
}

function overrideOptions(source: string): { title: string; value: string }[] {
  const groups = finalGroupGroupOptions.value
    .filter((g) => g.source === source)
    .map((g) => ({ title: g.title, value: g.value }))
  return [
    { title: '自动识别', value: '' },
    { title: '不处理该来源', value: 'none' },
    ...groups,
  ]
}

async function loadFinalGroupOptions(): Promise<void> {
  try {
    const groups = await endpoints.listGroups()
    finalGroupGroupOptions.value = groups.map((g) => ({
      title: g.name,
      value: g.name,
      source: g.source ?? '',
    }))
  } catch {
    finalGroupGroupOptions.value = []
  }
}

async function openFinalGroupPreview(): Promise<void> {
  if (editingId.value === 0) {
    notify('请先保存方案，再预览识别结果', 'error')
    return
  }
  finalGroupPreviewOpen.value = true
  finalGroupPreviewLoading.value = true
  finalGroupPreview.value = null
  void loadFinalGroupOptions()
  try {
    finalGroupPreview.value = await endpoints.previewProfileFinalGroup(editingId.value, draftFinalGroup())
  } catch (err) {
    finalGroupPreviewOpen.value = false
    notify(errorMessage(err), 'error')
  } finally {
    finalGroupPreviewLoading.value = false
  }
}

// 两个层编辑器的行状态（顺序即优先级）。
const ruleLayers = ref<LayerRow[]>(defaultLayers(RULE_LAYER_KEYS))
const groupLayers = ref<LayerRow[]>(defaultLayers(GROUP_LAYER_KEYS))

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

const profilePageCount = computed(() => Math.max(1, Math.ceil(profiles.value.length / profilePageSize.value)))
watch([profiles, profilePageSize], () => {
  profilePage.value = Math.min(profilePage.value, profilePageCount.value)
})
const pagedProfiles = computed(() => {
  const start = (profilePage.value - 1) * profilePageSize.value
  return profiles.value.slice(start, start + profilePageSize.value)
})

function setProfilePageSize(next: number): void {
  profilePageSize.value = next
  profilePage.value = 1
}

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
const conflictDialogOpen = ref(false)
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

function toggleId(list: number[], id: number): number[] {
  return list.includes(id) ? list.filter((x) => x !== id) : [...list, id]
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
    rule_sources: [],
    limit_sources: false,
    airport_ids: [],
    self_node_ids: [],
    final_group_enabled: false,
    final_group_mode: 'auto',
    final_group_name: FINAL_GROUP_DEFAULT_NAME,
    final_group_include_direct: true,
    final_group_keep_source_terminals: false,
    final_group_overrides: {},
  }
  ruleLayers.value = defaultLayers(RULE_LAYER_KEYS)
  groupLayers.value = defaultLayers(GROUP_LAYER_KEYS)
  // 打开弹窗时重新拉一次规则源与节点来源清单：清单可能已被其它页面改动。
  void loadRuleSourceOptions()
  void loadSourceOptions()
  void loadTargetOptions()
  void loadPipelineOptions()
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
    rule_sources: [...(row.rule_sources ?? [])],
    limit_sources: row.airport_ids !== null || row.self_node_ids !== null,
    airport_ids: [...(row.airport_ids ?? [])],
    self_node_ids: [...(row.self_node_ids ?? [])],
    final_group_enabled: row.final_group?.enabled ?? false,
    final_group_mode: row.final_group?.mode === 'custom' ? 'custom' : 'auto',
    final_group_name: row.final_group?.mode === 'custom'
      ? (row.final_group?.name || FINAL_GROUP_DEFAULT_NAME)
      : FINAL_GROUP_DEFAULT_NAME,
    final_group_include_direct: row.final_group?.include_direct ?? true,
    final_group_keep_source_terminals: row.final_group?.keep_source_terminals ?? false,
    final_group_overrides: { ...(row.final_group?.overrides ?? {}) },
  }
  ruleLayers.value = ruleRowsFrom(row.rule_layers)
  groupLayers.value = layersToRows(row.group_layers, GROUP_LAYER_KEYS)
  // 打开弹窗时重新拉一次规则源与节点来源清单：清单可能已被其它页面改动。
  void loadRuleSourceOptions()
  void loadSourceOptions()
  void loadTargetOptions()
  void loadPipelineOptions()
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
    rule_layers,
    rule_sources: [...draft.value.rule_sources],
    group_layers,
    final_group: draftFinalGroup(),
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

async function askDelete(row: Profile): Promise<void> {
  const ok = await confirmAction({
    title: '确认删除方案「' + row.name + '」？',
    body: '删除方案后不可恢复；使用该方案的订阅令牌会回退到未绑定状态。',
    confirmText: '删除',
    danger: true,
  })
  if (!ok) return
  try {
    await endpoints.deleteProfile(row.id)
    notify('已删除')
    await load()
  } catch (err) {
    notify(errorMessage(err), 'error')
  }
}

void load()
void loadRuleSourceOptions()
void loadPipelineOptions()
</script>

<template>
  <div>
    <SfPageHead title="方案" desc="方案 = 节点来源 + 规则层 + 分组层 + 输出格式，订阅令牌绑定后按方案生效。">
      <template #actions>
        <button class="sf-btn sf-btn--primary" type="button" data-testid="profile-new" @click="openCreate">
          <IconPlus :size="15" :stroke-width="2" />
          新增
        </button>
        <button class="sf-btn sf-btn--ghost" type="button" data-testid="profile-refresh" :disabled="loading" @click="load">
          <IconRefresh :size="15" :stroke-width="1.8" :class="{ 'pf-spin': loading }" />
          刷新
        </button>
      </template>
    </SfPageHead>

    <div v-if="error !== ''" class="sf-note sf-note--danger sf-rise" style="margin-bottom: 16px">
      <span class="sf-note__body">{{ error }}</span>
      <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" @click="load">重试</button>
    </div>

    <div v-if="profiles.length === 0 && !loading && error === ''" class="sf-panel">
      <SfEmpty title="还没有订阅方案" desc="新增一个方案，把节点来源、规则与输出格式组装成可订阅的配置。" />
    </div>

    <template v-else>
      <div class="pf-bar sf-rise sf-d1">
        <span class="sf-pagination__info">共 {{ profiles.length }} 条</span>
        <span class="sf-spacer" />
        <span class="sf-select-wrap" style="width: 96px">
          <select class="sf-select pf-size" data-testid="profile-page-size" :value="String(profilePageSize)" @change="setProfilePageSize(Number(($event.target as HTMLSelectElement).value))">
            <option v-for="n in [10, 25, 50, 100]" :key="n" :value="String(n)">{{ n }}</option>
          </select>
        </span>
      </div>

      <div class="sf-table-wrap sf-rise sf-d1" data-testid="profile-table">
        <table class="sf-table">
          <thead>
            <tr>
              <th>名称</th>
              <th>模板</th>
              <th>最终处理</th>
              <th>规则层</th>
              <th>分组层</th>
              <th>规则源</th>
              <th style="width: 80px">启用</th>
              <th style="width: 96px; text-align: right">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="loading && profiles.length === 0">
              <td v-for="n in 8" :key="n"><span class="sf-skeleton" style="width: 60%" /></td>
            </tr>
            <tr v-for="item in pagedProfiles" v-else :key="item.id" data-testid="profile-row">
              <td><span style="font-weight: 600">{{ item.name }}</span></td>
              <td><span class="pf-truncate sf-mono" :title="item.config">{{ templateText(item.config) }}</span></td>
              <td><span class="sf-faint" style="font-size: 12.5px">{{ pipelineText(item.pipeline) }}</span></td>
              <td><span data-testid="profile-rule-layers-text" style="font-size: 12.5px">{{ layerText(item.rule_layers) }}</span></td>
              <td><span data-testid="profile-group-layers-text" style="font-size: 12.5px">{{ layerText(item.group_layers) }}</span></td>
              <td>
                <span :data-testid="'profile-rule-sources-text-' + item.id" class="pf-truncate" :title="(item.rule_sources ?? []).join('、')">
                  {{ (item.rule_sources ?? []).join('、') || '默认' }}
                </span>
              </td>
              <td>
                <span class="sf-status">
                  <span class="sf-led" :class="item.enabled ? 'sf-led--ok' : 'sf-led--idle'" />
                  <span>{{ item.enabled ? '启用' : '停用' }}</span>
                </span>
              </td>
              <td>
                <span class="sf-table__actions">
                  <button class="sf-btn sf-btn--subtle sf-btn--icon sf-btn--sm" type="button" aria-label="编辑" data-testid="profile-edit" @click="openEdit(item)">
                    <IconPencil :size="15" :stroke-width="1.7" />
                  </button>
                  <button class="sf-btn sf-btn--subtle sf-btn--icon sf-btn--sm sf-btn--del" type="button" aria-label="删除" data-testid="profile-delete" @click="askDelete(item)">
                    <IconTrash :size="15" :stroke-width="1.7" />
                  </button>
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-if="profiles.length > profilePageSize" class="sf-pagination" style="border: none; padding: 12px 2px">
        <span class="sf-pagination__info">PAGE {{ profilePage }} / {{ profilePageCount }}</span>
        <span class="sf-spacer" />
        <button class="sf-btn sf-btn--ghost sf-btn--icon sf-btn--sm" type="button" aria-label="上一页" :disabled="profilePage <= 1" @click="profilePage = profilePage - 1">‹</button>
        <button class="sf-btn sf-btn--ghost sf-btn--icon sf-btn--sm" type="button" aria-label="下一页" :disabled="profilePage >= profilePageCount" @click="profilePage = profilePage + 1">›</button>
      </div>
    </template>

    <!-- 编辑方案 -->
    <SfDialog :open="dialog" :label="editingId === 0 ? 'CREATE PROFILE' : 'EDIT PROFILE'" :title="editingId === 0 ? '新建方案' : '编辑方案'" wide @close="dialog = false">
      <div class="pf-row">
        <SfField label="名称">
          <input v-model="draft.name" class="sf-input" data-testid="profile-name" spellcheck="false" />
        </SfField>
        <SfField label="说明">
          <input v-model="draft.description" class="sf-input" data-testid="profile-description" spellcheck="false" />
        </SfField>
      </div>
      <div class="pf-row">
        <SfField label="模板" hint="留空时使用默认模板">
          <input v-model="draft.config" class="sf-input sf-input--mono" data-testid="profile-config" spellcheck="false" />
        </SfField>
        <SfField label="最终处理管线" hint="各来源先按自己的管线处理并合并，这条管线最后执行；留空不处理。">
          <span class="sf-select-wrap">
            <select v-model="draft.pipeline" class="sf-select" data-testid="profile-pipeline">
              <option value="">不处理</option>
              <option v-for="item in pipelineItems" :key="item.value" :value="item.value">{{ item.title }}</option>
            </select>
          </span>
        </SfField>
      </div>
      <div class="pf-row">
        <SfField hint="留空时使用 clash.meta。">
          <template #label><span class="sf-field__label">目标格式 <SfInfoTip text="留空时使用 clash.meta。" /></span></template>
          <span class="sf-select-wrap">
            <select v-model="draft.target" class="sf-select" data-testid="profile-target">
              <option value="">默认（clash.meta）</option>
              <option v-for="t in targetOptions" :key="t" :value="t">{{ t }}</option>
            </select>
          </span>
        </SfField>
        <div class="pf-switch-box">
          <SfSwitch v-model="draft.enabled" label="启用" data-testid="profile-enabled" />
        </div>
      </div>
      <div class="pf-row">
        <SfField hint="按节点名称匹配的逗号分隔正则；留空表示不筛选。">
          <template #label><span class="sf-field__label">包含节点 <SfInfoTip text="按节点名称匹配的逗号分隔正则；留空表示不筛选。示例：香港,日本" /></span></template>
          <input v-model="draft.include" class="sf-input sf-input--mono" data-testid="profile-include" spellcheck="false" />
        </SfField>
        <SfField hint="按节点名称匹配的逗号分隔正则；留空表示不排除。">
          <template #label><span class="sf-field__label">排除节点 <SfInfoTip text="按节点名称匹配的逗号分隔正则；留空表示不排除。示例：过期,剩余" /></span></template>
          <input v-model="draft.exclude" class="sf-input sf-input--mono" data-testid="profile-exclude" spellcheck="false" />
        </SfField>
      </div>

      <!-- 规则源 -->
      <div class="pf-section">
        <div class="pf-section__head">
          <span class="sf-label-mono">RULE SOURCES / 规则源（顺序即优先级）</span>
          <SfInfoTip text="同一匹配条件出现在多个规则源时，按列表顺序由先出现的源生效。机场规则源显示为「机场规则 · 名称」。" />
          <span class="sf-spacer" />
          <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" data-testid="profile-rule-source-add" @click="openSourcePicker">
            <IconPlus :size="14" :stroke-width="2" />
            添加规则源
          </button>
        </div>
        <div class="pf-source-list" data-testid="profile-rule-sources">
          <div
            v-for="(name, index) in draft.rule_sources"
            :key="name"
            class="pf-source-row"
            :data-testid="'profile-rule-source-row-' + name"
          >
            <span class="pf-source-priority sf-mono">{{ String(index + 1).padStart(2, '0') }}</span>
            <span class="pf-source-name">{{ sourceTitle(name) }}</span>
            <span class="pf-source-meta">{{ sourceMeta(name) }}</span>
            <span class="sf-table__actions">
              <button class="sf-btn sf-btn--subtle sf-btn--icon sf-btn--sm" type="button" aria-label="上移" :disabled="index === 0" :data-testid="'profile-rule-source-up-' + name" @click="moveRuleSource(index, -1)">
                <IconChevronUp :size="15" :stroke-width="1.7" />
              </button>
              <button class="sf-btn sf-btn--subtle sf-btn--icon sf-btn--sm" type="button" aria-label="下移" :disabled="index === draft.rule_sources.length - 1" :data-testid="'profile-rule-source-down-' + name" @click="moveRuleSource(index, 1)">
                <IconChevronDown :size="15" :stroke-width="1.7" />
              </button>
              <button class="sf-btn sf-btn--subtle sf-btn--icon sf-btn--sm sf-btn--del" type="button" aria-label="移除" :data-testid="'profile-rule-source-remove-' + name" @click="removeRuleSource(name)">
                <IconX :size="15" :stroke-width="1.7" />
              </button>
            </span>
          </div>
          <div v-if="draft.rule_sources.length === 0" class="pf-source-empty" data-testid="profile-rule-sources-empty">
            未绑定规则源
          </div>
        </div>
        <SfNote v-if="sourceConflictTotal > 0" tone="warn" data-testid="profile-source-conflicts" style="margin-top: 10px">
          发现 {{ sourceConflictTotal }} 条跨源重复规则，当前按列表顺序由先出现的源生效。
          <button v-if="sourceConflictSamples.length > 0" class="sf-btn sf-btn--ghost sf-btn--sm" type="button" style="margin-left: 10px" data-testid="profile-source-conflicts-detail" @click="conflictDialogOpen = true">查看示例</button>
        </SfNote>
      </div>

      <!-- 统一最终选择组 -->
      <div class="pf-section">
        <div class="pf-section__head">
          <span class="sf-label-mono">FINAL GROUP / 统一最终选择组</span>
          <SfInfoTip text="开启后，机场、模板与规则源各自的最终节点选择组会归并为一个组；服务分组（Netflix、ChatGPT 等）保留。关闭时行为不变。" />
        </div>
        <SfSwitch v-model="draft.final_group_enabled" label="统一各来源的最终节点选择组" data-testid="profile-final-group-enabled" />
        <template v-if="draft.final_group_enabled">
          <div class="pf-row" style="margin-top: 12px">
            <SfField label="统一组" hint="自动指定使用标准名；自定义名称可沿用已有最终组或新建组。">
              <span class="sf-select-wrap">
                <select v-model="draft.final_group_mode" class="sf-select" data-testid="profile-final-group-mode">
                  <option v-for="opt in finalGroupModeOptions" :key="opt.value" :value="opt.value">{{ opt.title }}</option>
                </select>
              </span>
            </SfField>
            <SfField label="统一组名称" hint="规则目标与分组成员都会指向这个名字；不能与普通服务组重名。">
              <input v-model="draft.final_group_name" class="sf-input" data-testid="profile-final-group-name" :disabled="draft.final_group_mode === 'auto'" spellcheck="false" />
            </SfField>
          </div>
          <div class="sf-toolbar" style="margin-top: 4px">
            <SfSwitch v-model="draft.final_group_include_direct" label="包含 DIRECT" data-testid="profile-final-group-direct" />
            <SfSwitch v-model="draft.final_group_keep_source_terminals" label="保留来源最终组为子选项" data-testid="profile-final-group-keep" />
            <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" data-testid="profile-final-group-preview" @click="openFinalGroupPreview">
              <IconScanEye :size="14" :stroke-width="1.8" />
              预览识别结果
            </button>
          </div>
        </template>
      </div>

      <!-- 节点来源 -->
      <div class="pf-section">
        <div class="pf-section__head">
          <span class="sf-label-mono">NODE SOURCES / 节点来源</span>
          <SfInfoTip text="关闭时使用全部启用的来源；开启后只使用下方选择的来源，均不选择表示不使用该类来源。" />
        </div>
        <SfSwitch v-model="draft.limit_sources" label="限制机场与自建节点来源" data-testid="profile-limit-sources" />
        <template v-if="draft.limit_sources">
          <div class="pf-row" style="margin-top: 12px">
            <div class="pf-multiselect" data-testid="profile-airports">
              <span class="sf-field__label">机场来源</span>
              <label v-for="opt in airportOptions" :key="opt.value" class="pf-check">
                <input type="checkbox" class="sf-checkbox" :checked="draft.airport_ids.includes(opt.value)" @change="draft.airport_ids = toggleId(draft.airport_ids, opt.value)" />
                <span>{{ opt.title }}</span>
              </label>
              <span v-if="airportOptions.length === 0" class="sf-faint" style="font-size: 12.5px">暂无机场</span>
            </div>
            <div class="pf-multiselect" data-testid="profile-self-nodes">
              <span class="sf-field__label">自建节点来源</span>
              <label v-for="opt in selfNodeOptions" :key="opt.value" class="pf-check">
                <input type="checkbox" class="sf-checkbox" :checked="draft.self_node_ids.includes(opt.value)" @change="draft.self_node_ids = toggleId(draft.self_node_ids, opt.value)" />
                <span>{{ opt.title }}</span>
              </label>
              <span v-if="selfNodeOptions.length === 0" class="sf-faint" style="font-size: 12.5px">暂无自建节点</span>
            </div>
          </div>
        </template>
      </div>

      <!-- 层编辑器 -->
      <div v-for="editor in layerEditors" :key="editor.testid" class="pf-section" :data-testid="editor.testid">
        <div class="pf-section__head">
          <span class="sf-label-mono">{{ editor.caption }}</span>
          <SfInfoTip :text="editor.hint" />
        </div>
        <div class="pf-layers">
          <div
            v-for="(row, index) in editor.rows"
            :key="row.key"
            class="pf-layer-row"
            :data-testid="editor.prefix + '-layer-row-' + row.key"
          >
            <input
              type="checkbox"
              class="sf-checkbox"
              v-model="row.enabled"
              :data-testid="editor.prefix + '-layer-toggle-' + row.key"
              :aria-label="editor.caption + ' · ' + LAYER_LABELS[row.key]"
            />
            <span class="pf-layer-name">{{ LAYER_LABELS[row.key] }}</span>
            <span class="pf-layer-priority sf-mono">{{ layerPriority(editor.rows, row.key) }}</span>
            <span class="sf-spacer" />
            <template v-if="editor.move !== undefined">
              <button class="sf-btn sf-btn--subtle sf-btn--icon sf-btn--sm" type="button" aria-label="上移" :disabled="index === 0 || (editor.lockTemplate && row.key === 'template')" :data-testid="editor.prefix + '-layer-up-' + row.key" @click="editor.move(index, -1)">
                <IconChevronUp :size="15" :stroke-width="1.7" />
              </button>
              <button class="sf-btn sf-btn--subtle sf-btn--icon sf-btn--sm" type="button" aria-label="下移" :disabled="index === editor.rows.length - 1 || (editor.lockTemplate && row.key === 'template')" :data-testid="editor.prefix + '-layer-down-' + row.key" @click="editor.move(index, 1)">
                <IconChevronDown :size="15" :stroke-width="1.7" />
              </button>
            </template>
          </div>
        </div>
        <div v-if="editor.note !== ''" class="sf-field__hint">{{ editor.note }}</div>
      </div>

      <template #foot>
        <button class="sf-btn sf-btn--subtle" type="button" data-testid="profile-cancel" @click="dialog = false">取消</button>
        <button class="sf-btn sf-btn--primary" type="button" data-testid="profile-save" @click="save">保存</button>
      </template>
    </SfDialog>

    <!-- 规则源挑选器 -->
    <SfDialog :open="sourcePickerOpen" label="PICK SOURCES" title="添加规则源" @close="sourcePickerOpen = false">
      <input v-model="sourceSearch" class="sf-input" placeholder="搜索规则源…" data-testid="profile-source-search" />
      <div class="pf-picker-list" data-testid="profile-source-picker">
        <template v-if="filteredSourceOptions.length > 0">
          <label v-for="opt in filteredSourceOptions" :key="opt.value" class="pf-picker-row" data-testid="profile-source-option">
            <input
              type="checkbox"
              class="sf-checkbox"
              :checked="sourcePicked.includes(opt.value)"
              :data-testid="'profile-source-check-' + opt.value"
              @change="toggleSourcePick(opt.value)"
            />
            <span class="pf-picker-body">
              <span class="pf-picker-title">{{ opt.title }}</span>
              <span class="sf-faint" style="font-size: 12px">{{ (opt.origin === 'airport' ? '机场' : '远程') + ' · ' + (opt.ruleCount ?? 0) + ' 条' }}</span>
            </span>
          </label>
        </template>
        <div v-else class="pf-picker-empty" data-testid="profile-source-empty">没有可添加的规则源</div>
      </div>
      <template #foot>
        <button class="sf-btn sf-btn--subtle" type="button" data-testid="profile-source-cancel" @click="sourcePickerOpen = false">取消</button>
        <button class="sf-btn sf-btn--primary" type="button" :disabled="sourcePicked.length === 0" data-testid="profile-source-confirm" @click="addPickedSources">添加</button>
      </template>
    </SfDialog>

    <!-- 冲突示例 -->
    <SfDialog :open="conflictDialogOpen" label="CONFLICTS" title="跨源重复规则示例" wide @close="conflictDialogOpen = false">
      <p class="sf-field__hint" style="margin: 0">
        以下匹配条件同时存在于两个规则源。订阅生成时按规则源顺序取先出现的定义。
      </p>
      <div class="sf-table-wrap">
        <table class="sf-table">
          <thead>
            <tr><th>类型</th><th>参数</th><th>先出现</th><th>后出现</th></tr>
          </thead>
          <tbody>
            <tr v-for="(s, i) in sourceConflictSamples" :key="i">
              <td class="sf-mono" style="font-size: 12px">{{ s.type }}</td>
              <td class="sf-mono" style="font-size: 12px">{{ s.param }}</td>
              <td>{{ s.first }}</td>
              <td>{{ s.second }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <template #foot>
        <button class="sf-btn sf-btn--primary" type="button" data-testid="profile-source-conflicts-close" @click="conflictDialogOpen = false">关闭</button>
      </template>
    </SfDialog>

    <!-- 统一组预览 -->
    <SfDialog :open="finalGroupPreviewOpen" label="FINAL GROUP PREVIEW" title="统一最终选择组预览" wide @close="finalGroupPreviewOpen = false">
      <div v-if="finalGroupPreviewLoading" class="sf-faint" data-testid="profile-final-group-preview-loading" style="padding: 12px 0">
        正在读取分组与规则源…
      </div>
      <template v-else-if="finalGroupPreview">
        <div class="pf-preview-lead">
          统一组：<b>{{ finalGroupPreview.report.unified_name }}</b>
          <span class="sf-faint">（{{ finalGroupPreview.report.generated ? '将新建' : '复用已有分组' }}；分组 {{ finalGroupPreview.report.groups_before }} → {{ finalGroupPreview.report.groups_after }}；改写规则 {{ finalGroupPreview.report.rewritten_rules }} 条）</span>
        </div>
        <div class="sf-table-wrap" data-testid="profile-final-group-preview-terminals">
          <table class="sf-table">
            <thead>
              <tr><th>来源</th><th>识别到的最终组</th><th>依据</th><th style="min-width: 180px">来源覆盖</th></tr>
            </thead>
            <tbody>
              <tr v-for="t in finalGroupPreview.report.terminals" :key="t.source + t.group">
                <td class="sf-faint" style="font-size: 12px">{{ t.source }}</td>
                <td>{{ t.group }}</td>
                <td class="sf-faint" style="font-size: 12px">{{ t.evidence }}</td>
                <td>
                  <span class="sf-select-wrap">
                    <select
                      class="sf-select"
                      style="height: 30px; font-size: 12.5px"
                      :value="overrideValue(t.source)"
                      :data-testid="'profile-final-group-override-' + t.source"
                      @change="setOverride(t.source, ($event.target as HTMLSelectElement).value)"
                    >
                      <option v-for="opt in overrideOptions(t.source)" :key="opt.value" :value="opt.value">{{ opt.title }}</option>
                    </select>
                  </span>
                </td>
              </tr>
              <tr v-if="finalGroupPreview.report.terminals.length === 0">
                <td colspan="4" class="sf-faint" style="text-align: center">未识别到来源最终组；保存后仍会把 MATCH 指向统一组。</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-if="finalGroupPreview.report.rule_sources.length > 0">
          <div class="sf-label-mono" style="margin-bottom: 8px">规则源</div>
          <div class="sf-table-wrap" data-testid="profile-final-group-preview-rule-sources">
            <table class="sf-table">
              <thead>
                <tr><th>来源</th><th>兜底目标</th><th>选择目标</th><th style="text-align: right">影响规则</th><th style="min-width: 160px">覆盖选择目标</th></tr>
              </thead>
              <tbody>
                <tr v-for="rs in finalGroupPreview.report.rule_sources" :key="rs.name">
                  <td class="sf-mono" style="font-size: 12px">{{ rs.name }}</td>
                  <td>{{ rs.final_target || '—' }}</td>
                  <td>{{ (rs.selection_targets ?? []).join('、') || '—' }}</td>
                  <td class="sf-table__num" style="text-align: right">{{ rs.affected_rules ?? 0 }}</td>
                  <td>
                    <input
                      class="sf-input sf-input--mono"
                      style="height: 30px; font-size: 12px"
                      :value="ruleOverrideText(rs.name)"
                      placeholder="留空=自动；逗号分隔"
                      :data-testid="'profile-final-group-rule-override-' + rs.name"
                      @input="setRuleOverrideText(rs.name, ($event.target as HTMLInputElement).value)"
                    />
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
        <SfNote v-if="finalGroupPreview.warnings.length > 0" tone="info" data-testid="profile-final-group-preview-warnings">
          <span v-for="w in finalGroupPreview.warnings" :key="w" style="display: block">{{ w }}</span>
        </SfNote>
        <p class="sf-field__hint" style="margin: 0">
          将删除：{{ finalGroupPreview.report.removed_groups.join('、') || '无' }}；引用统一组的分组改写 {{ finalGroupPreview.report.rewritten_group_refs }} 处。修改来源覆盖后请点「重新识别」。
        </p>
      </template>
      <template #foot>
        <button class="sf-btn sf-btn--subtle" type="button" data-testid="profile-final-group-preview-close" @click="finalGroupPreviewOpen = false">关闭</button>
        <button class="sf-btn sf-btn--primary" type="button" :disabled="finalGroupPreviewLoading" data-testid="profile-final-group-preview-refresh" @click="openFinalGroupPreview">
          <IconRefresh :size="14" :stroke-width="1.8" />
          重新识别
        </button>
      </template>
    </SfDialog>
  </div>
</template>

<style scoped>
.pf-spin {
  animation: pf-rotate 0.9s linear infinite;
}

@keyframes pf-rotate {
  to {
    transform: rotate(360deg);
  }
}

.pf-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 10px;
}

.pf-size {
  height: 28px;
  font-size: 12.5px;
  padding: 0 26px 0 8px;
}

.pf-truncate {
  display: inline-block;
  max-width: 260px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  vertical-align: middle;
  font-size: 12px;
}

.pf-row {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 12px;
  align-items: start;
}

.pf-switch-box {
  display: flex;
  align-items: center;
  padding-top: 20px;
}

.pf-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding-top: 14px;
  border-top: 1px solid oklch(from var(--sf-border) l c h / 0.55);
}

.pf-section__head {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.pf-source-list {
  border: 1px solid var(--sf-border);
  border-radius: var(--sf-radius);
  overflow: hidden;
}

.pf-source-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 7px 12px;
  border-bottom: 1px solid oklch(from var(--sf-border) l c h / 0.5);
  background: var(--sf-surface-dim);
}

.pf-source-row:last-child {
  border-bottom: 0;
}

.pf-source-priority {
  color: var(--sf-accent);
  font-size: 10.5px;
  letter-spacing: 0.06em;
  flex: none;
}

.pf-source-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
  font-weight: 500;
}

.pf-source-meta {
  color: var(--sf-text-faint);
  font-size: 12px;
  white-space: nowrap;
}

.pf-source-empty {
  padding: 12px;
  font-size: 13px;
  color: var(--sf-text-faint);
}

.pf-layers {
  border: 1px solid var(--sf-border);
  border-radius: var(--sf-radius);
  overflow: hidden;
}

.pf-layer-row {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 40px;
  padding: 4px 12px;
  border-bottom: 1px solid oklch(from var(--sf-border) l c h / 0.5);
  background: var(--sf-surface-dim);
}

.pf-layer-row:last-child {
  border-bottom: 0;
}

.pf-layer-name {
  min-width: 72px;
  font-weight: 600;
  font-size: 13px;
}

.pf-layer-priority {
  font-size: 10.5px;
  color: var(--sf-text-faint);
  letter-spacing: 0.06em;
}

.pf-multiselect {
  border: 1px solid var(--sf-border);
  border-radius: var(--sf-radius);
  padding: 10px 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 180px;
  overflow-y: auto;
  background: var(--sf-surface-dim);
}

.pf-check {
  display: flex;
  align-items: center;
  gap: 9px;
  font-size: 13px;
  cursor: pointer;
}

.pf-picker-list {
  max-height: 40vh;
  overflow-y: auto;
  border: 1px solid var(--sf-border);
  border-radius: var(--sf-radius);
}

.pf-picker-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px;
  border-bottom: 1px solid oklch(from var(--sf-border) l c h / 0.5);
  cursor: pointer;
  transition: background var(--sf-t-fast) var(--sf-ease);
}

.pf-picker-row:hover {
  background: var(--sf-surface-dim);
}

.pf-picker-row:last-child {
  border-bottom: 0;
}

.pf-picker-body {
  display: flex;
  flex-direction: column;
  gap: 1px;
  min-width: 0;
}

.pf-picker-title {
  font-size: 13px;
  font-weight: 500;
}

.pf-picker-empty {
  padding: 28px;
  text-align: center;
  color: var(--sf-text-faint);
  font-size: 13px;
}

.pf-preview-lead {
  font-size: 13.5px;
  line-height: 1.7;
}

.pf-preview-lead b {
  color: var(--sf-accent);
}
</style>
