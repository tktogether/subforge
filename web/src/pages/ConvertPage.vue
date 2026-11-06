<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import {
  endpoints,
  type ConvertResponse,
  type Pipeline,
  type DiffLine,
} from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import { copyText } from '../utils/clipboard'
import SfPageHead from '../components/ui/SfPageHead.vue'
import SfField from '../components/ui/SfField.vue'
import SfSwitch from '../components/ui/SfSwitch.vue'
import SfNote from '../components/ui/SfNote.vue'
import SfEmpty from '../components/ui/SfEmpty.vue'
import SfInfoTip from '../components/ui/SfInfoTip.vue'
import SfCodeBlock from '../components/ui/SfCodeBlock.vue'
import SfDialog from '../components/ui/SfDialog.vue'
import { IconPlayerPlay, IconCopy, IconCheck, IconDownload, IconFileCode, IconGitCompare } from '@tabler/icons-vue'

const targets = ref<string[]>([])
const pipelines = ref<Pipeline[]>([])
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
  copied.value = await copyText(result.value.body)
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
})

onUnmounted(() => {
  if (objectUrl.value !== null) {
    URL.revokeObjectURL(objectUrl.value)
    objectUrl.value = null
  }
})
</script>

<template>
  <div>
    <SfPageHead>
      <template #actions>
        <button class="sf-btn sf-btn--ghost" type="button" data-testid="target-meanings-toggle" @click="targetHelpOpen = true">
          目标格式说明
        </button>
      </template>
    </SfPageHead>

    <div class="cv-workbench sf-rise sf-d1">
      <!-- 左：参数台 -->
      <section class="sf-panel" data-testid="convert-input-card">
        <div class="sf-panel__head">
          <span class="sf-label-mono">PARAMS / 转换参数</span>
        </div>
        <div class="sf-panel__body cv-form">
          <div class="cv-section">
            <div class="cv-section__head">
              <span class="cv-section__title">输入来源</span>
              <span class="cv-section__caption">订阅地址每行一个；两者至少填写一项。</span>
            </div>
            <SfField label="订阅地址（每行一个）">
              <textarea v-model="urls" class="sf-textarea sf-input--mono" rows="3" data-testid="urls" spellcheck="false" placeholder="https://example.com/sub"></textarea>
            </SfField>
            <SfField label="粘贴文本" hint="整段分享链接或 Base64，会写入临时文件后解析。">
              <textarea v-model="text" class="sf-textarea sf-input--mono" rows="2" data-testid="text" spellcheck="false"></textarea>
            </SfField>
          </div>

          <div class="cv-section">
            <div class="cv-section__head">
              <span class="cv-section__title">筛选条件</span>
              <span class="cv-section__caption">留空表示不筛选</span>
            </div>
            <div class="cv-trio">
              <SfField label="协议类型">
                <template #label><span class="sf-field__label">协议类型 <SfInfoTip text="只保留指定协议类型；逗号分隔，留空表示不过滤。示例：ss,vmess" /></span></template>
                <input v-model="types" class="sf-input sf-input--mono" data-testid="types" spellcheck="false" />
              </SfField>
              <SfField>
                <template #label><span class="sf-field__label">包含节点 <SfInfoTip text="按节点名称匹配的逗号分隔正则；匹配的节点才会保留。" /></span></template>
                <input v-model="include" class="sf-input sf-input--mono" data-testid="include" spellcheck="false" />
              </SfField>
              <SfField>
                <template #label><span class="sf-field__label">排除节点 <SfInfoTip text="按节点名称匹配的逗号分隔正则；匹配的节点会被移除。" /></span></template>
                <input v-model="exclude" class="sf-input sf-input--mono" data-testid="exclude" spellcheck="false" />
              </SfField>
            </div>
          </div>

          <div class="cv-section">
            <div class="cv-section__head">
              <span class="cv-section__title">输出设置</span>
            </div>
            <div class="cv-duo">
              <SfField>
                <template #label><span class="sf-field__label">目标格式 <SfInfoTip text="留空时使用 clash.meta。" /></span></template>
                <span class="sf-select-wrap">
                  <select v-model="selectedTarget" class="sf-select" data-testid="target-select">
                    <option value="">默认（clash.meta）</option>
                    <option v-for="t in targets" :key="t" :value="t">{{ t }}</option>
                  </select>
                </span>
              </SfField>
              <SfField>
                <template #label><span class="sf-field__label">管线 <SfInfoTip text="在节点进入渲染前执行的处理步骤；留空表示不使用管线。" /></span></template>
                <span class="sf-select-wrap">
                  <select v-model="pipeline" class="sf-select" data-testid="pipeline">
                    <option value="">不使用</option>
                    <option v-for="p in pipelines" :key="p.id" :value="p.name">{{ p.name }}{{ p.source === 'template' ? ' · 只读' : '' }}</option>
                  </select>
                </span>
              </SfField>
              <SfField>
                <template #label><span class="sf-field__label">转换模板 <SfInfoTip text="渲染使用的配置模板；留空表示使用默认模板。" /></span></template>
                <span class="sf-select-wrap">
                  <select v-model="configValue" class="sf-select" data-testid="config">
                    <option value="">默认模板</option>
                    <option v-for="c in configs" :key="c" :value="c">{{ c }}</option>
                  </select>
                </span>
              </SfField>
              <SfField label="节点排序">
                <span class="sf-select-wrap">
                  <select v-model="sort" class="sf-select" data-testid="sort">
                    <option value="false">保持来源顺序</option>
                    <option value="true">按节点名称排序</option>
                  </select>
                </span>
              </SfField>
            </div>
            <div class="cv-list-row">
              <div>
                <span class="sf-field__label">仅输出节点列表 <SfInfoTip text="只输出节点分享链接列表，不生成完整配置。" /></span>
                <div class="sf-field__hint">不生成完整配置，只输出分享链接</div>
              </div>
              <SfSwitch v-model="list" data-testid="list" />
            </div>
          </div>

          <div class="cv-actions">
            <button class="sf-btn sf-btn--primary" type="button" data-testid="submit" :disabled="loading" @click="submit">
              <IconPlayerPlay :size="15" :stroke-width="1.8" />
              {{ loading ? '转换中…' : '执行转换' }}
            </button>
            <span v-if="urls.trim() === '' && text.trim() === ''" class="cv-actions__note">
              订阅地址与粘贴文本不能同时为空。
            </span>
          </div>
        </div>
      </section>

      <!-- 右：产物输出 -->
      <section class="sf-panel sf-panel--ticks" data-testid="convert-result-card">
        <div class="sf-panel__head">
          <span class="sf-label-mono">OUTPUT / 转换结果</span>
          <span v-if="result !== null" class="sf-badge sf-badge--accent">{{ result.format }}</span>
          <span class="sf-spacer" />
          <template v-if="result !== null">
            <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" data-testid="copy" @click="copy">
              <IconCheck v-if="copied" :size="14" :stroke-width="2" style="color: var(--sf-ok)" />
              <IconCopy v-else :size="14" :stroke-width="1.8" />
              {{ copied ? '已复制' : '复制' }}
            </button>
            <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" data-testid="download" @click="download">
              <IconDownload :size="14" :stroke-width="1.8" />
              下载
            </button>
          </template>
        </div>

        <SfEmpty
          v-if="result === null"
          title="还没有转换结果"
          desc="填写左侧参数后点击「执行转换」，产物会在这里输出。"
        >
          <template #icon><IconFileCode :size="30" :stroke-width="1.4" /></template>
        </SfEmpty>
        <template v-else>
          <div class="cv-result-summary" data-testid="result-summary">
            <div class="cv-result-summary__item">
              <span class="cv-result-summary__label">节点</span>
              <span class="cv-result-summary__value">{{ result.nodes }}</span>
            </div>
            <div class="cv-result-summary__item">
              <span class="cv-result-summary__label">原始节点</span>
              <span class="cv-result-summary__value">{{ result.rawNodes }}</span>
            </div>
            <div class="cv-result-summary__item">
              <span class="cv-result-summary__label">缓存命中</span>
              <span class="cv-result-summary__value">{{ result.cached }}</span>
            </div>
          </div>
          <SfCodeBlock :code="result.body" :label="filenameFor(result.format)" plain max-height="480px" class="cv-result-body" />
          <div style="padding: 0 18px 16px; display: flex; flex-direction: column; gap: 10px">
            <SfNote v-if="result.failures !== null && result.failures.length > 0" tone="danger" data-testid="convert-failures">
              <b>失败来源</b>——以下来源未成功获取；其他来源的节点仍会正常输出。
              <ul class="cv-note-list">
                <li v-for="(f, i) in result.failures" :key="i">{{ f }}</li>
              </ul>
            </SfNote>
            <SfNote v-else tone="ok" data-testid="convert-failures-ok">全部来源成功。</SfNote>
            <SfNote v-if="result.warnings !== null && result.warnings.length > 0" tone="warn" data-testid="convert-warnings">
              <b>告警</b>——以下内容被跳过或降级，不影响其他节点的输出。
              <ul class="cv-note-list">
                <li v-for="(w, i) in result.warnings" :key="i">{{ w }}</li>
              </ul>
            </SfNote>
          </div>
        </template>
      </section>
    </div>

    <div v-if="error !== ''" class="sf-note sf-note--danger sf-rise" style="margin-top: 16px">{{ error }}</div>

    <!-- 产物对比 -->
    <section class="sf-panel sf-rise sf-d2" data-testid="diff-card" style="margin-top: 16px">
      <div class="sf-panel__head">
        <span class="sf-panel__title">产物对比</span>
        <SfInfoTip text="「-」表示仅在左侧存在，「+」表示仅在右侧存在；同一行被修改时会先删除再新增。" />
      </div>
      <div class="sf-panel__body">
        <div class="cv-diff-grid">
          <SfField label="基线 A">
            <textarea v-model="diffA" class="sf-textarea sf-input--mono" rows="6" data-testid="diff-a" spellcheck="false"></textarea>
          </SfField>
          <SfField label="当前 B">
            <textarea v-model="diffB" class="sf-textarea sf-input--mono" rows="6" data-testid="diff-b" spellcheck="false"></textarea>
          </SfField>
        </div>
        <div class="cv-actions" style="margin-top: 14px">
          <button class="sf-btn sf-btn--primary" type="button" data-testid="diff-run" :disabled="diffLoading" @click="runDiff">
            <IconGitCompare :size="15" :stroke-width="1.8" />
            {{ diffLoading ? '对比中…' : '执行对比' }}
          </button>
          <span class="sf-field__hint">按行比较两份产物，红色为删除，绿色为新增。</span>
        </div>
        <div v-if="diffError !== ''" data-testid="diff-error" class="sf-field__error" style="margin-top: 10px">{{ diffError }}</div>
        <div v-if="diffRan && diffLines.length === 0 && diffError === ''" data-testid="diff-empty" class="sf-note" style="margin-top: 12px">
          没有可对比的内容。
        </div>
        <div v-if="diffLines.length > 0" data-testid="diff-result" class="cv-diff-panel">
          <div v-for="(ln, i) in diffLines" :key="i" :class="lineClass(ln.type)" :data-testid="'diff-line-' + i" class="cv-diff-line">
            <span class="cv-diff-type">{{ ln.type }}</span>{{ ln.text }}
          </div>
        </div>
      </div>
    </section>

    <!-- 目标格式说明 -->
    <SfDialog :open="targetHelpOpen" label="TARGETS" title="目标格式说明" wide @close="targetHelpOpen = false">
      <p class="cv-dialog-lead">订阅地址会根据客户端类型自动选择目标格式；在本页选择后以本页设置为准。</p>
      <div class="sf-table-wrap" data-testid="target-meanings">
        <table class="sf-table">
          <thead>
            <tr><th>取值</th><th>产物形态</th><th>适用客户端</th></tr>
          </thead>
          <tbody>
            <tr v-for="m in targetMeanings" :key="m.target">
              <td><code class="sf-mono">{{ m.target }}</code></td>
              <td>{{ m.form }}</td>
              <td class="sf-muted">{{ m.where }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <template #foot>
        <button class="sf-btn sf-btn--primary" type="button" data-testid="target-meanings-close" @click="targetHelpOpen = false">关闭</button>
      </template>
    </SfDialog>
  </div>
</template>

<style scoped>
.cv-workbench {
  display: grid;
  grid-template-columns: minmax(380px, 460px) minmax(0, 1fr);
  gap: 16px;
  align-items: start;
}

@media (max-width: 1100px) {
  .cv-workbench {
    grid-template-columns: minmax(0, 1fr);
  }
}

.cv-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.cv-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.cv-section__head {
  display: flex;
  align-items: baseline;
  gap: 10px;
  flex-wrap: wrap;
}

.cv-section__title {
  font: 650 13.5px/1.3 var(--sf-font-sans);
}

.cv-section__title::before {
  content: '▸ ';
  color: var(--sf-accent);
}

.cv-section__caption {
  font-size: 12px;
  color: var(--sf-text-faint);
}

.cv-trio {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
  gap: 10px;
}

.cv-duo {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(170px, 1fr));
  gap: 10px;
}

.cv-list-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 12px;
  border: 1px solid var(--sf-border);
  border-radius: var(--sf-radius);
  background: var(--sf-surface-dim);
}

.cv-actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  /* 吸底：长表单滚动时「执行转换」始终可见 */
  position: sticky;
  bottom: 0;
  z-index: 2;
  margin: 4px -18px -18px;
  padding: 12px 18px 16px;
  background: var(--sf-surface);
  border-top: 1px solid var(--sf-border);
  border-radius: 0 0 var(--sf-radius-lg) var(--sf-radius-lg);
}

.cv-actions__note {
  font-size: 12.5px;
  color: var(--sf-text-faint);
}

.cv-result-summary {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 1px;
  background: var(--sf-border);
  border-bottom: 1px solid var(--sf-border);
}

.cv-result-summary__item {
  background: var(--sf-surface-dim);
  padding: 10px 18px;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.cv-result-summary__label {
  font: 500 10px/1.2 var(--sf-font-mono);
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--sf-text-faint);
}

.cv-result-summary__value {
  font: 600 17px/1.2 var(--sf-font-display);
  font-variant-numeric: tabular-nums;
}

.cv-result-body {
  border: none;
  border-radius: 0;
}

.cv-note-list {
  margin: 6px 0 0;
  padding-left: 18px;
  line-height: 1.7;
}

.cv-dialog-lead {
  margin: 0;
  font-size: 13px;
  line-height: 1.7;
  color: var(--sf-text-muted);
}

.cv-diff-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 12px;
}

.cv-diff-panel {
  margin-top: 12px;
  border: 1px solid var(--sf-border);
  border-radius: var(--sf-radius);
  overflow: hidden;
  background: var(--sf-bg-deep);
  max-height: 420px;
  overflow-y: auto;
}

.cv-diff-line {
  padding: 2px 12px;
  font-family: var(--sf-font-code);
  font-size: 12px;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  line-height: 1.65;
}

.cv-diff-type {
  display: inline-block;
  width: 1.2em;
  color: var(--sf-text-faint);
}

.line-same {
  color: var(--sf-text-faint);
}

.line-del {
  color: var(--sf-danger);
  background: var(--sf-danger-dim);
}

.line-add {
  color: var(--sf-ok);
  background: var(--sf-ok-dim);
}
</style>
