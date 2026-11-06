<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  endpoints,
  type Pipeline,
  type PipelineInput,
  type PipelineStep,
  type NodePreviewResult,
  type Script,
} from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import { notify } from '../composables/useNotify'
import { confirmAction } from '../composables/useConfirm'
import { PIPELINE_HELP } from '../content/help'
import { stepArgRef } from './pipelineStepArgs'
import SfPageHead from '../components/ui/SfPageHead.vue'
import SfField from '../components/ui/SfField.vue'
import SfSwitch from '../components/ui/SfSwitch.vue'
import SfDialog from '../components/ui/SfDialog.vue'
import SfNote from '../components/ui/SfNote.vue'
import SfEmpty from '../components/ui/SfEmpty.vue'
import {
  IconPlus,
  IconRefresh,
  IconPlayerPlay,
  IconPencil,
  IconTrash,
  IconArrowUp,
  IconArrowDown,
  IconArrowRight,
  IconAlertTriangle,
} from '@tabler/icons-vue'

const loading = ref(false)
const error = ref('')
const pipelines = ref<Pipeline[]>([])

const scripts = ref<Script[]>([])
const scriptsIndex = computed(() => {
  const m = new Map<string, boolean>()
  for (const s of scripts.value) m.set(s.name, s.enabled)
  return m
})

// 脚本引用只能从已保存脚本里选；标签带类型便于区分过滤/改写脚本。
const scriptOptions = computed(() =>
  scripts.value.map((s) => ({
    title: s.name + ' · ' + (s.kind === 'filter' ? '过滤' : s.kind === 'operate' ? '改写' : '未标注类型') + (s.enabled ? '' : '（已停用）'),
    value: s.name,
  })),
)

function scriptRefItems(step: { args: Record<string, string> }): { title: string; value: string }[] {
  const items = [...scriptOptions.value]
  const current = (step.args.ref ?? '').trim()
  if (current !== '' && !items.some((i) => i.value === current)) {
    items.unshift({ title: current + '（未找到）', value: current })
  }
  return items
}

const dialog = ref(false)
const editing = ref<Pipeline | null>(null)
const formName = ref('')
const formDescription = ref('')
const formEnabled = ref(true)
const formSteps = ref<{ op: string; args: Record<string, string>; on_error: string }[]>([])
const saving = ref(false)
const saveError = ref('')

const DEFAULT_TEST_NODES = JSON.stringify([
  { name: '香港 01', type: 'ss', server: 'hk.example.com', port: 8388 },
  { name: '日本 01', type: 'vmess', server: 'jp.example.com', port: 443 },
  { name: '美国 01', type: 'trojan', server: 'us.example.com', port: 443 },
], null, 2)

const testDialog = ref(false)
const testName = ref('')
const testSteps = ref<{ op: string; args: Record<string, string>; on_error?: string }[]>([])
const testNodesText = ref(DEFAULT_TEST_NODES)
const testLoading = ref(false)
const testError = ref('')
const testResult = ref<NodePreviewResult | null>(null)

function openTest(p: Pipeline): void {
  testName.value = p.name
  testSteps.value = parseSteps(p.steps_json).map((s) => ({ op: s.op, args: s.args ?? {}, on_error: s.on_error ?? '' }))
  testNodesText.value = DEFAULT_TEST_NODES
  testError.value = ''
  testResult.value = null
  testDialog.value = true
}

function openDraftTest(): void {
  testName.value = formName.value.trim() || '未命名管线'
  testSteps.value = formSteps.value.map((s) => ({ op: s.op, args: { ...s.args }, on_error: s.on_error }))
  testNodesText.value = DEFAULT_TEST_NODES
  testError.value = ''
  testResult.value = null
  testDialog.value = true
}

async function runPipelineTest(): Promise<void> {
  testLoading.value = true
  testError.value = ''
  testResult.value = null
  try {
    let nodes: unknown
    try {
      nodes = JSON.parse(testNodesText.value)
    } catch {
      testError.value = '测试节点不是合法 JSON'
      return
    }
    testResult.value = await endpoints.testPipeline({ name: testName.value, steps: testSteps.value, nodes: nodes as unknown[] })
  } catch (err) {
    testError.value = errorMessage(err)
  } finally {
    testLoading.value = false
  }
}

function parseSteps(s: string): PipelineStep[] {
  try {
    return JSON.parse(s) as PipelineStep[]
  } catch {
    return []
  }
}

const paramFields = (op: string): string[] => {
  switch (op) {
    case 'filter.regex':
    case 'filter.region':
    case 'filter.type':
      return ['include', 'exclude']
    case 'filter.script':
    case 'op.script':
      return ['ref', 'src', 'timeout']
    case 'op.rename':
    case 'op.emoji':
      return ['pattern', 'repl']
    case 'op.split':
      return ['sep']
    case 'op.sort':
      return ['by']
    default:
      return []
  }
}

function paramHint(op: string, field: string): string {
  if (op === 'filter.script' || op === 'op.script') {
    if (field === 'ref') return '从已保存的脚本中选择；脚本需处于启用状态。'
    if (field === 'src') return '直接填写内联脚本代码；与脚本引用二选一。'
    if (field === 'timeout') return '单次执行超时（毫秒），留空使用默认上限。'
  }
  const step = PIPELINE_HELP.steps.find((x) => x.kind === op)
  return step ? step.hint : field
}

const ARG_LABELS: Record<string, string> = {
  include: '包含节点（正则）',
  exclude: '排除节点（正则）',
  pattern: '匹配正则',
  repl: '替换为',
  sep: '分隔符',
  src: '代码',
  ref: '脚本引用',
  timeout: '超时（毫秒）',
}

const ON_ERROR_OPTIONS = [
  { title: '跟随默认', value: '' },
  { title: '跳过', value: 'skip' },
  { title: '中止', value: 'abort' },
  { title: '告警', value: 'warn' },
]

function stepScriptRef(step: { op: string; args: Record<string, string> }): string {
  if (step.op !== 'filter.script' && step.op !== 'op.script') return ''
  return step.args.ref ?? ''
}

function scriptRefStatus(step: { op: string; args: Record<string, string> }): 'ok' | 'disabled' | 'missing' | '' {
  const refName = stepScriptRef(step)
  if (refName === '') return ''
  const enabled = scriptsIndex.value.get(refName)
  if (enabled === undefined) return 'missing'
  return enabled ? 'ok' : 'disabled'
}

// D-16：脚本 kind 可能缺失（老脚本 kind 为空串或接口没带这个字段）。
// 统一归一成空串，绝不把 undefined 印到界面上。
function scriptRefKind(step: { op: string; args: Record<string, string> }): string {
  const refName = stepScriptRef(step)
  if (refName === '') return ''
  const found = scripts.value.find((s) => s.name === refName)
  if (found === undefined) return ''
  const kind = (found as { kind?: string }).kind
  return typeof kind === 'string' ? kind.trim() : ''
}

// W7D 改动 3：脚本 kind 只有 filter / operate 两种取值。
// filter.script 需要 filter 脚本，op.script 需要 operate 脚本；引用未找到或 kind 未知时不给提示。
function scriptRefKindMismatch(step: { op: string; args: Record<string, string> }): string {
  if (step.op !== 'filter.script' && step.op !== 'op.script') return ''
  const refName = stepScriptRef(step)
  if (refName === '') return ''
  // 老脚本（D-01 迁移前入库）kind 为空串或缺失：无法判定类型，不做「可能不匹配」提示，
  // 否则所有历史脚本都会被误报。
  const kind = scriptRefKind(step)
  if (kind === '') return ''
  const want = step.op === 'filter.script' ? 'filter' : 'operate'
  if (kind === want) return ''
  return `脚本类型不匹配：当前为${kind === 'filter' ? '过滤' : '改写'}脚本，该步骤需要${want === 'filter' ? '过滤' : '改写'}脚本`
}

// D-16：kind 缺失时只留一条中性提示，不与「可能不匹配」同屏矛盾。
function scriptRefOkText(step: { op: string; args: Record<string, string> }): string {
  return scriptRefKind(step) === '' ? '脚本已启用（类型未标注）' : '脚本已启用'
}

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    pipelines.value = await endpoints.pipelines('')
    scripts.value = await endpoints.listScripts()
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

function openCreate(): void {
  editing.value = null
  formName.value = ''
  formDescription.value = ''
  formEnabled.value = true
  formSteps.value = []
  saveError.value = ''
  dialog.value = true
}

function openEdit(p: Pipeline): void {
  editing.value = p
  formName.value = p.name
  formDescription.value = p.description
  formEnabled.value = p.enabled
  const parsed = parseSteps(p.steps_json)
  formSteps.value = parsed.map((s) => ({
    op: s.op,
    args: s.args ?? {},
    on_error: s.on_error ?? '',
  }))
  saveError.value = ''
  dialog.value = true
}

function addStep(): void {
  formSteps.value.push({ op: PIPELINE_HELP.steps[0].kind, args: {}, on_error: '' })
}

function removeStep(i: number): void {
  formSteps.value.splice(i, 1)
}

function moveStep(i: number, dir: -1 | 1): void {
  const j = i + dir
  if (j < 0 || j >= formSteps.value.length) return
  const arr = formSteps.value
  const tmp = arr[i]
  arr[i] = arr[j]
  arr[j] = tmp
}

async function save(): Promise<void> {
  saving.value = true
  saveError.value = ''
  try {
    const body: PipelineInput = {
      name: formName.value,
      description: formDescription.value,
      steps: formSteps.value.map((s) => {
        const step: PipelineStep = { op: s.op, args: s.args }
        if (s.on_error !== '') step.on_error = s.on_error
        return step
      }),
      enabled: formEnabled.value,
    }
    if (editing.value === null) {
      await endpoints.createPipeline(body)
    } else {
      await endpoints.updatePipeline(editing.value.id, body)
    }
    dialog.value = false
    notify(editing.value === null ? '已新增管线' : '已更新管线')
    await load()
  } catch (err) {
    saveError.value = errorMessage(err)
  } finally {
    saving.value = false
  }
}

async function askDelete(p: Pipeline): Promise<void> {
  const ok = await confirmAction({
    title: '确认删除管线「' + p.name + '」？',
    body: '删除后，引用该管线的订阅将不再应用这些处理步骤。',
    confirmText: '删除',
    danger: true,
  })
  if (!ok) return
  try {
    await endpoints.deletePipeline(p.id)
    notify('已删除')
    await load()
  } catch (err) {
    error.value = errorMessage(err)
  }
}

function stepLabel(op: string): string {
  return PIPELINE_HELP.steps.find((s) => s.kind === op)?.label ?? op
}

onMounted(() => {
  void load()
})
</script>

<template>
  <div>
    <SfPageHead>
      <template #actions>
        <button class="sf-btn sf-btn--primary" type="button" data-testid="pipeline-new" @click="openCreate">
          <IconPlus :size="15" :stroke-width="2" />
          新建管线
        </button>
        <button class="sf-btn sf-btn--ghost" type="button" data-testid="pipeline-refresh" :disabled="loading" @click="load">
          <IconRefresh :size="15" :stroke-width="1.8" :class="{ 'pl-spin': loading }" />
          刷新
        </button>
      </template>
    </SfPageHead>

    <div v-if="error !== ''" class="sf-note sf-note--danger sf-rise" style="margin-bottom: 16px">
      <IconAlertTriangle :size="16" :stroke-width="1.8" class="sf-note__icon" />
      <span class="sf-note__body">{{ error }}</span>
      <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" @click="load">重试</button>
    </div>

    <div v-if="pipelines.length === 0 && !loading && error === ''" class="sf-panel">
      <SfEmpty title="暂无管线" desc="管线是一串按顺序执行的节点处理步骤；不需要处理时可以不建。">
        <template #action>
          <button class="sf-btn sf-btn--primary sf-btn--sm" type="button" @click="openCreate">
            <IconPlus :size="14" :stroke-width="2" />
            新建管线
          </button>
        </template>
      </SfEmpty>
    </div>

    <div v-else class="sf-table-wrap sf-rise sf-d1" data-testid="pipeline-table">
      <table class="sf-table">
        <thead>
          <tr>
            <th>名称</th>
            <th style="text-align: right">步骤数</th>
            <th>来源</th>
            <th>状态</th>
            <th style="text-align: right">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in pipelines" :key="p.id" data-testid="pipeline-row">
            <td>
              <div class="pl-name">{{ p.name }}</div>
              <div v-if="p.description" class="pl-desc">{{ p.description }}</div>
            </td>
            <td class="sf-table__num" style="text-align: right">{{ parseSteps(p.steps_json).length }}</td>
            <td><span class="sf-faint" style="font-size: 12px">{{ p.source === 'ui' ? '界面创建' : '模板（只读）' }}</span></td>
            <td>
              <span class="sf-status">
                <span class="sf-led" :class="p.enabled ? 'sf-led--ok' : 'sf-led--idle'" />
                <span>{{ p.enabled ? '启用' : '停用' }}</span>
              </span>
            </td>
            <td>
              <span class="sf-table__actions">
                <button class="sf-btn sf-btn--subtle sf-btn--sm" type="button" data-testid="pipeline-test" @click="openTest(p)">
                  <IconPlayerPlay :size="14" :stroke-width="1.8" />
                  试运行
                </button>
                <button class="sf-btn sf-btn--subtle sf-btn--icon sf-btn--sm" type="button" aria-label="编辑" data-testid="pipeline-edit" :disabled="p.read_only" @click="openEdit(p)">
                  <IconPencil :size="15" :stroke-width="1.7" />
                </button>
                <button class="sf-btn sf-btn--subtle sf-btn--icon sf-btn--sm sf-btn--del" type="button" aria-label="删除" data-testid="pipeline-delete" :disabled="p.read_only" @click="askDelete(p)">
                  <IconTrash :size="15" :stroke-width="1.7" />
                </button>
              </span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- 编辑管线 -->
    <SfDialog :open="dialog" :label="editing === null ? 'CREATE PIPELINE' : 'EDIT PIPELINE'" :title="editing === null ? '新建管线' : '编辑管线'" wide @close="dialog = false">
      <SfField label="名称">
        <input v-model="formName" class="sf-input" data-testid="pipeline-name" spellcheck="false" />
      </SfField>
      <SfField label="说明">
        <input v-model="formDescription" class="sf-input" data-testid="pipeline-description" spellcheck="false" />
      </SfField>
      <SfSwitch v-model="formEnabled" label="启用此管线" data-testid="pipeline-enabled" />

      <div class="pl-steps-head">
        <span class="sf-label-mono">STEPS / 处理步骤</span>
        <span class="sf-field__hint">按从上到下的顺序执行</span>
        <span class="sf-spacer" />
        <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" data-testid="step-add" @click="addStep">
          <IconPlus :size="14" :stroke-width="2" />
          添加步骤
        </button>
      </div>

      <div v-if="formSteps.length === 0" class="sf-note" data-testid="step-empty">
        还没有步骤。点击「添加步骤」，选择过滤或改写操作。
      </div>

      <div v-for="(step, i) in formSteps" :key="i" data-testid="step-row" class="pl-step">
        <div class="pl-step__head">
          <span class="pl-step__index">{{ String(i + 1).padStart(2, '0') }}</span>
          <span class="pl-step__title">{{ stepLabel(step.op) }}</span>
          <span class="sf-spacer" />
          <button class="sf-btn sf-btn--subtle sf-btn--icon sf-btn--sm" type="button" aria-label="上移" title="上移" data-testid="step-up" :disabled="i === 0" @click="moveStep(i, -1)">
            <IconArrowUp :size="15" :stroke-width="1.7" />
          </button>
          <button class="sf-btn sf-btn--subtle sf-btn--icon sf-btn--sm" type="button" aria-label="下移" title="下移" data-testid="step-down" :disabled="i === formSteps.length - 1" @click="moveStep(i, 1)">
            <IconArrowDown :size="15" :stroke-width="1.7" />
          </button>
          <button class="sf-btn sf-btn--subtle sf-btn--icon sf-btn--sm sf-btn--del" type="button" aria-label="删除步骤" title="删除步骤" data-testid="step-remove" @click="removeStep(i)">
            <IconTrash :size="15" :stroke-width="1.7" />
          </button>
        </div>
        <div class="pl-step__body">
          <div class="pl-step__grid">
            <SfField label="步骤类型">
              <span class="sf-select-wrap">
                <select v-model="step.op" class="sf-select" data-testid="step-op">
                  <option v-for="s in PIPELINE_HELP.steps" :key="s.kind" :value="s.kind">{{ s.label }}</option>
                </select>
              </span>
            </SfField>
            <SfField label="失败处理">
              <span class="sf-select-wrap">
                <select v-model="step.on_error" class="sf-select" data-testid="step-on-error">
                  <option v-for="opt in ON_ERROR_OPTIONS" :key="opt.value" :value="opt.value">{{ opt.title }}</option>
                </select>
              </span>
            </SfField>
          </div>
          <p class="sf-field__hint" style="margin: 0">跳过：跳过该步骤并继续；中止：终止本次转换；告警：记录告警并继续。默认中止。</p>
          <div v-if="paramFields(step.op).length > 0" class="pl-step__grid pl-step__grid--args">
            <template v-for="f in paramFields(step.op)" :key="f">
              <SfField v-if="f === 'by'" label="排序字段">
                <span class="sf-select-wrap">
                  <select v-model="step.args.by" class="sf-select" data-testid="step-arg-by">
                    <option v-for="v in ['original', 'name', 'type', 'server', 'port']" :key="v" :value="v">{{ v }}</option>
                  </select>
                </span>
              </SfField>
              <SfField v-else-if="f === 'ref'" :hint="paramHint(step.op, f)">
                <template #label><span class="sf-field__label">{{ ARG_LABELS[f] }}</span></template>
                <span class="sf-select-wrap">
                  <select class="sf-select" data-testid="step-arg-ref" :value="step.args.ref ?? ''" @change="(e) => { step.args.ref = stepArgRef((e.target as HTMLSelectElement).value) }">
                    <option value="">不引用</option>
                    <option v-for="item in scriptRefItems(step)" :key="item.value" :value="item.value">{{ item.title }}</option>
                  </select>
                </span>
              </SfField>
              <SfField v-else :label="ARG_LABELS[f] ?? f" :hint="paramHint(step.op, f)">
                <input v-model="step.args[f]" class="sf-input" :class="{ 'sf-input--mono': f === 'src' || f === 'include' || f === 'exclude' || f === 'pattern' || f === 'repl' || f === 'sep' }" data-testid="step-arg" spellcheck="false" />
              </SfField>
            </template>
          </div>
          <template v-if="step.op === 'filter.script' || step.op === 'op.script'">
            <SfNote v-if="scriptRefStatus(step) === 'ok'" tone="ok" data-testid="script-ref-ok">{{ scriptRefOkText(step) }}</SfNote>
            <SfNote v-if="scriptRefStatus(step) === 'disabled'" tone="warn" data-testid="script-ref-disabled">脚本已存在但未启用</SfNote>
            <SfNote v-if="scriptRefStatus(step) === 'missing'" tone="danger" data-testid="script-ref-missing">未找到该脚本</SfNote>
            <SfNote v-if="scriptRefKindMismatch(step) !== ''" tone="warn" data-testid="script-ref-kind-mismatch">{{ scriptRefKindMismatch(step) }}</SfNote>
          </template>
        </div>
      </div>

      <div v-if="saveError !== ''" class="sf-field__error" data-testid="pipeline-save-error">{{ saveError }}</div>

      <template #foot>
        <button class="sf-btn sf-btn--ghost" type="button" data-testid="pipeline-test-draft" @click="openDraftTest">
          <IconPlayerPlay :size="14" :stroke-width="1.8" />
          试运行
        </button>
        <span class="sf-spacer" />
        <button class="sf-btn sf-btn--subtle" type="button" data-testid="pipeline-cancel" @click="dialog = false">取消</button>
        <button class="sf-btn sf-btn--primary" type="button" data-testid="pipeline-save" :disabled="saving" @click="save">{{ saving ? '保存中…' : '保存管线' }}</button>
      </template>
    </SfDialog>

    <!-- 试运行 -->
    <SfDialog :open="testDialog" label="DRY RUN" :title="'试运行管线 · ' + testName" wide @close="testDialog = false">
      <SfField label="测试节点（JSON 数组）" hint="默认提供香港/日本/美国三个样例，可直接修改。">
        <textarea v-model="testNodesText" class="sf-textarea pl-code-input" rows="8" data-testid="pipeline-test-nodes" spellcheck="false"></textarea>
      </SfField>
      <div>
        <button class="sf-btn sf-btn--primary" type="button" data-testid="pipeline-test-run" :disabled="testLoading" @click="runPipelineTest">
          <IconPlayerPlay :size="15" :stroke-width="1.8" />
          {{ testLoading ? '运行中…' : '运行' }}
        </button>
      </div>
      <div v-if="testError !== ''" class="sf-note sf-note--danger" data-testid="pipeline-test-error">{{ testError }}</div>
      <template v-if="testResult !== null">
        <div class="pl-test-summary" data-testid="pipeline-test-summary">
          <span>处理前 <b>{{ testResult.before_count }}</b> 个</span>
          <IconArrowRight :size="15" :stroke-width="1.8" class="sf-faint" />
          <span>处理后 <b>{{ testResult.after_count }}</b> 个</span>
        </div>
        <div class="sf-table-wrap" data-testid="pipeline-test-result">
          <table class="sf-table">
            <thead><tr><th>名称</th><th>类型</th><th>服务器</th></tr></thead>
            <tbody>
              <tr v-for="(n, i) in testResult.after" :key="i">
                <td>{{ n.name }}</td><td class="sf-faint">{{ n.type }}</td><td class="sf-table__num">{{ n.server }}:{{ n.port }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <SfNote v-if="testResult.warnings !== undefined && testResult.warnings.length > 0" tone="warn" data-testid="pipeline-test-warnings">
          <span v-for="(w, i) in testResult.warnings" :key="i" style="display: block">{{ w }}</span>
        </SfNote>
      </template>
      <template #foot>
        <button class="sf-btn sf-btn--primary" type="button" data-testid="pipeline-test-close" @click="testDialog = false">关闭</button>
      </template>
    </SfDialog>
  </div>
</template>

<style scoped>
.pl-spin {
  animation: pl-rotate 0.9s linear infinite;
}

@keyframes pl-rotate {
  to {
    transform: rotate(360deg);
  }
}

.pl-name {
  font-weight: 600;
}

.pl-desc {
  max-width: 520px;
  margin-top: 2px;
  overflow: hidden;
  color: var(--sf-text-muted);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pl-steps-head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 6px;
}

.pl-step {
  border: 1px solid var(--sf-border);
  border-radius: var(--sf-radius);
  background: var(--sf-surface-dim);
  overflow: hidden;
}

.pl-step__head {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px 6px 12px;
  border-bottom: 1px solid var(--sf-border);
}

.pl-step__index {
  font: 500 10.5px/1 var(--sf-font-mono);
  color: var(--sf-accent);
  letter-spacing: 0.08em;
}

.pl-step__title {
  font-size: 13px;
  font-weight: 600;
}

.pl-step__body {
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.pl-step__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(190px, 1fr));
  gap: 10px;
}

.pl-code-input {
  font-family: var(--sf-font-code);
  font-size: 12px;
  line-height: 1.6;
}

.pl-test-summary {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 13px;
  color: var(--sf-text-muted);
}

.pl-test-summary b {
  font: 600 16px/1 var(--sf-font-display);
  color: var(--sf-text);
  font-variant-numeric: tabular-nums;
}
</style>
