<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { endpoints, subscribeUrl, type SubToken, type TrafficStrategy, type TrafficRefreshResponse } from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import { notify } from '../composables/useNotify'
import { pageHelp } from '../content/help'
import PageHeader from '../components/PageHeader.vue'

const help = pageHelp('subscriptions')

// ---------- Block 1/2: tokens ----------
const tokens = ref<SubToken[]>([])
const selectedToken = ref('')
const origin = ref(window.location.origin)
const copied = ref(false)
const loadingTokens = ref(false)
const errorTokens = ref('')

const selected = computed(() => tokens.value.find((t) => t.token === selectedToken.value) ?? null)

const resource = {
  list: async () => {
    loadingTokens.value = true
    errorTokens.value = ''
    try {
      const r = await endpoints.listTokens()
      tokens.value = r
      return r as any[]
    } catch (err) {
      errorTokens.value = errorMessage(err)
      return []
    } finally {
      loadingTokens.value = false
    }
  },
  create: endpoints.createToken as (item: any) => Promise<any>,
  update: async (item: any) => item,
  remove: async (item: any) => endpoints.deleteToken(String(item.token)),
}

// token dialog
const tokenDialog = ref(false)
const editingToken = ref<SubToken | null>(null)
const tokenForm = ref({ token: '', name: '' })
function openCreateToken(): void {
  editingToken.value = null
  tokenForm.value = { token: '', name: '' }
  tokenDialog.value = true
}
function openEditToken(t: SubToken): void {
  editingToken.value = t
  tokenForm.value = { token: t.token, name: t.name }
  tokenDialog.value = true
}
async function saveToken(): Promise<void> {
  try {
    if (editingToken.value === null) {
      await endpoints.createToken({ token: tokenForm.value.token, name: tokenForm.value.name })
    }
    tokenDialog.value = false
    await resource.list()
    notify('已保存', 'ok')
  } catch (err) {
    notify(errorMessage(err), 'error')
  }
}
async function deleteToken(t: SubToken): Promise<void> {
  try {
    await endpoints.deleteToken(t.token)
    await resource.list()
    if (selectedToken.value === t.token) selectedToken.value = ''
    notify('已删除', 'ok')
  } catch (err) {
    notify(errorMessage(err), 'error')
  }
}

function maskToken(tok: string): string {
  if (tok.length <= 8) return tok
  return tok.slice(0, 4) + '…' + tok.slice(-4)
}

async function copyUrl(): Promise<void> {
  if (selected.value === null) return
  try {
    await navigator.clipboard.writeText(subscribeUrl(origin.value, selected.value.token))
    copied.value = true
    notify('已复制', 'ok')
  } catch {
    copied.value = false
    notify('复制失败', 'error')
  }
}

// ---------- Block 3: traffic strategy ----------
const FIELD_VALUES: string[] = ['sum', 'min', 'max', 'avg', 'first', 'last', 'none']
const PRESET_VALUES = ['sum', 'min', 'max', 'avg', 'first', 'last', 'min_quota', 'max_quota', 'weighted', 'off'] as const
const PRESET_LABELS: Record<string, string> = {
  sum: '求和：上传/下载/总量分别把各机场加起来，到期时间取最早的。适合「多个机场额度可叠加」的场景。',
  min: '取最小：四个字段都取各机场里最小的值。最保守。',
  max: '取最大：四个字段都取最大的值。最乐观。',
  avg: '取平均：四个字段都取平均值。',
  first: '只用第一个机场的值（按排序）。',
  last: '只用最后一个机场的值（按排序）。',
  min_quota: '取「总量最小」的那个机场的整份快照（上传/下载/到期也跟着它）。',
  max_quota: '取「总量最大」的那个机场的整份快照。',
  weighted: '按各机场节点数加权平均。',
  off: '不透传：完全不写 subscription-userinfo 响应头，客户端不显示余额。',
}

const preset = ref<string>('sum')
const ignoresFields = computed(() => ['min_quota', 'max_quota', 'weighted'].includes(preset.value))
const upload = ref<string>('sum')
const download = ref<string>('sum')
const total = ref<string>('sum')
const expire = ref<string>('min')

const savingStrategy = ref(false)
const previewHeader = ref('')
const previewOk = ref<boolean | null>(null)
const previewing = ref(false)

const strategyObj = computed<TrafficStrategy>(() => ({
  preset: preset.value,
  upload: upload.value,
  download: download.value,
  total: total.value,
  expire: expire.value,
}))

const PRESET_EXPAND: Record<string, Partial<TrafficStrategy>> = {
  sum: { upload: 'sum', download: 'sum', total: 'sum', expire: 'min' },
  min: { upload: 'min', download: 'min', total: 'min', expire: 'min' },
  max: { upload: 'max', download: 'max', total: 'max', expire: 'max' },
  avg: { upload: 'avg', download: 'avg', total: 'avg', expire: 'avg' },
  first: { upload: 'first', download: 'first', total: 'first', expire: 'first' },
  last: { upload: 'last', download: 'last', total: 'last', expire: 'last' },
}
function onPresetChange(v: string): void {
  preset.value = v
  const ex = PRESET_EXPAND[v]
  if (ex) {
    if (ex.upload) upload.value = ex.upload
    if (ex.download) download.value = ex.download
    if (ex.total) total.value = ex.total
    if (ex.expire) expire.value = ex.expire
  }
}

function applyQuick(combo: string): void {
  const [u, d, t, e] = combo.split('/')
  upload.value = u; download.value = d; total.value = t; expire.value = e
  preset.value = ''
}

async function loadStrategy(): Promise<void> {
  try {
    const r = await endpoints.getTrafficStrategy()
    const s = r.strategy
    preset.value = s.preset ?? 'sum'
    upload.value = s.upload ?? 'sum'
    download.value = s.download ?? 'sum'
    total.value = s.total ?? 'sum'
    expire.value = s.expire ?? 'min'
  } catch (err) {
    notify(errorMessage(err), 'error')
  }
}

async function saveStrategy(): Promise<void> {
  savingStrategy.value = true
  try {
    const r = await endpoints.setTrafficStrategy(strategyObj.value)
    const s = r.strategy
    preset.value = s.preset ?? preset.value
    upload.value = s.upload ?? upload.value
    download.value = s.download ?? download.value
    total.value = s.total ?? total.value
    expire.value = s.expire ?? expire.value
    notify('已保存', 'ok')
  } catch (err) {
    notify(errorMessage(err), 'error')
  } finally {
    savingStrategy.value = false
  }
}

async function previewStrategy(): Promise<void> {
  previewing.value = true
  previewHeader.value = ''
  previewOk.value = null
  try {
    const r = await endpoints.previewTraffic(strategyObj.value)
    previewHeader.value = r.header
    previewOk.value = r.ok
  } catch (err) {
    notify(errorMessage(err), 'error')
  } finally {
    previewing.value = false
  }
}

// ---------- Block 4: refresh ----------
const refreshLoading = ref(false)
const refreshError = ref('')
const refreshResult = ref<TrafficRefreshResponse | null>(null)
async function refreshAll(): Promise<void> {
  refreshLoading.value = true
  refreshError.value = ''
  try {
    const r = await endpoints.refreshAllAirports(false)
    refreshResult.value = r
  } catch (err) {
    refreshError.value = errorMessage(err)
  } finally {
    refreshLoading.value = false
  }
}

onMounted(() => {
  void resource.list()
  void loadStrategy()
})
</script>

<template>
  <PageHeader title="订阅" :subtitle="help.oneLiner" help-key="subscriptions" />

  <!-- Block 1: subscription URL -->
  <v-card variant="tonal" class="pa-4 mb-4" data-testid="block-url">
    <v-card-title>订阅地址</v-card-title>
    <v-card-text>
      <p class="text-body-2 mb-2">订阅地址就是客户端要填的 URL；令牌是它的凭据，换令牌等于换地址。</p>
      <v-select
        v-model="selectedToken"
        :items="tokens.map((t) => ({ title: t.name || t.token, value: t.token }))"
        label="选择令牌"
        hint="令牌是订阅地址的凭据"
        persistent-hint
        data-testid="token-select"
      />
      <div v-if="selected !== null" data-testid="url-preview">
        <code data-testid="subscription-url-code">{{ subscribeUrl(origin, selected.token) }}</code>
        <v-text-field
          :model-value="subscribeUrl(origin, selected.token)"
          readonly
          label="订阅 URL"
          hint="只读预览"
          persistent-hint
          data-testid="subscription-url"
        />
        <v-btn data-testid="copy-url-btn" variant="tonal" @click="copyUrl">复制</v-btn>
      </div>
      <div v-else data-testid="no-token-empty" class="text-medium-emphasis mt-2">
        请选择一个令牌。还没有订阅令牌。令牌是订阅地址的凭据，一个令牌对应一个订阅地址。先生成一个令牌，再把地址填进客户端。
      </div>
      <div data-testid="risk-hint" class="text-warning mt-3">
        任何拿到这个 URL 的人都能拉到你的节点；令牌泄露请立刻删除并重建。
      </div>
    </v-card-text>
  </v-card>

  <!-- Block 2: tokens -->
  <v-card variant="tonal" class="pa-4 mb-4" data-testid="block-tokens">
    <v-card-title>令牌</v-card-title>
    <v-card-text>
      <v-btn data-testid="create-token-btn" variant="tonal" class="mb-3" @click="openCreateToken">新建令牌</v-btn>
      <table data-testid="token-table" class="w-100">
        <thead><tr><th>令牌</th><th>名称</th><th>操作</th></tr></thead>
        <tbody>
          <tr v-for="t in tokens" :key="t.token" data-testid="token-row">
            <td><code>{{ maskToken(t.token) }}</code></td>
            <td>{{ t.name }}</td>
            <td>
              <v-btn size="small" variant="text" data-testid="token-edit" @click="openEditToken(t)">编辑</v-btn>
              <v-btn size="small" variant="text" color="error" data-testid="token-delete" @click="deleteToken(t)">删除</v-btn>
            </td>
          </tr>
        </tbody>
      </table>
      <div v-if="tokens.length === 0" class="text-medium-emphasis mt-2">还没有令牌。</div>
    </v-card-text>
  </v-card>

  <v-dialog v-model="tokenDialog" max-width="480">
    <v-card>
      <v-card-title>{{ editingToken ? '编辑令牌' : '新建令牌' }}</v-card-title>
      <v-card-text>
        <v-text-field v-model="tokenForm.token" label="令牌" hint="建议留空由后端生成，或自定义（唯一）" persistent-hint data-testid="token-field" />
        <v-text-field v-model="tokenForm.name" label="名称" hint="自己认得出的备注，例如「手机」「给朋友」" persistent-hint data-testid="name-field" />
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="tokenDialog = false">取消</v-btn>
        <v-btn color="primary" data-testid="token-save" @click="saveToken">保存</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <!-- Block 3: traffic strategy -->
  <v-card variant="tonal" class="pa-4 mb-4" data-testid="block-strategy">
    <v-card-title>流量余额聚合策略</v-card-title>
    <v-card-text>
      <v-select
        v-model="preset"
        :items="PRESET_VALUES.map((p) => ({ title: p, value: p }))"
        label="整体预设"
        hint="选预设会自动填入 4 个字段，仍可手工修改"
        persistent-hint
        data-testid="preset-select"
        @update:model-value="onPresetChange"
      />
      <div class="text-caption text-medium-emphasis mb-2">{{ PRESET_LABELS[preset] }}</div>
      <div v-if="ignoresFields" data-testid="preset-ignores-fields" class="text-warning mb-2">该预设直接取某个机场的整份快照，不使用字段级策略；要按字段独立聚合请改用 sum/min/max/avg/first/last。</div>
      <div class="d-flex ga-2 flex-wrap">
        <v-select v-model="upload" :items="FIELD_VALUES" label="上传" hint="上传字段策略" persistent-hint data-testid="upload-field" :disabled="ignoresFields" />
        <v-select v-model="download" :items="FIELD_VALUES" label="下载" hint="下载字段策略" persistent-hint data-testid="download-field" :disabled="ignoresFields" />
        <v-select v-model="total" :items="FIELD_VALUES" label="总量" hint="总量字段策略" persistent-hint data-testid="total-field" :disabled="ignoresFields" />
        <v-select v-model="expire" :items="FIELD_VALUES" label="到期" hint="到期字段策略" persistent-hint data-testid="expire-field" :disabled="ignoresFields" />
      </div>
      <div data-testid="field-zero-note" class="text-caption text-medium-emphasis mb-2">跳过值为 0 的机场：没上报余额的机场不会拉低结果，但 sum 会少算它的额度。</div>
      <div class="text-caption text-medium-emphasis mb-2">显式改过的字段优先于预设。</div>
      <div class="d-flex ga-2 flex-wrap mb-2">
        <v-btn size="small" variant="tonal" data-testid="quick-default" @click="applyQuick('sum/sum/sum/min')">默认</v-btn>
        <v-btn size="small" variant="tonal" data-testid="quick-strict" @click="applyQuick('min/min/min/min')">严格</v-btn>
        <v-btn size="small" variant="tonal" data-testid="quick-optimistic" @click="applyQuick('max/max/max/max')">乐观</v-btn>
        <v-btn size="small" variant="tonal" data-testid="quick-independent" @click="applyQuick('avg/avg/avg/avg')">独立</v-btn>
        <v-btn size="small" variant="tonal" data-testid="quick-hidden" @click="applyQuick('none/none/none/none')">隐藏</v-btn>
      </div>
      <div class="d-flex ga-2">
        <v-btn data-testid="save-strategy-btn" :loading="savingStrategy" @click="saveStrategy">保存</v-btn>
        <v-btn data-testid="preview-btn" variant="tonal" :loading="previewing" @click="previewStrategy">试算</v-btn>
      </div>
      <div v-if="previewHeader !== ''" data-testid="preview-header" class="mt-2"><code>{{ previewHeader }}</code></div>
      <div v-if="previewOk === false" data-testid="preview-okfalse" class="text-medium-emphasis mt-1">
        当前策略不会写入响应头（可能是「不透传」或还没有机场余额数据）。
      </div>
      <div data-testid="strategy-risk-hint" class="text-warning mt-3">
        策略只影响客户端显示的余额数字，不会改变各机场的真实配额；sum 在机场额度不叠加时会显示虚高的剩余量。
      </div>
    </v-card-text>
  </v-card>

  <!-- Block 4: refresh -->
  <v-card variant="tonal" class="pa-4 mb-4" data-testid="block-refresh">
    <v-card-title>余额刷新</v-card-title>
    <v-card-text>
      <p class="text-body-2 mb-2">刷新会逐个机场拉取上游订阅头，慢是正常的；失败不影响已有余额。</p>
      <p class="text-body-2 text-medium-emphasis mb-2">刷新失败不会把下游余额清零——subscription-userinfo 仍用上次成功采到的值，失败只记在 traffic_err 里。</p>
      <v-btn data-testid="refresh-btn" variant="tonal" :loading="refreshLoading" @click="refreshAll">立即刷新全部机场余额</v-btn>
      <div v-if="refreshError !== ''" data-testid="refresh-error" class="text-error mt-2">{{ refreshError }}</div>
      <div v-if="refreshResult !== null" data-testid="refresh-summary" class="mt-2">
        ok={{ refreshResult.ok }} / fail={{ refreshResult.fail }}
      </div>
      <table v-if="refreshResult !== null" data-testid="refresh-table" class="w-100 mt-2">
        <thead><tr><th>名称</th><th>成功</th><th>节点数</th><th>错误</th></tr></thead>
        <tbody>
          <tr v-for="r in (refreshResult?.results ?? [])" :key="r.airport_id" data-testid="refresh-row">
            <td>{{ r.name }}</td>
            <td>{{ r.ok ? '是' : '否' }}</td>
            <td>{{ r.node_count }}</td>
            <td data-testid="refresh-error">{{ r.err }}</td>
          </tr>
        </tbody>
      </table>
      <div v-if="refreshResult !== null && (refreshResult.results?.length ?? 0) === 0" data-testid="refresh-empty" class="text-medium-emphasis mt-2">
        还没有刷新结果。
      </div>
    </v-card-text>
  </v-card>
</template>