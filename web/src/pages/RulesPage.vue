<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { endpoints, type ProxyGroup, type Rule, type RuleInput, type RuleProvider, type Suggestion } from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import { notify } from '../composables/useNotify'
import { useCrud, type CrudResource } from '../composables/useCrud'
import PageHeader from '../components/PageHeader.vue'
import { fieldHelp, pageHelp } from '../content/help'
import AsyncSection from '../components/AsyncSection.vue'

// Tab
const tab = ref(0)

// ---------------------------------------------------------------------------
// Block 1: 规则行（D-09）
// 这里不再复用 ResourcePage.vue：多选列要长在规则表行里，而 ResourcePage 是
// 共享组件（不在本次可改范围）。表格在本页自建，但 1:1 保留了 ResourcePage
// 之前提供的 data-testid（table/add/refresh/edit/delete/dialog-*/confirm-*/
// field-*）与列顺序（类型/参数/目标分组/解析DNS/启用/来源），E2E t3a 的
// RL-01..RL-08 选择器契约不受影响。
// ---------------------------------------------------------------------------

// 表格行 = 规则本体 + 后端带出的展示字段（group_name/source）
type RuleRow = RuleInput & { id?: number; group_name?: string; source?: string }

const rulesResource: CrudResource<RuleRow> = {
  list: endpoints.listRules,
  create: (item) => endpoints.createRule(item),
  update: (item) => endpoints.updateRule(item.id as number, item),
  remove: (item) => endpoints.deleteRule(item.id as number),
}

const {
  items: rules,
  error: rulesError,
  loading: rulesLoading,
  load: loadRules,
  save: saveRule,
  destroy: destroyRule,
} = useCrud<RuleRow>(rulesResource)

// 筛选条件（前端过滤，后端 /api/rules 不带过滤参数）：
// 「全选」的口径 = 当前筛选命中的全部行，而不是当前页。
const ruleSearch = ref('')
const ruleEnabledFilter = ref<'all' | 'enabled' | 'disabled'>('all')

const filteredRules = computed<RuleRow[]>(() => {
  const q = ruleSearch.value.trim().toLowerCase()
  const filter = ruleEnabledFilter.value
  return rules.value.filter((r) => {
    if (filter === 'enabled' && !r.enabled) return false
    if (filter === 'disabled' && r.enabled) return false
    if (q === '') return true
    return [r.rule_type, r.rule_param, r.source ?? '', r.group_name ?? ''].some((v) =>
      String(v ?? '').toLowerCase().includes(q),
    )
  })
})

// 选中集合：v-data-table show-select + item-value="id"。
// select-strategy="all" 让表头全选覆盖 items（即当前筛选后的全量行），
// vuetify 默认的 "page" 策略只会选中当前页，不满足 D-09。
const selectedRuleIds = ref<number[]>([])

const ruleHeaders = [
  { key: 'rule_type', title: '类型' },
  { key: 'rule_param', title: '参数' },
  { key: 'group_name', title: '目标分组' },
  { key: 'resolve_dns', title: '解析DNS' },
  { key: 'enabled', title: '启用' },
  { key: 'source', title: '来源' },
  { key: 'actions', title: '操作' },
]

// 操作列单元格挂类（vuetify-overrides.css 据此做「单行不折 + 紧凑按钮」）。
function cellPropsFor(data: { column?: { key?: string | null } }): Record<string, string> {
  return data.column?.key === 'actions' ? { class: 'sf-table__actions' } : {}
}

const rulesEmptyState = pageHelp('rules').emptyState
const rulesEmpty = computed(() =>
  rulesEmptyState !== undefined && rules.value.length === 0 ? rulesEmptyState : rules.value.length === 0,
)

function reloadRules(): void {
  void loadRules()
}

// 新增/编辑弹窗
const ruleDialog = ref(false)
const ruleDraft = ref<RuleRow>({ rule_type: '', rule_param: '', rule_config: 0, resolve_dns: true, enabled: true })
const editingRule = ref<RuleRow | null>(null)

// 单条删除确认
const ruleConfirmOpen = ref(false)
const pendingRuleDelete = ref<RuleRow | null>(null)

// 批量删除确认
const bulkDeleteOpen = ref(false)
const bulkBusy = ref(false)

function blankRule(): RuleRow {
  return {
    rule_type: '',
    rule_param: '',
    rule_config: groupOptions.value.length > 0 ? groupOptions.value[0].value : 0,
    resolve_dns: true,
    enabled: true,
  }
}

function fieldHint(key: string): string {
  const h = fieldHelp(key)
  return h !== undefined && h.hint !== undefined ? h.hint : ''
}

function fieldExample(key: string): string {
  const h = fieldHelp(key)
  return h !== undefined && h.example !== undefined ? h.example : ''
}

function openRuleCreate(): void {
  ruleDraft.value = blankRule()
  editingRule.value = null
  ruleDialog.value = true
}

function openRuleEdit(row: RuleRow): void {
  ruleDraft.value = { ...row }
  editingRule.value = row
  ruleDialog.value = true
}

async function submitRule(): Promise<void> {
  const isCreate = editingRule.value === null
  const ok = await saveRule(ruleDraft.value, editingRule.value)
  if (ok) {
    ruleDialog.value = false
    notify(isCreate ? '已新增' : '已更新')
  } else {
    notify(rulesError.value, 'error')
  }
}

function askRuleDelete(row: RuleRow): void {
  pendingRuleDelete.value = row
  ruleConfirmOpen.value = true
}

async function doRuleDelete(): Promise<void> {
  const row = pendingRuleDelete.value
  ruleConfirmOpen.value = false
  if (row === null) {
    return
  }
  const ok = await destroyRule(row)
  notify(ok ? '已删除' : rulesError.value, ok ? 'ok' : 'error')
}

// 批量启停：POST /api/rules/bulk { ids, enabled } -> { updated }
async function bulkSetEnabled(enabled: boolean): Promise<void> {
  const ids = [...selectedRuleIds.value]
  if (ids.length === 0 || bulkBusy.value) return
  bulkBusy.value = true
  try {
    const res = await endpoints.bulkRules({ ids, enabled })
    notify('已' + (enabled ? '启用' : '禁用') + ' ' + (res.updated ?? ids.length) + ' 条', 'ok')
    selectedRuleIds.value = []
    await loadRules()
  } catch (err) {
    notify(errorMessage(err), 'error')
  } finally {
    bulkBusy.value = false
  }
}

// 批量删除：POST /api/rules/bulk { ids, delete: true } -> { deleted }
async function bulkDelete(): Promise<void> {
  const ids = [...selectedRuleIds.value]
  if (ids.length === 0 || bulkBusy.value) return
  bulkBusy.value = true
  try {
    const res = await endpoints.bulkRules({ ids, delete: true })
    notify('已删除 ' + (res.deleted ?? ids.length) + ' 条', 'ok')
    bulkDeleteOpen.value = false
    selectedRuleIds.value = []
    await loadRules()
  } catch (err) {
    notify(errorMessage(err), 'error')
  } finally {
    bulkBusy.value = false
  }
}

// Block 2: rule providers
const providersLoading = ref(false)
const providersError = ref('')
const providers = ref<RuleProvider[]>([])
const providerEnabledFilter = ref<string>('all')

async function loadProviders(): Promise<void> {
  providersLoading.value = true
  providersError.value = ''
  try {
    const enabled = providerEnabledFilter.value === 'all' ? undefined : providerEnabledFilter.value === 'enabled'
    providers.value = await endpoints.listRuleProviders(undefined, enabled)
  } catch (err) {
    providersError.value = errorMessage(err)
  } finally {
    providersLoading.value = false
  }
}

const togglingProvider = ref<number | null>(null)

async function toggleProvider(p: RuleProvider, value: boolean): Promise<void> {
  togglingProvider.value = p.id
  try {
    await endpoints.updateRuleProvider(p.id, value)
    p.enabled = value
  } catch (err) {
    notify(errorMessage(err), 'error')
  } finally {
    togglingProvider.value = null
  }
}

// T10: rule_config 分组下拉数据。拉失败只留「无分组（DIRECT）」，
// 不能把规则页主功能（列表/新增/编辑）拖死。
const groups = ref<ProxyGroup[]>([])
const groupOptions = computed(() => [
  { title: '无分组（DIRECT）', value: 0 },
  ...groups.value.map((g) => ({ title: `#${g.id} ${g.name}`, value: g.id })),
])

async function loadGroups(): Promise<void> {
  try {
    groups.value = await endpoints.listGroups()
  } catch {
    groups.value = []
  }
}

// Block 3: import candidates (rules with airport: source & enabled=false)
const candidatesLoading = ref(false)
const candidatesError = ref('')
const candidates = ref<Rule[]>([])
const selectedIds = ref<number[]>([])

async function loadCandidates(): Promise<void> {
  candidatesLoading.value = true
  candidatesError.value = ''
  try {
    const all = await endpoints.listRules()
    candidates.value = all.filter((r) => r.source?.startsWith('airport:') && r.enabled === false)
    selectedIds.value = []
  } catch (err) {
    candidatesError.value = errorMessage(err)
  } finally {
    candidatesLoading.value = false
  }
}

async function enableSelected(enabled: boolean): Promise<void> {
  if (selectedIds.value.length === 0) return
  try {
    await endpoints.bulkRules({ ids: [...selectedIds.value], enabled })
    notify('已更新', 'ok')
    await loadCandidates()
  } catch (err) {
    notify(errorMessage(err), 'error')
  }
}

async function enableAllAirport(enabled: boolean): Promise<void> {
  try {
    const all = await endpoints.listRules()
    const ids = all.filter((r) => (r.source ?? '').startsWith('airport:')).map((r) => r.id)
    if (ids.length === 0) { notify('没有来自机场的规则', 'error'); return }
    const r = await endpoints.bulkRules({ ids, enabled })
    notify('已更新 ' + (r.updated ?? 0) + ' 条', 'ok')
    await loadCandidates()
  } catch (err) {
    notify(errorMessage(err), 'error')
  }
}

// Block 4: suggestions
const suggestionsLoading = ref(false)
const suggestionsError = ref('')
const suggestions = ref<Suggestion[]>([])
const suggestionsUnsupported = ref(false)

async function loadSuggestions(): Promise<void> {
  suggestionsLoading.value = true
  suggestionsError.value = ''
  suggestionsUnsupported.value = false
  try {
    suggestions.value = await endpoints.listSuggestions()
  } catch (err) {
    const msg = errorMessage(err)
    if (msg.includes('501') || msg.includes('未启用')) {
      suggestionsUnsupported.value = true
    } else {
      suggestionsError.value = msg
    }
  } finally {
    suggestionsLoading.value = false
  }
}

async function confirmSuggestion(i: number): Promise<void> {
  try {
    await endpoints.confirmSuggestion(i)
    notify('已确认', 'ok')
    await loadSuggestions()
  } catch (err) {
    notify(errorMessage(err), 'error')
  }
}

async function rejectSuggestion(i: number): Promise<void> {
  try {
    await endpoints.rejectSuggestion(i)
    notify('已拒绝', 'ok')
    await loadSuggestions()
  } catch (err) {
    notify(errorMessage(err), 'error')
  }
}

async function applySuggestions(): Promise<void> {
  try {
    const r = await endpoints.applySuggestions()
    notify('已应用 ' + r.applied + ' 条', 'ok')
    await loadSuggestions()
  } catch (err) {
    notify(errorMessage(err), 'error')
  }
}

onMounted(() => {
  void loadRules()
  void loadProviders()
  void loadGroups()
  void loadCandidates()
  void loadSuggestions()
})
</script>

<template>
  <PageHeader title="规则" subtitle="规则行 / 规则集 / 导入候选 / Rule-Bot 建议" help-key="rules" />
  <v-tabs v-model="tab" class="mb-4">
    <v-tab>规则行</v-tab>
    <v-tab>规则集</v-tab>
    <v-tab>导入候选</v-tab>
    <v-tab>Rule-Bot 建议</v-tab>
  </v-tabs>

  <!-- Block 1: 规则行 -->
  <div v-if="tab === 0">
    <v-alert type="info" variant="tonal" density="comfortable" class="mb-4">
      <div>一行规则最终长这样：rule_type,rule_param,目标分组；勾了「解析DNS」会在末尾追加 no-resolve。</div>
      <div>rule_config 选目标代理分组；选「无分组（DIRECT）」表示该行直连。选不到对应分组（分组 id 已不存在）时同样退化为 DIRECT。</div>
      <div>禁用（enabled=false）的规则完全不参与输出。</div>
    </v-alert>
    <PageHeader title="规则行" subtitle="每行 = 输出的一条规则" help-key="rules">
      <template #actions>
        <v-btn color="primary" prepend-icon="mdi-plus" data-testid="add" @click="openRuleCreate">新增</v-btn>
        <v-btn variant="text" prepend-icon="mdi-refresh" data-testid="refresh" @click="reloadRules">刷新</v-btn>
      </template>
    </PageHeader>

    <AsyncSection
      :loading="rulesLoading"
      :error="rulesError"
      :empty="rulesEmpty"
      empty-text="暂无数据"
      @retry="reloadRules"
    >
      <div class="mb-2 d-flex ga-2 align-center flex-wrap">
        <v-text-field
          v-model="ruleSearch"
          label="搜索规则"
          data-testid="rule-search"
          density="compact"
          variant="outlined"
          hide-details
          style="max-width: 260px"
        ></v-text-field>
        <v-btn size="small" variant="tonal" class="mr-2" data-testid="filter-all" @click="ruleEnabledFilter = 'all'">全部</v-btn>
        <v-btn size="small" variant="tonal" class="mr-2" data-testid="filter-enabled" @click="ruleEnabledFilter = 'enabled'">已启用</v-btn>
        <v-btn size="small" variant="tonal" data-testid="filter-disabled" @click="ruleEnabledFilter = 'disabled'">已停用</v-btn>
        <v-spacer></v-spacer>
        <span class="text-caption text-medium-emphasis" data-testid="rule-selection-count">筛选命中 {{ filteredRules.length }} 条 · 已选 {{ selectedRuleIds.length }} 条</span>
      </div>

      <div v-if="selectedRuleIds.length > 0" class="mb-2 d-flex ga-2 align-center" data-testid="bulk-bar">
        <span class="text-body-2" data-testid="bulk-selected-count">已选 {{ selectedRuleIds.length }} 条</span>
        <v-btn size="small" color="primary" data-testid="bulk-enable" :loading="bulkBusy" @click="bulkSetEnabled(true)">批量启用</v-btn>
        <v-btn size="small" variant="tonal" data-testid="bulk-disable" :loading="bulkBusy" @click="bulkSetEnabled(false)">批量禁用</v-btn>
        <v-btn size="small" color="error" data-testid="bulk-delete" :loading="bulkBusy" @click="bulkDeleteOpen = true">批量删除</v-btn>
      </div>

      <v-data-table
        :items="filteredRules"
        :headers="ruleHeaders"
        :cell-props="cellPropsFor"
        show-select
        select-strategy="all"
        v-model="selectedRuleIds"
        item-value="id"
        density="comfortable"
        hover
        data-testid="table"
        class="resource-table"
      >
        <template #item.enabled="{ value }">
          <span class="sf-status">
            <span class="sf-status__dot" :class="value ? 'sf-status__dot--ok' : 'sf-status__dot--muted'"></span>
            <span>{{ value ? '启用' : '停用' }}</span>
          </span>
        </template>
        <template #item.actions="{ item }">
          <v-btn
            icon="mdi-pencil"
            size="small"
            variant="text"
            data-testid="edit"
            @click="openRuleEdit(item)"
          ></v-btn>
          <v-btn
            icon="mdi-delete"
            size="small"
            variant="text"
            color="error"
            data-testid="delete"
            @click="askRuleDelete(item)"
          ></v-btn>
        </template>
      </v-data-table>
    </AsyncSection>

    <!-- 新增/编辑规则 -->
    <v-dialog v-model="ruleDialog" max-width="560">
      <v-card>
        <v-card-title>{{ editingRule === null ? '新增' : '编辑' }}</v-card-title>
        <v-card-text>
          <v-text-field
            v-model="ruleDraft.rule_type"
            label="类型"
            variant="outlined"
            density="comfortable"
            class="mb-2"
            data-testid="field-text"
          ></v-text-field>
          <div v-if="fieldHint('rule_type') !== ''" data-testid="field-hint" class="text-caption text-medium-emphasis mb-2">
            {{ fieldHint('rule_type') }}
          </div>
          <div v-if="fieldExample('rule_type') !== ''" data-testid="field-example" class="text-caption text-medium-emphasis mb-2">
            示例：{{ fieldExample('rule_type') }}
          </div>
          <v-text-field
            v-model="ruleDraft.rule_param"
            label="参数"
            variant="outlined"
            density="comfortable"
            class="mb-2"
            data-testid="field-text"
          ></v-text-field>
          <div v-if="fieldHint('rule_param') !== ''" data-testid="field-hint" class="text-caption text-medium-emphasis mb-2">
            {{ fieldHint('rule_param') }}
          </div>
          <div v-if="fieldExample('rule_param') !== ''" data-testid="field-example" class="text-caption text-medium-emphasis mb-2">
            示例：{{ fieldExample('rule_param') }}
          </div>
          <v-select
            v-model="ruleDraft.rule_config"
            :items="groupOptions"
            item-title="title"
            item-value="value"
            label="配置"
            variant="outlined"
            density="comfortable"
            class="mb-2"
            hide-details
            data-testid="field-select"
          ></v-select>
          <div v-if="fieldHint('rule_config') !== ''" data-testid="field-hint" class="text-caption text-medium-emphasis mb-2">
            {{ fieldHint('rule_config') }}
          </div>
          <div v-if="fieldExample('rule_config') !== ''" data-testid="field-example" class="text-caption text-medium-emphasis mb-2">
            示例：{{ fieldExample('rule_config') }}
          </div>
          <v-switch
            v-model="ruleDraft.resolve_dns"
            label="解析DNS"
            color="primary"
            hide-details
            data-testid="field-switch"
          ></v-switch>
          <div v-if="fieldHint('resolve_dns') !== ''" data-testid="field-hint" class="text-caption text-medium-emphasis mb-2">
            {{ fieldHint('resolve_dns') }}
          </div>
          <div v-if="fieldExample('resolve_dns') !== ''" data-testid="field-example" class="text-caption text-medium-emphasis mb-2">
            示例：{{ fieldExample('resolve_dns') }}
          </div>
          <v-switch
            v-model="ruleDraft.enabled"
            label="启用"
            color="primary"
            hide-details
            data-testid="field-switch"
          ></v-switch>
          <div v-if="fieldHint('enabled') !== ''" data-testid="field-hint" class="text-caption text-medium-emphasis mb-2">
            {{ fieldHint('enabled') }}
          </div>
          <div v-if="fieldExample('enabled') !== ''" data-testid="field-example" class="text-caption text-medium-emphasis mb-2">
            示例：{{ fieldExample('enabled') }}
          </div>
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn variant="text" data-testid="dialog-cancel" @click="ruleDialog = false">取消</v-btn>
          <v-btn color="primary" data-testid="dialog-submit" @click="submitRule">保存</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <!-- 单条删除二次确认 -->
    <v-dialog v-model="ruleConfirmOpen" max-width="420">
      <v-card>
        <v-card-title>确认删除</v-card-title>
        <v-card-text>删除后不可恢复，确定继续？</v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn variant="text" data-testid="confirm-cancel" @click="ruleConfirmOpen = false">取消</v-btn>
          <v-btn color="error" data-testid="confirm-delete" @click="doRuleDelete">删除</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <!-- 批量删除二次确认（文案含「删除」与数量） -->
    <v-dialog v-model="bulkDeleteOpen" max-width="420">
      <v-card>
        <v-card-title>确认批量删除</v-card-title>
        <v-card-text data-testid="bulk-confirm-text">将删除 {{ selectedRuleIds.length }} 条规则，删除后不可恢复，确定继续？</v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn variant="text" data-testid="bulk-confirm-cancel" @click="bulkDeleteOpen = false">取消</v-btn>
          <v-btn color="error" data-testid="bulk-confirm-delete" @click="bulkDelete">删除</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>

  <!-- Block 2: 规则集 -->
  <div v-if="tab === 1">
    <AsyncSection :loading="providersLoading" :error="providersError" :empty="providers.length === 0" empty-text="暂无规则集" @retry="loadProviders">
      <div class="mb-2">
        <v-btn size="small" variant="tonal" class="mr-2" @click="providerEnabledFilter = 'all'; loadProviders()">全部</v-btn>
        <v-btn size="small" variant="tonal" class="mr-2" @click="providerEnabledFilter = 'enabled'; loadProviders()">已启用</v-btn>
        <v-btn size="small" variant="tonal" @click="providerEnabledFilter = 'disabled'; loadProviders()">已禁用</v-btn>
      </div>
      <v-list>
        <v-list-item v-for="p in providers" :key="p.id" :title="p.name" :subtitle="p.source + (p.enabled ? ' · 启用' : ' · 禁用')">
          <template #append>
            <v-switch :model-value="p.enabled" density="compact" :hide-details="true"
              data-testid="provider-switch" :loading="togglingProvider === p.id"
              @update:model-value="toggleProvider(p, $event as boolean)"></v-switch>
          </template>
        </v-list-item>
      </v-list>
      <v-expansion-panels class="mt-2">
        <v-expansion-panel v-for="p in providers" :key="'raw-' + p.id" :title="p.name + ' raw_json'">
          <v-expansion-panel-text eager>
            <pre class="text-caption">{{ p.raw_json }}</pre>
          </v-expansion-panel-text>
        </v-expansion-panel>
      </v-expansion-panels>
    </AsyncSection>
  </div>

  <!-- Block 3: 导入候选 -->
  <div v-if="tab === 2">
    <v-alert type="info" variant="tonal" density="comfortable" class="mb-4" data-testid="candidates-note">
      <div>候选 = 从机场导入进来、但还没有启用的规则。启用它们之前不会影响订阅输出。</div>
      <div>本页在前端筛选（服务端无过滤参数）。</div>
    </v-alert>
    <AsyncSection :loading="candidatesLoading" :error="candidatesError" :empty="candidates.length === 0" empty-text="" :empty-state="pageHelp('rules').emptyState" @retry="loadCandidates">
      <div class="mb-2">
        <v-btn size="small" color="primary" class="mr-2" @click="enableSelected(true)">启用选中</v-btn>
        <v-btn size="small" variant="tonal" class="mr-2" data-testid="disable-selected" :disabled="selectedIds.length === 0" @click="enableSelected(false)">禁用选中</v-btn>
        <v-btn size="small" color="success" data-testid="enable-all-airport" @click="enableAllAirport(true)">启用本机场全部（source 以 airport: 开头的全部规则）</v-btn>
      </div>
      <v-data-table :items="candidates" show-select v-model="selectedIds" item-value="id" density="comfortable">
        <template #item.rule_config="{ item }">
          <span :class="item.rule_config === 0 ? 'text-error font-weight-bold' : ''">
            {{ item.rule_config === 0 ? '目标分组不存在' : item.rule_config }}
          </span>
        </template>
        <template #item.source="{ item }">{{ item.source }}</template>
        <template #item.rule_type="{ item }">{{ item.rule_type }}</template>
      </v-data-table>
    </AsyncSection>
  </div>

  <!-- Block 4: Rule-Bot 建议 -->
  <div v-if="tab === 3">
    <v-card v-if="suggestionsUnsupported" data-testid="suggestions-unsupported" class="pa-4">
      <div class="text-h6">Rule-Bot 建议队列未启用</div>
      <div class="mt-2">建议来自定时任务失败检测；服务启动时会自动启用。如果你看到这条，说明当前进程没有装配调度器。</div>
      <v-btn size="small" color="primary" class="mt-2" @click="loadSuggestions">重试</v-btn>
    </v-card>
    <AsyncSection v-else :loading="suggestionsLoading" :error="suggestionsError" :empty="suggestions.length === 0" empty-text="暂无建议" @retry="loadSuggestions">
      <div class="mb-2">
        <v-btn size="small" color="primary" @click="applySuggestions">应用已确认建议</v-btn>
      </div>
      <v-card v-for="(s, i) in suggestions" :key="i" data-testid="suggestion-card" class="pa-4 mb-2">
        <div class="text-subtitle-1">{{ s.Anomaly.Type }}</div>
        <div class="text-medium-emphasis">{{ s.Anomaly.Message }}</div>
        <pre class="text-caption mt-2">{{ s.Rule }}</pre>
        <div class="mt-2">{{ s.Reason }}</div>
        <div class="text-caption text-medium-emphasis mt-2">{{ s.Anomaly.Source }} · {{ s.Anomaly.Timestamp }}</div>
        <span v-if="s.AutoApply === true" class="sf-status mt-2">
          <span class="sf-status__dot sf-status__dot--muted"></span>
          <span>自动应用</span>
        </span>
        <div class="text-caption text-medium-emphasis mt-2">确认只是标记；要让规则真正生效还要点顶部「应用已确认建议」。</div>
        <div class="mt-2">
          <v-btn size="small" color="success" class="mr-2" data-testid="suggestion-confirm" @click="confirmSuggestion(i)">确认</v-btn>
          <v-btn size="small" color="error" data-testid="suggestion-reject" @click="rejectSuggestion(i)">拒绝</v-btn>
        </div>
      </v-card>
    </AsyncSection>
  </div>
</template>
