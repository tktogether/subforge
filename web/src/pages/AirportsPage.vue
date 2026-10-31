<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import {
  endpoints,
  type Airport,
  type AirportInput,
  type ImportRulesOutcome,
  type ImportRulesResult,
  type SubscheckResult,
  type NodePreviewResult,
} from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import { notify } from '../composables/useNotify'
import { pageHelp, fieldHelp } from '../content/help'
import PageHeader from '../components/PageHeader.vue'
import AsyncSection from '../components/AsyncSection.vue'
import NodePreviewDialog from '../components/NodePreviewDialog.vue'
import { formatBytes, formatDuration, formatExpire } from '../format'

type AirportRow = Airport & {
  upload: number
  download: number
  total: number
  expire: number
  node_count: number
  traffic_at: number
  traffic_err: string
}

const help = pageHelp('airports')
const nameHelp = fieldHelp('airport.name')
const urlHelp = fieldHelp('airport.subscription_url')
const remarksHelp = fieldHelp('airport.remarks')

const loading = ref(false)
const error = ref('')
const rows = ref<AirportRow[]>([])
const emptyState = computed(() => help.emptyState)

// W6B 表格：minWidth 给每列下界（Vuetify 会把 minWidth 写成 th 的内联 min-width），
// 表头的 white-space:nowrap 由 vuetify-overrides.css 统一提供 → 1600px 下 9 列表头全部单行。
// align:'end' 让数字列（节点数 / 流量 / 到期时间 / 操作）右对齐，并吃到 tabular-nums。
// 操作列额外挂 sf-table__actions，供覆写层做「单行不折 + 紧凑按钮」。
const headers = [
  { key: 'name', title: '名称', minWidth: 140 },
  { key: 'subscription_url', title: '订阅地址' },
  { key: 'enabled', title: '启用', minWidth: 64 },
  { key: 'node_count', title: '节点数', align: 'end' as const, minWidth: 72 },
  { key: 'traffic', title: '流量 / 到期', align: 'end' as const, minWidth: 152 },
  { key: 'status', title: '采集状态', minWidth: 150 },
  {
    key: 'actions',
    title: '操作',
    align: 'end' as const,
    minWidth: 176,
    cellProps: { class: 'sf-table__actions' },
  },
]

const dialog = ref(false)
const editing = ref<AirportRow | null>(null)
const draft = ref<AirportInput>({ name: '', subscription_url: '', remarks: '', enabled: true, pipeline: '' })

const pipelineOptions = ref<{ title: string; value: string }[]>([])
const previewOpen = ref(false)
const previewLoading = ref(false)
const previewError = ref('')
const previewResult = ref<NodePreviewResult | null>(null)

async function previewProcess(): Promise<void> {
  if (editing.value === null) return
  previewOpen.value = true
  previewLoading.value = true
  previewError.value = ''
  previewResult.value = null
  try {
    previewResult.value = await endpoints.previewAirportProcess(editing.value.id, draft.value.pipeline ?? '')
  } catch (err) {
    previewError.value = errorMessage(err)
  } finally {
    previewLoading.value = false
  }
}
const pipelineItems = computed(() => {
  const items = [...pipelineOptions.value]
  const current = draft.value.pipeline ?? ''
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

const confirmOpen = ref(false)
const pendingDelete = ref<AirportRow | null>(null)

const page = ref(1)
const itemsPerPage = ref(10)
const pageCount = computed(() => Math.max(1, Math.ceil(rows.value.length / itemsPerPage.value)))
watch([rows, itemsPerPage], () => {
  page.value = Math.min(page.value, pageCount.value)
})

function setPageSize(next: number): void {
  itemsPerPage.value = next
  page.value = 1
}

const toggling = ref<number | null>(null)

async function toggleEnabled(row: AirportRow, value: boolean): Promise<void> {
  toggling.value = row.id
  try {
    await endpoints.updateAirport(row.id, {
      name: row.name,
      subscription_url: row.subscription_url,
      remarks: row.remarks,
      enabled: value,
    })
    row.enabled = value
    notify(value ? '已启用 ' + row.name : '已停用 ' + row.name)
  } catch (err) {
    notify(errorMessage(err), 'error')
  } finally {
    toggling.value = null
  }
}

const refreshing = ref(false)
const refreshSummary = ref<{ ok: number; fail: number } | null>(null)
const imports = ref<ImportRulesOutcome[]>([])
const refreshError = ref('')
const refreshResults = ref<import('../api/endpoints').TrafficRefreshResult[]>([])

const checking = ref(false)
const checkResults = ref<SubscheckResult[]>([])
const checkError = ref('')
const checkAt = ref('')
const checkSummary = computed(() => {
  const total = checkResults.value.length
  const ok = checkResults.value.filter((r) => r.ok).length
  return { ok, fail: total - ok, total }
})

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    rows.value = (await endpoints.listAirports()) as AirportRow[]
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

function blank(): AirportInput {
  return { name: '', subscription_url: '', remarks: '', enabled: true, pipeline: '' }
}

function openCreate(): void {
  draft.value = blank()
  editing.value = null
  void loadPipelineOptions()
  dialog.value = true
}

function openEdit(row: AirportRow): void {
  draft.value = {
    name: row.name,
    subscription_url: row.subscription_url,
    remarks: row.remarks,
    enabled: row.enabled,
    pipeline: row.pipeline ?? '',
  }
  editing.value = row
  void loadPipelineOptions()
  dialog.value = true
}

async function submit(): Promise<void> {
  const isCreate = editing.value === null
  try {
    if (isCreate) {
      await endpoints.createAirport(draft.value)
    } else {
      await endpoints.updateAirport(editing.value!.id, draft.value)
    }
    dialog.value = false
    notify(isCreate ? '已新增' : '已更新')
    await load()
  } catch (err) {
    notify(errorMessage(err), 'error')
  }
}

function askDelete(row: AirportRow): void {
  pendingDelete.value = row
  confirmOpen.value = true
}

async function doDelete(): Promise<void> {
  const row = pendingDelete.value
  confirmOpen.value = false
  if (row === null) {
    return
  }
  try {
    await endpoints.deleteAirport(row.id)
    notify('已删除')
    await load()
  } catch (err) {
    notify(errorMessage(err), 'error')
  }
}

async function refreshOne(row: AirportRow): Promise<void> {
  try {
    const r = await endpoints.refreshAirport(row.id, true)
    refreshResults.value = [r.result]
    if (r.import) {
      imports.value = [r.import]
    }
    notify(r.result.ok ? '刷新成功' : r.result.err, r.result.ok ? 'ok' : 'error')
    await load()
  } catch (err) {
    notify(errorMessage(err), 'error')
  }
}

// 导入规则弹窗（契约 v2 §6.4）：规则与分组已是全局库，导入时不再选方案；
// 导入的规则来源固定为 airport:<机场名>，只保留「自动创建缺失的目标分组」。
const importDialog = ref(false)
const importTarget = ref<AirportRow | null>(null)
const importCreateGroups = ref(true)
// 手动导入的结果保留在页面上：snackbar 会被上一条“已新增”占用/抢时间，用户可能看不到。
const manualImport = ref<{ name: string; result: ImportRulesResult } | null>(null)

function openImportDialog(row: AirportRow): void {
  importTarget.value = row
  importCreateGroups.value = true
  importDialog.value = true
}

async function confirmImportRules(): Promise<void> {
  const row = importTarget.value
  importDialog.value = false
  if (row === null) {
    return
  }
  try {
    const r = await endpoints.importAirportRules(row.id, {
      create_groups: importCreateGroups.value,
    })
    let text = (r.rule_source_rules ?? 0) > 0
      ? '规则源已更新：' + r.rule_source_rules + ' 条'
      : '未发现可保存的规则'
    if (r.groups_created && r.groups_created > 0) {
      text += '，新建分组 ' + r.groups_created
    }
    manualImport.value = { name: row.name, result: r }
    notify(text, 'ok')
    if (r.warnings && r.warnings.length > 0) {
      notify(r.warnings[0], 'info')
    }
    await load()
  } catch (err) {
    notify(errorMessage(err), 'error')
  }
}

async function checkOne(row: AirportRow): Promise<void> {
  await runCheck([row.id])
}

async function checkAll(): Promise<void> {
  await runCheck(undefined)
}

async function runCheck(ids?: number[]): Promise<void> {
  checking.value = true
  checkError.value = ''
  try {
    const r = await endpoints.checkAirports(ids)
    checkResults.value = r.results
    checkAt.value = r.at ?? ''
  } catch (err) {
    checkError.value = errorMessage(err)
  } finally {
    checking.value = false
  }
}

async function refreshAll(): Promise<void> {
  refreshing.value = true
  refreshError.value = ''
  try {
    const r = await endpoints.refreshAllAirports(true)
    refreshSummary.value = { ok: r.ok, fail: r.fail }
    imports.value = r.imports ?? []
    refreshResults.value = r.results ?? []
  } catch (err) {
    refreshError.value = errorMessage(err)
  } finally {
    refreshing.value = false
  }
}

function trafficText(row: AirportRow): string {
  if (row.total <= 0) {
    return '未知'
  }
  return formatBytes(row.upload + row.download) + ' / ' + formatBytes(row.total)
}

function expireText(row: AirportRow): string {
  if (row.expire <= 0) {
    return '未知'
  }
  return formatExpire(row.expire)
}

function formatCheckAt(iso: string): string {
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString('zh-CN', { hour12: false })
}

function relativeTime(ts: number): string {
  const diffMs = Date.now() - ts * 1000
  const minutes = Math.max(0, Math.floor(diffMs / 60000))
  if (minutes < 60) {
    return minutes + ' 分钟'
  }
  const hours = Math.floor(minutes / 60)
  if (hours < 24) {
    return hours + ' 小时'
  }
  return Math.floor(hours / 24) + ' 天'
}

onMounted(() => {
  void load()
  void loadPipelineOptions()
  void restoreLastCheck()
})

async function restoreLastCheck(): Promise<void> {
  try {
    const r = await endpoints.lastAirportCheck()
    checkResults.value = r.results
    checkAt.value = r.at ?? ''
  } catch {
    // 恢复失败保持静默
  }
}
</script>

<template>
  <div class="airports-page">
    <PageHeader :title="help.title" :subtitle="help.oneLiner" help-key="airports">
      <template #actions>
        <v-btn data-testid="add" color="primary" prepend-icon="mdi-plus" @click="openCreate">新增</v-btn>
        <v-btn data-testid="refresh" variant="tonal" prepend-icon="mdi-refresh" @click="load">刷新</v-btn>
        <v-btn data-testid="refresh-traffic-all" variant="tonal" prepend-icon="mdi-traffic-light" :loading="refreshing" @click="refreshAll">刷新全部</v-btn>
        <v-btn data-testid="check-all" variant="tonal" prepend-icon="mdi-pulse" :loading="checking" @click="checkAll">检测全部</v-btn>
      </template>
    </PageHeader>

    <v-alert v-if="manualImport" data-testid="manual-import-result" type="success" variant="tonal" closable
      class="mb-4" @click:close="manualImport = null">
      {{ manualImport.name }}：
      <template v-if="(manualImport.result.rule_source_rules ?? 0) > 0">
        规则源已更新 {{ manualImport.result.rule_source_rules }} 条
      </template>
      <template v-else>未发现可保存的规则</template>
      <span v-if="(manualImport.result.groups_created ?? 0) > 0">
        ，新建分组 {{ manualImport.result.groups_created }}
      </span>
    </v-alert>

    <AsyncSection :loading="loading" :error="error" :empty="rows.length === 0 ? (emptyState ?? false) : false" empty-text="还没有机场" @retry="load">
      <template v-slot:default>
        <div class="airports-table-bar">
          <span class="text-caption text-medium-emphasis">共 {{ rows.length }} 条</span>
          <v-spacer />
          <v-select
            :model-value="itemsPerPage"
            :items="[10, 25, 50, 100]"
            label="每页条数"
            density="compact"
            variant="outlined"
            hide-details
            data-testid="airport-page-size"
            class="airports-page-size"
            @update:model-value="setPageSize"
          />
        </div>
        <v-data-table
          :page="page"
          :items-per-page="itemsPerPage"
          :items-per-page-options="[10, 25, 50, 100]"
          :headers="headers"
          :items="rows"
          data-testid="table"
          class="airports-table"
        >
          <template #item.name="{ item }">
            <div class="airport-name" :title="item.subscription_url">{{ item.name }}</div>
            <div class="airport-pipeline" data-testid="airport-pipeline-text">处理：{{ item.pipeline || '不处理' }}</div>
          </template>
          <template #item.subscription_url="{ value }">
            <span class="airport-mono airport-truncate" :title="value">{{ value }}</span>
          </template>
          <template #item.node_count="{ value }">
            <span class="airport-num">{{ value }}</span>
          </template>
          <template #item.enabled="{ item }">
            <v-switch :model-value="item.enabled" density="compact" :hide-details="true"
              data-testid="airport-enabled" :loading="toggling === item.id"
              @update:model-value="toggleEnabled(item, $event as boolean)"></v-switch>
          </template>
          <template #item.traffic="{ item }">
            <div class="airport-traffic">
              <span :data-testid="item.total <= 0 ? 'traffic-unknown' : 'traffic-value'">{{ trafficText(item) }}</span>
              <span class="airport-expire">
                到期：<span :data-testid="item.expire <= 0 ? 'expire-unknown' : 'expire-value'">{{ expireText(item) }}</span>
              </span>
              <span v-if="item.traffic_err !== ''" data-testid="traffic-error" class="airport-traffic-err" :title="item.traffic_err + ' · 余额仍是上次成功值，没有被清零'">余额未更新</span>
            </div>
          </template>
          <template #item.status="{ item }">
            <div class="airport-status-cell">
              <span v-if="item.traffic_err !== ''" data-testid="traffic-err" class="sf-status airport-status-err" :title="item.traffic_err">
                <span class="sf-status__dot sf-status__dot--danger"></span>
                <span>采集失败</span>
              </span>
              <span v-else data-testid="traffic-status-ok" class="sf-status">
                <span class="sf-status__dot sf-status__dot--ok"></span>
                <span>正常</span>
              </span>
              <span v-if="item.traffic_at <= 0" data-testid="traffic-at" class="airport-status-time">尚未采集</span>
              <span v-else data-testid="traffic-stale" class="airport-status-time" :title="'上次成功：' + formatExpire(item.traffic_at)">
                上次成功：{{ relativeTime(item.traffic_at) }}前
              </span>
            </div>
          </template>
          <template #item.actions="{ item }">
            <div class="airport-actions">
              <v-btn icon="mdi-pencil" size="small" variant="text" title="编辑机场" aria-label="编辑机场" data-testid="edit" @click="openEdit(item)" />
              <v-btn icon="mdi-delete-outline" size="small" variant="text" color="error" title="删除机场" aria-label="删除机场" data-testid="delete" @click="askDelete(item)" />
              <v-btn icon="mdi-refresh" size="small" variant="text" title="刷新机场" aria-label="刷新机场" data-testid="refresh-traffic" @click="refreshOne(item)" />
              <v-btn icon="mdi-file-sync-outline" size="small" variant="text" title="更新规则源" aria-label="更新规则源" data-testid="import-rules" @click="openImportDialog(item)" />
              <v-btn icon="mdi-pulse" size="small" variant="text" title="检测订阅可用性" aria-label="检测订阅可用性" data-testid="check-one" @click="checkOne(item)" />
            </div>
          </template>
        </v-data-table>
      </template>
    </AsyncSection>

    <div class="airports-page__extras">

      <div v-if="refreshSummary !== null" data-testid="refresh-summary" class="airports-page__summary">
        刷新完成：成功 {{ refreshSummary.ok }}，失败 {{ refreshSummary.fail }}
      </div>
      <div v-if="refreshError !== ''" data-testid="refresh-error" class="text-error">{{ refreshError }}</div>

      <div v-if="imports.length > 0" data-testid="imports" class="airports-page__imports">
        <div v-for="imp in imports" :key="imp.airport_id + '-' + imp.name" class="airports-page__import">
          <span>{{ imp.name }}</span>
          <span>规则源 {{ imp.rule_source_rules ?? 0 }} 条</span>
          <span>提供者 {{ imp.providers_added }}</span>
          <span v-if="(imp.groups_created ?? 0) > 0">新建分组 {{ imp.groups_created }}</span>
          <span v-if="imp.err !== ''" class="text-error">{{ imp.err }}</span>
        </div>
      </div>

      <div v-if="refreshResults.length > 0" data-testid="refresh-results" class="airports-page__refresh-results">
        <div v-for="res in refreshResults" :key="res.airport_id" data-testid="refresh-result-row" class="airports-page__refresh-result">
          <span>{{ res.name }}</span>
          <span :class="res.ok ? 'text-success' : 'text-error'">{{ res.ok ? '成功' : '失败' }}</span>
          <span v-if="res.err !== ''" class="text-error">{{ res.err }}</span>
          <span v-else-if="res.note" data-testid="refresh-result-note" class="text-medium-emphasis">{{ res.note }}</span>
        </div>
      </div>

      <div v-if="checkError !== ''" data-testid="check-error" class="text-error">{{ checkError }}</div>
      <section v-if="checkResults.length > 0" data-testid="check-results" class="check-panel" aria-live="polite">
        <header class="check-panel__head">
          <div>
            <div class="check-panel__title">可用性检测结果</div>
            <div v-if="checkAt !== ''" class="check-panel__time">检测时间：{{ formatCheckAt(checkAt) }}</div>
          </div>
          <div data-testid="check-summary" class="check-panel__stats">
            <span class="check-stat check-stat--ok"><span class="check-stat__dot"></span>可用 {{ checkSummary.ok }}</span>
            <span class="check-stat check-stat--fail"><span class="check-stat__dot"></span>不可用 {{ checkSummary.fail }}</span>
            <span class="check-stat">共 {{ checkSummary.total }}</span>
          </div>
        </header>
        <div class="check-list">
          <div
            v-for="(res, index) in checkResults"
            :key="res.url + '-' + index"
            data-testid="check-result-row"
            class="check-item"
            :class="res.ok ? 'check-item--ok' : 'check-item--fail'"
          >
            <div class="check-item__line">
              <span class="check-item__status"><span class="check-stat__dot"></span>{{ res.ok ? '可用' : '不可用' }}</span>
              <span class="check-item__name" :title="res.url">{{ res.name || res.url }}</span>
              <span class="check-item__meta">
                <template v-if="res.ok">HTTP {{ res.status_code }} · {{ formatDuration(res.latency_ms ?? 0) }}</template>
                <template v-else-if="res.status_code > 0">HTTP {{ res.status_code }}</template>
                <template v-else>连接失败</template>
              </span>
            </div>
            <div v-if="!res.ok && res.err !== ''" class="check-item__error" :title="res.err">{{ res.err }}</div>
          </div>
        </div>
      </section>
    </div>

    <v-dialog v-model="dialog" max-width="520">
      <v-card>
        <v-card-title>{{ editing === null ? '新增机场' : '编辑机场' }}</v-card-title>
        <v-card-text>
          <v-text-field v-model="draft.name" data-testid="airport-name" label="名称" :hint="nameHelp?.hint" :placeholder="nameHelp?.example" persistent-hint></v-text-field>
          <v-text-field v-model="draft.subscription_url" data-testid="airport-url" label="订阅地址" :hint="urlHelp?.hint" :placeholder="urlHelp?.example" persistent-hint></v-text-field>
          <v-text-field v-model="draft.remarks" data-testid="airport-remarks" label="备注" :hint="remarksHelp?.hint" :placeholder="remarksHelp?.example" persistent-hint></v-text-field>
          <v-select
            v-model="draft.pipeline"
            :items="pipelineItems"
            label="处理管线"
            hint="机场节点先经过这条管线，再与其他来源合并；留空表示不处理。"
            persistent-hint
            clearable
            data-testid="airport-pipeline"
            class="mb-3"
          />
          <div class="d-flex align-center ga-2 mb-3">
            <v-btn variant="tonal" size="small" prepend-icon="mdi-eye-outline" :disabled="editing === null" data-testid="airport-preview-process" @click="previewProcess">预览处理</v-btn>
            <span class="text-caption text-medium-emphasis">{{ editing === null ? '保存机场后可预览处理结果' : '按当前选择的管线预览该机场的节点' }}</span>
          </div>
          <v-switch v-model="draft.enabled" data-testid="airport-enabled-new" label="启用"></v-switch>
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn variant="text" @click="dialog = false">取消</v-btn>
          <v-btn color="primary" data-testid="airport-save" @click="submit">保存</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog v-model="confirmOpen" max-width="420">
      <v-card>
        <v-card-title>确认删除</v-card-title>
        <v-card-text>
          删除机场会把它从所有方案的规则源绑定里解绑；规则源作为历史快照保留在「规则源」页，可稍后删除。已创建的规则/分组不会自动删除。确定删除 {{ pendingDelete === null ? '' : pendingDelete.name }} 吗？
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn variant="text" @click="confirmOpen = false">取消</v-btn>
          <v-btn color="error" @click="doDelete">删除</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog v-model="importDialog" max-width="480">
      <v-card>
        <v-card-title>更新规则源{{ importTarget === null ? '' : '：' + importTarget.name }}</v-card-title>
        <v-card-text>
          <v-checkbox v-model="importCreateGroups" label="自动创建缺失的目标分组"
            data-testid="import-rules-create-groups" density="compact"></v-checkbox>
          <div class="text-medium-emphasis" data-testid="import-rules-hint">
            系统会从机场订阅提取规则并保存为规则源；在方案中绑定后生效。
          </div>
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn variant="text" data-testid="import-rules-cancel" @click="importDialog = false">取消</v-btn>
          <v-btn color="primary" data-testid="import-rules-confirm" @click="confirmImportRules">确认更新</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <NodePreviewDialog
      v-model="previewOpen"
      title="机场处理预览"
      :loading="previewLoading"
      :error="previewError"
      :result="previewResult"
    />
  </div>
</template>


<style scoped>
/* 可用性检测结果：摘要 + 条目列表，避免裸文本堆叠 */
.check-panel {
  margin-top: 16px;
  border: 1px solid var(--sf-border);
  border-radius: 10px;
  background: var(--sf-surface);
  box-shadow: var(--sf-shadow);
  overflow: hidden;
}
.check-panel__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  padding: 14px 16px;
  border-bottom: 1px solid var(--sf-border);
  background: var(--sf-surface-raised);
}
.check-panel__title {
  font-size: 14px;
  font-weight: 600;
  color: var(--sf-text);
}
.check-panel__time {
  margin-top: 2px;
  font-size: 12px;
  color: var(--sf-text-muted);
}
.check-panel__stats {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.check-stat {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 9px;
  border: 1px solid var(--sf-border);
  border-radius: 999px;
  background: var(--sf-surface);
  color: var(--sf-text-muted);
  font-size: 12px;
  font-weight: 600;
}
.check-stat--ok {
  color: var(--sf-ok);
}
.check-stat--fail {
  color: var(--sf-danger);
}
.check-stat__dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: currentColor;
  flex: 0 0 auto;
}
.check-list {
  display: flex;
  flex-direction: column;
}
.check-item {
  padding: 10px 16px;
  border-bottom: 1px solid var(--sf-border);
}
.check-item:last-child {
  border-bottom: 0;
}
.check-item--ok {
  border-left: 3px solid var(--sf-ok);
}
.check-item--fail {
  border-left: 3px solid var(--sf-danger);
}
.check-item__line {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px 10px;
  min-width: 0;
}
.check-item__status {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  min-width: 54px;
  font-size: 12px;
  font-weight: 600;
}
.check-item--ok .check-item__status {
  color: var(--sf-ok);
}
.check-item--fail .check-item__status {
  color: var(--sf-danger);
}
.check-item__name {
  flex: 1 1 180px;
  min-width: 0;
  color: var(--sf-text);
  font-weight: 500;
  overflow-wrap: anywhere;
}
.check-item__meta {
  color: var(--sf-text-muted);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}
.check-item__error {
  display: -webkit-box;
  margin-top: 4px;
  overflow: hidden;
  color: var(--sf-danger);
  font-size: 12px;
  line-height: 1.5;
  overflow-wrap: anywhere;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.airports-table-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
}
.airports-page-size {
  max-width: 132px;
}
.airports-table :deep(.v-data-table-footer__items-per-page) {
  display: none;
}
.airports-table {
  border: 1px solid var(--sf-border);
  border-radius: var(--sf-radius);
}
.airports-table :deep(thead th) {
  background: var(--sf-surface-raised);
  color: var(--sf-text-muted);
  font-size: 12px;
  font-weight: 600;
  position: sticky;
  top: 0;
}
.airport-name {
  font-weight: 500;
  white-space: nowrap;
}
.airport-pipeline {
  margin-top: 2px;
  color: var(--sf-text-muted);
  font-size: 12px;
  white-space: nowrap;
}
.airport-mono {
  font-family: var(--sf-font-mono);
  font-variant-numeric: tabular-nums;
}
.airport-truncate {
  display: inline-block;
  /* W6B：380px 的内联块会把整列下界顶到 404px，9 列在 1600px 下放不下；
     收到 200px 后 URL 列 ≈224px，truncate + title 行为不变。 */
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.airport-num {
  font-variant-numeric: tabular-nums;
  text-align: right;
  display: block;
}
/* W6B：操作列单行——flex 一行 + 8px 间距，靠右（表头 align='end'） */
.airport-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 2px;
}
@media (max-width: 1200px) {
  .airports-table :deep(th:nth-child(2)),
  .airports-table :deep(td:nth-child(2)) {
    display: none;
  }
}
.airports-table :deep(.text-end),
.airports-table :deep(.text-right) {
  text-align: right;
  font-variant-numeric: tabular-nums;
}
/* W7A：流量列 8px 圆点 + 文字——替代 tonal chip，用户点名「启用配色完全不可见」 */
.airport-traffic {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 1px;
}
.airport-expire {
  color: var(--sf-text-muted);
  font-size: 11px;
  white-space: nowrap;
}
.airport-status-cell {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
}
.airport-status-time {
  color: var(--sf-text-muted);
  font-size: 11px;
  white-space: nowrap;
}
.airport-traffic-err {
  color: var(--sf-danger);
  max-width: 160px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: help;
}
/* W7A：状态列异常文案用 --sf-danger，色点走全局 .sf-status__dot--danger */
.airport-status-err {
  color: var(--sf-danger);
}
</style>
