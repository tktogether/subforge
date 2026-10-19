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
const sort = ref('false')
const list = ref(false)
const loading = ref(false)
const error = ref('')
const result = ref<ConvertResponse | null>(null)
const copied = ref(false)
const meaningsOpen = ref(false)

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
    const [t, p] = await Promise.all([endpoints.targets(), endpoints.pipelines('')])
    targets.value = t
    pipelines.value = p
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
      config: '',
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
          <div class="d-flex align-center target-meanings-toggle" data-testid="target-meanings-toggle" @click="meaningsOpen = !meaningsOpen">
            <v-icon size="small" class="mr-1">{{ meaningsOpen ? 'mdi-chevron-up' : 'mdi-chevron-down' }}</v-icon>
            <span class="text-subtitle-1">目标格式取值含义</span>
            <span class="text-caption text-medium-emphasis ml-2">默认收起，点击展开</span>
          </div>
          <div v-show="meaningsOpen" class="mt-2">
            <v-table density="compact" data-testid="target-meanings">
              <thead>
                <tr>
                  <th>target</th>
                  <th>产物形态</th>
                  <th>用在哪</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="m in targetMeanings" :key="m.target">
                  <td><code>{{ m.target }}</code></td>
                  <td>{{ m.form }}</td>
                  <td>{{ m.where }}</td>
                </tr>
              </tbody>
            </v-table>
            <div class="text-caption text-medium-emphasis mt-2">
              服务端也会按客户端 UA 自动推断 target（/sub 走的就是这条路）；网页转换页里显式选了就用选的。
            </div>
          </div>

          <v-select v-model="selectedTarget" :items="targets" label="目标格式" data-testid="target-select" class="mt-4" />
          <v-textarea v-model="urls" label="订阅 URL（每行一个）" rows="4" data-testid="urls" hint="支持 http(s)，也支持本地路径；一行一个" persistent-hint />
          <v-textarea v-model="text" label="粘贴文本" rows="3" data-testid="text" hint="把订阅内容直接粘进来，服务端会落到临时文件再解析，请求结束即删除" persistent-hint />
          <v-text-field v-model="types" label="类型过滤 types" data-testid="types" hint="只保留指定协议类型，逗号分隔，例如 ss,vmess；留空不过滤" persistent-hint />
          <v-text-field v-model="include" label="包含正则 include" data-testid="include" hint="按节点名匹配，逗号分隔；include 命中才留" persistent-hint />
          <v-text-field v-model="exclude" label="排除正则 exclude" data-testid="exclude" hint="按节点名匹配，逗号分隔；exclude 命中就丢；两个都填时先 include 再 exclude" persistent-hint />
          <v-select v-model="pipeline" :items="pipelineItems" label="管线" data-testid="pipeline" hint="管线是节点进来之后、渲染之前的一串处理步骤，详见「管线」页" persistent-hint />
          <div class="d-flex align-center">
            <router-link to="/pipelines" class="text-caption" data-testid="pipeline-link">打开管线页</router-link>
          </div>
          <v-switch v-model="list" label="输出节点列表" color="primary" data-testid="list" hint="打开后只输出节点列表（target=list 的分享链接形态），不生成完整配置" persistent-hint />
          <v-select v-model="sort" :items="['false', 'true']" label="排序 sort" data-testid="sort" hint="按节点名排序；默认保持来源顺序" persistent-hint />
          <v-btn color="primary" :loading="loading" data-testid="submit" class="mt-2" @click="submit">转换</v-btn>
          <div v-if="urls.trim() === '' && text.trim() === ''" class="text-caption text-medium-emphasis mt-2">
            至少要给一个来源：URL 与文本不能同时为空。
          </div>
        </v-card>
      </v-col>

      <v-col cols="12" md="6">
        <AsyncSection :loading="loading" :error="error" :empty="result === null ? (help.emptyState ?? false) : false" empty-text="结果将显示在这里" @retry="submit">
          <v-textarea :model-value="result?.body ?? ''" readonly rows="12" data-testid="result-body" />
          <div v-if="result !== null" class="mt-2" data-testid="result-summary">
            <div>生效格式：{{ result.format }}</div>
            <div>节点 {{ result.nodes }}（原始 {{ result.rawNodes }}）</div>
            <div>缓存命中 {{ result.cached }}</div>
          </div>
          <div v-if="result !== null" class="mt-2 d-flex gap-2">
            <v-btn variant="text" data-testid="copy" @click="copy">{{ copied ? '已复制' : '复制' }}</v-btn>
            <v-btn variant="text" data-testid="download" @click="download">下载</v-btn>
          </div>
          <div v-if="result?.failures && result.failures.length" data-testid="convert-failures">
            <div class="text-subtitle-2">失败</div>
            <div class="text-caption text-medium-emphasis">失败来源的原因；其余来源的节点照常输出——部分失败不影响整次转换</div>
            <ul><li v-for="(f, i) in result.failures" :key="i">{{ f }}</li></ul>
          </div>
          <div v-else-if="result !== null" data-testid="convert-failures-ok" class="text-caption text-success">全部来源成功</div>
          <div v-if="result?.warnings && result.warnings.length" data-testid="convert-warnings">
            <div class="text-subtitle-2">告警</div>
            <div class="text-caption text-medium-emphasis">渲染阶段跳过或降级的项 + 模板/管线/覆盖告警；不影响产物正确性，但静默丢弃没人会发现</div>
            <ul><li v-for="(w, i) in result.warnings" :key="i">{{ w }}</li></ul>
          </div>
        </AsyncSection>
      </v-col>
    </v-row>

    <v-card class="pa-4 mt-4" data-testid="diff-card">
      <div class="text-subtitle-1 mb-2">与上一次产物对比（diff）</div>
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
      <div class="text-caption text-medium-emphasis mt-2" data-testid="diff-help">
        「-」表示这行只在左边（基线）有，「+」表示只在右边（当前）有；顺序是「先删后增」，所以看到「-」紧跟「+」通常就是一行被改写了。
      </div>
      <div v-if="diffRan && diffLines.length === 0 && diffError === ''" data-testid="diff-empty" class="text-caption text-medium-emphasis mt-2">
        没有可对比的内容
      </div>
      <div v-if="diffLines.length > 0" data-testid="diff-result" class="mt-2">
        <div v-for="(ln, i) in diffLines" :key="i" :class="lineClass(ln.type)" :data-testid="'diff-line-' + i" class="diff-line">
          <span class="diff-type">{{ ln.type }}</span>{{ ln.text }}
        </div>
      </div>
    </v-card>
  </div>
</template>

<style scoped>
.target-meanings-toggle { cursor: pointer; user-select: none; }
.diff-line { font-family: monospace; white-space: pre; }
.diff-type { display: inline-block; width: 1.2em; }
.line-same { color: grey; }
.line-del { color: #d32f2f; }
.line-add { color: #2e7d32; }
</style>
