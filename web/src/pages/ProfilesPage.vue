<script setup lang="ts">
import { computed, ref } from 'vue'
import PageHeader from '../components/PageHeader.vue'
import AsyncSection from '../components/AsyncSection.vue'
import { endpoints, type Profile } from '../api/endpoints'
import { notify } from '../composables/useNotify'
import { errorMessage } from '../composables/useAsync'

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
  move: (index: number, delta: number) => void
  lockTemplate: boolean
}

const LAYER_KEYS: LayerKey[] = ['custom', 'airport', 'template']
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
  rule_layers: [...LAYER_KEYS],
  rule_sources: [],
  airport_ids: null,
  self_node_ids: null,
  group_layers: [...LAYER_KEYS],
  rule_count: 0,
  group_count: 0,
}

const profiles = ref<Profile[]>([])
const loading = ref(false)
const error = ref('')

// 规则源选项（D-33）：方案可绑定规则源；选项来自 GET /api/rule-sources 的 sources[].name。
const ruleSourceOptions = ref<string[]>([])

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
const ruleLayers = ref<LayerRow[]>(defaultLayers())
const groupLayers = ref<LayerRow[]>(defaultLayers())

const confirmOpen = ref(false)
const deleteTarget = ref<Profile>(EMPTY_PROFILE)

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

// 三层默认全开、顺序 custom → airport → template。
function defaultLayers(): LayerRow[] {
  return LAYER_KEYS.map((key) => ({ key, enabled: true }))
}

// 后端把空数组/空串当默认值；前端同样把缺失或空数组还原成默认三层。
// 顺序即优先级：保持后端给的顺序，未出现的层补在末尾并默认不启用。
function layersToRows(layers: string[] | undefined): LayerRow[] {
  const raw = Array.isArray(layers) && layers.length > 0 ? layers : LAYER_KEYS
  const known = raw.filter((k): k is LayerKey => (LAYER_KEYS as string[]).includes(k))
  const missing = LAYER_KEYS.filter((k) => !known.includes(k))
  return [...known, ...missing].map((key) => ({ key, enabled: known.includes(key) }))
}

// 规则层的 template 恒在末位（后端保存时也会强制移到最后）。
function ruleRowsFrom(layers: string[] | undefined): LayerRow[] {
  const rows = layersToRows(layers)
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

// 规则层移动：template 恒在末位，禁止任何会把它换出末位的移动。
function moveRuleLayer(index: number, delta: number): void {
  const rows = ruleLayers.value
  const target = index + delta
  if (target < 0 || target >= rows.length) return
  if (rows[index].key === 'template' || rows[target].key === 'template') return
  const next = rows.slice()
  const [row] = next.splice(index, 1)
  next.splice(target, 0, row)
  ruleLayers.value = next
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
    hint: '规则层 = 按顺序拼接，先出现者优先；模板规则始终最后（MATCH/FINAL）。',
    rows: ruleLayers.value,
    note: '模板规则始终最后（MATCH/FINAL）',
    move: moveRuleLayer,
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
    const res: { sources?: { name: string }[] } = await endpoints.listRuleSources()
    ruleSourceOptions.value = (res.sources ?? []).map((s) => s.name)
  } catch {
    // 规则源清单加载失败不阻断方案编辑：保留空选项，用户仍可保存其它字段
    ruleSourceOptions.value = []
  }
}

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
  ruleLayers.value = defaultLayers()
  groupLayers.value = defaultLayers()
  // 打开弹窗时重新拉一次规则源与节点来源清单：清单可能已被其它页面改动。
  void loadRuleSourceOptions()
  void loadSourceOptions()
  void loadTargetOptions()
  dialog.value = true
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
  groupLayers.value = layersToRows(row.group_layers)
  // 打开弹窗时重新拉一次规则源与节点来源清单：清单可能已被其它页面改动。
  void loadRuleSourceOptions()
  void loadSourceOptions()
  void loadTargetOptions()
  dialog.value = true
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
    <PageHeader title="订阅方案" subtitle="一个方案 = 一套模板 + 规则 + 分组；令牌绑定方案后彼此隔离">
      <template #actions>
        <v-btn color="primary" prepend-icon="mdi-plus" data-testid="profile-new" @click="openCreate">
          新增
        </v-btn>
        <v-btn variant="text" prepend-icon="mdi-refresh" data-testid="profile-refresh" @click="load">
          刷新
        </v-btn>
      </template>
    </PageHeader>

    <v-alert type="info" variant="tonal" class="mb-4" data-testid="profiles-help">
      <div class="text-body-2">
        <p class="mb-1"><strong>方案是什么：</strong>一个订阅方案 = 一套模板（INI config）+ 一套规则 + 一套分组；令牌绑定方案后，订阅只输出该方案的规则与分组。</p>
        <p class="mb-1"><strong>未分配：</strong>profile_id = 0 的规则与分组就是旧全局池，没绑定方案的令牌行为与改动前完全一致。</p>
        <p class="mb-1"><strong>删除方案：</strong>删除方案前要先把规则/分组移走或删除，否则后端会拒绝（方案下还有内容）。</p>
        <p class="mb-1"><strong>模板留空：</strong>模板留空 = 用默认配置；管线留空 = 用全局回落。</p>
        <p class="mb-0"><strong>层顺序：</strong>规则层 = 按顺序拼接，先出现者优先；分组层 = 同名分组先出现者胜出；模板层 = INI 模板里的规则/分组。</p>
      </div>
    </v-alert>

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
            variant="outlined"
            density="comfortable"
            class="mb-2"
            data-testid="profile-config"
          />
          <v-text-field
            v-model="draft.pipeline"
            label="管线"
            variant="outlined"
            density="comfortable"
            class="mb-2"
            data-testid="profile-pipeline"
          />
          <v-select
            v-model="draft.target"
            :items="targetOptions"
            label="目标格式"
            hint="留空 = clash.meta；与旧版令牌上的 target 同时存在时，旧令牌优先"
            persistent-hint
            clearable
            variant="outlined"
            density="comfortable"
            class="mb-2"
            data-testid="profile-target"
          />
          <v-text-field
            v-model="draft.include"
            label="包含节点"
            hint="逗号分隔的节点名正则，例如 香港,日本；留空 = 不筛选"
            persistent-hint
            variant="outlined"
            density="comfortable"
            class="mb-2"
            data-testid="profile-include"
          />
          <v-text-field
            v-model="draft.exclude"
            label="排除节点"
            hint="逗号分隔的节点名正则，例如 过期,剩余；留空 = 不排除"
            persistent-hint
            variant="outlined"
            density="comfortable"
            class="mb-2"
            data-testid="profile-exclude"
          />
          <v-select
            v-model="draft.rule_sources"
            :items="ruleSourceOptions"
            data-testid="profile-rule-sources"
            label="绑定规则源（产物内联进订阅，并替换模板规则）"
            multiple
            chips
            closable-chips
            variant="outlined"
            hint="可多选；按顺序拼接。规则源页执行更新生成产物后才会生效；产物里的 FINAL/MATCH 会被忽略。"
            persistent-hint
            class="mb-3"
          />
          <v-divider class="mb-3" />
          <div class="text-subtitle-2 mb-1">节点来源</div>
          <v-switch
            v-model="draft.limit_sources"
            label="限制机场/自建节点来源"
            color="primary"
            density="compact"
            hide-details
            data-testid="profile-limit-sources"
          />
          <div class="text-caption text-medium-emphasis mb-3">
            关闭 = 使用所有启用的来源；打开 = 只使用下面勾选的来源，全不选表示不使用该类来源。
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
            <div class="text-subtitle-2 mb-1">{{ editor.caption }}</div>
            <div class="text-caption text-medium-emphasis mb-1">{{ editor.hint }}</div>
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
                icon="mdi-chevron-up"
                size="x-small"
                variant="text"
                :disabled="index === 0 || (editor.lockTemplate && row.key === 'template')"
                :data-testid="editor.prefix + '-layer-up-' + row.key"
                @click="editor.move(index, -1)"
              />
              <v-btn
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
</style>
