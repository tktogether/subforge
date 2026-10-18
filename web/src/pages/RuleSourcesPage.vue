<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { endpoints, type RuleSource } from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import { notify } from '../composables/useNotify'
import PageHeader from '../components/PageHeader.vue'
import AsyncSection from '../components/AsyncSection.vue'



const loading = ref(false)
const error = ref('')
const sources = ref<RuleSource[]>([])
const manifestPath = ref('')

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    const r = await endpoints.listRuleSources()
    manifestPath.value = r.manifest_path
    sources.value = r.sources
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

async function updateAll(): Promise<void> {
  try {
    await endpoints.updateRuleSources()
    notify('更新完成', 'ok')
    await load()
  } catch (err) {
    notify(errorMessage(err), 'error')
  }
}

async function rollback(name: string): Promise<void> {
  try {
    const r = await endpoints.rollbackRuleSource(name)
    notify('已回滚: ' + r.path, 'ok')
    await load()
  } catch (err) {
    notify(errorMessage(err), 'error')
  }
}

const mrsDialog = ref(false)
const mrsName = ref('')
const mrsBehavior = ref('domain')

function openMrs(name: string): void {
  mrsName.value = name
  mrsBehavior.value = 'domain'
  mrsDialog.value = true
}

async function doMrs(): Promise<void> {
  try {
    const r = await endpoints.generateMRS(mrsName.value, mrsBehavior.value)
    notify('已生成: ' + r.path + ' (' + r.bytes + ' bytes)', 'ok')
    mrsDialog.value = false
    await load()
  } catch (err) {
    notify(errorMessage(err), 'error')
  }
}

onMounted(() => {
  void load()
})
</script>

<template>
  <PageHeader title="规则源" subtitle="规则源版本管理与 MRS 生成" />
  <AsyncSection :loading="loading" :error="error" :empty="sources.length === 0" empty-text="暂无规则源" @retry="load">
    <v-list>
      <v-list-item v-for="s in sources" :key="s.name" :title="s.name" :subtitle="s.latest + ' (' + s.versions.length + ' versions)'">
        <template #append>
          <v-btn size="small" variant="text" @click="updateAll">更新</v-btn>
          <v-btn size="small" variant="text" @click="rollback(s.name)">回滚</v-btn>
          <v-btn size="small" variant="text" color="primary" @click="openMrs(s.name)">生成 MRS</v-btn>
        </template>
      </v-list-item>
    </v-list>
  </AsyncSection>

  <v-dialog v-model="mrsDialog" max-width="480">
    <v-card>
      <v-card-title>生成 MRS - {{ mrsName }}</v-card-title>
      <v-card-text>
        <v-select v-model="mrsBehavior" :items="['domain', 'ipcidr']" label="行为" />
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="mrsDialog = false">取消</v-btn>
        <v-btn color="primary" @click="doMrs">生成</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
