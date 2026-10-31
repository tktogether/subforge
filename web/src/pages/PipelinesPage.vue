<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { endpoints, type Pipeline, type PipelineInput, type PipelineStep, type NodePreviewResult, type Script } from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import { pageHelp } from '../content/help'
import { PIPELINE_HELP } from '../content/help'
import PageHeader from '../components/PageHeader.vue'
import { stepArgRef } from './pipelineStepArgs'
import AsyncSection from '../components/AsyncSection.vue'

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
  }))
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

const confirmOpen = ref(false)
const pendingDelete = ref<Pipeline | null>(null)

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

const help = pageHelp('pipelines')

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

// W7D 改动 3：脚本 kind 只有 filter / operate 两种取值
// （internal/api/scripts_test_api.go:59 `if kind != "filter" && kind != "operate"`；
//  web/src/content/help.ts:367、web/src/pages/ScriptsPage.vue:107）。
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
    await load()
  } catch (err) {
    saveError.value = errorMessage(err)
  } finally {
    saving.value = false
  }
}

function askDelete(p: Pipeline): void {
  pendingDelete.value = p
  confirmOpen.value = true
}

async function doDelete(): Promise<void> {
  if (pendingDelete.value === null) return
  try {
    await endpoints.deletePipeline(pendingDelete.value.id)
    confirmOpen.value = false
    pendingDelete.value = null
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
  <PageHeader :title="help.title" :subtitle="help.oneLiner" help-key="pipelines">
    <template #actions>
      <v-btn color="primary" prepend-icon="mdi-plus" data-testid="pipeline-new" @click="openCreate">新建管线</v-btn>
      <v-btn variant="tonal" prepend-icon="mdi-refresh" data-testid="pipeline-refresh" @click="load">刷新</v-btn>
    </template>
  </PageHeader>

  <AsyncSection :loading="loading" :error="error" :empty="pipelines.length === 0 ? (help.emptyState ?? false) : false" empty-text="暂无管线" @retry="load">
    <v-table data-testid="pipeline-table" class="pipeline-table sf-table-rows">
      <thead>
        <tr>
          <th>名称</th>
          <th>步骤数</th>
          <th>来源</th>
          <th>状态</th>
          <th class="text-right">操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="p in pipelines" :key="p.id" data-testid="pipeline-row">
          <td>
            <div class="pipeline-name">{{ p.name }}</div>
            <div v-if="p.description" class="pipeline-desc">{{ p.description }}</div>
          </td>
          <td class="pipeline-steps">{{ parseSteps(p.steps_json).length }}</td>
          <td><span class="pipeline-source">{{ p.source === 'ui' ? '界面创建' : '模板（只读）' }}</span></td>
          <td>
            <span class="sf-status">
              <span class="sf-status__dot" :class="p.enabled ? 'sf-status__dot--ok' : 'sf-status__dot--muted'"></span>
              <span>{{ p.enabled ? '启用' : '停用' }}</span>
            </span>
          </td>
          <td class="sf-row-actions">
            <v-btn size="small" variant="text" data-testid="pipeline-test" @click="openTest(p)">试运行</v-btn>
            <v-btn size="small" variant="text" data-testid="pipeline-edit" :disabled="p.read_only" @click="openEdit(p)">编辑</v-btn>
            <v-btn size="small" variant="text" color="error" data-testid="pipeline-delete" :disabled="p.read_only" @click="askDelete(p)">删除</v-btn>
          </td>
        </tr>
      </tbody>
    </v-table>
  </AsyncSection>

  <v-dialog v-model="dialog" max-width="760" scrollable>
    <v-card>
      <v-card-title>{{ editing === null ? '新建管线' : '编辑管线' }}</v-card-title>
      <v-card-text class="pipeline-dialog__body">
        <div class="sf-form-section">
          <div class="sf-form-section__head">
            <span class="sf-form-section__title">基本信息</span>
          </div>
          <v-text-field v-model="formName" label="名称" variant="outlined" density="comfortable" hide-details class="mb-3" data-testid="pipeline-name" />
          <v-text-field v-model="formDescription" label="说明" variant="outlined" density="comfortable" hide-details class="mb-3" data-testid="pipeline-description" />
          <v-switch v-model="formEnabled" label="启用此管线" color="primary" hide-details data-testid="pipeline-enabled" />
        </div>

        <div class="sf-form-section">
          <div class="sf-form-section__head">
            <span class="sf-form-section__title">处理步骤</span>
            <span class="sf-form-section__caption">按从上到下的顺序执行</span>
            <v-spacer />
            <v-btn size="small" variant="tonal" prepend-icon="mdi-plus" data-testid="step-add" @click="addStep">添加步骤</v-btn>
          </div>

          <div v-if="formSteps.length === 0" class="step-empty" data-testid="step-empty">
            还没有步骤。点击「添加步骤」，选择过滤或改写操作。
          </div>

          <div v-for="(step, i) in formSteps" :key="i" data-testid="step-row" class="step-card">
            <div class="step-card__head">
              <span class="step-index">{{ i + 1 }}</span>
              <span class="step-title">{{ stepLabel(step.op) }}</span>
              <v-spacer />
              <v-btn icon="mdi-arrow-up" size="small" variant="text" :disabled="i === 0" title="上移" aria-label="上移" data-testid="step-up" @click="moveStep(i, -1)" />
              <v-btn icon="mdi-arrow-down" size="small" variant="text" :disabled="i === formSteps.length - 1" title="下移" aria-label="下移" data-testid="step-down" @click="moveStep(i, 1)" />
              <v-btn icon="mdi-delete-outline" size="small" variant="text" color="error" title="删除步骤" aria-label="删除步骤" data-testid="step-remove" @click="removeStep(i)" />
            </div>
            <div class="step-card__body">
              <v-select
                v-model="step.op"
                :items="PIPELINE_HELP.steps.map((s) => ({ title: s.label, value: s.kind }))"
                item-title="title"
                item-value="value"
                label="步骤类型"
                variant="outlined"
                density="comfortable"
                hide-details
                class="mb-3"
                data-testid="step-op"
              />
              <div v-if="paramFields(step.op).length > 0" class="sf-form-grid">
                <template v-for="f in paramFields(step.op)" :key="f">
                  <v-select
                    v-if="f === 'by'"
                    v-model="step.args.by"
                    :items="['original', 'name', 'type', 'server', 'port']"
                    label="排序字段"
                    variant="outlined"
                    density="comfortable"
                    hide-details
                    class="mb-3"
                    data-testid="step-arg-by"
                  />
                  <v-select
                    v-else-if="f === 'ref'"
                    :model-value="step.args.ref ?? ''"
                    :items="scriptRefItems(step)"
                    item-title="title"
                    item-value="value"
                    :label="ARG_LABELS[f]"
                    :hint="paramHint(step.op, f)"
                    variant="outlined"
                    density="comfortable"
                    persistent-hint
                    clearable
                    class="mb-3"
                    data-testid="step-arg-ref"
                    @update:model-value="(v: unknown) => { step.args.ref = stepArgRef(v) }"
                  />
                  <v-text-field
                    v-else
                    v-model="step.args[f]"
                    :label="ARG_LABELS[f] ?? f"
                    :hint="paramHint(step.op, f)"
                    variant="outlined"
                    density="comfortable"
                    persistent-hint
                    class="mb-3"
                    data-testid="step-arg"
                  />
                </template>
              </div>
              <v-select
                v-model="step.on_error"
                :items="ON_ERROR_OPTIONS"
                item-title="title"
                item-value="value"
                label="失败处理"
                variant="outlined"
                density="comfortable"
                hide-details
                class="mb-1"
                data-testid="step-on-error"
              />
              <div class="step-note">
                跳过：跳过该步骤并继续；中止：终止本次转换；告警：记录告警并继续。默认中止。
              </div>
              <template v-if="step.op === 'filter.script' || step.op === 'op.script'">
                <v-alert v-if="scriptRefStatus(step) === 'ok'" type="success" variant="tonal" density="compact" class="mt-2" data-testid="script-ref-ok">{{ scriptRefOkText(step) }}</v-alert>
                <v-alert v-if="scriptRefStatus(step) === 'disabled'" type="warning" variant="tonal" density="compact" class="mt-2" data-testid="script-ref-disabled">脚本已存在但未启用</v-alert>
                <v-alert v-if="scriptRefStatus(step) === 'missing'" type="error" variant="tonal" density="compact" class="mt-2" data-testid="script-ref-missing">未找到该脚本</v-alert>
                <v-alert v-if="scriptRefKindMismatch(step) !== ''" type="warning" variant="tonal" density="compact" class="mt-2" data-testid="script-ref-kind-mismatch">{{ scriptRefKindMismatch(step) }}</v-alert>
              </template>
            </div>
          </div>
        </div>
        <div v-if="saveError !== ''" class="text-error mt-2" data-testid="pipeline-save-error">{{ saveError }}</div>
      </v-card-text>
      <v-card-actions>
        <v-btn variant="tonal" prepend-icon="mdi-play" data-testid="pipeline-test-draft" @click="openDraftTest">试运行</v-btn>
        <v-spacer />
        <v-btn variant="text" data-testid="pipeline-cancel" @click="dialog = false">取消</v-btn>
        <v-btn color="primary" :loading="saving" data-testid="pipeline-save" @click="save">保存</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-dialog v-model="testDialog" max-width="760" scrollable data-testid="pipeline-test-dialog">
    <v-card>
      <v-card-title>试运行管线 · {{ testName }}</v-card-title>
      <v-card-text>
        <div class="sf-form-section">
          <div class="sf-form-section__head">
            <span class="sf-form-section__title">测试节点（JSON 数组）</span>
            <span class="sf-form-section__caption">默认提供香港/日本/美国三个样例，可直接修改</span>
          </div>
          <v-textarea v-model="testNodesText" rows="8" variant="outlined" density="comfortable" hide-details spellcheck="false" class="code-input" data-testid="pipeline-test-nodes" />
          <div class="mt-3">
            <v-btn color="primary" prepend-icon="mdi-play" :loading="testLoading" data-testid="pipeline-test-run" @click="runPipelineTest">运行</v-btn>
          </div>
        </div>
        <v-alert v-if="testError !== ''" type="error" variant="tonal" class="mt-3" data-testid="pipeline-test-error">{{ testError }}</v-alert>
        <template v-if="testResult !== null">
          <div class="preview-summary mt-3" data-testid="pipeline-test-summary">
            <span>处理前 <strong>{{ testResult.before_count }}</strong> 个</span>
            <v-icon size="16" icon="mdi-arrow-right" />
            <span>处理后 <strong>{{ testResult.after_count }}</strong> 个</span>
          </div>
          <v-table density="compact" class="sf-table-rows mt-2" data-testid="pipeline-test-result">
            <thead><tr><th>名称</th><th>类型</th><th>服务器</th></tr></thead>
            <tbody>
              <tr v-for="(n, i) in testResult.after" :key="i">
                <td>{{ n.name }}</td><td>{{ n.type }}</td><td>{{ n.server }}:{{ n.port }}</td>
              </tr>
            </tbody>
          </v-table>
          <v-alert v-if="testResult.warnings && testResult.warnings.length > 0" type="warning" variant="tonal" class="mt-3" data-testid="pipeline-test-warnings">
            <div v-for="(w, i) in testResult.warnings" :key="i">{{ w }}</div>
          </v-alert>
        </template>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" data-testid="pipeline-test-close" @click="testDialog = false">关闭</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-dialog v-model="confirmOpen" max-width="480" data-testid="pipeline-confirm-dialog">
    <v-card>
      <v-card-title>确认删除</v-card-title>
      <v-card-text>删除后，引用该管线的订阅将不再应用这些处理步骤。确定删除？</v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" data-testid="pipeline-confirm-cancel" @click="confirmOpen = false">取消</v-btn>
        <v-btn color="error" data-testid="pipeline-confirm-delete" @click="doDelete">删除</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<style scoped>
.code-input :deep(textarea) {
  font-family: var(--sf-font-mono);
  font-size: 12px;
  line-height: 1.6;
}
.preview-summary {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--sf-text-muted);
}
.preview-summary strong {
  color: var(--sf-text);
}
.pipeline-table :deep(tbody td) {
  height: 48px;
}
.pipeline-name {
  font-weight: 500;
}
.pipeline-desc {
  max-width: 520px;
  margin-top: 2px;
  overflow: hidden;
  color: var(--sf-text-muted);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.pipeline-steps {
  font-variant-numeric: tabular-nums;
}
.pipeline-source {
  color: var(--sf-text-muted);
  font-size: 12px;
}
.step-empty {
  padding: 20px 12px;
  border: 1px dashed var(--sf-border);
  border-radius: 8px;
  color: var(--sf-text-muted);
  font-size: 13px;
  text-align: center;
}
.step-card {
  margin-bottom: 12px;
  border: 1px solid var(--sf-border);
  border-radius: 8px;
  background: var(--sf-surface);
  overflow: hidden;
}
.step-card__head {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px 6px 12px;
  border-bottom: 1px solid var(--sf-border);
  background: var(--sf-surface-raised);
}
.step-index {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: var(--sf-accent);
  color: var(--sf-accent-fg);
  font-size: 12px;
  font-weight: 600;
  flex: 0 0 auto;
}
.step-title {
  font-size: 13px;
  font-weight: 600;
}
.step-card__body {
  padding: 12px;
}
.step-note {
  margin: -4px 0 0;
  color: var(--sf-text-muted);
  font-size: 12px;
  line-height: 1.5;
}
</style>
