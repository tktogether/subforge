<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { endpoints, type Pipeline } from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import PageHeader from '../components/PageHeader.vue'
import AsyncSection from '../components/AsyncSection.vue'

const loading = ref(false)
const error = ref('')
const pipelines = ref<Pipeline[]>([])

function parseSteps(s: string): string {
  try {
    const arr = JSON.parse(s) as Array<{ op?: string }>
    return arr.map((x) => x.op ?? '').filter(Boolean).join(', ')
  } catch {
    return s
  }
}

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    pipelines.value = await endpoints.pipelines('')
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void load()
})
</script>

<template>
  <PageHeader title="管线" subtitle="命名管线只读列表" />
  <AsyncSection :loading="loading" :error="error" :empty="pipelines.length === 0" empty-text="在配置文件中定义管线" @retry="load">
    <v-list>
      <v-list-item v-for="p in pipelines" :key="p.id" :title="p.name" :subtitle="p.source + (p.read_only ? ' · 只读' : '')">
        <template #append>
          <span class="text-caption">{{ parseSteps(p.steps_json) }}</span>
        </template>
      </v-list-item>
    </v-list>
  </AsyncSection>
</template>
