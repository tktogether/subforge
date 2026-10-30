<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { endpoints, type Script, type ScriptTestResult } from '../api/endpoints'
import { errorDetail, errorMessage } from '../composables/useAsync'
import { pageHelp, fieldHelp } from '../content/help'
import { SCRIPTS_HELP } from '../content/help'
import { formatDuration } from '../format'
import PageHeader from '../components/PageHeader.vue'
import AsyncSection from '../components/AsyncSection.vue'

const help = pageHelp('scripts')

const loading = ref(false)
const error = ref('')
// errorDetail 存原始报文（服务端内部错误等），只进折叠块，主文案仍是可读的一句话。
const errorDetailText = ref('')
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
// saveErrorDetail 存保存失败时的原始报文，供折叠详情展示（D-22）。
const saveErrorDetail = ref('')

// D-21：删除是破坏性操作，先记住待删脚本并弹确认框，确认后才真正删。
const confirmOpen = ref(false)
const pendingDelete = ref<Script | null>(null)

const testDialog = ref(false)
const testCode = ref('')
const testKind = ref('filter')
const testNodeText = ref('{"name":"ss-node","type":"ss","server":"1.2.3.4","port":8388}')
const testTimeout = ref(0)
const testResult = ref<ScriptTestResult | null>(null)
const testing = ref(false)
const referenceOpen = ref(false)

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  errorDetailText.value = ''
  try {
    scripts.value = await endpoints.listScripts()
  } catch (err) {
    error.value = errorMessage(err)
    errorDetailText.value = errorDetail(err)
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
  exampleMenu.value = false
  saveError.value = ''
  dialog.value = true
}

function openEdit(s: Script): void {
  editing.value = s
  formName.value = s.name
  formKind.value = s.kind
  formCode.value = s.code
  formEnabled.value = s.enabled
  exampleMenu.value = false
  saveError.value = ''
  dialog.value = true
}

async function save(): Promise<void> {
  saving.value = true
  saveError.value = ''
  saveErrorDetail.value = ''
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
    saveErrorDetail.value = errorDetail(err)
  } finally {
    saving.value = false
  }
}

// askDelete 不直接删，把目标记下来并弹确认框，防止误点（D-21）。
function askDelete(s: Script): void {
  pendingDelete.value = s
  confirmOpen.value = true
}

async function doDelete(): Promise<void> {
  const target = pendingDelete.value
  if (target === null) return
  try {
    await endpoints.deleteScript(target.id)
    confirmOpen.value = false
    pendingDelete.value = null
    await load()
  } catch (err) {
    error.value = errorMessage(err)
    errorDetailText.value = errorDetail(err)
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

// D-15：点「插入示例」展开示例列表；选中某个示例后才替换代码与类型。
// 旧实现会在展开时直接写入第一个示例，用户还没选择就被改了代码。
function toggleExamples(): void {
  exampleMenu.value = !exampleMenu.value
}

function onInsertExample(ex: { name: string; kind: 'filter' | 'operate'; code: string }): void {
  formKind.value = ex.kind
  formCode.value = ex.code
  exampleMenu.value = false
}

function onExampleSelect(name: string): void {
  const ex = SCRIPTS_HELP.examples.find((e) => e.name === name)
  if (ex) {
    onInsertExample(ex)
  }
}

function kindText(kind: string): string {
  if (kind === 'filter') return '过滤脚本'
  if (kind === 'operate') return '改写脚本'
  return '未标注类型'
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
      <v-btn variant="tonal" prepend-icon="mdi-book-open-variant" data-testid="scripts-reference" @click="referenceOpen = true">脚本参考</v-btn>
      <v-btn color="primary" prepend-icon="mdi-plus" data-testid="new-script" @click="openCreate">新建脚本</v-btn>
      <v-btn variant="tonal" prepend-icon="mdi-refresh" @click="load">刷新</v-btn>
    </template>
  </PageHeader>

  <div class="script-note">
    <v-icon size="18" icon="mdi-shield-check-outline" />
    <span>脚本在服务端沙箱中执行，请只运行可信代码。</span>
  </div>

  <v-dialog v-model="referenceOpen" max-width="760" scrollable data-testid="scripts-reference-dialog">
    <v-card>
      <v-card-title>脚本参考</v-card-title>
      <v-card-text>
        <p class="text-body-2 mb-3">{{ SCRIPTS_HELP.intro }}</p>
        <div class="text-subtitle-2 mb-1">运行环境</div>
        <ul class="help-list mb-4" data-testid="scripts-sandbox">
          <li v-for="(s, i) in SCRIPTS_HELP.sandbox" :key="i" data-testid="scripts-sandbox-item">{{ s }}</li>
        </ul>
        <div class="text-subtitle-2 mb-1">可用变量</div>
        <div data-testid="scripts-variables">
          <v-table density="compact" class="mb-4 sf-table-rows">
            <thead><tr><th>变量</th><th>类型</th><th>可写</th><th>说明</th></tr></thead>
            <tbody>
              <tr v-for="v in SCRIPTS_HELP.variables" :key="v.name" data-testid="scripts-variable-row">
                <td><code>{{ v.name }}</code></td>
                <td>{{ v.type }}</td>
                <td>{{ v.writable ? '可写' : '只读' }}</td>
                <td>{{ v.explain }}</td>
              </tr>
            </tbody>
          </v-table>
        </div>
        <div class="text-subtitle-2 mb-1">返回值规则</div>
        <div data-testid="scripts-return-rules">
          <div v-for="r in SCRIPTS_HELP.returnRules" :key="r.kind" class="return-card">
            <div class="return-card__head">
              <span class="return-card__title">{{ r.kind === 'filter' ? '过滤脚本' : '改写脚本' }}</span>
              <v-chip size="x-small" variant="tonal">{{ r.body }}</v-chip>
            </div>
            <div class="return-row"><span class="return-label">返回值</span><span>{{ r.ret }}</span></div>
            <div class="return-row"><span class="return-label">生效方式</span><span>{{ r.how }}</span></div>
            <div class="return-code return-code--ok" data-testid="return-good">正确示例：{{ r.good }}</div>
            <div class="return-code return-code--bad" data-testid="return-bad">错误示例：{{ r.bad }}</div>
          </div>
        </div>
        <div class="text-subtitle-2 mb-1">字段写回</div>
        <ul class="help-list mb-0" data-testid="scripts-writeback">
          <li v-for="(w, i) in SCRIPTS_HELP.writeback" :key="i">{{ w }}</li>
        </ul>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" data-testid="scripts-reference-close" @click="referenceOpen = false">关闭</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <AsyncSection
    :loading="loading"
    :error="error"
    :error-detail="errorDetailText"
    :empty="scripts.length === 0 ? (help.emptyState ?? false) : false"
    :empty-text="'暂无脚本'"
    @retry="load"
  >
    <v-table data-testid="scripts-list" class="scripts-table sf-table-rows">
      <thead>
        <tr>
          <th>名称</th>
          <th>类型</th>
          <th>状态</th>
          <th class="text-right">操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="s in scripts" :key="s.id" data-testid="script-row">
          <td>
            <div class="script-name">{{ s.name }}</div>
          </td>
          <td><span class="script-kind">{{ kindText(s.kind) }}</span></td>
          <td>
            <span class="sf-status">
              <span class="sf-status__dot" :class="s.enabled ? 'sf-status__dot--ok' : 'sf-status__dot--muted'"></span>
              <span>{{ s.enabled ? '启用' : '停用' }}</span>
            </span>
          </td>
          <td class="sf-row-actions">
            <v-btn size="small" variant="text" data-testid="script-edit" @click="openEdit(s)">编辑</v-btn>
            <v-btn size="small" variant="text" data-testid="script-test" @click="openTest(s)">试运行</v-btn>
            <v-btn size="small" variant="text" color="error" data-testid="script-delete" @click="askDelete(s)">删除</v-btn>
          </td>
        </tr>
      </tbody>
    </v-table>
  </AsyncSection>

  <!-- D-21：删除前二次确认，对齐机场/管线已有模式。 -->
  <v-dialog v-model="confirmOpen" max-width="480">
    <v-card>
      <v-card-title>确认删除</v-card-title>
      <v-card-text>
        删除脚本「{{ pendingDelete?.name ?? '' }}」后无法恢复，确定继续？
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" data-testid="script-confirm-cancel" @click="confirmOpen = false">取消</v-btn>
        <v-btn color="error" data-testid="script-confirm-delete" @click="doDelete">删除</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-dialog v-model="dialog" max-width="760" scrollable>
    <v-card>
      <v-card-title>{{ editing === null ? '新建脚本' : '编辑脚本' }}</v-card-title>
      <v-card-text class="script-dialog__body">
        <div class="sf-form-section">
          <div class="sf-form-section__head">
            <span class="sf-form-section__title">基本信息</span>
          </div>
          <div class="sf-form-grid">
            <v-text-field
              v-model="formName"
              label="名称"
              :hint="fieldHelp('script.name')?.hint"
              variant="outlined"
              density="comfortable"
              persistent-hint
              data-testid="field-hint"
            />
            <v-select
              v-model="formKind"
              :items="[{ title: '过滤脚本', value: 'filter' }, { title: '改写脚本', value: 'operate' }]"
              label="类型"
              variant="outlined"
              density="comfortable"
              hide-details
            />
          </div>
          <div class="script-kind-hint">
            <v-icon size="16" icon="mdi-information-outline" />
            <span>{{ SCRIPTS_HELP.returnRules.find((r) => r.kind === formKind)?.body }}：{{ SCRIPTS_HELP.returnRules.find((r) => r.kind === formKind)?.ret }}</span>
          </div>
        </div>

        <div class="sf-form-section">
          <div class="sf-form-section__head">
            <span class="sf-form-section__title">脚本代码</span>
            <span class="sf-form-section__caption">只提供 node 变量，单次执行最长 5 秒</span>
          </div>
          <v-textarea
            v-model="formCode"
            label="代码"
            rows="10"
            variant="outlined"
            density="comfortable"
            hide-details
            spellcheck="false"
            class="code-input"
            data-testid="script-code"
          />
          <div class="example-toolbar">
            <v-btn data-testid="example-insert" variant="tonal" size="small" prepend-icon="mdi-code-tags" @click="toggleExamples">插入示例</v-btn>
            <span class="sf-form-section__caption">选择一个示例会替换当前代码和类型</span>
          </div>
          <div v-if="exampleMenu" class="example-panel" data-testid="example-menu">
            <button
              v-for="ex in SCRIPTS_HELP.examples"
              :key="ex.name"
              type="button"
              class="example-option"
              data-testid="example-item"
              @click="onExampleSelect(ex.name)"
            >
              <span class="example-option__head">
                <span class="example-option__name">{{ ex.name }}</span>
                <v-chip size="x-small" variant="tonal">{{ ex.kind === 'filter' ? '过滤' : '改写' }}</v-chip>
              </span>
              <span class="example-option__why">{{ ex.why }}</span>
              <span v-if="ex.caution" data-testid="example-caution" class="example-option__caution">{{ ex.caution }}</span>
            </button>
          </div>
        </div>

        <div class="sf-form-section">
          <div class="sf-form-section__head">
            <span class="sf-form-section__title">状态</span>
          </div>
          <v-switch v-model="formEnabled" label="启用此脚本" color="primary" hide-details />
        </div>

        <div v-if="saveError !== ''" class="text-error mt-2" data-testid="script-save-error">{{ saveError }}</div>
        <!-- D-22：主文案是可读的一句话，原始报文进折叠块。 -->
        <details v-if="saveErrorDetail !== ''" data-testid="script-save-error-detail" class="mt-1">
          <summary class="text-body-2 text-medium-emphasis">查看错误详情</summary>
          <pre class="text-caption">{{ saveErrorDetail }}</pre>
        </details>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="dialog = false">取消</v-btn>
        <v-btn color="primary" :loading="saving" @click="save">保存</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-dialog v-model="testDialog" max-width="760" scrollable>
    <v-card>
      <v-card-title>试运行脚本</v-card-title>
      <v-card-text class="script-dialog__body">
        <div data-testid="scripts-test">
          <div class="sf-form-section">
            <div class="sf-form-section__head">
              <span class="sf-form-section__title">运行参数</span>
              <span class="sf-form-section__caption">使用示例节点测试脚本行为，不会改动已保存的脚本</span>
            </div>
            <div class="sf-form-grid">
              <v-select v-model="testKind" :items="[{ title: '过滤脚本', value: 'filter' }, { title: '改写脚本', value: 'operate' }]" label="类型" variant="outlined" density="comfortable" hide-details class="mb-3" />
              <v-text-field v-model.number="testTimeout" label="超时（毫秒）" type="number" variant="outlined" density="comfortable" hide-details class="mb-3" />
            </div>
            <v-textarea v-model="testCode" label="代码" rows="8" variant="outlined" density="comfortable" hide-details spellcheck="false" class="code-input mb-3" />
            <v-textarea v-model="testNodeText" label="节点 JSON" rows="4" variant="outlined" density="comfortable" hide-details spellcheck="false" class="code-input" />
          </div>

          <div class="sf-form-section">
            <div class="sf-form-section__head">
              <span class="sf-form-section__title">运行结果</span>
              <v-spacer />
              <v-btn color="primary" :loading="testing" prepend-icon="mdi-play" @click="runTest">运行</v-btn>
            </div>
            <div v-if="testResult === null" class="result-empty" data-testid="test-result-empty">
              点击「运行」后，这里会显示是否保留节点、字段变化与耗时。
            </div>
            <!-- D-23：结果区固定最大高度并可滚动，长结果不再被对话框底边裁切。 -->
            <div
              v-else
              data-testid="test-result"
              class="test-result-scroll result-panel"
            >
              <div class="result-panel__head">
                <v-chip :color="testResult.error ? 'error' : 'success'" size="small" variant="tonal" data-testid="test-status">
                  {{ testResult.error ? '失败' : '成功' }}
                </v-chip>
                <span class="text-caption text-medium-emphasis">耗时：{{ formatDuration(testResult.duration_ms) }}</span>
              </div>
              <div v-if="testResult.error" data-testid="test-error" class="text-error">{{ testResult.error }}</div>
              <div v-if="testResult.ok && testKind === 'filter'" class="mt-2">
                <span v-if="testResult.kept === true" data-testid="test-kept" class="text-success">保留</span>
                <span v-else data-testid="test-dropped" class="text-medium-emphasis">丢弃</span>
              </div>
              <div v-if="testResult.ok && testKind === 'operate'" class="mt-2">
                <div class="text-subtitle-2 mb-1">变更字段</div>
                <div v-for="(val, key) in (testResult.after ?? {})" :key="key" class="result-change">
                  <code>{{ key }}</code>：{{ JSON.stringify((testResult.before ?? {})[key]) }} → {{ val === null ? '已删除' : JSON.stringify(val) }}
                </div>
                <div v-if="Object.keys(testResult.after ?? {}).length === 0" class="text-caption text-medium-emphasis">本次执行没有修改任何字段。</div>
              </div>
            </div>
          </div>
        </div>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" data-testid="test-close" @click="testDialog = false">关闭</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<style scoped>
.help-list {
  padding-left: 20px;
}
.script-note {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
  padding: 8px 12px;
  border-left: 3px solid var(--sf-accent);
  border-radius: 0 6px 6px 0;
  background: var(--sf-surface-raised);
  color: var(--sf-text-muted);
  font-size: 13px;
}
.scripts-table :deep(tbody td) {
  height: 48px;
}
.script-name {
  font-weight: 500;
}
.script-kind {
  color: var(--sf-text-muted);
  font-size: 13px;
}
.script-dialog__body {
  max-height: 72vh;
  overflow-y: auto;
  scrollbar-gutter: stable;
}
.script-kind-hint {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  margin-top: 10px;
  padding: 8px 10px;
  border-radius: 6px;
  background: var(--sf-surface-raised);
  color: var(--sf-text-muted);
  font-size: 12px;
  line-height: 1.6;
}
.code-input :deep(textarea) {
  font-family: var(--sf-font-mono);
  font-size: 12px;
  line-height: 1.6;
}
.example-toolbar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 10px;
}
.example-panel {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  margin-top: 10px;
}
@media (max-width: 700px) {
  .example-panel {
    grid-template-columns: minmax(0, 1fr);
  }
}
.example-option {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
  padding: 10px 12px;
  border: 1px solid var(--sf-border);
  border-radius: 8px;
  background: var(--sf-surface);
  color: var(--sf-text);
  text-align: left;
  cursor: pointer;
  transition: border-color var(--sf-dur-micro), background var(--sf-dur-micro);
}
.example-option:hover {
  border-color: var(--sf-accent);
  background: var(--sf-surface-raised);
}
.example-option:focus-visible {
  outline: 2px solid var(--sf-accent);
  outline-offset: 2px;
}
.example-option__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  width: 100%;
}
.example-option__name {
  font-size: 13px;
  font-weight: 600;
}
.example-option__why {
  color: var(--sf-text-muted);
  font-size: 12px;
  line-height: 1.5;
}
.example-option__caution {
  color: var(--sf-warn);
  font-size: 12px;
}
.result-empty {
  padding: 18px 12px;
  border: 1px dashed var(--sf-border);
  border-radius: 8px;
  background: var(--sf-surface-raised);
  color: var(--sf-text-muted);
  font-size: 13px;
  text-align: center;
}
.result-panel {
  padding: 12px;
  border: 1px solid var(--sf-border);
  border-radius: 8px;
  background: var(--sf-surface);
}
.test-result-scroll {
  max-height: 40vh;
  overflow-y: auto;
  overflow-wrap: anywhere;
}
.result-panel__head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
}
.result-change {
  padding: 4px 0;
  border-bottom: 1px solid var(--sf-border);
  font-family: var(--sf-font-mono);
  font-size: 12px;
}
.result-change:last-child {
  border-bottom: 0;
}
.return-card {
  margin-bottom: 10px;
  padding: 10px 12px;
  border: 1px solid var(--sf-border);
  border-radius: 8px;
  background: var(--sf-surface);
}
.return-card__head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}
.return-card__title {
  font-size: 13px;
  font-weight: 600;
}
.return-row {
  display: flex;
  gap: 10px;
  margin-bottom: 4px;
  font-size: 13px;
}
.return-label {
  flex: 0 0 64px;
  color: var(--sf-text-muted);
}
.return-code {
  margin-top: 6px;
  padding: 6px 8px;
  border-radius: 6px;
  font-family: var(--sf-font-mono);
  font-size: 12px;
  overflow-wrap: anywhere;
}
.return-code--ok {
  background: color-mix(in srgb, var(--sf-ok) 10%, transparent);
  color: var(--sf-ok);
}
.return-code--bad {
  background: color-mix(in srgb, var(--sf-danger) 10%, transparent);
  color: var(--sf-danger);
}
</style>
