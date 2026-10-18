<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { endpoints, type Script, type ScriptTestResult } from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import PageHeader from '../components/PageHeader.vue'
import AsyncSection from '../components/AsyncSection.vue'

const loading = ref(false)
const error = ref('')
const scripts = ref<Script[]>([])

const dialog = ref(false)
const editing = ref<Script | null>(null)
const formName = ref('')
const formKind = ref('filter')
const formCode = ref('')
const formEnabled = ref(true)
const saving = ref(false)
const saveError = ref('')

const testDialog = ref(false)
const testCode = ref('')
const testKind = ref('filter')
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
  testResult.value = null
  testDialog.value = true
}

async function runTest(): Promise<void> {
  testing.value = true
  testResult.value = null
  try {
    testResult.value = await endpoints.testScript({ code: testCode.value, kind: testKind.value })
  } catch (err) {
    testResult.value = { ok: false, duration_ms: 0, error: errorMessage(err) }
  } finally {
    testing.value = false
  }
}

onMounted(() => {
  void load()
})
</script>

<template>
  <PageHeader title="脚本" subtitle="goja 脚本管理与试运行" />
  <v-alert type="warning" variant="tonal" class="mb-4">
    脚本在服务端 goja 沙箱执行，请只运行可信代码；默认由 script.enabled 关闭。
  </v-alert>
  <div class="d-flex ga-2 mb-4">
    <v-btn color="primary" @click="openCreate">新建脚本</v-btn>
    <v-btn variant="tonal" @click="load">刷新</v-btn>
  </div>
  <AsyncSection :loading="loading" :error="error" :empty="scripts.length === 0" empty-text="暂无脚本" @retry="load">
    <v-list>
      <v-list-item v-for="s in scripts" :key="s.id" :title="s.name" :subtitle="s.kind">
        <template #append>
          <v-btn size="small" variant="text" @click="openEdit(s)">编辑</v-btn>
          <v-btn size="small" variant="text" @click="openTest(s)">试运行</v-btn>
          <v-btn size="small" variant="text" color="error" @click="remove(s)">删除</v-btn>
        </template>
      </v-list-item>
    </v-list>
  </AsyncSection>

  <v-dialog v-model="dialog" max-width="640">
    <v-card>
      <v-card-title>{{ editing === null ? '新建脚本' : '编辑脚本' }}</v-card-title>
      <v-card-text>
        <v-text-field v-model="formName" label="名称" class="mb-2" />
        <v-select v-model="formKind" :items="['filter', 'operate']" label="类型" class="mb-2" />
        <v-textarea v-model="formCode" label="代码" rows="8" />
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

  <v-dialog v-model="testDialog" max-width="640">
    <v-card>
      <v-card-title>试运行脚本</v-card-title>
      <v-card-text>
        <v-select v-model="testKind" :items="['filter', 'operate']" label="类型" class="mb-2" />
        <v-textarea v-model="testCode" label="代码" rows="8" />
        <v-btn color="primary" :loading="testing" @click="runTest" class="mt-2">运行</v-btn>
        <div v-if="testResult !== null" class="mt-4">
          <div :class="testResult.ok ? 'text-success' : 'text-error'">{{ testResult.ok ? '成功' : '失败' }}（{{ testResult.duration_ms }}ms）</div>
          <div v-if="testResult.error" class="text-error">{{ testResult.error }}</div>
          <pre v-if="testResult.after" class="mt-2">{{ JSON.stringify(testResult.after, null, 2) }}</pre>
        </div>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="testDialog = false">关闭</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
