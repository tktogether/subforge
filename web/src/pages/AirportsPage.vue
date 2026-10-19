<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  endpoints,
  type Airport,
  type AirportInput,
  type ImportRulesOutcome,
  type SubscheckResult,
} from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import { notify } from '../composables/useNotify'
import { pageHelp, fieldHelp } from '../content/help'
import PageHeader from '../components/PageHeader.vue'
import AsyncSection from '../components/AsyncSection.vue'
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
  { key: 'name', title: '名称', minWidth: 88 },
  { key: 'subscription_url', title: '订阅地址' },
  { key: 'enabled', title: '启用', minWidth: 64 },
  { key: 'node_count', title: '节点数', align: 'end' as const, minWidth: 72 },
  { key: 'traffic', title: '流量(已用/总量)', align: 'end' as const, minWidth: 128 },
  { key: 'expire', title: '到期时间', align: 'end' as const, minWidth: 96 },
  { key: 'updated', title: '更新时间', minWidth: 152 },
  { key: 'status', title: '状态', minWidth: 72 },
  {
    key: 'actions',
    title: '操作',
    align: 'end' as const,
    minWidth: 288,
    cellProps: { class: 'sf-table__actions' },
  },
]

const dialog = ref(false)
const editing = ref<AirportRow | null>(null)
const draft = ref<AirportInput>({ name: '', subscription_url: '', remarks: '', enabled: true })

const confirmOpen = ref(false)
const pendingDelete = ref<AirportRow | null>(null)

const withRules = ref(false)
const refreshing = ref(false)
const refreshSummary = ref<{ ok: number; fail: number } | null>(null)
const imports = ref<ImportRulesOutcome[]>([])
const refreshError = ref('')
const refreshResults = ref<import('../api/endpoints').TrafficRefreshResult[]>([])

const checking = ref(false)
const checkResults = ref<SubscheckResult[]>([])
const checkError = ref('')

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
  return { name: '', subscription_url: '', remarks: '', enabled: true }
}

function openCreate(): void {
  draft.value = blank()
  editing.value = null
  dialog.value = true
}

function openEdit(row: AirportRow): void {
  draft.value = { name: row.name, subscription_url: row.subscription_url, remarks: row.remarks, enabled: row.enabled }
  editing.value = row
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
    const r = await endpoints.refreshAirport(row.id)
    refreshResults.value = [r.result]
    notify(r.result.ok ? '刷新成功' : r.result.err, r.result.ok ? 'ok' : 'error')
    await load()
  } catch (err) {
    notify(errorMessage(err), 'error')
  }
}

async function importRules(row: AirportRow): Promise<void> {
  try {
    const r = await endpoints.importAirportRules(row.id)
    notify('已导入规则：新增 ' + r.rules_added + '，跳过 ' + r.rules_skipped, 'ok')
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
    const r = await endpoints.refreshAllAirports(withRules.value)
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
  void restoreLastCheck()
})

async function restoreLastCheck(): Promise<void> {
  try {
    const r = await endpoints.lastAirportCheck()
    checkResults.value = r.results
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
        <v-btn data-testid="refresh-traffic-all" variant="tonal" prepend-icon="mdi-traffic-light" :loading="refreshing" @click="refreshAll">刷新全部流量</v-btn>
        <v-btn data-testid="check-all" variant="tonal" prepend-icon="mdi-pulse" :loading="checking" @click="checkAll">测活全部</v-btn>
      </template>
    </PageHeader>

    <AsyncSection :loading="loading" :error="error" :empty="rows.length === 0 ? (emptyState ?? false) : false" empty-text="还没有机场" @retry="load">
      <template v-slot:default>
        <v-data-table :headers="headers" :items="rows" data-testid="table" class="airports-table">
          <template #item.subscription_url="{ value }">
            <span class="airport-mono airport-truncate" :title="value">{{ value }}</span>
          </template>
          <template #item.node_count="{ value }">
            <span class="airport-num">{{ value }}</span>
          </template>
          <template #item.enabled="{ item }">
            <v-chip size="small" :color="item.enabled ? 'success' : 'default'">{{ item.enabled ? '启用' : '停用' }}</v-chip>
          </template>
          <template #item.traffic="{ item }">
            <div>
              <span :data-testid="item.total <= 0 ? 'traffic-unknown' : 'traffic-value'">{{ trafficText(item) }}</span>
              <div v-if="item.traffic_err !== ''" data-testid="traffic-error" class="text-error">
                <div>{{ item.traffic_err }}</div>
                <div>余额仍是上次成功值，没有被清零</div>
              </div>
            </div>
          </template>
          <template #item.expire="{ item }">
            <span :data-testid="item.expire <= 0 ? 'expire-unknown' : 'expire-value'">{{ expireText(item) }}</span>
          </template>
          <template #item.updated="{ item }">
            <span v-if="item.traffic_at <= 0" data-testid="traffic-at">从未采集</span>
            <span v-else data-testid="traffic-stale">上次成功：{{ formatExpire(item.traffic_at) }}（{{ relativeTime(item.traffic_at) }}前）</span>
          </template>
          <template #item.status="{ item }">
            <span v-if="item.traffic_err !== ''" data-testid="traffic-err" class="text-error">{{ item.traffic_err }}</span>
            <span v-else data-testid="traffic-status-ok">正常</span>
          </template>
          <template #item.actions="{ item }">
            <div class="airport-actions">
              <v-btn size="small" variant="text" data-testid="edit" @click="openEdit(item)">编辑</v-btn>
              <v-btn size="small" variant="text" color="error" data-testid="delete" @click="askDelete(item)">删除</v-btn>
              <v-btn size="small" variant="text" data-testid="refresh-traffic" @click="refreshOne(item)">刷新流量</v-btn>
              <v-btn size="small" variant="text" data-testid="import-rules" @click="importRules(item)">导入规则</v-btn>
              <v-btn size="small" variant="text" data-testid="check-one" @click="checkOne(item)">测活</v-btn>
            </div>
          </template>
        </v-data-table>
      </template>
    </AsyncSection>

    <div class="airports-page__extras">
      <div class="airports-page__section">
        <v-switch v-model="withRules" data-testid="with-rules" label="刷新时同时导入规则"></v-switch>
      </div>

      <div v-if="refreshSummary !== null" data-testid="refresh-summary" class="airports-page__summary">
        刷新完成：成功 {{ refreshSummary.ok }}，失败 {{ refreshSummary.fail }}
      </div>
      <div v-if="refreshError !== ''" data-testid="refresh-error" class="text-error">{{ refreshError }}</div>

      <div v-if="imports.length > 0" data-testid="imports" class="airports-page__imports">
        <div v-for="imp in imports" :key="imp.airport_id + '-' + imp.name" class="airports-page__import">
          <span>{{ imp.name }}</span>
          <span>新增 {{ imp.rules_added }}</span>
          <span>跳过 {{ imp.rules_skipped }}</span>
          <span>提供者 {{ imp.providers_added }}</span>
          <span v-if="imp.err !== ''" class="text-error">{{ imp.err }}</span>
        </div>
      </div>

      <div v-if="refreshResults.length > 0" data-testid="refresh-results" class="airports-page__refresh-results">
        <div v-for="res in refreshResults" :key="res.airport_id" data-testid="refresh-result-row" class="airports-page__refresh-result">
          <span>{{ res.name }}</span>
          <span :class="res.ok ? 'text-success' : 'text-error'">{{ res.ok ? '成功' : '失败' }}</span>
          <span v-if="res.err !== ''" class="text-error">{{ res.err }}</span>
        </div>
      </div>

      <div v-if="checkError !== ''" data-testid="check-error" class="text-error">{{ checkError }}</div>
      <div v-if="checkResults.length > 0" data-testid="check-results" class="airports-page__checks">
        <div v-for="res in checkResults" :key="res.url" data-testid="check-result-row" class="airports-page__check">
          <span>{{ res.url }}</span>
          <span v-if="res.ok" class="text-success">可用</span>
          <span v-else class="text-error">不可用</span>
          <span v-if="res.ok">{{ formatDuration(res.latency) }}</span>
          <span v-if="!res.ok && res.err !== ''" class="text-error">{{ res.err }}</span>
        </div>
      </div>
    </div>

    <v-dialog v-model="dialog" max-width="520">
      <v-card>
        <v-card-title>{{ editing === null ? '新增机场' : '编辑机场' }}</v-card-title>
        <v-card-text>
          <v-text-field v-model="draft.name" label="名称" :hint="nameHelp?.hint" :placeholder="nameHelp?.example" persistent-hint></v-text-field>
          <v-text-field v-model="draft.subscription_url" label="订阅地址" :hint="urlHelp?.hint" :placeholder="urlHelp?.example" persistent-hint></v-text-field>
          <v-text-field v-model="draft.remarks" label="备注" :hint="remarksHelp?.hint" :placeholder="remarksHelp?.example" persistent-hint></v-text-field>
          <v-switch v-model="draft.enabled" label="启用"></v-switch>
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn variant="text" @click="dialog = false">取消</v-btn>
          <v-btn color="primary" @click="submit">保存</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog v-model="confirmOpen" max-width="420">
      <v-card>
        <v-card-title>确认删除</v-card-title>
        <v-card-text>
          删除机场会同时删除其导入的规则关联，确定删除 {{ pendingDelete === null ? '' : pendingDelete.name }} 吗？
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn variant="text" @click="confirmOpen = false">取消</v-btn>
          <v-btn color="error" @click="doDelete">删除</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>


<style scoped>
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
  gap: 8px;
}
.airports-table :deep(.text-end),
.airports-table :deep(.text-right) {
  text-align: right;
  font-variant-numeric: tabular-nums;
}
/* 流量列右对齐后异常文案会跟着右对齐、很难读，单独拉回左对齐 */
.airports-table :deep(td.text-end) .text-error {
  text-align: left;
}
</style>
