<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { endpoints, type LogEntry } from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import { pageHelp } from '../content/help'
import PageHeader from '../components/PageHeader.vue'
import AsyncSection from '../components/AsyncSection.vue'

const help = pageHelp('logs')

const logs = ref<LogEntry[]>([])
const keyword = ref('')
const filteredLogs = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  if (kw === '') return logs.value
  return logs.value.filter((l) => l.line.toLowerCase().includes(kw))
})
const autoRefresh = ref(false)
const error = ref('')
const loading = ref(false)
const container = ref<HTMLElement | null>(null)
let timer: ReturnType<typeof setInterval> | null = null
let lastId = 0

function formatTs(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString('zh-CN', { hour12: false })
}

function isHighlight(line: string): boolean {
  const lower = line.toLowerCase()
  return lower.includes('error') || lower.includes('panic') || lower.includes('fatal')
}

async function fetchLogs(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    const res = await endpoints.logs(lastId)
    const newEntries = res.logs ?? []
    // 增量追加，不整表替换，避免自动刷新时页面跳动；id 不大于 since 的行丢弃（幂等）
    for (const entry of newEntries) {
      if (entry.id <= lastId) continue
      logs.value.push(entry)
      lastId = entry.id
    }
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

function onAutoChange(value: boolean | null): void {
  autoRefresh.value = value === true
  if (autoRefresh.value) {
    if (timer === null) {
      timer = setInterval(() => {
        void fetchLogs()
      }, 3000)
    }
  } else if (timer !== null) {
    clearInterval(timer)
    timer = null
  }
}

function clear(): void {
  // 只清前端列表，不会删除服务端日志；since 重置为当前最大 id，之后只拉新增行
  const maxId = Math.max(lastId, ...logs.value.map((e) => e.id))
  logs.value = []
  lastId = maxId
}

function scrollToBottom(): void {
  const el = container.value
  if (el !== null) {
    el.scrollTop = el.scrollHeight
  }
}

onMounted(() => {
  void fetchLogs()
})

onUnmounted(() => {
  if (timer !== null) {
    clearInterval(timer)
    timer = null
  }
})
</script>

<template>
  <PageHeader :title="help.title" :subtitle="help.oneLiner" help-key="logs" />

  <p class="text-body-2 text-medium-emphasis mb-3">
    最多显示最近 200 行。包含 error、panic 或 fatal 的行会高亮，仅用于定位关键词，不代表日志级别。
  </p>

  <div class="d-flex flex-wrap ga-2 mb-4 align-center">
    <v-btn variant="tonal" data-testid="refresh" @click="fetchLogs">刷新</v-btn>
    <v-switch
      :model-value="autoRefresh"
      data-testid="auto-refresh"
      label="自动刷新"
      hide-details
      @update:model-value="onAutoChange"
    />
    <v-btn variant="tonal" data-testid="clear" @click="clear">清空显示</v-btn>
    <v-btn variant="tonal" data-testid="scroll-bottom" @click="scrollToBottom">定位到最新</v-btn>
  </div>

  <v-text-field
    v-model="keyword"
    label="按关键字过滤（大小写不敏感）"
    clearable
    density="compact"
    data-testid="log-filter"
    class="mb-2"
  ></v-text-field>

  <div v-if="error !== ''" class="text-error mb-2" data-testid="log-error">{{ error }}</div>

  <AsyncSection
    :loading="loading && logs.length === 0"
    :error="''"
    :empty="logs.length === 0 ? { title: '暂无日志', detail: '执行转换或请求订阅后，日志会显示在这里。' } : false"
    empty-text="暂无日志"
  >
    <v-card ref="container" class="pa-2" data-testid="log-container" style="max-height: 480px; overflow: auto;">
      <div v-if="filteredLogs.length === 0 && logs.length > 0" class="text-caption" data-testid="log-filter-empty">没有匹配「{{ keyword }}」的日志</div>
      <div v-for="entry in filteredLogs" :key="entry.id" data-testid="log-row" class="d-flex ga-2 mb-1">
        <span class="text-caption" style="font-family: monospace; white-space: nowrap;">{{ formatTs(entry.ts) }}</span>
        <pre :class="isHighlight(entry.line) ? 'text-error' : ''" style="font-family: monospace; white-space: pre; margin: 0; overflow-x: auto; flex: 1;">{{ entry.line }}</pre>
      </div>
    </v-card>
  </AsyncSection>
</template>
