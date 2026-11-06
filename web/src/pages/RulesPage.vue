<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { endpoints, type ProxyGroup, type RuleInput, type RuleProvider, type Suggestion } from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import { notify } from '../composables/useNotify'
import { useCrud, type CrudResource } from '../composables/useCrud'
import { confirmAction } from '../composables/useConfirm'
import { fieldHelp } from '../content/help'
import SfPageHead from '../components/ui/SfPageHead.vue'
import SfTabs from '../components/ui/SfTabs.vue'
import SfField from '../components/ui/SfField.vue'
import SfSwitch from '../components/ui/SfSwitch.vue'
import SfDialog from '../components/ui/SfDialog.vue'
import SfEmpty from '../components/ui/SfEmpty.vue'
import SfPagination from '../components/ui/SfPagination.vue'
import SfNote from '../components/ui/SfNote.vue'
import SfInfoTip from '../components/ui/SfInfoTip.vue'
import {
  IconPlus,
  IconRefresh,
  IconPencil,
  IconTrash,
  IconSearch,
  IconAlertTriangle,
  IconFilter,
} from '@tabler/icons-vue'

// Tab
const tab = ref('rules')
const tabs = [
  { value: 'rules', label: '规则行' },
  { value: 'providers', label: '规则集' },
  { value: 'suggestions', label: '规则建议' },
]

// ---------------------------------------------------------------------------
// Block 1: 规则行（D-09）
// 服务端分页 + 服务端筛选；选择列长在表格行里，testid 契约与列顺序保持不变。
// ---------------------------------------------------------------------------

type RuleRow = RuleInput & { id?: number; group_name?: string; source?: string }

// 来源筛选（CONTRACT-V2 §6.2）：null = 全部来源（请求不带参数），'' = 自定义，
// 其它 = 指定 source（如 airport:PEK）。v1 的「按方案归属」模型已作废。
const sourceFilter = ref<string | null>(null)
const ruleSourceCounts = ref<{ source: string; count: number }[]>([])

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

const selectedRuleIds = ref<number[]>([])

const allPageSelected = computed(() => {
  const ids = rules.value.map((r) => r.id as number)
  return ids.length > 0 && ids.every((id) => selectedRuleIds.value.includes(id))
})
const somePageSelected = computed(() => {
  const ids = rules.value.map((r) => r.id as number)
  return ids.some((id) => selectedRuleIds.value.includes(id))
})

function toggleAllPage(): void {
  const ids = rules.value.map((r) => r.id as number)
  if (allPageSelected.value) {
    selectedRuleIds.value = selectedRuleIds.value.filter((id) => !ids.includes(id))
  } else {
    selectedRuleIds.value = [...new Set([...selectedRuleIds.value, ...ids])]
  }
}

function toggleRow(id: number | undefined): void {
  if (id === undefined) return
  if (selectedRuleIds.value.includes(id)) {
    selectedRuleIds.value = selectedRuleIds.value.filter((x) => x !== id)
  } else {
    selectedRuleIds.value = [...selectedRuleIds.value, id]
  }
}

function setEnabledFilter(value: 'all' | 'enabled' | 'disabled'): void {
  ruleEnabledFilter.value = value
  rulePage.value = 1
  void loadRules()
}

function setRulePageSize(next: number): void {
  rulePageSize.value = next
  rulePage.value = 1
  void loadRules()
}

function onRulePage(offset: number): void {
  rulePage.value = Math.floor(offset / rulePageSize.value) + 1
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

const rulesEmpty = computed(() => ruleTotal.value === 0 && !rulesLoading.value)

// 新增/编辑弹窗
const ruleDialog = ref(false)
const ruleDraft = ref<RuleRow>({ rule_type: '', rule_param: '', rule_config: 0, resolve_dns: true, enabled: true })
const editingRule = ref<RuleRow | null>(null)

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

async function askRuleDelete(row: RuleRow): Promise<void> {
  const ok = await confirmAction({ title: '确认删除该规则？', body: '删除后不可恢复。', confirmText: '删除', danger: true })
  if (!ok) return
  const done = await destroyRule(row)
  notify(done ? '已删除' : rulesError.value, done ? 'ok' : 'error')
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
const bulkBusy = ref(false)

async function bulkDelete(): Promise<void> {
  const ids = [...selectedRuleIds.value]
  if (ids.length === 0 || bulkBusy.value) return
  const ok = await confirmAction({
    title: '确认批量删除？',
    body: '将删除 ' + ids.length + ' 条规则，删除后不可恢复。',
    confirmText: '删除',
    danger: true,
  })
  if (!ok) return
  bulkBusy.value = true
  try {
    const res = await endpoints.bulkRules({ ids, delete: true })
    notify('已删除 ' + (res.deleted ?? ids.length) + ' 条', 'ok')
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
const providerRaw = ref<number | null>(null)

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

async function sourceDeleteAll(): Promise<void> {
  const source = sourceTarget()
  if (source === null || sourceBusy.value) return
  const ok = await confirmAction({
    title: '确认按来源删除？',
    body: '将删除来源 “' + source + '” 的 ' + ruleTotal.value + ' 条规则，此操作不可撤销。',
    confirmText: '删除',
    danger: true,
  })
  if (!ok) return
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

// Block 3: suggestions
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
  selectedRuleIds.value = []
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
  <div>
    <SfPageHead title="规则" desc="管理分流规则、规则集与规则建议。">
      <template #actions>
        <template v-if="tab === 'rules'">
          <button class="sf-btn sf-btn--primary" type="button" data-testid="add" @click="openRuleCreate">
            <IconPlus :size="15" :stroke-width="2" />
            新增
          </button>
          <button class="sf-btn sf-btn--ghost" type="button" data-testid="refresh" :disabled="rulesLoading" @click="loadRules()">
            <IconRefresh :size="15" :stroke-width="1.8" :class="{ 'rl-spin': rulesLoading }" />
            刷新
          </button>
        </template>
      </template>
    </SfPageHead>

    <SfTabs v-model="tab" :tabs="tabs" class="sf-rise sf-d1" style="margin-bottom: 18px" />

    <!-- Block 1: 规则行 -->
    <div v-if="tab === 'rules'">
      <div class="rl-toolbar sf-rise sf-d1">
        <span class="sf-select-wrap rl-source">
          <select v-model="sourceFilter" class="sf-select" data-testid="rules-source-filter">
            <option v-for="opt in sourceOptions" :key="String(opt.value)" :value="opt.value">{{ opt.title }}</option>
          </select>
        </span>
        <span class="rl-search-wrap">
          <IconSearch :size="15" :stroke-width="1.8" class="rl-search-icon" aria-hidden="true" />
          <input v-model="ruleSearch" class="sf-input rl-search" placeholder="搜索规则…" data-testid="rule-search" />
        </span>
        <span class="sf-seg" role="group" aria-label="启用筛选">
          <button type="button" class="sf-seg__btn" :class="{ 'is-active': ruleEnabledFilter === 'all' }" data-testid="filter-all" @click="setEnabledFilter('all')">全部</button>
          <button type="button" class="sf-seg__btn" :class="{ 'is-active': ruleEnabledFilter === 'enabled' }" data-testid="filter-enabled" @click="setEnabledFilter('enabled')">已启用</button>
          <button type="button" class="sf-seg__btn" :class="{ 'is-active': ruleEnabledFilter === 'disabled' }" data-testid="filter-disabled" @click="setEnabledFilter('disabled')">已停用</button>
        </span>
        <span class="sf-select-wrap rl-size">
          <select class="sf-select" :value="String(rulePageSize)" data-testid="rule-page-size" @change="setRulePageSize(Number(($event.target as HTMLSelectElement).value))">
            <option v-for="n in [25, 50, 100]" :key="n" :value="String(n)">{{ n }} 条/页</option>
          </select>
        </span>
        <span class="sf-spacer" />
        <span class="sf-faint" style="font-size: 12px" data-testid="rule-selection-count">命中 {{ ruleTotal }} 条 · 已选 {{ selectedRuleIds.length }} 条</span>
      </div>
      <p class="sf-field__hint" style="margin: 0 0 12px">「自定义」为手工录入或从规则源复制的规则；其余来源随导入更新。</p>

      <div v-if="selectedRuleIds.length > 0" class="sf-note sf-note--info rl-bulkbar" data-testid="bulk-bar">
        <span data-testid="bulk-selected-count">已选 {{ selectedRuleIds.length }} 条</span>
        <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" data-testid="bulk-enable" :disabled="bulkBusy" @click="bulkSetEnabled(true)">批量启用</button>
        <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" data-testid="bulk-disable" :disabled="bulkBusy" @click="bulkSetEnabled(false)">批量禁用</button>
        <button class="sf-btn sf-btn--danger sf-btn--sm" type="button" data-testid="bulk-delete" :disabled="bulkBusy" @click="bulkDelete()">批量删除</button>
      </div>

      <div v-if="sourceReady" class="sf-note rl-bulkbar" data-testid="source-batch-bar">
        <span>当前来源「{{ sourceFilter }}」整批：</span>
        <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" data-testid="enable-source-all" :disabled="sourceBusy" @click="sourceEnableAll(true)">启用全部</button>
        <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" data-testid="disable-source-all" :disabled="sourceBusy" @click="sourceEnableAll(false)">停用全部</button>
        <button class="sf-btn sf-btn--danger sf-btn--sm" type="button" data-testid="delete-source-all" :disabled="sourceBusy" @click="sourceDeleteAll()">删除全部</button>
      </div>

      <div v-if="rulesError !== ''" class="sf-note sf-note--danger" style="margin-bottom: 12px">
        <IconAlertTriangle :size="16" :stroke-width="1.8" class="sf-note__icon" />
        <span class="sf-note__body">{{ rulesError }}</span>
        <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" @click="loadRules()">重试</button>
      </div>

      <div v-if="rulesEmpty && rulesError === ''" class="sf-panel">
        <SfEmpty title="暂无规则" desc="新增一条规则，或从机场页 / 规则源页导入。">
          <template #icon><IconFilter :size="30" :stroke-width="1.4" /></template>
        </SfEmpty>
      </div>

      <div v-else class="sf-table-wrap" data-testid="table">
        <table class="sf-table sf-table--scroll">
          <thead>
            <tr>
              <th style="width: 38px">
                <input
                  type="checkbox"
                  class="sf-checkbox"
                  aria-label="全选本页"
                  :checked="allPageSelected"
                  :indeterminate="!allPageSelected && somePageSelected"
                  @change="toggleAllPage"
                />
              </th>
              <th style="width: 132px">类型</th>
              <th>参数</th>
              <th>目标分组</th>
              <th style="width: 84px">解析DNS</th>
              <th style="width: 84px">启用</th>
              <th style="width: 150px">来源</th>
              <th style="width: 92px; text-align: right">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="rulesLoading && rules.length === 0">
              <td v-for="n in 8" :key="n"><span class="sf-skeleton" style="width: 60%" /></td>
            </tr>
            <tr v-for="row in rules" v-else :key="row.id">
              <td>
                <input
                  type="checkbox"
                  class="sf-checkbox"
                  :aria-label="'选择规则 #' + row.id"
                  :checked="selectedRuleIds.includes(row.id as number)"
                  @change="toggleRow(row.id)"
                />
              </td>
              <td><span class="sf-mono" style="font-size: 12px; white-space: nowrap">{{ row.rule_type }}</span></td>
              <td><span class="sf-mono rl-param" :title="row.rule_param">{{ row.rule_param }}</span></td>
              <td>{{ row.group_name !== undefined && row.group_name !== '' ? row.group_name : '—' }}</td>
              <td>
                <span class="sf-status">
                  <span class="sf-led" :class="row.resolve_dns ? 'sf-led--info' : 'sf-led--idle'" />
                  <span class="sf-faint">{{ row.resolve_dns ? '是' : '否' }}</span>
                </span>
              </td>
              <td>
                <span class="sf-status">
                  <span class="sf-led" :class="row.enabled ? 'sf-led--ok' : 'sf-led--idle'" />
                  <span>{{ row.enabled ? '启用' : '停用' }}</span>
                </span>
              </td>
              <td><span class="sf-faint" style="font-size: 12px">{{ row.source === undefined || row.source === '' ? '自定义' : row.source }}</span></td>
              <td>
                <span class="sf-table__actions">
                  <button class="sf-btn sf-btn--subtle sf-btn--icon sf-btn--sm" type="button" aria-label="编辑" data-testid="edit" @click="openRuleEdit(row)">
                    <IconPencil :size="15" :stroke-width="1.7" />
                  </button>
                  <button class="sf-btn sf-btn--subtle sf-btn--icon sf-btn--sm sf-btn--del" type="button" aria-label="删除" data-testid="delete" @click="askRuleDelete(row)">
                    <IconTrash :size="15" :stroke-width="1.7" />
                  </button>
                </span>
              </td>
            </tr>
          </tbody>
        </table>
        <SfPagination :total="ruleTotal" :limit="rulePageSize" :offset="(rulePage - 1) * rulePageSize" @update:offset="onRulePage" />
      </div>

      <!-- 新增/编辑规则 -->
      <SfDialog :open="ruleDialog" :label="editingRule === null ? 'CREATE RULE' : 'EDIT RULE'" :title="editingRule === null ? '新增规则' : '编辑规则'" @close="ruleDialog = false">
        <SfField :hint="fieldHint('rule_type')">
          <template #label><span class="sf-field__label">类型 <SfInfoTip :text="fieldHint('rule_type')" /></span></template>
          <input v-model="ruleDraft.rule_type" class="sf-input sf-input--mono" data-testid="field-text" spellcheck="false" />
        </SfField>
        <SfField :hint="fieldHint('rule_param')">
          <template #label><span class="sf-field__label">参数 <SfInfoTip :text="fieldHint('rule_param')" /></span></template>
          <input v-model="ruleDraft.rule_param" class="sf-input sf-input--mono" data-testid="field-text" spellcheck="false" />
        </SfField>
        <SfField :hint="fieldHint('rule_config')">
          <template #label><span class="sf-field__label">目标分组 <SfInfoTip :text="fieldHint('rule_config')" /></span></template>
          <span class="sf-select-wrap">
            <select v-model="ruleDraft.rule_config" class="sf-select" data-testid="field-select">
              <option v-for="opt in groupOptions" :key="opt.value" :value="opt.value">{{ opt.title }}</option>
            </select>
          </span>
        </SfField>
        <div class="rl-switch-row">
          <span>解析 DNS <SfInfoTip :text="fieldHint('resolve_dns')" /></span>
          <SfSwitch v-model="ruleDraft.resolve_dns" data-testid="field-switch" />
        </div>
        <div class="rl-switch-row">
          <span>启用 <SfInfoTip :text="fieldHint('enabled')" /></span>
          <SfSwitch v-model="ruleDraft.enabled" data-testid="field-switch" />
        </div>
        <template #foot>
          <button class="sf-btn sf-btn--subtle" type="button" data-testid="dialog-cancel" @click="ruleDialog = false">取消</button>
          <button class="sf-btn sf-btn--primary" type="button" data-testid="dialog-submit" @click="submitRule">保存</button>
        </template>
      </SfDialog>
    </div>

    <!-- Block 2: 规则集 -->
    <div v-if="tab === 'providers'">
      <div class="sf-toolbar" style="margin-bottom: 14px">
        <span class="sf-seg" role="group" aria-label="规则集筛选">
          <button type="button" class="sf-seg__btn" :class="{ 'is-active': providerEnabledFilter === 'all' }" @click="providerEnabledFilter = 'all'; loadProviders()">全部</button>
          <button type="button" class="sf-seg__btn" :class="{ 'is-active': providerEnabledFilter === 'enabled' }" @click="providerEnabledFilter = 'enabled'; loadProviders()">已启用</button>
          <button type="button" class="sf-seg__btn" :class="{ 'is-active': providerEnabledFilter === 'disabled' }" @click="providerEnabledFilter = 'disabled'; loadProviders()">已停用</button>
        </span>
        <span class="sf-spacer" />
        <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" :disabled="providersLoading" @click="loadProviders()">
          <IconRefresh :size="14" :stroke-width="1.8" />
          刷新
        </button>
      </div>

      <div v-if="providersError !== ''" class="sf-note sf-note--danger" style="margin-bottom: 12px">
        <span class="sf-note__body">{{ providersError }}</span>
        <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" @click="loadProviders()">重试</button>
      </div>

      <div v-if="providers.length === 0 && !providersLoading && providersError === ''" class="sf-panel">
        <SfEmpty title="暂无规则集" desc="规则集（rule provider）由机场导入或规则源生成。" />
      </div>

      <div v-else class="sf-table-wrap">
        <table class="sf-table">
          <thead>
            <tr>
              <th>名称</th>
              <th>来源</th>
              <th style="width: 100px">启用</th>
              <th style="width: 90px; text-align: right">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="providersLoading && providers.length === 0">
              <td v-for="n in 4" :key="n"><span class="sf-skeleton" style="width: 60%" /></td>
            </tr>
            <tr v-for="p in providers" v-else :key="p.id">
              <td><span class="sf-mono" style="font-size: 12.5px">{{ p.name }}</span></td>
              <td><span class="sf-faint" style="font-size: 12px">{{ p.source }}</span></td>
              <td>
                <SfSwitch :model-value="p.enabled" :disabled="togglingProvider === p.id" data-testid="provider-switch" @update:model-value="toggleProvider(p, $event)" />
              </td>
              <td style="text-align: right">
                <button class="sf-btn sf-btn--subtle sf-btn--sm" type="button" @click="providerRaw = providerRaw === p.id ? null : p.id">
                  {{ providerRaw === p.id ? '收起' : 'raw_json' }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
        <div v-for="p in providers" v-show="providerRaw === p.id" :key="'raw-' + p.id" style="padding: 12px 16px; border-top: 1px solid var(--sf-border); background: var(--sf-bg-deep)">
          <pre class="rl-raw">{{ p.raw_json }}</pre>
        </div>
      </div>
    </div>

    <!-- Block 3: 规则建议 -->
    <div v-if="tab === 'suggestions'">
      <div v-if="suggestionsUnsupported" data-testid="suggestions-unsupported">
        <SfNote tone="warn">
          <b>规则建议队列未启用</b>——建议由定时任务生成；当前进程未启用调度器。
          <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" style="margin-left: 10px" @click="loadSuggestions">重试</button>
        </SfNote>
      </div>
      <template v-else>
        <div v-if="suggestionsError !== ''" class="sf-note sf-note--danger" style="margin-bottom: 12px">
          <span class="sf-note__body">{{ suggestionsError }}</span>
          <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" @click="loadSuggestions">重试</button>
        </div>
        <div v-if="suggestions.length === 0 && !suggestionsLoading && suggestionsError === ''" class="sf-panel">
          <SfEmpty title="暂无建议" desc="建议由定时任务根据转换失败/告警自动生成。" />
        </div>
        <template v-else>
          <div style="margin-bottom: 14px">
            <button class="sf-btn sf-btn--primary sf-btn--sm" type="button" @click="applySuggestions">应用已确认建议</button>
          </div>
          <div class="rl-suggestions">
            <div v-for="(s, i) in suggestions" :key="i" data-testid="suggestion-card" class="sf-panel rl-suggestion">
              <div class="rl-suggestion__head">
                <span class="sf-badge sf-badge--accent">{{ s.Anomaly.Type }}</span>
                <span class="sf-badge" :class="s.AutoApply === true ? 'sf-badge--info' : ''">{{ s.AutoApply === true ? '自动应用' : '需人工确认' }}</span>
                <span class="sf-spacer" />
                <span class="sf-faint sf-mono" style="font-size: 11px">{{ s.Anomaly.Source }} · {{ s.Anomaly.Timestamp }}</span>
              </div>
              <p class="rl-suggestion__msg">{{ s.Anomaly.Message }}</p>
              <pre class="rl-suggestion__rule">{{ s.Rule }}</pre>
              <p class="rl-suggestion__reason">{{ s.Reason }}</p>
              <p class="sf-field__hint" style="margin: 0">确认后仍需点击「应用已确认建议」才会写入规则。</p>
              <div class="sf-toolbar">
                <button class="sf-btn sf-btn--primary sf-btn--sm" type="button" data-testid="suggestion-confirm" @click="confirmSuggestion(i)">确认</button>
                <button class="sf-btn sf-btn--danger sf-btn--sm" type="button" data-testid="suggestion-reject" @click="rejectSuggestion(i)">拒绝</button>
              </div>
            </div>
          </div>
        </template>
      </template>
    </div>
  </div>
</template>

<style scoped>
.rl-spin {
  animation: rl-rotate 0.9s linear infinite;
}

@keyframes rl-rotate {
  to {
    transform: rotate(360deg);
  }
}

.rl-toolbar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
}

.rl-source {
  width: 220px;
  flex: none;
}

.rl-search-wrap {
  position: relative;
  display: inline-flex;
  width: 264px;
}

.rl-search-icon {
  position: absolute;
  left: 10px;
  top: 50%;
  transform: translateY(-50%);
  color: var(--sf-text-faint);
  pointer-events: none;
}

.rl-search {
  padding-left: 31px;
  font-size: 13px;
}

.rl-size {
  width: 104px;
  flex: none;
}

.rl-bulkbar {
  margin-bottom: 12px;
  flex-wrap: wrap;
  align-items: center;
}

.rl-param {
  font-size: 12px;
  /* break-word 而非 anywhere：anywhere 会把列的最小内容宽度压到两字符，表格布局下参数列被挤成竖排 */
  overflow-wrap: break-word;
  min-width: 96px;
}

.rl-switch-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  font-size: 13.5px;
  padding: 4px 0;
}

.rl-raw {
  margin: 0;
  font-family: var(--sf-font-code);
  font-size: 11.5px;
  line-height: 1.65;
  color: var(--sf-text-muted);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  max-height: 320px;
  overflow-y: auto;
}

.rl-suggestions {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.rl-suggestion {
  padding: 16px 18px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.rl-suggestion__head {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.rl-suggestion__msg {
  margin: 0;
  font-size: 13.5px;
  line-height: 1.65;
}

.rl-suggestion__rule {
  margin: 0;
  padding: 9px 12px;
  border: 1px solid var(--sf-border);
  border-radius: var(--sf-radius-sm);
  background: var(--sf-bg-deep);
  font-family: var(--sf-font-code);
  font-size: 11.5px;
  line-height: 1.6;
  color: var(--sf-text-muted);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.rl-suggestion__reason {
  margin: 0;
  font-size: 13px;
  color: var(--sf-text-muted);
  line-height: 1.65;
}
</style>
