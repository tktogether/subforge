<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { endpoints, type Stats, type HistoryEntry } from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import PageHeader from '../components/PageHeader.vue'
import AsyncSection from '../components/AsyncSection.vue'

const loading = ref(false)
const error = ref('')
const stats = ref<Stats | null>(null)
const history = ref<HistoryEntry[]>([])

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    const [s, h] = await Promise.all([endpoints.stats(), endpoints.history()])
    stats.value = s
    history.value = h
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

const empty = (): boolean => stats.value === null || stats.value.last === null

onMounted(() => {
  void load()
})
</script>

<template>
  <PageHeader title="概览" subtitle="转换统计与最近历史" />
  <AsyncSection :loading="loading" :error="error" :empty="empty()" empty-text="暂无转换记录" @retry="load">
    <div class="d-flex ga-4 mb-4 flex-wrap">
      <v-card variant="tonal" class="pa-4 flex-grow-1">
        <div class="text-caption text-medium-emphasis">历史条数</div>
        <div class="text-h5">{{ stats?.history.total ?? 0 }}</div>
      </v-card>
      <v-card variant="tonal" class="pa-4 flex-grow-1">
        <div class="text-caption text-medium-emphasis">缓存命中率</div>
        <div class="text-h5">{{ ((stats?.history.cacheHitRate ?? 0) * 100).toFixed(1) }}%</div>
      </v-card>
      <v-card variant="tonal" class="pa-4 flex-grow-1">
        <div class="text-caption text-medium-emphasis">URL 总数</div>
        <div class="text-h5">{{ stats?.history.urlTotal ?? 0 }}</div>
      </v-card>
      <v-card variant="tonal" class="pa-4 flex-grow-1">
        <div class="text-caption text-medium-emphasis">日志行数</div>
        <div class="text-h5">{{ stats?.logs ?? 0 }}</div>
      </v-card>
    </div>
    <v-data-table :items="history" density="comfortable" hover>
      <template #item.at="{ value }">{{ value }}</template>
    </v-data-table>
  </AsyncSection>
</template>
