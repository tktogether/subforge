<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { endpoints, type ProxyGroup, type RuleInput, type RuleProvider, type Suggestion } from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import { notify } from '../composables/useNotify'
import { useCrud, type CrudResource } from '../composables/useCrud'
import PageHeader from '../components/PageHeader.vue'
import { fieldHelp } from '../content/help'
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

// 来源筛选（CONTRACT-V2 §6.2）：null = 全部来源（请求不带参数），'' = 自定义，
// 其它 = 指定 source（如 airport:PEK）。v1 的「按方案归属」模型已作废。
const sourceFilter = ref<string | null>(null)
const ruleSourceCounts = ref<{ source: string; count: number }[]>([])

// 来源下拉选项：全部来源 / 自定义 / 历史来源。轻量端点只返回 source+count，
// 不再为了下拉去拉全量规则。
const sourceOptions = computed<{ title: string; value: string | null }[]>(() => [
  { title: '全部来源', value: null },
  { title: '自定义', value: '' },
  ...ruleSourceCounts.value
    .filter((s) => s.source !== '')
    .sort((a, b) => a.source.localeCompare(b.source))
    .map((s) => ({ title: s.source + ' · ' + s.count + ' 条', value: s.source })),
])

async function loadSources(): Promise<void> {
  try {
    ruleSourceCounts.value = await endpoints.listRuleSourceCounts()
  } catch {
    ruleSourceCounts.value = []
  }
}

// 服务端分页 + 服务端筛选：页面每次只取当前页，首屏不再拉全量。
const ruleTotal = ref(0)
const rulePage = ref(1)
const rulePageSize = ref(25)
const ruleSearch = ref('')
const ruleEnabledFilter = ref<'all' | 'enabled' | 'disabled'>('all')

async function fetchRules(): Promise<RuleRow[]> {
  const enabled = ruleEnabledFilter.value === 'all' ? undefined : ruleEnabledFilter.value === 'enabled'
  const r = await endpoints.listRulesPage({
    source: sourceFilter.value,
    enabled,
    q: ruleSearch.value.trim(),
    limit: rulePageSize.value,
    offset: (rulePage.value - 1) * rulePageSize.value,
  })
  ruleTotal.value = r.total
  return r.rules as RuleRow[]
}

const rulesResource: CrudResource<RuleRow> = {
  list: fetchRules,
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

// 筛选条件全部走服务端：enabled/q/source 改变后重置到第一页再取数。
const selectedRuleIds = ref<number[]>([])

function setEnabledFilter(value: 'all' | 'enabled' | 'disabled'): void {
  ruleEnabledFilter.value = value
  rulePage.value = 1
  void loadRules()
}

function onRuleOptions(options: { page: number; itemsPerPage: number }): void {
  if (options.page === rulePage.value && options.itemsPerPage === rulePageSize.value) {
    return
  }
  rulePage.value = options.page
  rulePageSize.value = options.itemsPerPage
  void loadRules()
}

let searchTimer: ReturnType<typeof setTimeout> | undefined
watch(ruleSearch, () => {
  if (searchTimer !== undefined) {
    clearTimeout(searchTimer)
  }
  searchTimer = setTimeout(() => {
    rulePage.value = 1
    void loadRules()
  }, 300)
})

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

const rulesEmpty = computed(() => ruleTotal.value === 0 && !rulesLoading.value)

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

// 按来源整批操作（历史来源兼容）：必须走带 source 的 bulk 接口，
// 不允许「拉全量再在前端筛 id」。
const sourceBusy = ref(false)
const sourceDeleteOpen = ref(false)

function sourceTarget(): string | null {
  const filter = sourceFilter.value
  return filter === null || filter === '' ? null : filter
}

const sourceReady = computed(() => sourceTarget() !== null)

async function sourceEnableAll(enabled: boolean): Promise<void> {
  const source = sourceTarget()
  if (source === null || sourceBusy.value) return
  sourceBusy.value = true
  try {
    const res = await endpoints.bulkRules({ source, sourcePrefix: false, enabled })
    notify('已' + (enabled ? '启用' : '停用') + ' ' + (res.updated ?? ruleTotal.value) + ' 条', 'ok')
    await loadRules()
  } catch (err) {
    notify(errorMessage(err), 'error')
  } finally {
    sourceBusy.value = false
  }
}

function openSourceDelete(): void {
  if (sourceTarget() === null || sourceBusy.value) return
  sourceDeleteOpen.value = true
}

async function confirmSourceDelete(): Promise<void> {
  const source = sourceTarget()
  sourceDeleteOpen.value = false
  if (source === null || sourceBusy.value) return
  sourceBusy.value = true
  try {
    const res = await endpoints.bulkRules({ source, sourcePrefix: false, delete: true })
    notify('已删除 ' + (res.deleted ?? ruleTotal.value) + ' 条', 'ok')
    await loadRules()
    await loadSources()
  } catch (err) {
    notify(errorMessage(err), 'error')
  } finally {
    sourceBusy.value = false
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

// 切换来源筛选：重置到第一页再取数。
watch(sourceFilter, () => {
  rulePage.value = 1
  void loadRules()
})

onBeforeUnmount(() => {
  if (searchTimer !== undefined) {
    clearTimeout(searchTimer)
  }
})

onMounted(() => {
  void loadRules()
  void loadProviders()
  void loadGroups()
  void loadSources()
  void loadSuggestions()
})
</script>

<template>
  <PageHeader title="规则" subtitle="规则行 / 规则集 / Rule-Bot 建议" help-key="rules" />
  <div class="mb-4 d-flex ga-2 align-center flex-wrap">
    <v-select
      v-model="sourceFilter"
      :items="sourceOptions"
      item-title="title"
      item-value="value"
      label="来源筛选"
      density="compact"
      variant="outlined"
      hide-details
      style="max-width: 260px"
      data-testid="rules-source-filter"
    ></v-select>
    <span class="text-caption text-medium-emphasis">全部来源 = 不带过滤；自定义 = 手工录入或从规则源挑选的规则；其它为历史来源。</span>
  </div>
  <v-tabs v-model="tab" class="mb-4">
    <v-tab>规则行</v-tab>
    <v-tab>规则集</v-tab>
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
        <v-btn size="small" variant="tonal" class="mr-2" data-testid="filter-all" @click="setEnabledFilter('all')">全部</v-btn>
        <v-btn size="small" variant="tonal" class="mr-2" data-testid="filter-enabled" @click="setEnabledFilter('enabled')">已启用</v-btn>
        <v-btn size="small" variant="tonal" data-testid="filter-disabled" @click="setEnabledFilter('disabled')">已停用</v-btn>
        <v-spacer></v-spacer>
        <span class="text-caption text-medium-emphasis" data-testid="rule-selection-count">筛选命中 {{ ruleTotal }} 条 · 已选 {{ selectedRuleIds.length }} 条</span>
      </div>

      <div v-if="selectedRuleIds.length > 0" class="mb-2 d-flex ga-2 align-center" data-testid="bulk-bar">
        <span class="text-body-2" data-testid="bulk-selected-count">已选 {{ selectedRuleIds.length }} 条</span>
        <v-btn size="small" color="primary" data-testid="bulk-enable" :loading="bulkBusy" @click="bulkSetEnabled(true)">批量启用</v-btn>
        <v-btn size="small" variant="tonal" data-testid="bulk-disable" :loading="bulkBusy" @click="bulkSetEnabled(false)">批量禁用</v-btn>
        <v-btn size="small" color="error" data-testid="bulk-delete" :loading="bulkBusy" @click="bulkDeleteOpen = true">批量删除</v-btn>
      </div>

      <div v-if="sourceReady" class="mb-2 d-flex ga-2 align-center flex-wrap" data-testid="source-batch-bar">
        <span class="text-caption text-medium-emphasis">当前来源整批：</span>
        <v-btn size="small" color="success" data-testid="enable-source-all" :loading="sourceBusy" @click="sourceEnableAll(true)">启用全部</v-btn>
        <v-btn size="small" variant="tonal" data-testid="disable-source-all" :loading="sourceBusy" @click="sourceEnableAll(false)">停用全部</v-btn>
        <v-btn size="small" color="error" data-testid="delete-source-all" :loading="sourceBusy" @click="openSourceDelete">删除全部</v-btn>
      </div>

    <AsyncSection
      :loading="rulesLoading"
      :error="rulesError"
      :empty="rulesEmpty"
      empty-text="暂无数据"
      @retry="reloadRules"
    >
      <v-data-table-server
        :items="rules"
        :headers="ruleHeaders"
        :items-length="ruleTotal"
        :items-per-page="rulePageSize"
        :page="rulePage"
        :items-per-page-options="[25, 50, 100]"
        :cell-props="cellPropsFor"
        show-select
        select-strategy="all"
        v-model="selectedRuleIds"
        item-value="id"
        density="comfortable"
        hover
        data-testid="table"
        class="resource-table"
        @update:options="onRuleOptions"
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
      </v-data-table-server>
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

  <!-- 按来源整批删除二次确认（历史来源兼容） -->
  <v-dialog v-model="sourceDeleteOpen" max-width="420" data-testid="confirm-source-delete">
    <v-card>
      <v-card-title>确认按来源删除</v-card-title>
      <v-card-text data-testid="confirm-source-delete-text">将删除来源 “{{ sourceFilter }}” 的 {{ ruleTotal }} 条规则，此操作不可撤销。</v-card-text>
      <v-card-actions>
        <v-spacer></v-spacer>
        <v-btn variant="text" data-testid="confirm-source-delete-cancel" @click="sourceDeleteOpen = false">取消</v-btn>
        <v-btn color="error" data-testid="confirm-source-delete-ok" @click="confirmSourceDelete">删除</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <!-- Block 3: Rule-Bot 建议 -->
  <div v-if="tab === 2">
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
