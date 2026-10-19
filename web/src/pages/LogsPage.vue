<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { endpoints, type LogEntry } from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import { pageHelp } from '../content/help'
import PageHeader from '../components/PageHeader.vue'
import AsyncSection from '../components/AsyncSection.vue'

const help = pageHelp('logs')

const logs = ref<LogEntry[]>([])
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

  <v-alert type="info" variant="tonal" class="mb-4">
    <div class="mb-1"><strong>日志级别：</strong>本页<strong>不区分级别</strong>，显示的是服务标准输出里被捕获的原始行（含 <code>[serve]</code> 前缀的业务日志与错误）。没有单独的 error/warn 过滤。行内出现 error/panic/fatal 时的红色只是<strong>纯前端关键字高亮</strong>，不是服务端分级。</div>
    <div class="mb-1"><strong>时间范围：</strong>最多显示最近 200 行。缓冲区有上限，更早的行会被丢弃——<strong>看不到不代表没发生</strong>。</div>
    <div><strong>跟到底：</strong>勾选「自动刷新」后每 3 秒拉取一次新增行（按 <code>id</code> 增量拉，不会重复）。排查线上问题时建议开着，但<strong>离开页面要记得关</strong>（或直接切页），否则会一直轮询。</div>
  </v-alert>

  <div class="d-flex flex-wrap ga-2 mb-4 align-center">
    <v-btn variant="tonal" data-testid="refresh" @click="fetchLogs">刷新</v-btn>
    <v-switch
      :model-value="autoRefresh"
      data-testid="auto-refresh"
      label="自动刷新"
      hide-details
      @update:model-value="onAutoChange"
    />
    <v-btn variant="tonal" data-testid="clear" @click="clear">清屏</v-btn>
    <v-btn variant="tonal" data-testid="scroll-bottom" @click="scrollToBottom">滚动到底部</v-btn>
  </div>

  <div v-if="error !== ''" class="text-error mb-2" data-testid="log-error">{{ error }}</div>

  <AsyncSection
    :loading="loading && logs.length === 0"
    :error="''"
    :empty="logs.length === 0 ? { title: '暂无日志', detail: '服务刚启动时日志缓冲区可能是空的；做一次转换或让客户端请求一次订阅地址，这里就会出现内容。' } : false"
    empty-text="暂无日志"
  >
    <v-card ref="container" class="pa-2" data-testid="log-container" style="max-height: 480px; overflow: auto;">
      <div v-for="entry in logs" :key="entry.id" data-testid="log-row" class="d-flex ga-2 mb-1">
        <span class="text-caption" style="font-family: monospace; white-space: nowrap;">{{ formatTs(entry.ts) }}</span>
        <pre :class="isHighlight(entry.line) ? 'text-error' : ''" style="font-family: monospace; white-space: pre; margin: 0; overflow-x: auto; flex: 1;">{{ entry.line }}</pre>
      </div>
    </v-card>
  </AsyncSection>
</template>
