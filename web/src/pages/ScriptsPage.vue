<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { endpoints, type Script, type ScriptTestResult } from '../api/endpoints'
import { errorDetail, errorMessage } from '../composables/useAsync'
import { fieldHelp, SCRIPTS_HELP } from '../content/help'
import { formatDuration } from '../format'
import { notify } from '../composables/useNotify'
import { confirmAction } from '../composables/useConfirm'
import SfPageHead from '../components/ui/SfPageHead.vue'
import SfField from '../components/ui/SfField.vue'
import SfSwitch from '../components/ui/SfSwitch.vue'
import SfDialog from '../components/ui/SfDialog.vue'
import SfNote from '../components/ui/SfNote.vue'
import SfEmpty from '../components/ui/SfEmpty.vue'
import {
  IconBook2,
  IconPlus,
  IconRefresh,
  IconPencil,
  IconPlayerPlay,
  IconTrash,
  IconAlertTriangle,
  IconCode,
} from '@tabler/icons-vue'

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
    notify(editing.value === null ? '已新增脚本' : '已更新脚本')
    await load()
  } catch (err) {
    saveError.value = errorMessage(err)
    saveErrorDetail.value = errorDetail(err)
  } finally {
    saving.value = false
  }
}

// 删除是破坏性操作，先确认再删。
async function askDelete(s: Script): Promise<void> {
  const ok = await confirmAction({
    title: '确认删除脚本「' + s.name + '」？',
    body: '删除脚本后无法恢复，引用它的管线步骤会失效。',
    confirmText: '删除',
    danger: true,
  })
  if (!ok) return
  try {
    await endpoints.deleteScript(s.id)
    notify('已删除')
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

function onExampleSelect(name: string): void {
  const ex = SCRIPTS_HELP.examples.find((e) => e.name === name)
  if (ex) {
    formKind.value = ex.kind
    formCode.value = ex.code
    exampleMenu.value = false
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
  <div>
    <SfPageHead>
      <template #actions>
        <button class="sf-btn sf-btn--ghost" type="button" data-testid="scripts-reference" @click="referenceOpen = true">
          <IconBook2 :size="15" :stroke-width="1.7" />
          脚本参考
        </button>
        <button class="sf-btn sf-btn--primary" type="button" data-testid="new-script" @click="openCreate">
          <IconPlus :size="15" :stroke-width="2" />
          新建脚本
        </button>
        <button class="sf-btn sf-btn--ghost" type="button" :disabled="loading" @click="load">
          <IconRefresh :size="15" :stroke-width="1.8" :class="{ 'sc-spin': loading }" />
          刷新
        </button>
      </template>
    </SfPageHead>

    <SfNote tone="info" class="sf-rise sf-d1" style="margin-bottom: 16px">
      脚本在服务端沙箱中执行，请只运行可信代码。
    </SfNote>

    <div v-if="error !== ''" class="sf-note sf-note--danger sf-rise" style="margin-bottom: 16px">
      <IconAlertTriangle :size="16" :stroke-width="1.8" class="sf-note__icon" />
      <span class="sf-note__body">{{ error }}</span>
      <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" @click="load">重试</button>
    </div>

    <div v-if="scripts.length === 0 && !loading && error === ''" class="sf-panel">
      <SfEmpty title="暂无脚本" desc="脚本用于管线中的 filter.script / op.script 步骤；可以从「脚本参考」里的示例开始。">
        <template #icon><IconCode :size="30" :stroke-width="1.4" /></template>
        <template #action>
          <button class="sf-btn sf-btn--primary sf-btn--sm" type="button" @click="openCreate">
            <IconPlus :size="14" :stroke-width="2" />
            新建脚本
          </button>
        </template>
      </SfEmpty>
    </div>

    <div v-else class="sf-table-wrap sf-rise sf-d1" data-testid="scripts-list">
      <table class="sf-table">
        <thead>
          <tr>
            <th>名称</th>
            <th>类型</th>
            <th>状态</th>
            <th style="text-align: right">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="s in scripts" :key="s.id" data-testid="script-row">
            <td><span class="sc-name">{{ s.name }}</span></td>
            <td><span class="sf-faint" style="font-size: 13px">{{ kindText(s.kind) }}</span></td>
            <td>
              <span class="sf-status">
                <span class="sf-led" :class="s.enabled ? 'sf-led--ok' : 'sf-led--idle'" />
                <span>{{ s.enabled ? '启用' : '停用' }}</span>
              </span>
            </td>
            <td>
              <span class="sf-table__actions">
                <button class="sf-btn sf-btn--subtle sf-btn--sm" type="button" data-testid="script-test" @click="openTest(s)">
                  <IconPlayerPlay :size="14" :stroke-width="1.8" />
                  试运行
                </button>
                <button class="sf-btn sf-btn--subtle sf-btn--sm" type="button" data-testid="script-edit" @click="openEdit(s)">
                  <IconPencil :size="14" :stroke-width="1.7" />
                  编辑
                </button>
                <button class="sf-btn sf-btn--subtle sf-btn--icon sf-btn--sm sf-btn--del" type="button" aria-label="删除" data-testid="script-delete" @click="askDelete(s)">
                  <IconTrash :size="15" :stroke-width="1.7" />
                </button>
              </span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- 编辑脚本 -->
    <SfDialog :open="dialog" :label="editing === null ? 'CREATE SCRIPT' : 'EDIT SCRIPT'" :title="editing === null ? '新建脚本' : '编辑脚本'" wide @close="dialog = false">
      <div class="sc-duo">
        <SfField :hint="fieldHelp('script.name')?.hint ?? ''">
          <template #label><span class="sf-field__label">名称</span></template>
          <input v-model="formName" class="sf-input" data-testid="field-hint" spellcheck="false" />
        </SfField>
        <SfField label="类型">
          <span class="sf-select-wrap">
            <select v-model="formKind" class="sf-select">
              <option value="filter">过滤脚本</option>
              <option value="operate">改写脚本</option>
            </select>
          </span>
        </SfField>
      </div>
      <SfNote tone="info" class="sc-kind-hint">
        {{ SCRIPTS_HELP.returnRules.find((r) => r.kind === formKind)?.body }}：{{ SCRIPTS_HELP.returnRules.find((r) => r.kind === formKind)?.ret }}
      </SfNote>

      <SfField hint="只提供 node 变量，单次执行最长 5 秒。">
        <template #label><span class="sf-field__label">脚本代码</span></template>
        <textarea v-model="formCode" class="sf-textarea sc-code" rows="10" data-testid="script-code" spellcheck="false"></textarea>
      </SfField>

      <div class="sc-example-bar">
        <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" data-testid="example-insert" @click="toggleExamples">
          <IconCode :size="14" :stroke-width="1.8" />
          插入示例
        </button>
        <span class="sf-field__hint">选择一个示例会替换当前代码和类型</span>
      </div>
      <div v-if="exampleMenu" class="sc-examples" data-testid="example-menu">
        <button
          v-for="ex in SCRIPTS_HELP.examples"
          :key="ex.name"
          type="button"
          class="sc-example"
          data-testid="example-item"
          @click="onExampleSelect(ex.name)"
        >
          <span class="sc-example__head">
            <span class="sc-example__name">{{ ex.name }}</span>
            <span class="sf-badge" :class="ex.kind === 'filter' ? 'sf-badge--info' : 'sf-badge--accent'">{{ ex.kind === 'filter' ? '过滤' : '改写' }}</span>
          </span>
          <span class="sc-example__why">{{ ex.why }}</span>
          <span v-if="ex.caution" data-testid="example-caution" class="sc-example__caution">{{ ex.caution }}</span>
        </button>
      </div>

      <SfSwitch v-model="formEnabled" label="启用此脚本" />

      <div v-if="saveError !== ''" class="sf-field__error" data-testid="script-save-error">{{ saveError }}</div>
      <details v-if="saveErrorDetail !== ''" data-testid="script-save-error-detail" class="sc-detail">
        <summary>查看错误详情</summary>
        <pre>{{ saveErrorDetail }}</pre>
      </details>

      <template #foot>
        <span class="sf-spacer" />
        <button class="sf-btn sf-btn--subtle" type="button" @click="dialog = false">取消</button>
        <button class="sf-btn sf-btn--primary" type="button" :disabled="saving" @click="save">{{ saving ? '保存中…' : '保存脚本' }}</button>
      </template>
    </SfDialog>

    <!-- 试运行 -->
    <SfDialog :open="testDialog" label="DRY RUN" title="试运行脚本" wide @close="testDialog = false">
      <div data-testid="scripts-test">
        <div class="sc-duo">
          <SfField label="类型">
            <span class="sf-select-wrap">
              <select v-model="testKind" class="sf-select">
                <option value="filter">过滤脚本</option>
                <option value="operate">改写脚本</option>
              </select>
            </span>
          </SfField>
          <SfField label="超时（毫秒）">
            <input v-model.number="testTimeout" class="sf-input sf-table__num" type="number" min="0" />
          </SfField>
        </div>
        <SfField label="代码">
          <textarea v-model="testCode" class="sf-textarea sc-code" rows="8" spellcheck="false"></textarea>
        </SfField>
        <SfField label="节点 JSON">
          <textarea v-model="testNodeText" class="sf-textarea sc-code" rows="4" spellcheck="false"></textarea>
        </SfField>

        <div class="sc-test-head">
          <span class="sf-label-mono">RESULT / 运行结果</span>
          <span class="sf-spacer" />
          <button class="sf-btn sf-btn--primary sf-btn--sm" type="button" :disabled="testing" @click="runTest">
            <IconPlayerPlay :size="14" :stroke-width="1.8" />
            {{ testing ? '运行中…' : '运行' }}
          </button>
        </div>
        <div v-if="testResult === null" class="sf-note" data-testid="test-result-empty">
          点击「运行」后，这里会显示是否保留节点、字段变化与耗时。
        </div>
        <!-- D-23：结果区固定最大高度并可滚动，长结果不再被对话框底边裁切。 -->
        <div v-else data-testid="test-result" class="sc-result">
          <div class="sc-result__head">
            <span class="sf-badge" :class="testResult.error ? 'sf-badge--danger' : 'sf-badge--ok'" data-testid="test-status">
              {{ testResult.error ? '失败' : '成功' }}
            </span>
            <span class="sf-faint" style="font-size: 12px">耗时：{{ formatDuration(testResult.duration_ms) }}</span>
          </div>
          <div v-if="testResult.error" data-testid="test-error" class="sf-field__error">{{ testResult.error }}</div>
          <div v-if="testResult.ok && testKind === 'filter'">
            <span v-if="testResult.kept === true" data-testid="test-kept" class="sf-badge sf-badge--ok">保留</span>
            <span v-else data-testid="test-dropped" class="sf-badge">丢弃</span>
          </div>
          <div v-if="testResult.ok && testKind === 'operate'">
            <div class="sf-label-mono" style="margin-bottom: 8px">变更字段</div>
            <div v-for="(val, key) in testResult.after ?? {}" :key="key" class="sc-change">
              <code>{{ key }}</code>：{{ JSON.stringify((testResult.before ?? {})[key]) }} → {{ val === null ? '已删除' : JSON.stringify(val) }}
            </div>
            <div v-if="Object.keys(testResult.after ?? {}).length === 0" class="sf-faint" style="font-size: 12.5px">本次执行没有修改任何字段。</div>
          </div>
        </div>
      </div>
      <template #foot>
        <button class="sf-btn sf-btn--primary" type="button" data-testid="test-close" @click="testDialog = false">关闭</button>
      </template>
    </SfDialog>

    <!-- 脚本参考 -->
    <SfDialog :open="referenceOpen" label="REFERENCE" title="脚本参考" wide @close="referenceOpen = false">
      <p class="sc-ref-intro">{{ SCRIPTS_HELP.intro }}</p>
      <div class="sc-ref-block">
        <span class="sf-label-mono">运行环境</span>
        <ul class="sc-ref-list" data-testid="scripts-sandbox">
          <li v-for="(s, i) in SCRIPTS_HELP.sandbox" :key="i" data-testid="scripts-sandbox-item">{{ s }}</li>
        </ul>
      </div>
      <div class="sc-ref-block">
        <span class="sf-label-mono">可用变量</span>
        <div class="sf-table-wrap" data-testid="scripts-variables">
          <table class="sf-table">
            <thead><tr><th>变量</th><th>类型</th><th>可写</th><th>说明</th></tr></thead>
            <tbody>
              <tr v-for="v in SCRIPTS_HELP.variables" :key="v.name" data-testid="scripts-variable-row">
                <td><code class="sf-mono">{{ v.name }}</code></td>
                <td class="sf-faint">{{ v.type }}</td>
                <td>{{ v.writable ? '可写' : '只读' }}</td>
                <td class="sf-muted">{{ v.explain }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
      <div class="sc-ref-block" data-testid="scripts-return-rules">
        <span class="sf-label-mono">返回值规则</span>
        <div v-for="r in SCRIPTS_HELP.returnRules" :key="r.kind" class="sc-return">
          <div class="sc-return__head">
            <span class="sc-return__title">{{ r.kind === 'filter' ? '过滤脚本' : '改写脚本' }}</span>
            <span class="sf-badge sf-badge--accent">{{ r.body }}</span>
          </div>
          <div class="sc-return__row"><span>返回值</span><span>{{ r.ret }}</span></div>
          <div class="sc-return__row"><span>生效方式</span><span>{{ r.how }}</span></div>
          <div class="sc-return__code sc-return__code--ok" data-testid="return-good">正确示例：{{ r.good }}</div>
          <div class="sc-return__code sc-return__code--bad" data-testid="return-bad">错误示例：{{ r.bad }}</div>
        </div>
      </div>
      <div class="sc-ref-block">
        <span class="sf-label-mono">字段写回</span>
        <ul class="sc-ref-list" data-testid="scripts-writeback">
          <li v-for="(w, i) in SCRIPTS_HELP.writeback" :key="i">{{ w }}</li>
        </ul>
      </div>
      <template #foot>
        <button class="sf-btn sf-btn--primary" type="button" data-testid="scripts-reference-close" @click="referenceOpen = false">关闭</button>
      </template>
    </SfDialog>
  </div>
</template>

<style scoped>
.sc-spin {
  animation: sc-rotate 0.9s linear infinite;
}

@keyframes sc-rotate {
  to {
    transform: rotate(360deg);
  }
}

.sc-name {
  font-weight: 600;
}

.sc-duo {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 12px;
}

.sc-kind-hint {
  margin-top: -6px;
}

.sc-code {
  font-family: var(--sf-font-code);
  font-size: 12px;
  line-height: 1.65;
}

.sc-example-bar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}

.sc-examples {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

@media (max-width: 700px) {
  .sc-examples {
    grid-template-columns: minmax(0, 1fr);
  }
}

.sc-example {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
  padding: 10px 12px;
  border: 1px solid var(--sf-border);
  border-radius: var(--sf-radius);
  background: var(--sf-surface-dim);
  color: var(--sf-text);
  text-align: left;
  cursor: pointer;
  transition: border-color var(--sf-t-fast) var(--sf-ease), background var(--sf-t-fast) var(--sf-ease);
}

.sc-example:hover {
  border-color: var(--sf-accent-border);
  background: var(--sf-accent-dim);
}

.sc-example__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  width: 100%;
}

.sc-example__name {
  font-size: 13px;
  font-weight: 600;
}

.sc-example__why {
  color: var(--sf-text-muted);
  font-size: 12px;
  line-height: 1.5;
}

.sc-example__caution {
  color: var(--sf-warn);
  font-size: 12px;
}

.sc-detail {
  font-size: 12.5px;
  color: var(--sf-text-faint);
}

.sc-detail summary {
  cursor: pointer;
}

.sc-detail pre {
  margin: 8px 0 0;
  padding: 10px 12px;
  border: 1px solid var(--sf-border);
  border-radius: var(--sf-radius-sm);
  background: var(--sf-bg-deep);
  font-size: 11.5px;
  line-height: 1.6;
  overflow-wrap: anywhere;
  white-space: pre-wrap;
}

.sc-test-head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 4px;
}

.sc-result {
  border: 1px solid var(--sf-border);
  border-radius: var(--sf-radius);
  background: var(--sf-surface-dim);
  padding: 12px 14px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  max-height: 40vh;
  overflow-y: auto;
}

.sc-result__head {
  display: flex;
  align-items: center;
  gap: 10px;
}

.sc-change {
  padding: 5px 0;
  border-bottom: 1px solid oklch(from var(--sf-border) l c h / 0.55);
  font-family: var(--sf-font-code);
  font-size: 12px;
  overflow-wrap: anywhere;
}

.sc-change:last-child {
  border-bottom: 0;
}

.sc-ref-intro {
  margin: 0;
  font-size: 13px;
  line-height: 1.7;
  color: var(--sf-text-muted);
}

.sc-ref-block {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.sc-ref-list {
  margin: 0;
  padding-left: 18px;
  display: flex;
  flex-direction: column;
  gap: 5px;
  font-size: 13px;
  line-height: 1.65;
  color: var(--sf-text-muted);
}

.sc-return {
  border: 1px solid var(--sf-border);
  border-radius: var(--sf-radius);
  padding: 11px 13px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.sc-return__head {
  display: flex;
  align-items: center;
  gap: 8px;
}

.sc-return__title {
  font-size: 13px;
  font-weight: 600;
}

.sc-return__row {
  display: flex;
  gap: 10px;
  font-size: 13px;
  line-height: 1.6;
}

.sc-return__row span:first-child {
  flex: 0 0 64px;
  color: var(--sf-text-faint);
  font-size: 12px;
  margin-top: 1px;
}

.sc-return__code {
  padding: 6px 9px;
  border-radius: var(--sf-radius-sm);
  font-family: var(--sf-font-code);
  font-size: 12px;
  overflow-wrap: anywhere;
}

.sc-return__code--ok {
  background: var(--sf-ok-dim);
  color: var(--sf-ok);
}

.sc-return__code--bad {
  background: var(--sf-danger-dim);
  color: var(--sf-danger);
}
</style>
