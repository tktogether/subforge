<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { endpoints, type LogEntry } from '../api/endpoints'
import { copyText } from '../utils/clipboard'
import { errorMessage } from '../composables/useAsync'
import { notify } from '../composables/useNotify'
import SfPageHead from '../components/ui/SfPageHead.vue'
import SfSwitch from '../components/ui/SfSwitch.vue'
import SfDialog from '../components/ui/SfDialog.vue'
import SfCodeBlock from '../components/ui/SfCodeBlock.vue'
import SfNote from '../components/ui/SfNote.vue'
import { IconRefresh, IconEraser, IconCrosshair, IconTerminal2 } from '@tabler/icons-vue'

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

const LEVEL_OPTIONS: { title: string; value: 'all' | LogLevel }[] = [
  { title: '全部', value: 'all' },
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
  const matched = logs.value.filter((entry) => {
    if (levelFilter.value !== 'all' && classify(entry.line) !== levelFilter.value) {
      return false
    }
    if (kw === '') return true
    return entry.line.toLowerCase().includes(kw)
  })
  return matched.slice().reverse()
})

const longLine = (line: string): boolean => line.length > 220 || line.includes('\n')

function formatTs(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString('zh-CN', { hour12: false })
}

void formatTs

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

function onAutoChange(value: boolean): void {
  autoRefresh.value = value
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

function scrollToLatest(): void {
  const latest = container.value?.querySelector('[data-testid="log-row"]')
  if (latest !== null && latest !== undefined && typeof latest.scrollIntoView === 'function') {
    latest.scrollIntoView({ block: 'start' })
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
  const ok = await copyText(detailEntry.value.line)
  notify(ok ? '已复制日志内容' : '复制失败', ok ? 'ok' : 'error')
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
  <div>
    <SfPageHead />

    <SfNote class="sf-rise sf-d1" tone="info" style="margin-bottom: 16px">
      最新日志在前，最多 200 行；长日志可打开详情查看全文。
    </SfNote>

    <section class="sf-panel sf-rise sf-d1">
      <div class="sf-panel__body lg-toolbar" style="padding: 12px 16px">
        <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" data-testid="refresh" :disabled="loading" @click="fetchLogs">
          <IconRefresh :size="14" :stroke-width="1.8" :class="{ 'lg-spin': loading }" />
          刷新
        </button>
        <SfSwitch :model-value="autoRefresh" data-testid="auto-refresh" label="自动刷新（3s）" @update:model-value="onAutoChange" />
        <span class="sf-seg" role="group" aria-label="级别筛选">
          <button
            v-for="opt in LEVEL_OPTIONS"
            :key="opt.value"
            type="button"
            class="sf-seg__btn"
            :class="{ 'is-active': levelFilter === opt.value }"
            :data-testid="'log-level-' + opt.value"
            @click="levelFilter = opt.value"
          >{{ opt.title }}</button>
        </span>
        <input v-model="keyword" class="sf-input sf-input--mono lg-search" placeholder="搜索日志…" data-testid="log-filter" />
        <span class="sf-spacer" />
        <button class="sf-btn sf-btn--subtle sf-btn--sm" type="button" data-testid="clear" @click="clear">
          <IconEraser :size="14" :stroke-width="1.8" />
          清空显示
        </button>
        <button class="sf-btn sf-btn--subtle sf-btn--sm" type="button" data-testid="scroll-bottom" @click="scrollToLatest">
          <IconCrosshair :size="14" :stroke-width="1.8" />
          定位到最新
        </button>
      </div>

      <div v-if="error !== ''" class="sf-field__error" style="padding: 0 16px 10px" data-testid="log-error">{{ error }}</div>

      <div v-if="logs.length === 0 && !loading" class="sf-empty" style="border-top: 1px solid var(--sf-border)">
        <div class="sf-empty__icon"><IconTerminal2 :size="30" :stroke-width="1.4" /></div>
        <div class="sf-empty__title">暂无日志</div>
        <p class="sf-empty__desc">执行转换或请求订阅后，日志会显示在这里。</p>
      </div>

      <div v-else ref="container" class="lg-list" data-testid="log-container">
        <div v-if="filteredLogs.length === 0 && logs.length > 0" class="lg-nomatch" data-testid="log-filter-empty">
          没有匹配当前筛选条件的日志
        </div>
        <article
          v-for="entry in filteredLogs"
          :key="entry.id"
          data-testid="log-row"
          class="lg-row"
          :class="'lg-row--' + classify(entry.line)"
        >
          <span class="lg-led" :class="'lg-led--' + classify(entry.line)" aria-hidden="true" />
          <span class="lg-level" :class="'lg-level--' + classify(entry.line)">{{ levelText(classify(entry.line)) }}</span>
          <span class="lg-msg" :class="{ 'lg-msg--clamped': longLine(entry.line) }">{{ entry.line }}</span>
          <button v-if="longLine(entry.line)" class="sf-btn sf-btn--subtle sf-btn--sm lg-more" type="button" data-testid="log-detail-open" @click="openDetail(entry)">
            查看完整内容
          </button>
        </article>
      </div>
    </section>

    <SfDialog :open="detailOpen" label="LOG DETAIL" title="日志详情" wide @close="closeDetail">
      <template v-if="detailEntry !== null">
        <div class="lg-detail-meta">
          <span class="sf-mono">{{ formatTs(detailEntry.ts) }}</span>
          <span class="sf-badge" :class="'sf-badge--' + (classify(detailEntry.line) === 'error' ? 'danger' : classify(detailEntry.line) === 'warn' ? 'warn' : 'info')">
            {{ levelText(classify(detailEntry.line)) }}
          </span>
        </div>
        <SfCodeBlock :code="detailEntry.line" plain />
      </template>
      <template #foot>
        <button class="sf-btn sf-btn--ghost" type="button" data-testid="log-detail-copy" @click="copyDetail">复制内容</button>
        <button class="sf-btn sf-btn--primary" type="button" data-testid="log-detail-close" @click="closeDetail">关闭</button>
      </template>
    </SfDialog>
  </div>
</template>

<style scoped>
.lg-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px 12px;
}

.lg-search {
  max-width: 240px;
  min-width: 170px;
}

.lg-spin {
  animation: lg-rotate 0.9s linear infinite;
}

@keyframes lg-rotate {
  to {
    transform: rotate(360deg);
  }
}

/* 日志流 */
.lg-list {
  border-top: 1px solid var(--sf-border);
  max-height: 640px;
  overflow-y: auto;
  padding: 6px 0;
}

.lg-nomatch {
  padding: 18px 16px;
  font-size: 12.5px;
  color: var(--sf-text-faint);
}

.lg-row {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 7px 16px;
  transition: background var(--sf-t-fast) var(--sf-ease);
}

.lg-row:hover {
  background: var(--sf-surface-dim);
}

.lg-row--error {
  background: oklch(from var(--sf-danger) l c h / 0.05);
}

.lg-row--error:hover {
  background: oklch(from var(--sf-danger) l c h / 0.09);
}

.lg-led {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex: none;
  margin-top: 6px;
  background: var(--sf-text-faint);
}

.lg-led--error {
  background: var(--sf-danger);
  box-shadow: 0 0 6px oklch(from var(--sf-danger) l c h / 0.6);
}

.lg-led--warn {
  background: var(--sf-warn);
}

.lg-time {
  font-family: var(--sf-font-mono);
  font-size: 11px;
  color: var(--sf-text-faint);
  white-space: nowrap;
  margin-top: 1px;
  font-variant-numeric: tabular-nums;
}

.lg-level {
  font: 600 10.5px/1.5 var(--sf-font-mono);
  letter-spacing: 0.08em;
  white-space: nowrap;
  margin-top: 1px;
  min-width: 2.5em;
}

.lg-level--error { color: var(--sf-danger); }
.lg-level--warn { color: var(--sf-warn); }
.lg-level--info { color: var(--sf-text-faint); }

.lg-msg {
  flex: 1;
  min-width: 0;
  font-family: var(--sf-font-code);
  font-size: 12.5px;
  line-height: 1.6;
  color: var(--sf-text-muted);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  word-break: break-word;
}

.lg-row--error .lg-msg {
  color: var(--sf-text);
}

.lg-msg--clamped {
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.lg-more {
  flex: none;
  margin-top: 0;
}

.lg-detail-meta {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 12.5px;
  color: var(--sf-text-muted);
}
</style>
