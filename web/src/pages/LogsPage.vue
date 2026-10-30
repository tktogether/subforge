<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { endpoints, type LogEntry } from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import { notify } from '../composables/useNotify'
import { pageHelp } from '../content/help'
import PageHeader from '../components/PageHeader.vue'
import AsyncSection from '../components/AsyncSection.vue'

const help = pageHelp('logs')

type LogLevel = 'error' | 'warn' | 'info'

const logs = ref<LogEntry[]>([])
const keyword = ref('')
const levelFilter = ref<'all' | LogLevel>('all')
const autoRefresh = ref(false)
const error = ref('')
const loading = ref(false)
const container = ref<HTMLElement | null>(null)
const detailEntry = ref<LogEntry | null>(null)
const detailOpen = ref(false)
let timer: ReturnType<typeof setInterval> | null = null
let lastId = 0

const LEVEL_OPTIONS = [
  { title: '全部级别', value: 'all' },
  { title: '错误', value: 'error' },
  { title: '警告', value: 'warn' },
  { title: '信息', value: 'info' },
]

function classify(line: string): LogLevel {
  const lower = line.toLowerCase()
  if (lower.includes('error') || lower.includes('panic') || lower.includes('fatal')) return 'error'
  if (lower.includes('warn')) return 'warn'
  return 'info'
}

function levelText(level: LogLevel): string {
  if (level === 'error') return '错误'
  if (level === 'warn') return '警告'
  return '信息'
}

const filteredLogs = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  return logs.value.filter((entry) => {
    if (levelFilter.value !== 'all' && classify(entry.line) !== levelFilter.value) {
      return false
    }
    if (kw === '') return true
    return entry.line.toLowerCase().includes(kw)
  })
})

const longLine = (line: string): boolean => line.length > 220 || line.includes('\n')

function formatTs(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString('zh-CN', { hour12: false })
}

async function fetchLogs(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    const res = await endpoints.logs(lastId)
    const newEntries = res.logs ?? []
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
  const maxId = Math.max(lastId, ...logs.value.map((e) => e.id))
  logs.value = []
  lastId = maxId
}

function scrollToBottom(): void {
  const last = container.value?.lastElementChild
  if (last !== null && last !== undefined && typeof last.scrollIntoView === 'function') {
    last.scrollIntoView({ block: 'end' })
  }
}

function openDetail(entry: LogEntry): void {
  detailEntry.value = entry
  detailOpen.value = true
}

function closeDetail(): void {
  detailOpen.value = false
}

async function copyDetail(): Promise<void> {
  if (detailEntry.value === null) return
  try {
    await navigator.clipboard.writeText(detailEntry.value.line)
    notify('已复制日志内容', 'ok')
  } catch {
    notify('复制失败', 'error')
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
    最多显示最近 200 行。级别由前端按关键词归类，仅用于浏览；长日志显示摘要，可打开详情查看全文。
  </p>

  <v-card class="pa-3 mb-3">
    <div class="log-toolbar">
      <v-btn variant="tonal" data-testid="refresh" @click="fetchLogs">刷新</v-btn>
      <v-switch
        :model-value="autoRefresh"
        data-testid="auto-refresh"
        label="自动刷新"
        color="primary"
        density="compact"
        hide-details
        @update:model-value="onAutoChange"
      />
      <v-select
        v-model="levelFilter"
        :items="LEVEL_OPTIONS"
        item-title="title"
        item-value="value"
        label="级别筛选"
        density="compact"
        variant="outlined"
        hide-details
        data-testid="log-level-filter"
        class="log-toolbar__level"
      />
      <v-text-field
        v-model="keyword"
        label="搜索日志"
        placeholder="输入关键字"
        clearable
        density="compact"
        variant="outlined"
        hide-details
        data-testid="log-filter"
        class="log-toolbar__search"
      />
      <v-spacer />
      <v-btn variant="tonal" data-testid="clear" @click="clear">清空显示</v-btn>
      <v-btn variant="tonal" data-testid="scroll-bottom" @click="scrollToBottom">定位到最新</v-btn>
    </div>
  </v-card>

  <div v-if="error !== ''" class="text-error mb-2" data-testid="log-error">{{ error }}</div>

  <AsyncSection
    :loading="loading && logs.length === 0"
    :error="''"
    :empty="logs.length === 0 ? { title: '暂无日志', detail: '执行转换或请求订阅后，日志会显示在这里。' } : false"
    empty-text="暂无日志"
  >
    <div ref="container" class="log-list" data-testid="log-container">
      <div v-if="filteredLogs.length === 0 && logs.length > 0" class="text-caption py-3" data-testid="log-filter-empty">
        没有匹配当前筛选条件的日志
      </div>
      <article
        v-for="entry in filteredLogs"
        :key="entry.id"
        data-testid="log-row"
        class="log-row"
        :class="'log-row--' + classify(entry.line)"
      >
        <div class="log-row__meta">
          <span class="log-row__time">{{ formatTs(entry.ts) }}</span>
          <span class="log-row__level" :class="'log-row__level--' + classify(entry.line)">
            <span class="log-row__dot" aria-hidden="true"></span>{{ levelText(classify(entry.line)) }}
          </span>
          <v-btn
            v-if="longLine(entry.line)"
            variant="text"
            size="x-small"
            data-testid="log-detail-open"
            @click="openDetail(entry)"
          >查看完整内容</v-btn>
        </div>
        <div class="log-row__message" :class="{ 'log-row__message--clamped': longLine(entry.line) }">
          {{ entry.line }}
        </div>
      </article>
    </div>
  </AsyncSection>

  <v-dialog :model-value="detailOpen" max-width="760" scrollable data-testid="log-detail-dialog" @update:model-value="closeDetail">
    <v-card>
      <v-card-title>日志详情</v-card-title>
      <v-card-text>
        <div class="log-detail__meta">
          <span>{{ detailEntry === null ? '' : formatTs(detailEntry.ts) }}</span>
          <span class="log-detail__level" :class="detailEntry === null ? '' : 'log-row__level--' + classify(detailEntry.line)">
            {{ detailEntry === null ? '' : levelText(classify(detailEntry.line)) }}
          </span>
        </div>
        <pre class="log-detail__text">{{ detailEntry?.line }}</pre>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" data-testid="log-detail-copy" @click="copyDetail">复制内容</v-btn>
        <v-btn variant="text" data-testid="log-detail-close" @click="closeDetail">关闭</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<style scoped>
.log-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 12px;
}
.log-toolbar__level {
  max-width: 140px;
}
.log-toolbar__search {
  max-width: 240px;
  min-width: 180px;
}
.log-list {
  min-width: 0;
}
.log-row {
  min-width: 0;
  margin-bottom: 8px;
  padding: 10px 12px;
  border: 1px solid var(--sf-border);
  border-left-width: 3px;
  border-radius: var(--sf-radius);
  background: var(--sf-surface);
}
.log-row--error {
  border-left-color: var(--sf-danger);
}
.log-row--warn {
  border-left-color: var(--sf-warn);
}
.log-row--info {
  border-left-color: var(--sf-info);
}
.log-row__meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px 10px;
  min-width: 0;
}
.log-row__time {
  color: var(--sf-text-muted);
  font-family: var(--sf-font-mono);
  font-size: 12px;
  white-space: nowrap;
}
.log-row__level {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 12px;
  font-weight: 600;
}
.log-row__level--error {
  color: var(--sf-danger);
}
.log-row__level--warn {
  color: var(--sf-warn);
}
.log-row__level--info {
  color: var(--sf-info);
}
.log-row__dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: currentColor;
}
.log-row__message {
  margin-top: 6px;
  color: var(--sf-text);
  font-family: var(--sf-font-mono);
  font-size: 12.5px;
  line-height: 1.55;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  word-break: break-word;
}
.log-row__message--clamped {
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.log-detail__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-bottom: 8px;
  color: var(--sf-text-muted);
  font-size: 12px;
}
.log-detail__level {
  font-weight: 600;
}
.log-detail__text {
  margin: 0;
  padding: 12px;
  border: 1px solid var(--sf-border);
  border-radius: var(--sf-radius);
  background: var(--sf-surface-raised);
  color: var(--sf-text);
  font-family: var(--sf-font-mono);
  font-size: 12.5px;
  line-height: 1.55;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  word-break: break-word;
}
</style>
