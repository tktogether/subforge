<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { endpoints, type Pipeline, type PipelineInput, type PipelineStep, type Script } from '../api/endpoints'
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

// W7D：ref 参数的下拉数据源（可选已有脚本，也允许手输尚未创建的脚本名）。
const scriptNames = computed(() => scripts.value.map((s) => s.name))

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
  const step = PIPELINE_HELP.steps.find((x) => x.kind === op)
  return step ? step.hint : field
}

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

// W7D 改动 3：脚本 kind 只有 filter / operate 两种取值
// （internal/api/scripts_test_api.go:59 `if kind != "filter" && kind != "operate"`；
//  web/src/content/help.ts:367、web/src/pages/ScriptsPage.vue:107）。
// filter.script 需要 filter 脚本，op.script 需要 operate 脚本；引用未找到或 kind 未知时不给提示。
function scriptRefKindMismatch(step: { op: string; args: Record<string, string> }): string {
  if (step.op !== 'filter.script' && step.op !== 'op.script') return ''
  const refName = stepScriptRef(step)
  if (refName === '') return ''
  const found = scripts.value.find((s) => s.name === refName)
  if (found === undefined) return ''
  // 老脚本（D-01 迁移前入库）kind 为空串：无法判定类型，不做「可能不匹配」提示，
  // 否则所有历史脚本都会被误报。
  if (found.kind === '') return ''
  const want = step.op === 'filter.script' ? 'filter' : 'operate'
  if (found.kind === want) return ''
  return `类型可能不匹配：脚本 kind=${found.kind}，该步骤需要 ${want}`
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

onMounted(() => {
  void load()
})
</script>

<template>
  <PageHeader :title="help.title" :subtitle="help.oneLiner" help-key="pipelines">
    <template #actions>
      <v-btn color="primary" data-testid="pipeline-new" @click="openCreate">新建管线</v-btn>
      <v-btn variant="tonal" data-testid="pipeline-refresh" @click="load">刷新</v-btn>
    </template>
  </PageHeader>

  <div data-testid="pipeline-intro" class="text-body-2 mb-2">{{ PIPELINE_HELP.intro }}</div>
  <div data-testid="pipeline-order" class="mb-2">
    <div v-for="(o, i) in PIPELINE_HELP.order" :key="i" data-testid="pipeline-order-item" class="text-body-2">{{ o }}</div>
  </div>
  <div data-testid="pipeline-script-step" class="text-body-2 mb-4">{{ PIPELINE_HELP.scriptStep }}</div>

  <AsyncSection :loading="loading" :error="error" :empty="pipelines.length === 0 ? (help.emptyState ?? false) : false" empty-text="暂无管线" @retry="load">
    <v-table data-testid="pipeline-table">
      <thead>
        <tr>
          <th>名称</th>
          <th>步骤数</th>
          <th>来源</th>
          <th>启用</th>
          <th>操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="p in pipelines" :key="p.id" data-testid="pipeline-row">
          <td>{{ p.name }}</td>
          <td>{{ parseSteps(p.steps_json).length }}</td>
          <td>{{ p.source === 'ui' ? '界面' : '模板（只读）' }}</td>
          <td>{{ p.enabled ? '启用' : '禁用' }}</td>
          <td>
            <v-btn size="small" variant="text" data-testid="pipeline-edit" :disabled="p.read_only" @click="openEdit(p)">编辑</v-btn>
            <v-btn size="small" variant="text" color="error" data-testid="pipeline-delete" :disabled="p.read_only" @click="askDelete(p)">删除</v-btn>
          </td>
        </tr>
      </tbody>
    </v-table>
  </AsyncSection>

  <v-dialog v-model="dialog" max-width="720">
    <v-card>
      <v-card-title>{{ editing === null ? '新建管线' : '编辑管线' }}</v-card-title>
      <v-card-text>
        <v-text-field v-model="formName" label="名称" class="mb-2" data-testid="pipeline-name" />
        <v-text-field v-model="formDescription" label="说明" class="mb-2" data-testid="pipeline-description" />
        <v-switch v-model="formEnabled" label="启用" data-testid="pipeline-enabled" />
        <div class="mt-4">
          <div v-for="(step, i) in formSteps" :key="i" data-testid="step-row" class="mb-3 pa-2">
            <v-select
              v-model="step.op"
              :items="PIPELINE_HELP.steps.map((s) => ({ title: s.label, value: s.kind }))"
              item-title="title"
              item-value="value"
              label="op"
              class="mb-2"
              data-testid="step-op"
            />
            <template v-for="f in paramFields(step.op)" :key="f">
              <v-select
                v-if="f === 'by'"
                v-model="step.args.by"
                :items="['original', 'name', 'type', 'server', 'port']"
                label="by"
                class="mb-2"
                data-testid="step-arg-by"
              />
              <v-combobox
                v-else-if="f === 'ref'"
                :model-value="step.args.ref ?? ''"
                :items="scriptNames"
                :label="f + '（选择已有脚本，也可手输）'"
                :hint="paramHint(step.op, f)"
                persistent-hint
                class="mb-2"
                data-testid="step-arg-ref"
                @update:model-value="(v: unknown) => { step.args.ref = stepArgRef(v) }"
              />
              <v-text-field
                v-else
                v-model="step.args[f]"
                :label="f"
                :hint="paramHint(step.op, f)"
                class="mb-2"
                data-testid="step-arg"
              />
            </template>
            <v-select
              v-model="step.on_error"
              :items="['', 'abort', 'warn', 'skip']"
              item-title="title"
              item-value="value"

              label="on_error"
              class="mb-2"
              data-testid="step-on-error"
            />
            <div class="text-caption text-medium-emphasis mb-2">
              skip：跳过该步，保留已处理结果，产物继续生成；abort：中止整次转换并报错（默认）；warn：记录告警，继续执行后续步骤。不填就跟随段落默认。
            </div>
            <div v-if="step.op === 'filter.script' || step.op === 'op.script'">
              <div v-if="scriptRefStatus(step) === 'ok'" data-testid="script-ref-ok" class="text-success text-body-2">脚本已启用</div>
              <div v-if="scriptRefStatus(step) === 'disabled'" data-testid="script-ref-disabled" class="text-warning text-body-2">脚本已存在但未启用</div>
              <div v-if="scriptRefStatus(step) === 'missing'" data-testid="script-ref-missing" class="text-error text-body-2">未找到该脚本</div>
              <div v-if="scriptRefKindMismatch(step) !== ''" data-testid="script-ref-kind-mismatch" class="text-warning text-body-2">{{ scriptRefKindMismatch(step) }}</div>
            </div>
            <div class="d-flex ga-2">
              <v-btn size="small" variant="text" data-testid="step-up" @click="moveStep(i, -1)">上移</v-btn>
              <v-btn size="small" variant="text" data-testid="step-down" @click="moveStep(i, 1)">下移</v-btn>
              <v-btn size="small" variant="text" color="error" data-testid="step-remove" @click="removeStep(i)">删除</v-btn>
            </div>
          </div>
        </div>
        <v-btn data-testid="step-add" variant="tonal" @click="addStep">添加步骤</v-btn>
        <div v-if="saveError !== ''" class="text-error mt-2" data-testid="pipeline-save-error">{{ saveError }}</div>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" data-testid="pipeline-cancel" @click="dialog = false">取消</v-btn>
        <v-btn color="primary" :loading="saving" data-testid="pipeline-save" @click="save">保存</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-dialog v-model="confirmOpen" max-width="480">
    <v-card>
      <v-card-title>确认删除</v-card-title>
      <v-card-text>删除后引用它的订阅产物会退回默认步骤，确定继续？</v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" data-testid="pipeline-confirm-cancel" @click="confirmOpen = false">取消</v-btn>
        <v-btn color="error" data-testid="pipeline-confirm-delete" @click="doDelete">删除</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
