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
  type TrafficRefreshResult,
} from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import { notify } from '../composables/useNotify'
import { confirmAction } from '../composables/useConfirm'
import { fieldHelp } from '../content/help'
import SfPageHead from '../components/ui/SfPageHead.vue'
import SfField from '../components/ui/SfField.vue'
import SfSwitch from '../components/ui/SfSwitch.vue'
import SfDialog from '../components/ui/SfDialog.vue'
import SfNote from '../components/ui/SfNote.vue'
import SfEmpty from '../components/ui/SfEmpty.vue'
import SfNodePreview from '../components/ui/SfNodePreview.vue'
import { formatBytes, formatDuration, formatExpire } from '../format'
import {
  IconPlus,
  IconRefresh,
  IconPencil,
  IconTrash,
  IconActivity,
  IconFileImport,
  IconRadar,
  IconInbox,
  IconAlertTriangle,
} from '@tabler/icons-vue'

type AirportRow = Airport & {
  upload: number
  download: number
  total: number
  expire: number
  node_count: number
  traffic_at: number
  traffic_err: string
}

const nameHelp = fieldHelp('airport.name')
const urlHelp = fieldHelp('airport.subscription_url')
const remarksHelp = fieldHelp('airport.remarks')
const failurePolicyHelp = fieldHelp('airport.failure_policy')
const outboundHelp = fieldHelp('airport.outbound_node')

const failurePolicyItems = [
  { title: '忽略该来源，继续生成订阅', value: 'ignore' },
  { title: '整份订阅报错，客户端保留旧配置', value: 'error' },
]

const selfNodeOptions = ref<{ title: string; value: string }[]>([])
const outboundItems = computed(() => {
  const items = [
    { title: '跟随默认出网节点', value: '' },
    { title: '强制直连', value: 'direct' },
    ...selfNodeOptions.value,
  ]
  const current = draft.value.outbound_node ?? ''
  if (current !== '' && current !== 'direct' && !items.some((i) => i.value === current)) {
    items.push({ title: `节点 ${current}（不存在）`, value: current })
  }
  return items
})
const selfNodeNames = computed(() => new Map(selfNodeOptions.value.map((o) => [o.value, o.title])))
function outboundLabel(value: string): string {
  if (value === 'direct') return '直连'
  return selfNodeNames.value.get(value) ?? `节点 ${value}`
}

async function loadSelfNodeOptions(): Promise<void> {
  try {
    const list = await endpoints.listSelfNodes()
    selfNodeOptions.value = list.map((n) => ({
      title: (n.name || `节点 ${n.id}`) + (n.enabled ? '' : '（已停用）'),
      value: String(n.id),
    }))
  } catch {
    selfNodeOptions.value = []
  }
}

const loading = ref(false)
const error = ref('')
const rows = ref<AirportRow[]>([])

const dialog = ref(false)
const editing = ref<AirportRow | null>(null)
const draft = ref<AirportInput>({ name: '', subscription_url: '', remarks: '', enabled: true, pipeline: '', failure_policy: 'ignore', outbound_node: '' })

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

const page = ref(1)
const itemsPerPage = ref(10)
const pageCount = computed(() => Math.max(1, Math.ceil(rows.value.length / itemsPerPage.value)))
watch([rows, itemsPerPage], () => {
  page.value = Math.min(page.value, pageCount.value)
})
const pagedRows = computed(() => {
  const start = (page.value - 1) * itemsPerPage.value
  return rows.value.slice(start, start + itemsPerPage.value)
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
const refreshResults = ref<TrafficRefreshResult[]>([])

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
  return { name: '', subscription_url: '', remarks: '', enabled: true, pipeline: '', failure_policy: 'ignore', outbound_node: '' }
}

function openCreate(): void {
  draft.value = blank()
  editing.value = null
  void loadPipelineOptions()
  void loadSelfNodeOptions()
  dialog.value = true
}

function openEdit(row: AirportRow): void {
  draft.value = {
    name: row.name,
    subscription_url: row.subscription_url,
    remarks: row.remarks,
    enabled: row.enabled,
    pipeline: row.pipeline ?? '',
    failure_policy: row.failure_policy ?? 'ignore',
    outbound_node: row.outbound_node ?? '',
  }
  editing.value = row
  void loadPipelineOptions()
  void loadSelfNodeOptions()
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

async function askDelete(row: AirportRow): Promise<void> {
  const ok = await confirmAction({
    title: '确认删除机场「' + row.name + '」？',
    body: '删除机场会把它从所有方案的规则源绑定里解绑；规则源作为历史快照保留在「规则源」页，可稍后删除。已创建的规则/分组不会自动删除。',
    confirmText: '删除',
    danger: true,
  })
  if (!ok) {
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
    if (r.groups_created !== undefined && r.groups_created > 0) {
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

function trafficPercent(row: AirportRow): number {
  if (row.total <= 0) {
    return 0
  }
  return Math.min(100, ((row.upload + row.download) / row.total) * 100)
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
  void loadSelfNodeOptions()
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
  <div>
    <SfPageHead>
      <template #actions>
        <button class="sf-btn sf-btn--primary" type="button" data-testid="add" @click="openCreate">
          <IconPlus :size="15" :stroke-width="2" />
          新增
        </button>
        <button class="sf-btn sf-btn--ghost" type="button" data-testid="refresh" :disabled="loading" @click="load">
          <IconRefresh :size="15" :stroke-width="1.8" :class="{ 'ap-spin': loading }" />
          刷新
        </button>
        <button class="sf-btn sf-btn--ghost" type="button" data-testid="refresh-traffic-all" :disabled="refreshing" @click="refreshAll">
          <IconActivity :size="15" :stroke-width="1.8" />
          {{ refreshing ? '刷新中…' : '流量刷新' }}
        </button>
        <button class="sf-btn sf-btn--ghost" type="button" data-testid="check-all" :disabled="checking" @click="checkAll">
          <IconRadar :size="15" :stroke-width="1.8" />
          {{ checking ? '检测中…' : '检测全部' }}
        </button>
      </template>
    </SfPageHead>

    <SfNote v-if="manualImport !== null" tone="ok" data-testid="manual-import-result" style="margin-bottom: 16px">
      <b>{{ manualImport.name }}</b>——
      <template v-if="(manualImport.result.rule_source_rules ?? 0) > 0">规则源已更新 {{ manualImport.result.rule_source_rules }} 条</template>
      <template v-else>未发现可保存的规则</template>
      <template v-if="(manualImport.result.groups_created ?? 0) > 0">，新建分组 {{ manualImport.result.groups_created }}</template>
    </SfNote>

    <div v-if="error !== ''" class="sf-note sf-note--danger sf-rise" style="margin-bottom: 16px">
      <IconAlertTriangle :size="16" :stroke-width="1.8" class="sf-note__icon" />
      <span class="sf-note__body">{{ error }}</span>
      <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" @click="load">重试</button>
    </div>

    <div v-if="rows.length === 0 && !loading && error === ''" class="sf-panel">
      <SfEmpty title="还没有机场" desc="添加上游订阅地址后，SubForge 会定时采集节点、流量与规则。">
        <template #icon><IconInbox :size="30" :stroke-width="1.4" /></template>
        <template #action>
          <button class="sf-btn sf-btn--primary sf-btn--sm" type="button" @click="openCreate">
            <IconPlus :size="14" :stroke-width="2" />
            新增机场
          </button>
        </template>
      </SfEmpty>
    </div>

    <template v-else>
      <div class="ap-bar sf-rise sf-d1">
        <span class="sf-pagination__info">共 {{ rows.length }} 条</span>
        <span class="sf-spacer" />
        <label class="ap-size">
          <span class="sf-faint" style="font-size: 12px">每页</span>
          <span class="sf-select-wrap" style="width: 76px">
            <select class="sf-select ap-size__select" data-testid="airport-page-size" :value="String(itemsPerPage)" @change="setPageSize(Number(($event.target as HTMLSelectElement).value))">
              <option v-for="n in [10, 25, 50, 100]" :key="n" :value="String(n)">{{ n }}</option>
            </select>
          </span>
        </label>
      </div>

      <div class="sf-table-wrap sf-rise sf-d1" data-testid="table">
        <table class="sf-table sf-table--scroll">
          <thead>
            <tr>
              <th style="min-width: 150px">名称</th>
              <th class="ap-col-url">订阅地址</th>
              <th style="width: 70px">启用</th>
              <th style="text-align: right; width: 70px">节点数</th>
              <th style="min-width: 150px; text-align: right">流量 / 到期</th>
              <th style="min-width: 140px">采集状态</th>
              <th style="width: 186px; text-align: right">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="loading && rows.length === 0">
              <td v-for="n in 7" :key="n"><span class="sf-skeleton" style="width: 60%" /></td>
            </tr>
            <tr v-for="row in pagedRows" v-else :key="row.id">
              <td>
                <div class="ap-name" :title="row.subscription_url">{{ row.name }}</div>
                <div class="ap-pipeline" data-testid="airport-pipeline-text">
                  处理：{{ row.pipeline || '不处理' }}<template v-if="row.failure_policy === 'error'"> · 失败时报错</template><template v-if="row.outbound_node"> · 出网：{{ outboundLabel(row.outbound_node) }}</template>
                </div>
              </td>
              <td class="ap-col-url"><span class="sf-mono ap-url" :title="row.subscription_url">{{ row.subscription_url }}</span></td>
              <td>
                <SfSwitch
                  :model-value="row.enabled"
                  :disabled="toggling === row.id"
                  data-testid="airport-enabled"
                  @update:model-value="toggleEnabled(row, $event)"
                />
              </td>
              <td class="sf-table__num" style="text-align: right">{{ row.node_count }}</td>
              <td style="text-align: right">
                <div class="ap-traffic">
                  <span><span class="sf-table__num" :data-testid="row.total <= 0 ? 'traffic-unknown' : 'traffic-value'">{{ trafficText(row) }}</span><span class="ap-expire"> · 到期 <span :data-testid="row.expire <= 0 ? 'expire-unknown' : 'expire-value'">{{ expireText(row) }}</span></span></span>
                  <div v-if="row.total > 0" class="sf-meter ap-meter" aria-hidden="true">
                    <span
                      class="sf-meter__fill"
                      :class="trafficPercent(row) >= 90 ? 'sf-meter__fill--danger' : trafficPercent(row) >= 70 ? 'sf-meter__fill--warn' : ''"
                      :style="{ width: trafficPercent(row) + '%' }"
                    />
                  </div>
                  <span v-if="row.traffic_err !== ''" data-testid="traffic-error" class="ap-traffic-err" :title="row.traffic_err + ' · 余额仍是上次成功值，没有被清零'">余额未更新</span>
                </div>
              </td>
              <td>
                <div class="ap-status">
                  <span v-if="row.traffic_err !== ''" data-testid="traffic-err" class="sf-status" :title="row.traffic_err">
                    <span class="sf-led sf-led--danger" />
                    <span style="color: var(--sf-danger)">采集失败</span>
                  </span>
                  <span v-else data-testid="traffic-status-ok" class="sf-status">
                    <span class="sf-led sf-led--ok" />
                    <span>正常</span>
                  </span>
                  <span v-if="row.traffic_at <= 0" data-testid="traffic-at" class="ap-status-time">尚未采集</span>
                  <span v-else data-testid="traffic-stale" class="ap-status-time" :title="'上次成功：' + formatExpire(row.traffic_at)">
                    上次成功 {{ relativeTime(row.traffic_at) }}前
                  </span>
                </div>
              </td>
              <td>
                <span class="sf-table__actions">
                  <button class="sf-btn sf-btn--subtle sf-btn--icon sf-btn--sm" type="button" aria-label="编辑机场" data-testid="edit" @click="openEdit(row)">
                    <span class="sf-tip" data-tip="编辑机场" style="display: inline-flex"><IconPencil :size="15" :stroke-width="1.7" /></span>
                  </button>
                  <button class="sf-btn sf-btn--subtle sf-btn--icon sf-btn--sm" type="button" aria-label="刷新流量" data-testid="refresh-traffic" @click="refreshOne(row)">
                    <span class="sf-tip" data-tip="刷新流量" style="display: inline-flex"><IconRefresh :size="15" :stroke-width="1.7" /></span>
                  </button>
                  <button class="sf-btn sf-btn--subtle sf-btn--icon sf-btn--sm" type="button" aria-label="更新规则源" data-testid="import-rules" @click="openImportDialog(row)">
                    <span class="sf-tip" data-tip="更新规则源" style="display: inline-flex"><IconFileImport :size="15" :stroke-width="1.7" /></span>
                  </button>
                  <button class="sf-btn sf-btn--subtle sf-btn--icon sf-btn--sm" type="button" aria-label="检测订阅可用性" data-testid="check-one" @click="checkOne(row)">
                    <span class="sf-tip" data-tip="检测订阅可用性" style="display: inline-flex"><IconRadar :size="15" :stroke-width="1.7" /></span>
                  </button>
                  <button class="sf-btn sf-btn--subtle sf-btn--icon sf-btn--sm sf-btn--del" type="button" aria-label="删除机场" data-testid="delete" @click="askDelete(row)">
                    <span class="sf-tip" data-tip="删除机场" style="display: inline-flex"><IconTrash :size="15" :stroke-width="1.7" /></span>
                  </button>
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-if="rows.length > itemsPerPage" class="sf-pagination" style="border: none; padding: 12px 2px">
        <span class="sf-pagination__info">PAGE {{ page }} / {{ pageCount }}</span>
        <span class="sf-spacer" />
        <button class="sf-btn sf-btn--ghost sf-btn--icon sf-btn--sm" type="button" aria-label="上一页" :disabled="page <= 1" @click="page = page - 1">‹</button>
        <button class="sf-btn sf-btn--ghost sf-btn--icon sf-btn--sm" type="button" aria-label="下一页" :disabled="page >= pageCount" @click="page = page + 1">›</button>
      </div>
    </template>

    <!-- 刷新 / 导入 / 检测结果 -->
    <div class="ap-extras">
      <SfNote v-if="refreshSummary !== null" tone="info" data-testid="refresh-summary">
        刷新完成：成功 {{ refreshSummary.ok }}，失败 {{ refreshSummary.fail }}
      </SfNote>
      <div v-if="refreshError !== ''" data-testid="refresh-error" class="sf-field__error">{{ refreshError }}</div>

      <div v-if="imports.length > 0" data-testid="imports" class="sf-note sf-note--ok">
        <span class="sf-note__body">
          <div v-for="imp in imports" :key="imp.airport_id + '-' + imp.name" class="ap-import-line">
            <b>{{ imp.name }}</b>——规则源 {{ imp.rule_source_rules ?? 0 }} 条 · 提供者 {{ imp.providers_added }}<template v-if="(imp.groups_created ?? 0) > 0"> · 新建分组 {{ imp.groups_created }}</template><span v-if="imp.err !== ''" style="color: var(--sf-danger)"> · {{ imp.err }}</span>
          </div>
        </span>
      </div>

      <div v-if="refreshResults.length > 0" data-testid="refresh-results" class="sf-note">
        <span class="sf-note__body">
          <div v-for="res in refreshResults" :key="res.airport_id" data-testid="refresh-result-row" class="ap-import-line">
            <b>{{ res.name }}</b>——<span :style="{ color: res.ok ? 'var(--sf-ok)' : 'var(--sf-danger)' }">{{ res.ok ? '成功' : '失败' }}</span><template v-if="res.err !== ''"><span style="color: var(--sf-danger)"> · {{ res.err }}</span></template><template v-else-if="res.note !== undefined"> · {{ res.note }}</template>
          </div>
        </span>
      </div>

      <div v-if="checkError !== ''" data-testid="check-error" class="sf-field__error">{{ checkError }}</div>
      <section v-if="checkResults.length > 0" data-testid="check-results" class="sf-panel" aria-live="polite">
        <div class="sf-panel__head">
          <div>
            <div class="sf-panel__title">可用性检测结果 · {{ checkSummary.total }} 项</div>
            <div v-if="checkAt !== ''" class="sf-field__hint" style="margin-top: 2px">检测时间：{{ formatCheckAt(checkAt) }}</div>
          </div>
          <span class="sf-spacer" />
        </div>
        <div class="ap-check-list">
          <div
            v-for="(res, index) in checkResults"
            :key="res.url + '-' + index"
            data-testid="check-result-row"
            class="ap-check-item"
          >
            <span class="sf-led" :class="res.ok ? 'sf-led--ok' : 'sf-led--danger'" />
            <span class="ap-check-item__status" :style="{ color: res.ok ? 'var(--sf-ok)' : 'var(--sf-danger)' }">{{ res.ok ? '可用' : '不可用' }}</span>
            <span class="ap-check-item__name" :title="res.url">{{ res.name || res.url }}</span>
            <span class="ap-check-item__meta sf-table__num">
              <template v-if="res.ok">HTTP {{ res.status_code }} · {{ formatDuration(res.latency_ms ?? 0) }}</template>
              <template v-else-if="res.status_code > 0">HTTP {{ res.status_code }}</template>
              <template v-else>连接失败</template>
            </span>
            <div v-if="!res.ok && res.err !== ''" class="ap-check-item__error" :title="res.err">{{ res.err }}</div>
          </div>
        </div>
      </section>
    </div>

    <!-- 编辑机场 -->
    <SfDialog :open="dialog" :label="editing === null ? 'CREATE AIRPORT' : 'EDIT AIRPORT'" :title="editing === null ? '新增机场' : '编辑机场'" @close="dialog = false">
      <SfField :hint="nameHelp?.hint ?? ''">
        <template #label><span class="sf-field__label">名称</span></template>
        <input v-model="draft.name" class="sf-input" data-testid="airport-name" :placeholder="nameHelp?.example ?? ''" spellcheck="false" />
      </SfField>
      <SfField :hint="urlHelp?.hint ?? ''">
        <template #label><span class="sf-field__label">订阅地址</span></template>
        <input v-model="draft.subscription_url" class="sf-input sf-input--mono" data-testid="airport-url" :placeholder="urlHelp?.example ?? ''" spellcheck="false" />
      </SfField>
      <SfField :hint="remarksHelp?.hint ?? ''">
        <template #label><span class="sf-field__label">备注</span></template>
        <input v-model="draft.remarks" class="sf-input" data-testid="airport-remarks" :placeholder="remarksHelp?.example ?? ''" spellcheck="false" />
      </SfField>
      <SfField :hint="failurePolicyHelp?.hint ?? ''">
        <template #label><span class="sf-field__label">拉取失败时</span></template>
        <span class="sf-select-wrap">
          <select v-model="draft.failure_policy" class="sf-select" data-testid="airport-failure-policy">
            <option v-for="opt in failurePolicyItems" :key="opt.value" :value="opt.value">{{ opt.title }}</option>
          </select>
        </span>
      </SfField>
      <SfField :hint="outboundHelp?.hint ?? ''">
        <template #label><span class="sf-field__label">出网节点</span></template>
        <span class="sf-select-wrap">
          <select v-model="draft.outbound_node" class="sf-select" data-testid="airport-outbound">
            <option v-for="opt in outboundItems" :key="opt.value" :value="opt.value">{{ opt.title }}</option>
          </select>
        </span>
      </SfField>
      <SfField hint="机场节点先经过这条管线，再与其他来源合并；留空表示不处理。">
        <template #label><span class="sf-field__label">处理管线</span></template>
        <span class="sf-select-wrap">
          <select v-model="draft.pipeline" class="sf-select" data-testid="airport-pipeline">
            <option value="">不处理</option>
            <option v-for="item in pipelineItems" :key="item.value" :value="item.value">{{ item.title }}</option>
          </select>
        </span>
      </SfField>
      <div class="ap-dialog-row">
        <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" data-testid="airport-preview-process" :disabled="editing === null" @click="previewProcess">
          预览处理
        </button>
        <span class="sf-field__hint">{{ editing === null ? '保存机场后可预览处理结果' : '按当前选择的管线预览该机场的节点' }}</span>
      </div>
      <SfSwitch v-model="draft.enabled" label="启用" data-testid="airport-enabled-new" />
      <template #foot>
        <button class="sf-btn sf-btn--subtle" type="button" @click="dialog = false">取消</button>
        <button class="sf-btn sf-btn--primary" type="button" data-testid="airport-save" @click="submit">保存</button>
      </template>
    </SfDialog>

    <!-- 更新规则源 -->
    <SfDialog :open="importDialog" label="IMPORT RULES" :title="'更新规则源' + (importTarget === null ? '' : '：' + importTarget.name)" @close="importDialog = false">
      <label class="ap-check-row" data-testid="import-rules-create-groups">
        <input v-model="importCreateGroups" type="checkbox" class="sf-checkbox" />
        <span>自动创建缺失的目标分组</span>
      </label>
      <SfNote tone="info" data-testid="import-rules-hint">
        系统会从机场订阅提取规则并保存为规则源；在方案中绑定后生效。
      </SfNote>
      <template #foot>
        <button class="sf-btn sf-btn--subtle" type="button" data-testid="import-rules-cancel" @click="importDialog = false">取消</button>
        <button class="sf-btn sf-btn--primary" type="button" data-testid="import-rules-confirm" @click="confirmImportRules">确认更新</button>
      </template>
    </SfDialog>

    <SfNodePreview
      :open="previewOpen"
      title="机场处理预览"
      :loading="previewLoading"
      :error="previewError"
      :result="previewResult"
      @close="previewOpen = false"
    />
  </div>
</template>

<style scoped>
.ap-spin {
  animation: ap-rotate 0.9s linear infinite;
}

@keyframes ap-rotate {
  to {
    transform: rotate(360deg);
  }
}

.ap-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 10px;
}

.ap-size {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.ap-size__select {
  height: 28px;
  font-size: 12.5px;
  padding: 0 26px 0 8px;
}

.ap-name {
  font-weight: 600;
  white-space: nowrap;
}

.ap-pipeline {
  margin-top: 2px;
  color: var(--sf-text-faint);
  font-size: 11.5px;
  white-space: nowrap;
}

.ap-url {
  display: inline-block;
  max-width: 210px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  vertical-align: middle;
  font-size: 11.5px;
  color: var(--sf-text-muted);
}

.ap-traffic {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 4px;
}

.ap-meter {
  width: 110px;
}

.ap-expire {
  color: var(--sf-text-faint);
  font-size: 11px;
  white-space: nowrap;
}

.ap-traffic-err {
  color: var(--sf-danger);
  max-width: 160px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: help;
  font-size: 11px;
}

.ap-status {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 3px;
}

.ap-status-time {
  color: var(--sf-text-faint);
  font-size: 11px;
  white-space: nowrap;
}

.ap-col-url {
  min-width: 160px;
}

@media (max-width: 1200px) {
  .ap-col-url {
    display: none;
  }
}

.ap-extras {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-top: 16px;
}

.ap-import-line {
  line-height: 1.7;
  overflow-wrap: anywhere;
}

.ap-check-list {
  display: flex;
  flex-direction: column;
}

.ap-check-item {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px 10px;
  padding: 10px 18px;
  border-bottom: 1px solid oklch(from var(--sf-border) l c h / 0.55);
}

.ap-check-item:last-child {
  border-bottom: 0;
}

.ap-check-item__status {
  min-width: 3.5em;
  font-size: 12.5px;
  font-weight: 600;
}

.ap-check-item__name {
  flex: 1 1 180px;
  min-width: 0;
  font-weight: 500;
  overflow-wrap: anywhere;
}

.ap-check-item__meta {
  color: var(--sf-text-faint);
  font-size: 12px;
  white-space: nowrap;
}

.ap-check-item__error {
  flex-basis: 100%;
  color: var(--sf-danger);
  font-size: 12px;
  line-height: 1.5;
  overflow-wrap: anywhere;
}

.ap-dialog-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.ap-check-row {
  display: inline-flex;
  align-items: center;
  gap: 9px;
  cursor: pointer;
  font-size: 13.5px;
}
</style>
