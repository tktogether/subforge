<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { endpoints, type LogEntry } from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import PageHeader from '../components/PageHeader.vue'

const logs = ref<LogEntry[]>([])
const paused = ref(false)
const error = ref('')
let timer: ReturnType<typeof setInterval> | null = null
let lastId = 0

async function fetchLogs(): Promise<void> {
  if (paused.value) return
  try {
    const res = await endpoints.logs(lastId)
    for (const entry of res.logs) {
      logs.value.push(entry)
      if (entry.id > lastId) lastId = entry.id
    }
  } catch (err) {
    error.value = errorMessage(err)
  }
}

function togglePause(): void {
  paused.value = !paused.value
}

function clear(): void {
  logs.value = []
  lastId = 0
}

onMounted(() => {
  void fetchLogs()
  timer = setInterval(() => {
    void fetchLogs()
  }, 2000)
})

onUnmounted(() => {
  if (timer !== null) clearInterval(timer)
})
</script>

<template>
  <PageHeader title="日志" subtitle="轮询日志" />
  <div class="d-flex ga-2 mb-4">
    <v-btn variant="tonal" @click="togglePause">{{ paused ? '继续' : '暂停' }}</v-btn>
    <v-btn variant="tonal" @click="clear">清屏</v-btn>
  </div>
  <div v-if="error !== ''" class="text-error mb-2">{{ error }}</div>
  <v-card variant="tonal" class="pa-4" style="max-height: 400px; overflow: auto">
    <div v-for="entry in logs" :key="entry.id">{{ entry.ts }} {{ entry.line }}</div>
    <div v-if="logs.length === 0" class="text-medium-emphasis">暂无日志</div>
  </v-card>
</template>
