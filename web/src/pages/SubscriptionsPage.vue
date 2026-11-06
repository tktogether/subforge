<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { endpoints, subscribeUrl, type Profile, type SubToken, type TrafficStrategy, type TrafficUsage, type TrafficRefreshResponse } from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import { notify } from '../composables/useNotify'
import { confirmAction } from '../composables/useConfirm'
import { copyText } from '../utils/clipboard'
import SfPageHead from '../components/ui/SfPageHead.vue'
import SfField from '../components/ui/SfField.vue'
import SfDialog from '../components/ui/SfDialog.vue'
import SfNote from '../components/ui/SfNote.vue'
import SfEmpty from '../components/ui/SfEmpty.vue'
import SfInfoTip from '../components/ui/SfInfoTip.vue'
import { formatBytes, formatExpire } from '../format'
import { IconPlus, IconCopy, IconPencil, IconTrash, IconRefresh } from '@tabler/icons-vue'

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
const loadingTokens = ref(false)
const errorTokens = ref('')

const selected = computed(() => tokens.value.find((t) => t.token === selectedToken.value) ?? null)

async function loadTokens(): Promise<void> {
  loadingTokens.value = true
  errorTokens.value = ''
  try {
    tokens.value = await endpoints.listTokens()
    if (selectedToken.value === '' && tokens.value.length > 0) {
      selectedToken.value = tokens.value[0].token
    }
  } catch (err) {
    errorTokens.value = errorMessage(err)
  } finally {
    loadingTokens.value = false
  }
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
    await loadTokens()
    notify('已保存', 'ok')
  } catch (err) {
    notify(errorMessage(err), 'error')
  }
}
async function deleteToken(t: SubToken): Promise<void> {
  const ok = await confirmAction({
    title: '确认删除令牌？',
    body: '使用该令牌的订阅地址会立即失效；客户端需要换成新地址。',
    confirmText: '删除',
    danger: true,
  })
  if (!ok) return
  try {
    await endpoints.deleteToken(t.token)
    await loadTokens()
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
  const ok = await copyText(subscribeUrl(origin.value, selected.value.token))
  notify(ok ? '已复制订阅地址' : '复制失败', ok ? 'ok' : 'error')
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
const previewUsage = ref<TrafficUsage | null>(null)
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
  previewUsage.value = null
  previewOk.value = null
  try {
    const r = await endpoints.previewTraffic(strategyObj.value)
    previewHeader.value = r.header
    previewUsage.value = r.usage ?? null
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
    const r = await endpoints.refreshAllAirports(true)
    refreshResult.value = r
  } catch (err) {
    refreshError.value = errorMessage(err)
  } finally {
    refreshLoading.value = false
  }
}

onMounted(() => {
  void loadTokens()
  void loadStrategy()
  void loadProfiles()
})
</script>

<template>
  <div>
    <SfPageHead title="订阅" desc="管理订阅令牌与流量余额聚合策略。">
      <template #actions>
        <button class="sf-btn sf-btn--primary" type="button" data-testid="create-token-btn" @click="openCreateToken">
          <IconPlus :size="15" :stroke-width="2" />
          新建令牌
        </button>
      </template>
    </SfPageHead>

    <!-- 订阅地址 -->
    <section class="sf-panel sf-panel--ticks sf-rise sf-d1" data-testid="block-url" style="margin-bottom: 16px">
      <div class="sf-panel__head">
        <span class="sf-label-mono">SUBSCRIPTION URL / 订阅地址</span>
      </div>
      <div class="sf-panel__body">
        <p class="sf-field__hint" style="margin: 0 0 14px">
          订阅地址是客户端填写的 URL；令牌是地址中的访问凭据。同一地址可以长期重复使用。
        </p>
        <SfField label="选择令牌" hint="选择用于生成订阅地址的令牌" style="max-width: 360px">
          <span class="sf-select-wrap">
            <select v-model="selectedToken" class="sf-select" data-testid="token-select">
              <option value="" disabled>选择令牌…</option>
              <option v-for="t in tokens" :key="t.token" :value="t.token">{{ t.name || t.token }}</option>
            </select>
          </span>
        </SfField>
        <div v-if="selected !== null" data-testid="url-preview" class="sb-urlbox">
          <code class="sb-urlbox__url sf-mono" data-testid="subscription-url-code">{{ subscribeUrl(origin, selected.token) }}</code>
          <button class="sf-btn sf-btn--primary sf-btn--sm" type="button" data-testid="copy-url-btn" @click="copyUrl">
            <IconCopy :size="14" :stroke-width="1.8" />
            复制地址
          </button>
        </div>
        <div v-else data-testid="no-token-empty" class="sf-faint" style="font-size: 13px">
          暂无订阅令牌。请先新建令牌，再复制生成的订阅地址。
        </div>
        <SfNote tone="warn" data-testid="risk-hint" style="margin-top: 14px">
          持有该地址即可获取节点配置。如令牌泄露，请删除并重新创建。
        </SfNote>
      </div>
    </section>

    <!-- 令牌 -->
    <section class="sf-panel sf-rise sf-d2" data-testid="block-tokens" style="margin-bottom: 16px">
      <div class="sf-panel__head">
        <span class="sf-panel__title">令牌</span>
        <span class="sf-spacer" />
        <button class="sf-btn sf-btn--subtle sf-btn--sm" type="button" @click="loadTokens">
          <IconRefresh :size="14" :stroke-width="1.8" :class="{ 'sb-spin': loadingTokens }" />
          刷新
        </button>
      </div>
      <div v-if="tokens.length === 0 && errorTokens === ''" data-testid="token-empty">
        <SfEmpty title="还没有令牌" desc="点击「新建令牌」创建第一个访问凭据。" />
      </div>
      <div v-else-if="errorTokens !== ''" class="sf-field__error" style="padding: 0 18px 14px">{{ errorTokens }}</div>
      <div v-else class="sf-table-wrap" data-testid="token-table" style="border: none; border-top: 1px solid var(--sf-border); border-radius: 0">
        <table class="sf-table">
          <thead>
            <tr><th>令牌</th><th>名称</th><th>方案</th><th style="text-align: right; width: 110px">操作</th></tr>
          </thead>
          <tbody>
            <tr v-for="t in tokens" :key="t.token" data-testid="token-row">
              <td><code class="sb-token" :title="t.token">{{ maskToken(t.token) }}</code></td>
              <td>{{ t.name || '—' }}</td>
              <td data-testid="token-profile-name"><span :class="{ 'sf-faint': t.profile_name === undefined || t.profile_name === '' }">{{ t.profile_name ?? '未绑定方案' }}</span></td>
              <td>
                <span class="sf-table__actions">
                  <button class="sf-btn sf-btn--subtle sf-btn--icon sf-btn--sm" type="button" aria-label="编辑" data-testid="token-edit" @click="openEditToken(t)">
                    <IconPencil :size="15" :stroke-width="1.7" />
                  </button>
                  <button class="sf-btn sf-btn--subtle sf-btn--icon sf-btn--sm sf-btn--del" type="button" aria-label="删除" data-testid="token-delete" @click="deleteToken(t)">
                    <IconTrash :size="15" :stroke-width="1.7" />
                  </button>
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <!-- 流量策略 -->
    <section class="sf-panel sf-rise sf-d3" data-testid="block-strategy" style="margin-bottom: 16px">
      <div class="sf-panel__head">
        <span class="sf-panel__title">流量余额聚合策略</span>
        <span class="sf-spacer" />
        <button class="sf-btn sf-btn--subtle sf-btn--sm" type="button" data-testid="preview-btn" :disabled="previewing" @click="previewStrategy">
          {{ previewing ? '试算中…' : '试算' }}
        </button>
        <button class="sf-btn sf-btn--primary sf-btn--sm" type="button" data-testid="save-strategy-btn" :disabled="savingStrategy" @click="saveStrategy">
          {{ savingStrategy ? '保存中…' : '保存策略' }}
        </button>
      </div>
      <div class="sf-panel__body">
        <div class="sb-strategy">
          <SfField label="整体预设" hint="选预设会自动填入 4 个字段，仍可手工修改">
            <span class="sf-select-wrap">
              <select class="sf-select" data-testid="preset-select" :value="preset" @change="onPresetChange(($event.target as HTMLSelectElement).value)">
                <option v-for="p in PRESET_VALUES" :key="p" :value="p">{{ p }}</option>
              </select>
            </span>
          </SfField>
          <SfField label="上传">
            <span class="sf-select-wrap">
              <select v-model="upload" class="sf-select" data-testid="upload-field" :disabled="ignoresFields">
                <option v-for="v in FIELD_VALUES" :key="v" :value="v">{{ v }}</option>
              </select>
            </span>
          </SfField>
          <SfField label="下载">
            <span class="sf-select-wrap">
              <select v-model="download" class="sf-select" data-testid="download-field" :disabled="ignoresFields">
                <option v-for="v in FIELD_VALUES" :key="v" :value="v">{{ v }}</option>
              </select>
            </span>
          </SfField>
          <SfField label="总量">
            <span class="sf-select-wrap">
              <select v-model="total" class="sf-select" data-testid="total-field" :disabled="ignoresFields">
                <option v-for="v in FIELD_VALUES" :key="v" :value="v">{{ v }}</option>
              </select>
            </span>
          </SfField>
          <SfField label="到期">
            <span class="sf-select-wrap">
              <select v-model="expire" class="sf-select" data-testid="expire-field" :disabled="ignoresFields">
                <option v-for="v in FIELD_VALUES" :key="v" :value="v">{{ v }}</option>
              </select>
            </span>
          </SfField>
        </div>
        <p class="sb-preset-label sf-mono">{{ PRESET_LABELS[preset] ?? '自定义字段级策略。' }}</p>
        <SfNote v-if="ignoresFields" tone="warn" data-testid="preset-ignores-fields" style="margin-bottom: 12px">
          该预设使用单个机场的完整数据，不使用字段级策略。
        </SfNote>
        <div class="sf-toolbar" style="margin-bottom: 12px">
          <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" data-testid="quick-default" @click="applyQuick('sum/sum/sum/min')">求和</button>
          <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" data-testid="quick-strict" @click="applyQuick('min/min/min/min')">最小值</button>
          <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" data-testid="quick-optimistic" @click="applyQuick('max/max/max/max')">最大值</button>
          <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" data-testid="quick-independent" @click="applyQuick('avg/avg/avg/avg')">平均值</button>
          <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" data-testid="quick-hidden" @click="applyQuick('none/none/none/none')">不透传</button>
        </div>
        <p class="sf-field__hint" style="margin: 0 0 4px">
          数值为 0 的机场不参与计算。
          <SfInfoTip text="未获取到流量信息的机场会被跳过；求和时其额度不会计入。" />
        </p>
        <div v-if="previewUsage !== null" data-testid="preview-summary" class="sb-preview">
          <div class="sb-preview__item">
            <span class="sb-preview__label">上传</span>
            <span class="sb-preview__value">{{ formatBytes(previewUsage.upload ?? 0) }}</span>
          </div>
          <div class="sb-preview__item">
            <span class="sb-preview__label">下载</span>
            <span class="sb-preview__value">{{ formatBytes(previewUsage.download ?? 0) }}</span>
          </div>
          <div class="sb-preview__item">
            <span class="sb-preview__label">总量</span>
            <span class="sb-preview__value">{{ formatBytes(previewUsage.total ?? 0) }}</span>
          </div>
          <div class="sb-preview__item">
            <span class="sb-preview__label">到期</span>
            <span class="sb-preview__value">{{ formatExpire(previewUsage.expire ?? 0) }}</span>
          </div>
        </div>
        <p v-if="previewHeader !== ''" class="sf-mono sb-header" data-testid="preview-header">{{ previewHeader }}</p>
        <p v-if="previewOk === false" data-testid="preview-okfalse" class="sf-field__hint" style="margin: 8px 0 0">
          当前策略不会写入响应头（可能是「不透传」或还没有机场余额数据）。
        </p>
        <SfNote tone="info" data-testid="strategy-risk-hint" style="margin-top: 14px">
          策略只影响客户端显示的余额，不会改变各机场的真实配额。
        </SfNote>
      </div>
    </section>

    <!-- 刷新全部机场 -->
    <section class="sf-panel sf-rise sf-d4" data-testid="block-refresh">
      <div class="sf-panel__head">
        <span class="sf-panel__title">刷新全部机场</span>
        <span class="sf-spacer" />
        <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" data-testid="refresh-btn" :disabled="refreshLoading" @click="refreshAll">
          <IconRefresh :size="14" :stroke-width="1.8" :class="{ 'sb-spin': refreshLoading }" />
          {{ refreshLoading ? '刷新中…' : '立即刷新全部机场' }}
        </button>
      </div>
      <div class="sf-panel__body">
        <p class="sf-field__hint" style="margin: 0 0 4px">
          逐个请求机场的订阅地址，更新节点、流量与规则源。刷新失败时保留上一次成功获取的余额。
        </p>
        <div v-if="refreshError !== ''" data-testid="refresh-error" class="sf-field__error">{{ refreshError }}</div>
        <p v-if="refreshResult !== null" data-testid="refresh-summary" class="sf-faint" style="font-size: 12.5px">
          成功 {{ refreshResult.ok }} 个，失败 {{ refreshResult.fail }} 个
        </p>
        <div v-if="refreshResult !== null && (refreshResult.results?.length ?? 0) > 0" class="sf-table-wrap" data-testid="refresh-table">
          <table class="sf-table">
            <thead><tr><th>名称</th><th style="width: 70px">成功</th><th style="text-align: right; width: 80px">节点数</th><th>错误</th></tr></thead>
            <tbody>
              <tr v-for="r in refreshResult?.results ?? []" :key="r.airport_id" data-testid="refresh-row">
                <td>{{ r.name }}</td>
                <td>
                  <span class="sf-status">
                    <span class="sf-led" :class="r.ok ? 'sf-led--ok' : 'sf-led--danger'" />
                    <span>{{ r.ok ? '是' : '否' }}</span>
                  </span>
                </td>
                <td class="sf-table__num" style="text-align: right">{{ r.node_count }}</td>
                <td><span class="sf-faint" style="font-size: 12px">{{ r.err }}</span></td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-if="refreshResult !== null && (refreshResult.results?.length ?? 0) === 0" data-testid="refresh-empty" class="sf-faint" style="font-size: 12.5px">
          还没有刷新结果。
        </p>
      </div>
    </section>

    <!-- 令牌编辑 -->
    <SfDialog :open="tokenDialog" :label="editingToken ? 'EDIT TOKEN' : 'CREATE TOKEN'" :title="editingToken ? '编辑令牌' : '新建令牌'" @close="tokenDialog = false">
      <SfField hint="留空由系统生成，或填写唯一值" label="令牌">
        <input v-model="tokenForm.token" class="sf-input sf-input--mono" data-testid="token-field" :disabled="editingToken !== null" spellcheck="false" />
      </SfField>
      <SfField hint="用于区分用途，例如设备名称" label="名称">
        <input v-model="tokenForm.name" class="sf-input" data-testid="name-field" spellcheck="false" />
      </SfField>
      <SfField label="方案">
        <span class="sf-select-wrap">
          <select v-model="tokenForm.profile_id" class="sf-select" data-testid="token-profile">
            <option v-for="opt in profileOptions" :key="opt.value" :value="opt.value">{{ opt.title }}</option>
          </select>
        </span>
      </SfField>
      <SfNote tone="info" data-testid="token-profile-hint">
        令牌只作为访问凭据；目标格式、模板、管线与节点筛选由绑定的方案决定。
      </SfNote>
      <template #foot>
        <button class="sf-btn sf-btn--subtle" type="button" @click="tokenDialog = false">取消</button>
        <button class="sf-btn sf-btn--primary" type="button" data-testid="token-save" @click="saveToken">保存</button>
      </template>
    </SfDialog>
  </div>
</template>

<style scoped>
.sb-spin {
  animation: sb-rotate 0.9s linear infinite;
}

@keyframes sb-rotate {
  to {
    transform: rotate(360deg);
  }
}

.sb-urlbox {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 14px;
  padding: 12px 14px;
  border: 1px solid var(--sf-border-strong);
  border-radius: var(--sf-radius);
  background: var(--sf-bg-deep);
  flex-wrap: wrap;
}

.sb-urlbox__url {
  flex: 1;
  min-width: 200px;
  font-size: 12.5px;
  color: var(--sf-text);
  overflow-wrap: anywhere;
  line-height: 1.6;
}

.sb-token {
  font-family: var(--sf-font-mono);
  font-size: 12px;
  color: var(--sf-text-muted);
}

.sb-strategy {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 12px;
  margin-bottom: 12px;
}

.sb-preset-label {
  margin: 0 0 12px;
  font-size: 11.5px;
  color: var(--sf-text-faint);
  line-height: 1.6;
}

.sb-preview {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(130px, 1fr));
  gap: 1px;
  background: var(--sf-border);
  border: 1px solid var(--sf-border);
  border-radius: var(--sf-radius);
  overflow: hidden;
  margin-top: 12px;
}

.sb-preview__item {
  background: var(--sf-surface-dim);
  padding: 10px 14px;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.sb-preview__label {
  font: 500 10px/1.2 var(--sf-font-mono);
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--sf-text-faint);
}

.sb-preview__value {
  font: 600 15px/1.3 var(--sf-font-display);
  font-variant-numeric: tabular-nums;
  overflow-wrap: anywhere;
}

.sb-header {
  margin: 10px 0 0;
  padding: 8px 12px;
  border: 1px solid var(--sf-border);
  border-radius: var(--sf-radius-sm);
  background: var(--sf-bg-deep);
  font-size: 11px;
  color: var(--sf-text-muted);
  overflow-wrap: anywhere;
}
</style>
