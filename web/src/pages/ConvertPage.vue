<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import {
  endpoints,
  type ConvertResponse,
  type Pipeline,
  type DiffLine,
} from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import { pageHelp } from '../content/help'
import PageHeader from '../components/PageHeader.vue'
import AsyncSection from '../components/AsyncSection.vue'
import InfoTip from '../components/InfoTip.vue'

const help = pageHelp('convert')

const targets = ref<string[]>([])
const pipelines = ref<Pipeline[]>([])
const pipelineItems = computed(() =>
  pipelines.value.map((p) => ({
    title: p.name + (p.source === 'template' ? ' · 只读' : ''),
    value: p.name,
  }))
)
const selectedTarget = ref('')
const urls = ref('')
const text = ref('')
const types = ref('')
const include = ref('')
const exclude = ref('')
const pipeline = ref('')
const configs = ref<string[]>([])
const configValue = ref('')
const sort = ref('false')
const list = ref(false)
const loading = ref(false)
const error = ref('')
const result = ref<ConvertResponse | null>(null)
const copied = ref(false)
const targetHelpOpen = ref(false)

function closeTargetHelp(): void {
  targetHelpOpen.value = false
}

const objectUrl = ref<string | null>(null)

// diff
const diffA = ref('')
const diffB = ref('')
const diffLoading = ref(false)
const diffError = ref('')
const diffLines = ref<DiffLine[]>([])
const diffRan = ref(false)

const TARGET_MEANINGS: { target: string; form: string; where: string }[] = [
  { target: 'clash.meta', form: 'Clash/Mihomo YAML 配置', where: 'Clash Verge / Mihomo / ClashX Meta（默认值）' },
  { target: 'surge', form: 'Surge 配置', where: 'Surge' },
  { target: 'quan', form: 'Quantumult 配置', where: 'Quantumult' },
  { target: 'quanx', form: 'Quantumult X 配置', where: 'Quantumult X' },
  { target: 'loon', form: 'Loon 配置', where: 'Loon' },
  { target: 'surfboard', form: 'Surfboard 配置', where: 'Surfboard' },
  { target: 'mellow', form: 'Mellow 配置', where: 'Mellow' },
  { target: 'list', form: '分享链接清单（一行一个）', where: '只想拿到链接，不要配置' },
  { target: 'mixed', form: 'Base64 混合订阅', where: '通用 base64 订阅' },
  { target: 'sing-box', form: 'sing-box JSON', where: 'sing-box' },
  { target: 'singbox', form: 'sing-box JSON', where: 'sing-box' },
  { target: 'v2ray', form: 'base64 分享链接', where: 'v2rayN / Nekoray / hiddify' },
  { target: 'ss', form: 'SS 订阅（明文）', where: '各类 SS 客户端' },
  { target: 'ssd', form: 'SS 订阅（base64）', where: '各类 SS 客户端' },
  { target: 'shadowsocks', form: 'SS 订阅', where: '各类 SS 客户端' },
  { target: 'ssr', form: 'SSR 订阅', where: 'SSR 客户端' },
  { target: 'shadowsocksr', form: 'SSR 订阅', where: 'SSR 客户端' },
  { target: 'trojan', form: 'Trojan 订阅', where: 'Trojan 客户端' },
  { target: 'ss-sub', form: 'SS-android 入口', where: 'Shadowsocks-android' },
  { target: 'ss-sub-json', form: 'SS-android JSON 入口', where: 'Shadowsocks-android' },
  { target: 'ss-sub-b64', form: 'SS-android base64 入口', where: 'Shadowsocks-android' },
  { target: 'ssjson', form: 'SS-android JSON', where: 'Shadowsocks-android' },
]

const meaningByTarget = new Map(TARGET_MEANINGS.map((m) => [m.target, m]))

// 取值含义表按 /api/targets 的真实返回值生成（后端支持哪些 target 不在这里猜测）；
// 未收录的取值归入「其他」。接口尚未返回时退回已收录的常用项，避免表格空白。
const targetMeanings = computed(() => {
  const names = targets.value.length > 0 ? targets.value : TARGET_MEANINGS.map((m) => m.target)
  return names.map((name) => {
    const known = meaningByTarget.get(name)
    return {
      target: name,
      form: known ? known.form : '其他（未收录的取值）',
      where: known ? known.where : '以 /api/targets 为准，可原样填进 target',
    }
  })
})

async function loadMeta(): Promise<void> {
  try {
    const [t, p, c] = await Promise.all([endpoints.targets(), endpoints.pipelines(''), endpoints.configs()])
    targets.value = t
    pipelines.value = p
    configs.value = c.configs
  } catch (err) {
    error.value = errorMessage(err)
  }
}

async function submit(): Promise<void> {
  if (urls.value.trim() === '' && text.value.trim() === '') {
    error.value = 'URL 与文本不能同时为空'
    return
  }
  loading.value = true
  error.value = ''
  result.value = null
  try {
    result.value = await endpoints.convert({
      target: selectedTarget.value,
      urls: urls.value.split('\n').map((s) => s.trim()).filter((s) => s !== ''),
      text: text.value,
      types: types.value.split(',').map((s) => s.trim()).filter((s) => s !== ''),
      include: include.value.split(',').map((s) => s.trim()).filter((s) => s !== ''),
      exclude: exclude.value.split(',').map((s) => s.trim()).filter((s) => s !== ''),
      pipeline: pipeline.value,
      config: configValue.value,
      sort: sort.value,
      list: list.value,
      override: '',
    })
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

async function copy(): Promise<void> {
  if (result.value === null) return
  try {
    await navigator.clipboard.writeText(result.value.body)
    copied.value = true
  } catch {
    copied.value = false
  }
}

function filenameFor(format: string): string {
  if (format === 'list') return 'list.txt'
  if (format === 'sing-box' || format === 'singbox') return 'sing-box.json'
  return format + '.yaml'
}

function download(): void {
  if (result.value === null) return
  const blob = new Blob([result.value.body], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  objectUrl.value = url
  const a = document.createElement('a')
  a.href = url
  a.download = filenameFor(result.value.format)
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
  objectUrl.value = null
}

async function runDiff(): Promise<void> {
  diffLoading.value = true
  diffError.value = ''
  diffRan.value = true
  try {
    const r = await endpoints.diff(diffA.value, diffB.value)
    diffLines.value = r.lines
  } catch (err) {
    diffError.value = errorMessage(err)
    diffLines.value = []
  } finally {
    diffLoading.value = false
  }
}

function lineClass(t: string): string {
  if (t === '-') return 'line-del'
  if (t === '+') return 'line-add'
  return 'line-same'
}

onMounted(() => {
  void loadMeta()
  // default diffB to last result body when available is handled in template via v-model default; keep empty here
})

onUnmounted(() => {
  if (objectUrl.value !== null) {
    URL.revokeObjectURL(objectUrl.value)
    objectUrl.value = null
  }
})
</script>

<template>
  <div class="convert-page">
    <PageHeader :title="help.title" :subtitle="help.oneLiner" help-key="convert" />

    <v-row>
      <v-col cols="12" md="6">
        <v-card class="pa-4" data-testid="convert-input-card">
          <div class="d-flex align-center mb-3">
            <span class="text-subtitle-1">转换参数</span>
            <v-spacer />
            <v-btn variant="text" size="small" prepend-icon="mdi-help-circle-outline" data-testid="target-meanings-toggle" @click="targetHelpOpen = true">目标格式说明</v-btn>
          </div>

          <div class="sf-convert-field">
            <div class="sf-convert-label">目标格式 <InfoTip text="留空时使用 clash.meta。" label="目标格式" /></div>
            <v-select v-model="selectedTarget" :items="targets" hide-details data-testid="target-select" />
          </div>
          <div class="sf-convert-field">
            <div class="sf-convert-label">订阅地址 <InfoTip text="每行填写一个上游订阅地址，支持 http 与 https。" example="https://example.com/sub?token=xxx" label="订阅地址" /></div>
            <v-textarea v-model="urls" rows="4" hide-details data-testid="urls" />
          </div>
          <div class="sf-convert-field">
            <div class="sf-convert-label">粘贴文本 <InfoTip text="直接粘贴订阅内容；服务端写入临时文件后解析，请求结束后删除。" label="粘贴文本" /></div>
            <v-textarea v-model="text" rows="3" hide-details data-testid="text" />
          </div>
          <div class="sf-convert-field">
            <div class="sf-convert-label">协议类型 <InfoTip text="只保留指定协议类型；逗号分隔，留空表示不过滤。" example="ss,vmess" label="协议类型" /></div>
            <v-text-field v-model="types" hide-details data-testid="types" />
          </div>
          <div class="sf-convert-field">
            <div class="sf-convert-label">包含节点 <InfoTip text="按节点名称匹配的逗号分隔正则；匹配的节点才会保留。" label="包含节点" /></div>
            <v-text-field v-model="include" hide-details data-testid="include" />
          </div>
          <div class="sf-convert-field">
            <div class="sf-convert-label">排除节点 <InfoTip text="按节点名称匹配的逗号分隔正则；匹配的节点会被移除。" label="排除节点" /></div>
            <v-text-field v-model="exclude" hide-details data-testid="exclude" />
          </div>
          <div class="sf-convert-field">
            <div class="sf-convert-label">管线 <InfoTip text="在节点进入渲染前执行的处理步骤；留空表示不使用管线。" label="管线" /></div>
            <v-select v-model="pipeline" :items="pipelineItems" hide-details data-testid="pipeline" />
          </div>
          <div class="sf-convert-field">
            <div class="sf-convert-label">转换模板 <InfoTip text="渲染使用的配置模板；留空表示使用默认模板。" label="转换模板" /></div>
            <v-select v-model="configValue" :items="configs" hide-details data-testid="config" />
          </div>
          <div class="mb-3">
            <router-link to="/pipelines" class="text-caption" data-testid="pipeline-link">打开管线页</router-link>
          </div>
          <div class="sf-convert-field sf-convert-field--switch">
            <span>仅输出节点列表</span>
            <InfoTip text="只输出节点分享链接列表，不生成完整配置。" label="仅输出节点列表" />
            <v-spacer />
            <v-switch v-model="list" color="primary" hide-details data-testid="list" />
          </div>
          <div class="sf-convert-field">
            <div class="sf-convert-label">节点排序 <InfoTip text="选择保持来源顺序或按节点名称排序。" label="节点排序" /></div>
            <v-select v-model="sort" :items="[{ title: '保持来源顺序', value: 'false' }, { title: '按节点名称排序', value: 'true' }]" hide-details data-testid="sort" />
          </div>
          <v-btn color="primary" :loading="loading" data-testid="submit" class="mt-2" @click="submit">转换</v-btn>
          <div v-if="urls.trim() === '' && text.trim() === ''" class="text-caption text-medium-emphasis mt-2">
            订阅地址与粘贴文本不能同时为空。
          </div>
        </v-card>
      </v-col>

      <v-col cols="12" md="6">
        <AsyncSection :loading="loading" :error="error" :empty="result === null ? (help.emptyState ?? false) : false" empty-text="结果将显示在这里" @retry="submit">
          <v-textarea :model-value="result?.body ?? ''" readonly rows="12" data-testid="result-body" />
          <div v-if="result !== null" class="mt-2" data-testid="result-summary">
            <div>输出格式：{{ result.format }}</div>
            <div>节点：{{ result.nodes }}（原始 {{ result.rawNodes }}）</div>
            <div>缓存命中：{{ result.cached }}</div>
          </div>
          <div v-if="result !== null" class="mt-2 d-flex gap-2">
            <v-btn variant="text" data-testid="copy" @click="copy">{{ copied ? '已复制' : '复制' }}</v-btn>
            <v-btn variant="text" data-testid="download" @click="download">下载</v-btn>
          </div>
          <div v-if="result?.failures && result.failures.length" data-testid="convert-failures">
            <div class="text-subtitle-2">失败来源</div>
            <div class="text-caption text-medium-emphasis">以下来源未成功获取；其他来源的节点仍会正常输出。</div>
            <ul><li v-for="(f, i) in result.failures" :key="i">{{ f }}</li></ul>
          </div>
          <div v-else-if="result !== null" data-testid="convert-failures-ok" class="text-caption text-success">全部来源成功</div>
          <div v-if="result?.warnings && result.warnings.length" data-testid="convert-warnings">
            <div class="text-subtitle-2">告警</div>
            <div class="text-caption text-medium-emphasis">以下内容被跳过或降级，不影响其他节点的输出。</div>
            <ul><li v-for="(w, i) in result.warnings" :key="i">{{ w }}</li></ul>
          </div>
        </AsyncSection>
      </v-col>
    </v-row>

    <v-card class="pa-4 mt-4" data-testid="diff-card">
      <div class="d-flex align-center mb-2">
        <span class="text-subtitle-1">产物对比</span>
        <InfoTip text="「-」表示仅在左侧存在，「+」表示仅在右侧存在；同一行被修改时会先删除再新增。" label="产物对比" />
      </div>
      <v-row>
        <v-col cols="12" md="6">
          <v-textarea v-model="diffA" label="基线 A" rows="6" data-testid="diff-a" />
        </v-col>
        <v-col cols="12" md="6">
          <v-textarea v-model="diffB" label="当前 B" rows="6" data-testid="diff-b" />
        </v-col>
      </v-row>
      <v-btn color="primary" :loading="diffLoading" data-testid="diff-run" @click="runDiff">对比</v-btn>
      <div v-if="diffError !== ''" data-testid="diff-error" class="text-error mt-2">{{ diffError }}</div>
      <div v-if="diffRan && diffLines.length === 0 && diffError === ''" data-testid="diff-empty" class="text-caption text-medium-emphasis mt-2">
        没有可对比的内容
      </div>
      <div v-if="diffLines.length > 0" data-testid="diff-result" class="mt-2">
        <div v-for="(ln, i) in diffLines" :key="i" :class="lineClass(ln.type)" :data-testid="'diff-line-' + i" class="diff-line">
          <span class="diff-type">{{ ln.type }}</span>{{ ln.text }}
        </div>
      </div>
    </v-card>

    <v-dialog :model-value="targetHelpOpen" max-width="760" scrollable data-testid="target-meanings-dialog" @update:model-value="closeTargetHelp">
      <v-card>
        <v-card-title>目标格式说明</v-card-title>
        <v-card-text>
          <p class="text-body-2 text-medium-emphasis mb-3">
            订阅地址会根据客户端类型自动选择目标格式；在本页选择后以本页设置为准。
          </p>
          <v-table density="compact" data-testid="target-meanings">
            <thead>
              <tr><th>取值</th><th>产物形态</th><th>适用客户端</th></tr>
            </thead>
            <tbody>
              <tr v-for="m in targetMeanings" :key="m.target">
                <td><code>{{ m.target }}</code></td>
                <td>{{ m.form }}</td>
                <td>{{ m.where }}</td>
              </tr>
            </tbody>
          </v-table>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" data-testid="target-meanings-close" @click="closeTargetHelp">关闭</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>

<style scoped>
.sf-convert-field {
  margin-bottom: 12px;
}
.sf-convert-label {
  display: flex;
  align-items: center;
  gap: 2px;
  min-height: 24px;
  margin-bottom: 2px;
  font-size: 13px;
  color: var(--sf-text-muted);
}
.sf-convert-field--switch {
  display: flex;
  align-items: center;
  gap: 4px;
  min-height: 40px;
  font-size: 13px;
  color: var(--sf-text-muted);
}
.diff-line { font-family: monospace; white-space: pre; }
.diff-type { display: inline-block; width: 1.2em; }
.line-same { color: var(--sf-text-faint); }
.line-del { color: var(--sf-danger); }
.line-add { color: var(--sf-ok); }
</style>
