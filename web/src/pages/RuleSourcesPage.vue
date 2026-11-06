<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { endpoints, type RuleSource, type RuleSourceUpdateResult, type RuleSourcesResponse, type SelfNode } from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import { notify } from '../composables/useNotify'
import { confirmAction } from '../composables/useConfirm'
import { fieldHelp } from '../content/help'
import SfPageHead from '../components/ui/SfPageHead.vue'
import SfField from '../components/ui/SfField.vue'
import SfDialog from '../components/ui/SfDialog.vue'
import SfEmpty from '../components/ui/SfEmpty.vue'
import SfInfoTip from '../components/ui/SfInfoTip.vue'
import { formatBytes } from '../format'
import { IconRefresh, IconPlus, IconDatabase, IconSearch, IconAlertTriangle } from '@tabler/icons-vue'

const loading = ref(false)
const error = ref('')
const data = ref<RuleSourcesResponse | null>(null)
const sources = ref<RuleSource[]>([])
const manifestPath = ref('')
const defaultManifestPath = ref('')
const outputDir = ref('')
const exists = ref(false)
const hint = ref('')

const mrsDialog = ref(false)
const mrsName = ref('')
const mrsBehavior = ref('domain')

const addDialog = ref(false)
const addName = ref('')
const addUrl = ref('')
const addFormat = ref('')
const addOutbound = ref('')
const addSaving = ref(false)

const selfNodes = ref<SelfNode[]>([])
const outboundHelp = fieldHelp('ruleSource.outbound_node')
const outboundItems = computed(() => [
  { title: '跟随默认出网节点', value: '' },
  { title: '强制直连', value: 'direct' },
  ...selfNodes.value.map((n) => ({
    title: (n.name || `节点 ${n.id}`) + (n.enabled ? '' : '（已停用）'),
    value: String(n.id),
  })),
])
const outboundNames = computed(() => new Map(selfNodes.value.map((n) => [String(n.id), n.name || `节点 ${n.id}`])))
function outboundLabel(value: string): string {
  if (value === 'direct') return '直连'
  return outboundNames.value.get(value) ?? `节点 ${value}`
}

const outboundDialog = ref(false)
const outboundName = ref('')
const outboundValue = ref('')
const outboundSaving = ref(false)
const renameDialog = ref(false)
const renameName = ref('')
const renameValue = ref('')
const renameSaving = ref(false)
const updatingOne = ref('')
// updating 表示「有一条会改动版本状态的请求正在飞」：全部更新、单源更新、回滚、生成 MRS。
// 请求期间把所有会再次发请求的按钮一起 disable —— 之前没有这个状态，双击「全部更新」
// 会真的发两次请求、写两次版本文件（D-12）。
const updating = ref(false)
// updatingAll 单独标记「全部更新」这一路，用于 update-all 按钮自己的 loading。
const updatingAll = ref(false)

const updateResults = ref<RuleSourceUpdateResult[]>([])
const showUpdateResults = ref(false)

async function loadSelfNodes(): Promise<void> {
  try {
    selfNodes.value = await endpoints.listSelfNodes()
  } catch {
    selfNodes.value = []
  }
}

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    const r = await endpoints.listRuleSources()
    data.value = r
    manifestPath.value = r.manifest_path
    defaultManifestPath.value = r.default_manifest_path ?? ''
    outputDir.value = r.output_dir ?? ''
    exists.value = r.exists ?? false
    hint.value = r.hint ?? ''
    sources.value = r.sources ?? []
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

async function updateAll(): Promise<void> {
  if (updating.value) return
  updating.value = true
  updatingAll.value = true
  try {
    const r = await endpoints.updateRuleSources()
    updateResults.value = r.results ?? []
    showUpdateResults.value = true
    notify('更新完成', 'ok')
    await load()
  } catch (err) {
    showUpdateFailure(err)
  } finally {
    updating.value = false
    updatingAll.value = false
  }
}

async function doRollback(name: string): Promise<void> {
  if (updating.value) return
  const ok = await confirmAction({
    title: '确认回滚 ' + name + '？',
    body: '回滚会把该规则源的当前版本换成上一个版本，订阅产物在下一次生成时才会变化。回滚只切换当前版本，不删除任何版本文件，也不改配置。',
    confirmText: '确认回滚',
  })
  if (!ok) return
  updating.value = true
  try {
    const r = await endpoints.rollbackRuleSource(name)
    notify('已回滚: ' + r.path, 'ok')
    await load()
  } catch (err) {
    notify(errorMessage(err), 'error')
  } finally {
    updating.value = false
  }
}

function openMrs(name: string): void {
  mrsName.value = name
  mrsBehavior.value = 'domain'
  mrsDialog.value = true
}

async function doMrs(): Promise<void> {
  if (updating.value) return
  updating.value = true
  try {
    const r = await endpoints.generateMRS(mrsName.value, mrsBehavior.value)
    notify('已生成: ' + r.path + ' (' + formatBytes(r.bytes) + ')', 'ok')
    mrsDialog.value = false
    await load()
  } catch (err) {
    notify(errorMessage(err), 'error')
  } finally {
    updating.value = false
  }
}

function openAdd(): void {
  addName.value = ''
  addUrl.value = ''
  addFormat.value = ''
  addOutbound.value = ''
  addDialog.value = true
}

async function confirmAdd(): Promise<void> {
  if (!addUrl.value.trim() || addSaving.value) return
  addSaving.value = true
  try {
    await endpoints.addRuleSource({
      name: addName.value.trim() || undefined,
      url: addUrl.value.trim(),
      input_format: addFormat.value.trim() || undefined,
      outbound_node: addOutbound.value,
    })
    addDialog.value = false
    notify('已添加: ' + (addName.value.trim() || addUrl.value.trim()), 'ok')
    await load()
  } catch (err) {
    notify(errorMessage(err), 'error')
  } finally {
    addSaving.value = false
  }
}

function openOutbound(source: RuleSource): void {
  if (source.origin === 'airport') return
  outboundName.value = source.name
  outboundValue.value = source.outbound_node ?? ''
  outboundDialog.value = true
}

async function confirmOutbound(): Promise<void> {
  if (outboundSaving.value) return
  outboundSaving.value = true
  try {
    await endpoints.setRuleSourceOutbound(outboundName.value, outboundValue.value)
    outboundDialog.value = false
    notify('出网节点已保存', 'ok')
    await load()
  } catch (err) {
    notify(errorMessage(err), 'error')
  } finally {
    outboundSaving.value = false
  }
}

function openRename(source: RuleSource): void {
  if (source.origin === 'airport') return
  renameName.value = source.name
  renameValue.value = source.name
  renameDialog.value = true
}

async function confirmRename(): Promise<void> {
  if (renameSaving.value) return
  const next = renameValue.value.trim()
  if (next === '' || next === renameName.value) return
  renameSaving.value = true
  try {
    await endpoints.renameRuleSource(renameName.value, next)
    renameDialog.value = false
    notify('已改名为 ' + next, 'ok')
    await load()
  } catch (err) {
    notify(errorMessage(err), 'error')
  } finally {
    renameSaving.value = false
  }
}

async function confirmRemove(name: string): Promise<void> {
  const ok = await confirmAction({
    title: '删除规则源「' + name + '」？',
    body: '该源的本地版本文件不会被删除。',
    confirmText: '删除',
    danger: true,
  })
  if (!ok) return
  try {
    await endpoints.removeRuleSource(name)
    notify('已删除: ' + name, 'ok')
    await load()
  } catch (err) {
    notify(errorMessage(err), 'error')
  }
}

// showUpdateFailure 把更新失败渲染出来。
//
// 服务端在 500 里也带着逐源结果（每条的原因在里面），只弹一句
// 「所有 1 个 source 更新失败」等于把唯一有用的信息丢掉。
function showUpdateFailure(err: unknown): void {
  const body = (err as { body?: unknown }).body
  let message = errorMessage(err)
  let results: RuleSourceUpdateResult[] = []
  if (typeof body === 'string' && body !== '') {
    try {
      const parsed = JSON.parse(body) as { error?: string; results?: RuleSourceUpdateResult[] }
      if (typeof parsed.error === 'string' && parsed.error !== '') message = parsed.error
      if (Array.isArray(parsed.results)) results = parsed.results
    } catch {
      // 响应体不是 JSON（网关错误页等）：保留 errorMessage 的兜底文案。
    }
  }
  if (results.length > 0) {
    updateResults.value = results
    showUpdateResults.value = true
  }
  notify(message, 'error')
}

async function updateOne(name: string): Promise<void> {
  if (updating.value || updatingOne.value) return
  updating.value = true
  updatingOne.value = name
  try {
    const r = await endpoints.updateRuleSourceOne(name)
    updateResults.value = r.results ?? []
    showUpdateResults.value = true
    await load()
  } catch (err) {
    showUpdateFailure(err)
  } finally {
    updating.value = false
    updatingOne.value = ''
  }
}

function originText(s: RuleSource): string {
  return s.origin === 'airport' ? '机场' : '远程'
}

function updatedText(ts?: number): string {
  if (!ts) return '—'
  const d = new Date(ts * 1000)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

// ---- 从规则源挑选规则到自定义层 ----
const rulesDialog = ref(false)
const rulesSource = ref<RuleSource | null>(null)
const rulesList = ref<string[]>([])
const rulesTotal = ref(0)
const rulesOffset = ref(0)
const rulesLimit = 50
const rulesQuery = ref('')
const rulesLoading = ref(false)
const rulesError = ref('')
const rulesSelected = ref<string[]>([])
const rulesConflict = ref<'skip' | 'overwrite'>('skip')
const rulesImporting = ref(false)

const rulesTotalPages = computed(() => Math.max(1, Math.ceil(rulesTotal.value / rulesLimit)))
const rulesPage = computed(() => Math.floor(rulesOffset.value / rulesLimit) + 1)

async function loadRules(): Promise<void> {
  const src = rulesSource.value
  if (src === null) return
  rulesLoading.value = true
  rulesError.value = ''
  try {
    const r = await endpoints.listRuleSourceRules(src.name, {
      q: rulesQuery.value,
      offset: rulesOffset.value,
      limit: rulesLimit,
    })
    rulesList.value = r.rules
    rulesTotal.value = r.total
    rulesOffset.value = r.offset
  } catch (err) {
    rulesError.value = errorMessage(err)
  } finally {
    rulesLoading.value = false
  }
}

function openRules(s: RuleSource): void {
  rulesSource.value = s
  rulesDialog.value = true
  rulesList.value = []
  rulesTotal.value = 0
  rulesOffset.value = 0
  rulesQuery.value = ''
  rulesSelected.value = []
  rulesConflict.value = 'skip'
  rulesError.value = ''
  void loadRules()
}

function searchRules(): void {
  rulesOffset.value = 0
  void loadRules()
}

function clearRulesSearch(): void {
  rulesQuery.value = ''
  rulesOffset.value = 0
  void loadRules()
}

function onRulesPage(next: number): void {
  rulesOffset.value = Math.max(0, (next - 1) * rulesLimit)
  void loadRules()
}

function isRuleSelected(rule: string): boolean {
  return rulesSelected.value.includes(rule)
}

function toggleRule(rule: string): void {
  const i = rulesSelected.value.indexOf(rule)
  if (i >= 0) {
    rulesSelected.value.splice(i, 1)
  } else {
    rulesSelected.value.push(rule)
  }
}

function selectCurrentPage(): void {
  for (const rule of rulesList.value) {
    if (!rulesSelected.value.includes(rule)) {
      rulesSelected.value.push(rule)
    }
  }
}

function clearSelectedRules(): void {
  rulesSelected.value = []
}

async function importSelectedRules(): Promise<void> {
  const src = rulesSource.value
  if (src === null || rulesSelected.value.length === 0 || rulesImporting.value) return
  rulesImporting.value = true
  try {
    const r = await endpoints.importRuleSourceRules({
      name: src.name,
      rules: [...rulesSelected.value],
      conflict: rulesConflict.value,
    })
    let text = '已加入自定义：新增 ' + r.added
    if (r.skipped > 0) text += '，跳过 ' + r.skipped
    if (r.overwrote > 0) text += '，覆盖 ' + r.overwrote
    notify(text, 'ok')
    if (r.warnings && r.warnings.length > 0) {
      notify(r.warnings[0], 'info')
    }
    rulesDialog.value = false
  } catch (err) {
    notify(errorMessage(err), 'error')
  } finally {
    rulesImporting.value = false
  }
}

onMounted(() => {
  void load()
  void loadSelfNodes()
})
</script>

<template>
  <div>
    <SfPageHead title="规则源" desc="远程规则文件的版本化仓库：更新、回滚、生成 MRS，并把规则复制进自定义层。">
      <template #actions>
        <button class="sf-btn sf-btn--ghost" type="button" data-testid="update-all" :disabled="updating" @click="updateAll">
          <IconRefresh :size="15" :stroke-width="1.8" :class="{ 'rs-spin': updatingAll }" />
          {{ updatingAll ? '更新中…' : '全部更新' }}
        </button>
        <button class="sf-btn sf-btn--primary" type="button" data-testid="source-add" @click="openAdd">
          <IconPlus :size="15" :stroke-width="2" />
          新增规则源
        </button>
      </template>
    </SfPageHead>

    <!-- Manifest -->
    <section class="sf-panel sf-rise sf-d1" data-testid="manifest-card" style="margin-bottom: 16px">
      <div class="sf-panel__head">
        <span class="sf-label-mono">MANIFEST / 清单</span>
        <span class="sf-spacer" />
        <span v-if="exists" data-testid="manifest-exists" class="sf-status">
          <span class="sf-led sf-led--ok" />
          <span>已就绪</span>
        </span>
      </div>
      <div class="sf-panel__body rs-manifest">
        <div class="rs-manifest__item" data-testid="manifest-path">
          <span class="rs-manifest__label">生效路径</span>
          <code class="sf-mono">{{ manifestPath }}</code>
        </div>
        <div v-if="defaultManifestPath !== ''" class="rs-manifest__item" data-testid="manifest-default-path">
          <span class="rs-manifest__label">默认路径</span>
          <code class="sf-mono">{{ defaultManifestPath }}</code>
        </div>
        <div v-if="outputDir !== ''" class="rs-manifest__item" data-testid="manifest-output-dir">
          <span class="rs-manifest__label">输出目录</span>
          <code class="sf-mono">{{ outputDir }}</code>
        </div>
        <div v-if="!exists" data-testid="manifest-hint" class="sf-note sf-note--warn" style="margin-top: 6px">{{ hint }}</div>
      </div>
    </section>

    <div v-if="error !== ''" class="sf-note sf-note--danger sf-rise" style="margin-bottom: 16px">
      <IconAlertTriangle :size="16" :stroke-width="1.8" class="sf-note__icon" />
      <span class="sf-note__body">{{ error }}</span>
      <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" @click="load">重试</button>
    </div>

    <div v-if="sources.length === 0 && !loading && error === ''" class="sf-panel">
      <SfEmpty title="暂无规则源" desc="添加远程规则文件地址，或从机场页导入订阅自带规则。">
        <template #icon><IconDatabase :size="30" :stroke-width="1.4" /></template>
        <template #action>
          <button class="sf-btn sf-btn--primary sf-btn--sm" type="button" @click="openAdd">
            <IconPlus :size="14" :stroke-width="2" />
            新增规则源
          </button>
        </template>
      </SfEmpty>
    </div>

    <div v-else class="sf-table-wrap sf-rise sf-d2" data-testid="source-table">
      <table class="sf-table">
        <thead>
          <tr>
            <th>名称</th>
            <th>来源</th>
            <th style="text-align: right">规则数</th>
            <th>更新时间</th>
            <th style="text-align: right">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading && sources.length === 0">
            <td v-for="n in 5" :key="n"><span class="sf-skeleton" style="width: 60%" /></td>
          </tr>
          <tr v-for="s in sources" v-else :key="s.name" data-testid="source-row">
            <td>
              <button type="button" class="rs-name" data-testid="source-rules" @click="openRules(s)">{{ s.label || s.name }}</button>
              <span v-if="s.airport_missing" class="sf-badge sf-badge--warn" style="margin-left: 8px" data-testid="source-airport-missing">机场已删除</span>
            </td>
            <td>
              <div>{{ originText(s) }}</div>
              <div v-if="s.outbound_node" class="sf-field__hint" style="margin: 0" data-testid="source-outbound">出网：{{ outboundLabel(s.outbound_node) }}</div>
            </td>
            <td class="sf-table__num" style="text-align: right">{{ s.rule_count ?? 0 }}</td>
            <td class="sf-table__num">{{ updatedText(s.updated_at) }}</td>
            <td>
              <span class="sf-table__actions">
                <button class="sf-btn sf-btn--subtle sf-btn--sm" type="button" data-testid="source-outbound-btn" :disabled="updating || s.origin === 'airport'" :title="s.origin === 'airport' ? '机场规则源的出网跟随机场设置' : '设置该规则源的出网节点'" @click="openOutbound(s)">出网</button>
                <button class="sf-btn sf-btn--subtle sf-btn--sm" type="button" data-testid="source-rename" :disabled="updating || s.origin === 'airport'" :title="s.origin === 'airport' ? '机场规则源名称跟随机场，请修改机场名称' : '重命名规则源'" @click="openRename(s)">改名</button>
                <button class="sf-btn sf-btn--subtle sf-btn--sm" type="button" data-testid="source-update" :disabled="updating" @click="updateOne(s.name)">{{ updatingOne === s.name ? '更新中…' : '更新' }}</button>
                <button class="sf-btn sf-btn--subtle sf-btn--sm" type="button" data-testid="source-rollback" :disabled="updating || s.versions.length === 0" @click="doRollback(s.name)">回滚</button>
                <button class="sf-btn sf-btn--subtle sf-btn--sm" type="button" data-testid="source-mrs" :disabled="updating || s.versions.length === 0" @click="openMrs(s.name)">生成 MRS</button>
                <button class="sf-btn sf-btn--subtle sf-btn--sm" type="button" data-testid="source-remove" :disabled="sources.length <= 1 || (s.origin === 'airport' && !s.airport_missing)" style="color: var(--sf-danger)" @click="confirmRemove(s.name)">删除</button>
              </span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Update results -->
    <section v-if="showUpdateResults" data-testid="update-results" class="sf-panel sf-rise" style="margin-top: 16px">
      <div class="sf-panel__head">
        <span class="sf-panel__title">更新结果</span>
      </div>
      <div class="sf-panel__body rs-results">
        <div v-for="(r, i) in updateResults" :key="i" class="rs-result">
          <div class="rs-result__head">
            <span class="rs-result__name">{{ r.source }}</span>
            <span class="sf-badge" :class="r.err !== '' ? 'sf-badge--danger' : r.from_cache ? 'sf-badge--info' : 'sf-badge--ok'">
              {{ r.err !== '' ? '失败' : r.from_cache ? '命中缓存' : '已更新' }}
            </span>
          </div>
          <div class="rs-result__meta">版本 {{ r.version }} · <code class="sf-mono">{{ r.path }}</code></div>
          <div v-if="r.err !== ''" data-testid="source-update-error" class="rs-result__err">{{ r.err }}</div>
          <div v-if="r.err !== ''" class="sf-field__hint">该源保留上一版本，产物不受影响</div>
        </div>
      </div>
    </section>

    <!-- 规则挑选器 -->
    <SfDialog :open="rulesDialog" label="PICK RULES" :title="'从规则源复制规则：' + (rulesSource?.label || rulesSource?.name || '')" wide @close="rulesDialog = false">
      <p class="sf-field__hint" style="margin: 0">
        选中的规则会复制到自定义层；规则源后续更新或删除不影响已复制的规则。重复规则默认跳过，可改为覆盖。
      </p>
      <div class="rs-picker-bar">
        <span class="rs-picker-search">
          <IconSearch :size="15" :stroke-width="1.8" class="rs-picker-icon" aria-hidden="true" />
          <input
            v-model="rulesQuery"
            class="sf-input"
            style="padding-left: 31px; height: 32px; font-size: 13px"
            placeholder="搜索规则…"
            data-testid="source-rules-search"
            @keyup.enter="searchRules"
          />
        </span>
        <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" data-testid="source-rules-search-btn" @click="searchRules">搜索</button>
        <button class="sf-btn sf-btn--subtle sf-btn--sm" type="button" data-testid="source-rules-clear-search" @click="clearRulesSearch">清空</button>
        <span class="sf-spacer" />
        <span class="sf-select-wrap" style="width: 150px">
          <select v-model="rulesConflict" class="sf-select" data-testid="source-rules-conflict" style="height: 32px; font-size: 12.5px">
            <option value="skip">重复时跳过</option>
            <option value="overwrite">重复时覆盖</option>
          </select>
        </span>
      </div>

      <div v-if="rulesError !== ''" class="sf-note sf-note--danger">{{ rulesError }}</div>

      <div class="rs-picker-list" data-testid="source-rules-table">
        <div v-if="rulesLoading" style="padding: 14px; display: flex; flex-direction: column; gap: 8px">
          <span v-for="n in 6" :key="n" class="sf-skeleton" :style="{ width: 30 + n * 9 + '%' }" />
        </div>
        <template v-else>
          <label
            v-for="(rule, index) in rulesList"
            :key="index + '-' + rule"
            class="rs-picker-row"
            data-testid="source-rule-row"
          >
            <input type="checkbox" class="sf-checkbox" :checked="isRuleSelected(rule)" data-testid="source-rule-check" @change="toggleRule(rule)" />
            <code class="rs-picker-rule">{{ rule }}</code>
          </label>
          <div v-if="rulesList.length === 0" data-testid="source-rules-empty" class="rs-picker-empty">
            没有匹配的规则
          </div>
        </template>
      </div>

      <div class="rs-picker-foot">
        <span class="sf-pagination__info" data-testid="source-rules-count">共 {{ rulesTotal }} 条 · 已选 {{ rulesSelected.length }} 条</span>
        <button class="sf-btn sf-btn--subtle sf-btn--sm" type="button" data-testid="source-rules-select-page" @click="selectCurrentPage">全选本页</button>
        <button class="sf-btn sf-btn--subtle sf-btn--sm" type="button" data-testid="source-rules-clear-selected" @click="clearSelectedRules">清空已选</button>
        <span class="sf-spacer" />
        <span v-if="rulesTotalPages > 1" class="sf-toolbar" data-testid="source-rules-pagination">
          <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" :disabled="rulesPage <= 1" @click="onRulesPage(rulesPage - 1)">‹ 上一页</button>
          <span class="sf-pagination__info">{{ rulesPage }} / {{ rulesTotalPages }}</span>
          <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" :disabled="rulesPage >= rulesTotalPages" @click="onRulesPage(rulesPage + 1)">下一页 ›</button>
        </span>
      </div>

      <template #foot>
        <button class="sf-btn sf-btn--subtle" type="button" data-testid="source-rules-cancel" @click="rulesDialog = false">取消</button>
        <button class="sf-btn sf-btn--primary" type="button" :disabled="rulesSelected.length === 0 || rulesImporting" data-testid="source-rules-import" @click="importSelectedRules">
          {{ rulesImporting ? '复制中…' : '复制到自定义' }}
        </button>
      </template>
    </SfDialog>

    <!-- MRS -->
    <SfDialog :open="mrsDialog" label="GENERATE MRS" :title="'生成 MRS - ' + mrsName" @close="mrsDialog = false">
      <SfField label="行为" hint="domain=域名集；ipcidr=IP 段集。">
        <span class="sf-select-wrap">
          <select v-model="mrsBehavior" class="sf-select" :disabled="updating">
            <option value="domain">domain</option>
            <option value="ipcidr">ipcidr</option>
          </select>
        </span>
      </SfField>
      <template #foot>
        <button class="sf-btn sf-btn--subtle" type="button" :disabled="updating" @click="mrsDialog = false">取消</button>
        <button class="sf-btn sf-btn--primary" type="button" :disabled="updating" @click="doMrs">生成</button>
      </template>
    </SfDialog>

    <!-- 新增规则源 -->
    <SfDialog :open="addDialog" label="ADD SOURCE" title="新增规则源" @close="addDialog = false">
      <SfField hint="留空时使用订阅地址的最后一段。">
        <template #label><span class="sf-field__label">名称 <SfInfoTip text="留空时使用订阅地址的最后一段。" /></span></template>
        <input v-model="addName" class="sf-input" data-testid="add-name" spellcheck="false" />
      </SfField>
      <SfField>
        <template #label><span class="sf-field__label">订阅地址</span></template>
        <input v-model="addUrl" class="sf-input sf-input--mono" data-testid="add-url" placeholder="https://example.com/rules.yaml" spellcheck="false" />
      </SfField>
      <SfField hint="可选 ini、clash 或 auto；留空时自动识别。">
        <template #label><span class="sf-field__label">输入格式</span></template>
        <input v-model="addFormat" class="sf-input sf-input--mono" data-testid="add-format" spellcheck="false" />
      </SfField>
      <SfField :hint="outboundHelp?.hint ?? ''">
        <template #label><span class="sf-field__label">出网节点 <SfInfoTip :text="outboundHelp?.hint ?? ''" /></span></template>
        <span class="sf-select-wrap">
          <select v-model="addOutbound" class="sf-select" data-testid="add-outbound">
            <option v-for="opt in outboundItems" :key="opt.value" :value="opt.value">{{ opt.title }}</option>
          </select>
        </span>
      </SfField>
      <template #foot>
        <button class="sf-btn sf-btn--subtle" type="button" data-testid="add-cancel" @click="addDialog = false">取消</button>
        <button class="sf-btn sf-btn--primary" type="button" data-testid="add-confirm" :disabled="addSaving" @click="confirmAdd">{{ addSaving ? '添加中…' : '添加' }}</button>
      </template>
    </SfDialog>

    <!-- 出网节点 -->
    <SfDialog :open="outboundDialog" label="OUTBOUND" :title="'设置出网节点 - ' + outboundName" @close="outboundDialog = false">
      <SfField :hint="outboundHelp?.hint ?? ''" label="出网节点">
        <span class="sf-select-wrap">
          <select v-model="outboundValue" class="sf-select" data-testid="source-outbound-select">
            <option v-for="opt in outboundItems" :key="opt.value" :value="opt.value">{{ opt.title }}</option>
          </select>
        </span>
      </SfField>
      <template #foot>
        <button class="sf-btn sf-btn--subtle" type="button" data-testid="source-outbound-cancel" @click="outboundDialog = false">取消</button>
        <button class="sf-btn sf-btn--primary" type="button" :disabled="outboundSaving" data-testid="source-outbound-confirm" @click="confirmOutbound">保存</button>
      </template>
    </SfDialog>

    <!-- 重命名 -->
    <SfDialog :open="renameDialog" label="RENAME" title="重命名规则源" @close="renameDialog = false">
      <SfField hint="引用该规则源的方案会自动更新绑定" label="新名称">
        <input v-model="renameValue" class="sf-input" data-testid="source-rename-input" spellcheck="false" />
      </SfField>
      <template #foot>
        <button class="sf-btn sf-btn--subtle" type="button" data-testid="source-rename-cancel" @click="renameDialog = false">取消</button>
        <button class="sf-btn sf-btn--primary" type="button" :disabled="renameValue.trim() === '' || renameValue.trim() === renameName || renameSaving" data-testid="source-rename-confirm" @click="confirmRename">保存</button>
      </template>
    </SfDialog>
  </div>
</template>

<style scoped>
.rs-spin {
  animation: rs-rotate 0.9s linear infinite;
}

@keyframes rs-rotate {
  to {
    transform: rotate(360deg);
  }
}

.rs-manifest {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.rs-manifest__item {
  display: flex;
  align-items: baseline;
  gap: 12px;
  min-width: 0;
}

.rs-manifest__label {
  flex: none;
  width: 62px;
  font-size: 12px;
  color: var(--sf-text-faint);
}

.rs-manifest__item code {
  font-size: 12px;
  color: var(--sf-text-muted);
  overflow-wrap: anywhere;
}

.rs-name {
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--sf-accent);
  font: inherit;
  font-weight: 600;
  font-size: 13px;
  cursor: pointer;
  text-decoration: underline;
  text-underline-offset: 3px;
  text-decoration-color: oklch(from var(--sf-accent) l c h / 0.4);
}

.rs-name:hover {
  text-decoration-color: var(--sf-accent);
}

.rs-results {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.rs-result {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.rs-result__head {
  display: flex;
  align-items: center;
  gap: 10px;
}

.rs-result__name {
  font-weight: 600;
  font-size: 13.5px;
}

.rs-result__meta {
  font-size: 12px;
  color: var(--sf-text-faint);
  overflow-wrap: anywhere;
}

.rs-result__err {
  font-size: 12.5px;
  color: var(--sf-danger);
  line-height: 1.6;
  overflow-wrap: anywhere;
}

.rs-picker-bar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.rs-picker-search {
  position: relative;
  display: inline-flex;
  width: 240px;
}

.rs-picker-icon {
  position: absolute;
  left: 10px;
  top: 50%;
  transform: translateY(-50%);
  color: var(--sf-text-faint);
  pointer-events: none;
}

.rs-picker-list {
  max-height: 46vh;
  overflow-y: auto;
  border: 1px solid var(--sf-border);
  border-radius: var(--sf-radius);
  background: var(--sf-bg-deep);
}

.rs-picker-row {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 7px 12px;
  border-bottom: 1px solid oklch(from var(--sf-border) l c h / 0.45);
  cursor: pointer;
  transition: background var(--sf-t-fast) var(--sf-ease);
}

.rs-picker-row:hover {
  background: var(--sf-surface-dim);
}

.rs-picker-row:last-child {
  border-bottom: 0;
}

.rs-picker-row input {
  margin-top: 2px;
}

.rs-picker-rule {
  font-family: var(--sf-font-code);
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  color: var(--sf-text-muted);
}

.rs-picker-empty {
  padding: 28px;
  text-align: center;
  color: var(--sf-text-faint);
  font-size: 13px;
}

.rs-picker-foot {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}
</style>
