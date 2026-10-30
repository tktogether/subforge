<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { endpoints, type RuleSource, type RuleSourceUpdateResult, type RuleSourcesResponse } from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import { notify } from '../composables/useNotify'
import { pageHelp, glossaryFor } from '../content/help'
import PageHeader from '../components/PageHeader.vue'
import AsyncSection from '../components/AsyncSection.vue'
import { formatBytes } from '../format'

const help = pageHelp('rule-sources')

const loading = ref(false)
const error = ref('')
const data = ref<RuleSourcesResponse | null>(null)
const sources = ref<RuleSource[]>([])
const manifestPath = ref('')
const defaultManifestPath = ref('')
const outputDir = ref('')
const exists = ref(false)
const hint = ref('')

const rollbackDialog = ref(false)
const rollbackName = ref('')

const mrsDialog = ref(false)
const mrsName = ref('')
const mrsBehavior = ref('domain')

const addDialog = ref(false)
const addName = ref('')
const addUrl = ref('')
const addFormat = ref('')
const addSaving = ref(false)
const removeDialog = ref(false)
const removeName = ref('')
const removeSaving = ref(false)
const updatingOne = ref('')
// updating 表示「有一条会改动版本状态的请求正在飞」：全部更新、单源更新、回滚、生成 MRS。
// 请求期间把所有会再次发请求的按钮一起 disable —— 之前没有这个状态，双击「全部更新」
// 会真的发两次请求、写两次版本文件（D-12）。
const updating = ref(false)
// updatingAll 单独标记「全部更新」这一路，用于 update-all 按钮自己的 loading。
const updatingAll = ref(false)

const updateResults = ref<RuleSourceUpdateResult[]>([])
const showUpdateResults = ref(false)

const emptyState = computed(() => help.emptyState)

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

function openRollback(name: string): void {
  rollbackName.value = name
  rollbackDialog.value = true
}

async function doRollback(): Promise<void> {
  if (updating.value) return
  updating.value = true
  try {
    const r = await endpoints.rollbackRuleSource(rollbackName.value)
    notify('已回滚: ' + r.path, 'ok')
    rollbackDialog.value = false
    await load()
  } catch (err) {
    notify(errorMessage(err), 'error')
  } finally {
    updating.value = false
  }
}

function cancelRollback(): void {
  rollbackDialog.value = false
  rollbackName.value = ''
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

function cancelMrs(): void {
  mrsDialog.value = false
}

function openAdd(): void {
  addName.value = ''
  addUrl.value = ''
  addFormat.value = ''
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

function openRemove(name: string): void {
  removeName.value = name
  removeDialog.value = true
}

async function confirmRemove(): Promise<void> {
  if (removeSaving.value) return
  removeSaving.value = true
  try {
    await endpoints.removeRuleSource(removeName.value)
    removeDialog.value = false
    notify('已删除: ' + removeName.value, 'ok')
    await load()
  } catch (err) {
    notify(errorMessage(err), 'error')
  } finally {
    removeSaving.value = false
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

const rulesPage = computed({
  get: () => Math.floor(rulesOffset.value / rulesLimit) + 1,
  set: (p: number) => {
    rulesOffset.value = Math.max(0, (p - 1) * rulesLimit)
    void loadRules()
  },
})
const rulesTotalPages = computed(() => Math.max(1, Math.ceil(rulesTotal.value / rulesLimit)))

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
})
</script>

<template>
  <PageHeader title="规则源" :subtitle="help.oneLiner" help-key="rule-sources">
    <template #actions>
      <v-btn data-testid="update-all" variant="tonal" :loading="updatingAll" :disabled="updating" @click="updateAll">全部更新</v-btn>
      <v-btn data-testid="source-add" color="primary" variant="tonal" @click="openAdd">新增规则源</v-btn>

  <!-- Pick rules from source into custom layer -->
  <v-dialog v-model="rulesDialog" max-width="900">
    <v-card data-testid="source-rules-dialog">
      <v-card-title>从规则源挑选 · {{ rulesSource?.label || rulesSource?.name }}</v-card-title>
      <v-card-text>
        <div class="text-caption text-medium-emphasis mb-2">
          选中的规则会复制到自定义层；源后续更新/删除不影响已复制规则。重复规则默认跳过，可切换为覆盖。
        </div>
        <div class="d-flex ga-2 align-center mb-2 flex-wrap">
          <v-text-field v-model="rulesQuery" label="搜索规则" density="compact" hide-details data-testid="source-rules-search" @keyup.enter="searchRules" />
          <v-btn variant="tonal" data-testid="source-rules-search-btn" @click="searchRules">搜索</v-btn>
          <v-btn variant="text" data-testid="source-rules-clear-search" @click="clearRulesSearch">清空</v-btn>
        </div>
        <v-alert v-if="rulesError !== ''" type="error" variant="tonal" class="mb-2">{{ rulesError }}</v-alert>
        <v-progress-linear v-if="rulesLoading" indeterminate class="mb-2" />
        <div class="rule-picker-list">
          <v-table density="compact" data-testid="source-rules-table">
            <thead>
              <tr><th style="width: 48px"></th><th>规则</th></tr>
            </thead>
            <tbody>
              <tr v-for="(rule, index) in rulesList" :key="index + '-' + rule" data-testid="source-rule-row">
                <td>
                  <v-checkbox :model-value="isRuleSelected(rule)" hide-details density="compact" data-testid="source-rule-check" @update:model-value="toggleRule(rule)" />
                </td>
                <td><code class="rule-source-rule">{{ rule }}</code></td>
              </tr>
            </tbody>
          </v-table>
          <div v-if="!rulesLoading && rulesList.length === 0" data-testid="source-rules-empty" class="text-medium-emphasis py-4 text-center">
            没有匹配的规则
          </div>
        </div>
        <div class="d-flex align-center ga-3 mt-2 flex-wrap">
          <span class="text-body-2" data-testid="source-rules-count">共 {{ rulesTotal }} 条，已选 {{ rulesSelected.length }} 条</span>
          <v-btn size="small" variant="text" data-testid="source-rules-select-page" @click="selectCurrentPage">全选本页</v-btn>
          <v-btn size="small" variant="text" data-testid="source-rules-clear-selected" @click="clearSelectedRules">清空已选</v-btn>
          <v-spacer />
          <v-select
            v-model="rulesConflict"
            :items="[{ title: '重复时跳过', value: 'skip' }, { title: '重复时覆盖', value: 'overwrite' }]"
            item-title="title"
            item-value="value"
            density="compact"
            hide-details
            style="max-width: 160px"
            data-testid="source-rules-conflict"
          />
        </div>
        <v-pagination
          v-if="rulesTotalPages > 1"
          v-model="rulesPage"
          :length="rulesTotalPages"
          :total-visible="7"
          density="compact"
          class="mt-2"
          data-testid="source-rules-pagination"
        />
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" data-testid="source-rules-cancel" @click="rulesDialog = false">取消</v-btn>
        <v-btn color="primary" :loading="rulesImporting" :disabled="rulesSelected.length === 0" data-testid="source-rules-import" @click="importSelectedRules">加入自定义</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
  </PageHeader>

      <!-- Manifest card -->
    <v-card data-testid="manifest-card" class="mb-4">
      <v-card-title>清单路径</v-card-title>
      <v-card-text>
        <div data-testid="manifest-path">生效路径：{{ manifestPath }}</div>
        <div v-if="defaultManifestPath !== ''" data-testid="manifest-default-path">默认路径：{{ defaultManifestPath }}</div>
        <div v-if="outputDir !== ''" data-testid="manifest-output-dir">输出目录：{{ outputDir }}</div>
        <div v-if="!exists" data-testid="manifest-hint" class="text-warning mt-2">{{ hint }}</div>
        <span v-if="exists" data-testid="manifest-exists" class="sf-status mt-2">
          <span class="sf-status__dot sf-status__dot--ok"></span>
          <span>已就绪</span>
        </span>
      </v-card-text>
    </v-card>

<AsyncSection :loading="loading" :error="error" :empty="sources.length === 0 ? (emptyState ?? false) : false" empty-text="暂无规则源" @retry="load">
    <!-- Source table -->
    <v-table data-testid="source-table">
      <thead>
        <tr>
          <th>名称</th>
          <th>来源</th>
          <th>规则数</th>
          <th>更新时间</th>
          <th>操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="s in sources" :key="s.name" data-testid="source-row">
          <td>
            <v-btn variant="text" size="small" data-testid="source-rules" @click="openRules(s)">{{ s.label || s.name }}</v-btn>
          </td>
          <td>{{ originText(s) }}</td>
          <td>{{ s.rule_count ?? 0 }}</td>
          <td>{{ updatedText(s.updated_at) }}</td>
          <td>
            <v-btn size="small" variant="text" data-testid="source-update" :loading="updatingOne === s.name" :disabled="updating" @click="updateOne(s.name)">更新</v-btn>
            <v-btn size="small" variant="text" data-testid="source-rollback" :disabled="updating || s.versions.length === 0" @click="openRollback(s.name)">回滚</v-btn>
            <v-btn size="small" variant="text" color="primary" data-testid="source-mrs" :disabled="updating || s.versions.length === 0" @click="openMrs(s.name)">生成 MRS</v-btn>
            <v-btn size="small" variant="text" data-testid="source-remove" :disabled="sources.length <= 1 || s.origin === 'airport'" @click="openRemove(s.name)">删除</v-btn>
          </td>
        </tr>
      </tbody>
    </v-table>

    <!-- MRS format + retention explanation -->
    <v-card class="mt-4">
      <v-card-title>MRS 格式说明</v-card-title>
      <v-card-text>{{ glossaryFor('MRS') }}</v-card-text>
      <v-card-text class="text-caption text-medium-emphasis">版本保留策略：每个源保留最近 5 个版本，可回滚；更新失败不会删掉旧版本。</v-card-text>
    </v-card>

    <!-- Update results -->
    <div v-if="showUpdateResults" data-testid="update-results" class="mt-4">
      <v-card>
        <v-card-title>更新结果</v-card-title>
        <v-card-text>
          <div v-for="(r, i) in updateResults" :key="i">
            <div>来源：{{ r.source }}</div>
            <div>版本：{{ r.version }}</div>
            <div>路径：{{ r.path }}</div>
            <span v-if="r.from_cache" data-testid="from-cache">命中缓存</span>
            <div v-if="r.err !== ''" data-testid="source-update-error" class="text-error">{{ r.err }}</div>
            <div v-if="r.err !== ''" class="text-caption text-medium-emphasis">该源保留上一版本，产物不受影响</div>
          </div>
        </v-card-text>
      </v-card>
    </div>
  </AsyncSection>

  <!-- Rollback confirm dialog -->
  <v-dialog v-model="rollbackDialog" max-width="480">
    <v-card>
      <v-card-title>确认回滚 {{ rollbackName }}</v-card-title>
      <v-card-text>
        回滚会把该规则源的当前版本换成上一个版本，订阅产物在下一次生成时才会变化。回滚只切换当前版本，不删除任何版本文件，也不改配置。
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" :disabled="updating" @click="cancelRollback">取消</v-btn>
        <v-btn color="primary" :loading="updating" :disabled="updating" @click="doRollback">确认回滚</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <!-- MRS dialog -->
  <v-dialog v-model="mrsDialog" max-width="480">
    <v-card>
      <v-card-title>生成 MRS - {{ mrsName }}</v-card-title>
      <v-card-text>
        <v-select v-model="mrsBehavior" :items="['domain', 'ipcidr']" label="行为" :disabled="updating" />
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" :disabled="updating" @click="cancelMrs">取消</v-btn>
        <v-btn color="primary" :loading="updating" :disabled="updating" @click="doMrs">生成</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <!-- Add rule source dialog -->
  <v-dialog v-model="addDialog" max-width="520">
    <v-card data-testid="add-dialog">
      <v-card-title>新增规则源</v-card-title>
      <v-card-text>
        <v-text-field data-testid="add-name" v-model="addName" label="名称（留空自动取 URL 末段）" />
        <v-text-field data-testid="add-url" v-model="addUrl" label="订阅地址" placeholder="https://… 支持 http/https" />
        <v-text-field data-testid="add-format" v-model="addFormat" label="输入格式（可选：ini / clash / auto）" />
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn data-testid="add-cancel" variant="text" @click="addDialog = false">取消</v-btn>
        <v-btn data-testid="add-confirm" :loading="addSaving" @click="confirmAdd">添加</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <!-- Remove rule source dialog -->
  <v-dialog v-model="removeDialog" max-width="420">
    <v-card data-testid="remove-dialog">
      <v-card-title>删除规则源</v-card-title>
      <v-card-text>确定删除「{{ removeName }}」？该源的本地版本文件不会被删除。</v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn data-testid="remove-cancel" variant="text" @click="removeDialog = false">取消</v-btn>
        <v-btn data-testid="remove-confirm" :loading="removeSaving" @click="confirmRemove">删除</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<style scoped>
.rule-picker-list {
  max-height: 50vh;
  overflow-y: auto;
  border: 1px solid var(--sf-border);
  border-radius: 8px;
}
.rule-source-rule {
  white-space: pre-wrap;
  word-break: break-all;
  font-size: 12px;
}
</style>
