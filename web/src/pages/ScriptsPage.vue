<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { endpoints, type Script, type ScriptTestResult } from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import { pageHelp, fieldHelp } from '../content/help'
import { SCRIPTS_HELP } from '../content/help'
import { formatDuration } from '../format'
import PageHeader from '../components/PageHeader.vue'
import AsyncSection from '../components/AsyncSection.vue'

const help = pageHelp('scripts')

const loading = ref(false)
const error = ref('')
const scripts = ref<Script[]>([])

const dialog = ref(false)
const editing = ref<Script | null>(null)
const formName = ref('')
const formKind = ref('filter')
const formCode = ref('')
const formEnabled = ref(true)
const exampleMenu = ref(false)
const saving = ref(false)
const saveError = ref('')

const testDialog = ref(false)
const testCode = ref('')
const testKind = ref('filter')
const testNodeText = ref('{"name":"ss-node","type":"ss","server":"1.2.3.4","port":8388}')
const testTimeout = ref(0)
const testResult = ref<ScriptTestResult | null>(null)
const testing = ref(false)

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
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
  formKind.value = 'filter'
  formCode.value = ''
  formEnabled.value = true
  saveError.value = ''
  dialog.value = true
}

function openEdit(s: Script): void {
  editing.value = s
  formName.value = s.name
  formKind.value = s.kind
  formCode.value = s.code
  formEnabled.value = s.enabled
  saveError.value = ''
  dialog.value = true
}

async function save(): Promise<void> {
  saving.value = true
  saveError.value = ''
  try {
    if (editing.value === null) {
      await endpoints.createScript({ name: formName.value, kind: formKind.value, code: formCode.value, enabled: formEnabled.value })
    } else {
      await endpoints.updateScript(editing.value.id, { name: formName.value, kind: formKind.value, code: formCode.value, enabled: formEnabled.value })
    }
    dialog.value = false
    await load()
  } catch (err) {
    saveError.value = errorMessage(err)
  } finally {
    saving.value = false
  }
}

async function remove(s: Script): Promise<void> {
  try {
    await endpoints.deleteScript(s.id)
    await load()
  } catch (err) {
    error.value = errorMessage(err)
  }
}

function openTest(s: Script): void {
  testCode.value = s.code
  testKind.value = s.kind
  testNodeText.value = '{"name":"ss-node","type":"ss","server":"1.2.3.4","port":8388}'
  testTimeout.value = 0
  testResult.value = null
  testDialog.value = true
}

function toggleExamples(): void {
  exampleMenu.value = !exampleMenu.value
}

function onInsertExample(ex: { name: string; kind: 'filter' | 'operate'; code: string }): void {
  formKind.value = ex.kind
  formCode.value = ex.code
}

function onExampleSelect(name: string): void {
  const ex = SCRIPTS_HELP.examples.find((e) => e.name === name)
  if (ex) {
    onInsertExample(ex)
  }
}

async function runTest(): Promise<void> {
  if (testTimeout.value > 30000) {
    testResult.value = { ok: false, kept: false, before: {}, after: {}, duration_ms: 0, error: 'timeout 超过 30000ms 上限' }
    return
  }
  testing.value = true
  testResult.value = null
  let node: Record<string, unknown> | undefined
  try {
    node = JSON.parse(testNodeText.value) as Record<string, unknown>
  } catch {
    testResult.value = { ok: false, kept: false, before: {}, after: {}, duration_ms: 0, error: '节点 JSON 解析失败' }
    testing.value = false
    return
  }
  try {
    testResult.value = await endpoints.testScript({ code: testCode.value, kind: testKind.value, node, timeout: testTimeout.value || undefined })
  } catch (err) {
    testResult.value = { ok: false, kept: false, before: {}, after: {}, duration_ms: 0, error: errorMessage(err) }
  } finally {
    testing.value = false
  }
}

onMounted(() => {
  void load()
})
</script>

<template>
  <PageHeader :title="help.title" :subtitle="help.oneLiner" help-key="scripts">
    <template #actions>
      <v-btn color="primary" data-testid="new-script" @click="openCreate">新建脚本</v-btn>
      <v-btn variant="tonal" @click="load">刷新</v-btn>
    </template>
  </PageHeader>

  <div data-testid="scripts-sandbox" class="mb-4">
    <v-alert type="warning" variant="tonal" class="mb-2">脚本在服务端 goja 沙箱执行，请只运行可信代码；默认由 script.enabled 关闭。</v-alert>
    <div v-for="(s, i) in SCRIPTS_HELP.sandbox" :key="i" data-testid="scripts-sandbox-item" class="text-body-2 mb-1">{{ s }}</div>
  </div>

  <div data-testid="scripts-variables" class="mb-4">
    <div class="text-subtitle-1 mb-2">变量</div>
    <table>
      <thead><tr><th>变量</th><th>类型</th><th>可写</th><th>说明</th></tr></thead>
      <tbody>
        <tr v-for="v in SCRIPTS_HELP.variables" :key="v.name" data-testid="scripts-variable-row">
          <td>{{ v.name }}</td>
          <td>{{ v.type }}</td>
          <td>{{ v.writable ? '可写' : '只读' }}</td>
          <td>{{ v.explain }}</td>
        </tr>
      </tbody>
    </table>
  </div>

  <div data-testid="scripts-return-rules" class="mb-4">
    <div v-for="r in SCRIPTS_HELP.returnRules" :key="r.kind" class="mb-3">
      <div class="text-subtitle-1">{{ r.kind }}</div>
      <div>脚本体：{{ r.body }}</div>
      <div>返回值：{{ r.ret }}</div>
      <div>生效：{{ r.how }}</div>
      <div data-testid="return-good">正确示例：{{ r.good }}</div>
      <div data-testid="return-bad">错误示例：{{ r.bad }}</div>
    </div>
  </div>

  <div data-testid="scripts-writeback" class="mb-4">
    <div v-for="(w, i) in SCRIPTS_HELP.writeback" :key="i" class="text-body-2 mb-1">{{ w }}</div>
  </div>

  <AsyncSection :loading="loading" :error="error" :empty="scripts.length === 0 ? (help.emptyState ?? false) : false" :empty-text="'暂无脚本'" @retry="load">
    <v-list data-testid="scripts-list">
      <v-list-item v-for="s in scripts" :key="s.id" :title="s.name" :subtitle="s.kind">
        <template #append>
          <v-btn size="small" variant="text" @click="openEdit(s)">编辑</v-btn>
          <v-btn size="small" variant="text" @click="openTest(s)">试运行</v-btn>
          <v-btn size="small" variant="text" color="error" @click="remove(s)">删除</v-btn>
        </template>
      </v-list-item>
    </v-list>
  </AsyncSection>

  <v-dialog v-model="dialog" max-width="720">
    <v-card>
      <v-card-title>{{ editing === null ? '新建脚本' : '编辑脚本' }}</v-card-title>
      <v-card-text>
        <v-text-field v-model="formName" label="名称" class="mb-2" :hint="fieldHelp('script.name')?.hint" persistent-hint data-testid="field-hint" />
        <v-select v-model="formKind" :items="['filter', 'operate']" label="类型" class="mb-2" />
        <div class="text-body-2 mb-2">{{ SCRIPTS_HELP.returnRules.find((r) => r.kind === formKind)?.body }}</div>
        <v-textarea v-model="formCode" label="代码" rows="8" />
        <v-btn data-testid="example-insert" variant="tonal" class="mb-2" @click="toggleExamples">插入示例</v-btn>
        <div v-if="exampleMenu">
          <div v-for="ex in SCRIPTS_HELP.examples" :key="ex.name" data-testid="example-item" @click="onExampleSelect(ex.name)">
            <div>{{ ex.name }}</div>
            <div class="text-caption">{{ ex.why }}</div>
            <div v-if="ex.caution" data-testid="example-caution" class="text-caption text-warning">{{ ex.caution }}</div>
          </div>
        </div>
        <v-switch v-model="formEnabled" label="启用" />
        <div v-if="saveError !== ''" class="text-error mt-2">{{ saveError }}</div>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="dialog = false">取消</v-btn>
        <v-btn color="primary" :loading="saving" @click="save">保存</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-dialog v-model="testDialog" max-width="720">
    <v-card>
      <v-card-title>试运行脚本</v-card-title>
      <v-card-text>
        <div data-testid="scripts-test">
          <v-select v-model="testKind" :items="['filter', 'operate']" label="类型" class="mb-2" />
          <v-textarea v-model="testCode" label="代码" rows="8" class="mb-2" />
          <v-textarea v-model="testNodeText" label="节点 JSON" rows="4" class="mb-2" />
          <v-text-field v-model.number="testTimeout" label="超时毫秒" type="number" class="mb-2" />
          <v-btn color="primary" :loading="testing" @click="runTest">运行</v-btn>
        </div>
        <div v-if="testResult !== null" data-testid="test-result" class="mt-4">
          <div v-if="testResult.error" data-testid="test-error" class="text-error">{{ testResult.error }}</div>
          <div v-if="testResult.ok && testKind === 'filter'">
            <span v-if="testResult.kept === true" data-testid="test-kept">保留 ✅</span>
            <span v-else data-testid="test-dropped">丢弃 ⛔</span>
          </div>
          <div v-if="testResult.ok && testKind === 'operate'">
            <div>变更字段</div>
            <div v-for="(val, key) in (testResult.after ?? {})" :key="key">
              {{ key }}: {{ JSON.stringify((testResult.before ?? {})[key]) }} → {{ val === null ? '已删除' : JSON.stringify(val) }}
            </div>
          </div>
          <div>{{ testResult.ok ? '成功' : '失败' }}（{{ testResult.duration_ms }}ms）</div>
        <div class="mt-2">耗时：{{ formatDuration(testResult.duration_ms) }}</div>
        </div>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="testDialog = false">关闭</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
