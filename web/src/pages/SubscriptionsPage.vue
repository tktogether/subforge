<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { endpoints, subscribeUrl, type Profile, type SubToken, type TrafficStrategy, type TrafficRefreshResponse } from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import { notify } from '../composables/useNotify'
import { pageHelp } from '../content/help'
import PageHeader from '../components/PageHeader.vue'
import InfoTip from '../components/InfoTip.vue'

const help = pageHelp('subscriptions')

// 方案（Profile）列表：令牌可以绑定方案，绑定后该订阅只使用该方案的规则与分组。
// 拉取失败降级为「只有未分配」，不阻塞令牌列表本身。
const profiles = ref<Profile[]>([])

async function loadProfiles(): Promise<void> {
  try {
    profiles.value = await endpoints.listProfiles()
  } catch {
    profiles.value = []
  }
}

// 方案下拉选项：未分配(0) + 各方案（title=方案名，value=方案 id）。
const profileOptions = computed(() => [
  { title: '未绑定方案', value: 0 },
  ...profiles.value.map((p) => ({ title: p.name, value: p.id })),
])

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
interface TokenForm {
  token: string
  name: string
  // 绑定的方案 id：0 = 未分配（旧的全局池）。
  profile_id: number
}
const tokenDialog = ref(false)
const editingToken = ref<SubToken | null>(null)
function emptyTokenForm(): TokenForm {
  return { token: '', name: '', profile_id: 0 }
}
const tokenForm = ref<TokenForm>(emptyTokenForm())
function openCreateToken(): void {
  editingToken.value = null
  tokenForm.value = emptyTokenForm()
  tokenDialog.value = true
}
function openEditToken(t: SubToken): void {
  editingToken.value = t
  tokenForm.value = {
    token: t.token,
    name: t.name,
    profile_id: t.profile_id ?? 0,
  }
  tokenDialog.value = true
}
async function saveToken(): Promise<void> {
  try {
    const f = tokenForm.value
    if (editingToken.value === null) {
      // 令牌只做凭据；输出配置（目标/模板/管线/包含/排除）在方案页维护。
      await endpoints.createToken({
        token: f.token,
        name: f.name,
        profile_id: f.profile_id,
      })
    } else {
      // 编辑只提交凭据字段与绑定方案；旧版令牌上的 target 等字段保持原值，避免静默清空。
      await endpoints.updateToken(f.token, {
        name: f.name,
        profile_id: f.profile_id,
      })
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
  sum: '各机场流量分别相加，到期时间取最早。',
  min: '各字段取所有机场中的最小值。',
  max: '各字段取所有机场中的最大值。',
  avg: '各字段取所有机场的平均值。',
  first: '使用排序第一的机场的数据。',
  last: '使用排序最后的机场的数据。',
  min_quota: '使用总流量最小的机场的完整数据。',
  max_quota: '使用总流量最大的机场的完整数据。',
  weighted: '按各机场节点数加权平均。',
  off: '不写入流量信息，客户端不显示余额。',
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
  void loadProfiles()
})
</script>

<template>
  <PageHeader title="订阅" :subtitle="help.oneLiner" help-key="subscriptions" />

  <!-- Block 1: subscription URL -->
  <v-card class="pa-4 mb-4" data-testid="block-url">
    <v-card-title>订阅地址</v-card-title>
    <v-card-text>
      <p class="text-body-2 mb-2">订阅地址是客户端填写的 URL；令牌是地址中的访问凭据。同一地址可以长期重复使用。</p>
      <v-select
        v-model="selectedToken"
        :items="tokens.map((t) => ({ title: t.name || t.token, value: t.token }))"
        label="选择令牌"
        hint="选择用于生成订阅地址的令牌"
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
        暂无订阅令牌。请先新建令牌，再复制生成的订阅地址。
      </div>
      <div data-testid="risk-hint" class="text-warning mt-3">
        持有该地址即可获取节点配置。如令牌泄露，请删除并重新创建。
      </div>
    </v-card-text>
  </v-card>

  <!-- Block 2: tokens -->
  <v-card class="pa-4 mb-4" data-testid="block-tokens">
    <v-card-title>令牌</v-card-title>
    <v-card-text>
      <v-btn data-testid="create-token-btn" variant="tonal" class="mb-3" @click="openCreateToken">新建令牌</v-btn>
      <table data-testid="token-table" class="w-100">
        <thead><tr><th>令牌</th><th>名称</th><th>方案</th><th>操作</th></tr></thead>
        <tbody>
          <tr v-for="t in tokens" :key="t.token" data-testid="token-row">
            <td><code>{{ maskToken(t.token) }}</code></td>
            <td>{{ t.name }}</td>
            <td data-testid="token-profile-name">{{ t.profile_name ?? '未绑定方案' }}</td>
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
        <v-text-field v-model="tokenForm.token" label="令牌" hint="留空由系统生成，或填写唯一值" persistent-hint data-testid="token-field" />
        <v-text-field v-model="tokenForm.name" label="名称" hint="用于区分用途，例如设备名称" persistent-hint data-testid="name-field" />
        <v-select
          v-model="tokenForm.profile_id"
          :items="profileOptions"
          item-title="title"
          item-value="value"
          label="方案"
          persistent-hint
          data-testid="token-profile"
        />
        <div data-testid="token-profile-hint" class="text-caption text-medium-emphasis mb-2">
          令牌只作为访问凭据；目标格式、模板、管线与节点筛选由绑定的方案决定。
        </div>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="tokenDialog = false">取消</v-btn>
        <v-btn color="primary" data-testid="token-save" @click="saveToken">保存</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <!-- Block 3: traffic strategy -->
  <v-card class="pa-4 mb-4" data-testid="block-strategy">
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
      <div v-if="ignoresFields" data-testid="preset-ignores-fields" class="text-warning mb-2">该预设使用单个机场的完整数据，不使用字段级策略。</div>
      <div class="d-flex ga-2 flex-wrap">
        <v-select v-model="upload" :items="FIELD_VALUES" label="上传" hint="上传流量策略" persistent-hint data-testid="upload-field" :disabled="ignoresFields" />
        <v-select v-model="download" :items="FIELD_VALUES" label="下载" hint="下载流量策略" persistent-hint data-testid="download-field" :disabled="ignoresFields" />
        <v-select v-model="total" :items="FIELD_VALUES" label="总量" hint="总流量策略" persistent-hint data-testid="total-field" :disabled="ignoresFields" />
        <v-select v-model="expire" :items="FIELD_VALUES" label="到期" hint="到期时间策略" persistent-hint data-testid="expire-field" :disabled="ignoresFields" />
      </div>
      <div class="d-flex align-center ga-1 mb-2">
        <span class="text-caption text-medium-emphasis" data-testid="field-zero-note">数值为 0 的机场不参与计算。</span>
        <InfoTip text="未获取到流量信息的机场会被跳过；求和时其额度不会计入。" label="流量字段说明" />
      </div>
      <div class="d-flex ga-2 flex-wrap mb-2">
        <v-btn size="small" variant="tonal" data-testid="quick-default" @click="applyQuick('sum/sum/sum/min')">求和</v-btn>
        <v-btn size="small" variant="tonal" data-testid="quick-strict" @click="applyQuick('min/min/min/min')">最小值</v-btn>
        <v-btn size="small" variant="tonal" data-testid="quick-optimistic" @click="applyQuick('max/max/max/max')">最大值</v-btn>
        <v-btn size="small" variant="tonal" data-testid="quick-independent" @click="applyQuick('avg/avg/avg/avg')">平均值</v-btn>
        <v-btn size="small" variant="tonal" data-testid="quick-hidden" @click="applyQuick('none/none/none/none')">不透传</v-btn>
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
        策略只影响客户端显示的余额，不会改变各机场的真实配额。
      </div>
    </v-card-text>
  </v-card>

  <!-- Block 4: refresh -->
  <v-card class="pa-4 mb-4" data-testid="block-refresh">
    <v-card-title>余额刷新</v-card-title>
    <v-card-text>
      <p class="text-body-2 mb-2">刷新会逐个请求机场的订阅地址。刷新失败时保留上一次成功获取的余额。</p>
      <v-btn data-testid="refresh-btn" variant="tonal" :loading="refreshLoading" @click="refreshAll">立即刷新全部机场余额</v-btn>
      <div v-if="refreshError !== ''" data-testid="refresh-error" class="text-error mt-2">{{ refreshError }}</div>
      <div v-if="refreshResult !== null" data-testid="refresh-summary" class="mt-2">
        成功 {{ refreshResult.ok }} 个，失败 {{ refreshResult.fail }} 个
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