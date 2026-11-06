<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import {
  IconRefresh,
  IconChevronDown,
  IconArrowRight,
  IconAntennaBars5,
  IconFilter,
  IconKey,
  IconRoute,
  IconAlertTriangle,
} from '@tabler/icons-vue'
import { endpoints, type Stats, type HistoryEntry, type Overview } from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import SfPageHead from '../components/ui/SfPageHead.vue'
import SfNote from '../components/ui/SfNote.vue'
import { parseVersion } from './overviewVersion'

const loading = ref(false)
const error = ref('')
const overview = ref<Overview | null>(null)
const stats = ref<Stats | null>(null)
const history = ref<HistoryEntry[]>([])
const expanded = ref<number | null>(null)

const counts = computed(() => overview.value?.counts ?? null)

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    const [o, s, h] = await Promise.all([endpoints.overview(), endpoints.stats(), endpoints.history()])
    overview.value = o
    stats.value = s
    history.value = h
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

function countText(n: number | undefined): string {
  return n === undefined ? '—' : String(n)
}

function fmtTime(at: string): string {
  try {
    return new Date(at).toLocaleString()
  } catch {
    return at
  }
}

function uptimeText(): string {
  if (overview.value === null) return '—'
  const started = new Date(overview.value.startedAt).getTime()
  if (Number.isNaN(started)) return overview.value.uptime || '—'
  const minutes = Math.max(0, Math.floor((Date.now() - started) / 60000))
  if (minutes < 1) return '不到 1 分钟'
  if (minutes < 60) return minutes + ' 分钟'
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return hours + ' 小时 ' + (minutes % 60) + ' 分钟'
  return Math.floor(hours / 24) + ' 天 ' + (hours % 24) + ' 小时'
}

const versionParts = computed(() => parseVersion(overview.value?.version ?? ''))

function scaleText(): string {
  if (counts.value === null) return '—'
  const c = counts.value
  if (c.airports === 0 && c.groups === 0 && c.rule_sources === 0) return '暂无资源'
  return c.airports + ' 机场 · ' + c.groups + ' 分组 · ' + c.rule_sources + ' 规则源'
}

function scaleSegments(): { n: number; unit: string }[] {
  const c = counts.value
  if (c === null) return []
  return [
    { n: c.airports, unit: '机场' },
    { n: c.groups, unit: '分组' },
    { n: c.rule_sources, unit: '规则源' },
  ]
}

function toggleExpand(id: number): void {
  expanded.value = expanded.value === id ? null : id
}

const resourceCells = computed(() => {
  const c = counts.value
  return [
    { label: '机场', value: countText(c?.airports), hint: '启用 ' + countText(c?.airports_enabled) },
    { label: '机场节点', value: countText(c?.airport_nodes), hint: '上游最近采集总数' },
    { label: '自建节点', value: countText(c?.self_nodes), hint: '启用 ' + countText(c?.self_nodes_enabled) },
    { label: '方案', value: countText(c?.profiles), hint: '启用 ' + countText(c?.profiles_enabled) },
    { label: '订阅令牌', value: countText(c?.tokens), hint: '用于生成订阅地址' },
    { label: '代理分组', value: countText(c?.groups), hint: '含导入与自定义' },
    { label: '自定义规则', value: countText(c?.rules), hint: '启用 ' + countText(c?.rules_enabled) + ' 条' },
    { label: '规则源', value: countText(c?.rule_sources), hint: '有产物 ' + countText(c?.rule_sources_ready) },
    { label: '规则源规则', value: countText(c?.rule_source_rules), hint: '各源当前版本合计' },
  ]
})

const bootSteps = [
  { to: '/airports', icon: IconAntennaBars5, title: '机场', desc: '添加上游订阅地址' },
  { to: '/groups', icon: IconFilter, title: '分组与规则', desc: '设置节点分组与分流' },
  { to: '/subscriptions', icon: IconKey, title: '订阅', desc: '创建令牌并获取订阅地址' },
  { to: '/pipelines', icon: IconRoute, title: '管线', desc: '为节点加过滤与排序' },
]

onMounted(() => {
  void load()
})
</script>

<template>
  <SfPageHead>
    <template #actions>
      <button class="sf-btn sf-btn--ghost" type="button" data-testid="overview-refresh" :disabled="loading" @click="load">
        <IconRefresh :size="15" :stroke-width="1.8" :class="{ 'sf-spin': loading }" />
        {{ loading ? '刷新中…' : '刷新' }}
      </button>
    </template>
  </SfPageHead>

  <div v-if="error !== ''" class="sf-note sf-note--danger sf-rise" style="margin-bottom: 16px">
    <IconAlertTriangle :size="16" :stroke-width="1.8" class="sf-note__icon" />
    <span class="sf-note__body">{{ error }}</span>
    <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" @click="load">重试</button>
  </div>

  <!-- 引擎遥测：主角面板 -->
  <section v-if="overview !== null" class="sf-panel sf-panel--ticks sf-rise sf-d1" data-testid="block-health" style="margin-bottom: 16px">
    <div class="ov-hero">
      <div class="ov-hero__grid">
        <div class="sf-stat" data-testid="service-uptime">
          <div class="sf-stat__label">UPTIME / 运行时长</div>
          <div class="sf-stat__value">{{ uptimeText() }}</div>
        </div>
        <div class="sf-stat" data-testid="service-version">
          <div class="sf-stat__label">VERSION / 版本</div>
          <div class="sf-stat__value">{{ versionParts.main }}</div>
          <div v-if="versionParts.meta !== ''" class="ov-version__meta">{{ versionParts.meta }}</div>
        </div>
        <div class="sf-stat" data-testid="service-started">
          <div class="sf-stat__label">BOOT AT / 启动时间</div>
          <div class="sf-stat__value sf-stat__value--sm">{{ fmtTime(overview.startedAt) }}</div>
        </div>
        <div class="sf-stat" data-testid="service-scale">
          <div class="sf-stat__label">SCALE / 数据规模</div>
          <div class="sf-stat__value sf-stat__value--sm" v-if="scaleSegments().length === 0">{{ scaleText() }}</div>
          <div v-else class="sf-stat__value sf-stat__value--sm ov-scale">
            <span v-for="(seg, i) in scaleSegments()" :key="seg.unit" class="ov-scale__seg">
              <span class="sf-num">{{ seg.n }}</span>&nbsp;{{ seg.unit }}<span v-if="i < scaleSegments().length - 1" class="ov-scale__dot" aria-hidden="true">·</span>
            </span>
          </div>
        </div>
      </div>
    </div>
  </section>

  <!-- 首次使用路径 -->
  <section class="sf-panel sf-rise sf-d2" data-testid="block-intro" style="margin-bottom: 16px">
    <div class="sf-panel__head">
      <span class="sf-panel__title">首次使用路径</span>
      <span class="sf-spacer" />
      <span class="sf-faint" style="font-size: 12px">概览页只读，修改都在对应页面做</span>
    </div>
    <div class="sf-panel__body ov-boot">
      <RouterLink v-for="(step, i) in bootSteps" :key="step.to" :to="step.to" class="ov-boot__step">
        <span class="ov-boot__index">{{ String(i + 1).padStart(2, '0') }}</span>
        <component :is="step.icon" :size="19" :stroke-width="1.6" class="ov-boot__icon" aria-hidden="true" />
        <span class="ov-boot__body">
          <span class="ov-boot__title">{{ step.title }}</span>
          <span class="ov-boot__desc">{{ step.desc }}</span>
        </span>
        <IconArrowRight :size="15" :stroke-width="1.8" class="ov-boot__arrow" aria-hidden="true" />
      </RouterLink>
    </div>
  </section>

  <!-- 资源遥测 -->
  <section class="sf-rise sf-d3" data-testid="block-resources" style="margin-bottom: 16px">
    <div class="ov-section-label sf-label-mono">RESOURCES / 资源遥测</div>
    <div class="sf-grid-stats ov-resources" data-testid="resource-metrics">
      <div v-for="cell in resourceCells" :key="cell.label" class="sf-stat">
        <div class="sf-stat__label">{{ cell.label }}</div>
        <div class="sf-stat__value" :data-testid="'resource-' + cell.label">{{ cell.value }}</div>
        <div class="sf-stat__hint">{{ cell.hint }}</div>
      </div>
    </div>
  </section>

  <!-- 转换运行数据 -->
  <section class="sf-panel sf-rise sf-d4" data-testid="block-conversion">
    <div class="sf-panel__head">
      <span class="sf-panel__title">转换运行数据</span>
      <span class="sf-badge" data-testid="conversion-reset-hint">本次启动</span>
      <span class="sf-spacer" />
      <span class="sf-faint sf-mono" style="font-size: 11px" data-testid="conversion-meta">
        记录 {{ stats?.history.total ?? 0 }} · 日志缓冲 {{ stats?.logs ?? 0 }} 行
      </span>
    </div>
    <div class="sf-panel__body">
      <div v-if="stats?.last == null" class="sf-note" data-testid="no-records-hint">
        <span class="sf-note__body">
          本次启动还没有转换记录。在转换页执行一次转换，或请求一次订阅地址后，这里会显示最近一次转换的节点、分组、规则与缓存数据。
        </span>
      </div>
      <div v-else class="sf-grid-stats" data-testid="conversion-metrics">
        <div class="sf-stat">
          <div class="sf-stat__label">节点数</div>
          <div class="sf-stat__value" data-testid="metric-nodes">{{ stats.last.nodes }}</div>
          <div class="sf-stat__hint">最近一次输出的节点数</div>
        </div>
        <div class="sf-stat">
          <div class="sf-stat__label">原始节点</div>
          <div class="sf-stat__value" data-testid="metric-raw-nodes">{{ stats.last.rawNodes }}</div>
          <div class="sf-stat__hint">去重与过滤前解析到的</div>
        </div>
        <div class="sf-stat">
          <div class="sf-stat__label">分组数</div>
          <div class="sf-stat__value" data-testid="metric-groups">{{ stats.last.groups }}</div>
          <div class="sf-stat__hint">实际生效的分组</div>
        </div>
        <div class="sf-stat">
          <div class="sf-stat__label">规则数</div>
          <div class="sf-stat__value" data-testid="metric-rules">{{ stats.last.rules }}</div>
          <div class="sf-stat__hint">实际生效的规则行</div>
        </div>
        <div class="sf-stat">
          <div class="sf-stat__label">缓存命中</div>
          <div class="sf-stat__value sf-stat__value--accent" data-testid="metric-last-cached">{{ stats.last.cached }}/{{ stats.last.urlCount }}</div>
          <div class="sf-stat__hint">命中缓存的来源占比</div>
        </div>
      </div>
    </div>

    <div class="sf-panel__head" style="border-top: 1px solid var(--sf-border); border-bottom: none">
      <span class="sf-panel__title">最近转换历史</span>
    </div>
    <div v-if="history.length === 0" data-testid="history-empty">
      <div class="sf-empty">
        <div class="sf-empty__title">还没有转换记录</div>
        <p class="sf-empty__desc">在转换页执行一次转换，或请求一次订阅地址后，这里会显示记录。</p>
        <div class="sf-empty__action">
          <RouterLink to="/convert" class="sf-btn sf-btn--primary sf-btn--sm" data-testid="history-empty-action">
            去转换
            <IconArrowRight :size="14" :stroke-width="1.8" />
          </RouterLink>
        </div>
      </div>
    </div>
    <div v-else class="sf-table-wrap" style="border: none; border-top: 1px solid var(--sf-border); border-radius: 0" data-testid="history-table">
      <table class="sf-table">
        <thead>
          <tr>
            <th style="width: 34px"></th>
            <th>时间</th>
            <th>目标格式</th>
            <th style="text-align: right">节点</th>
            <th style="text-align: right">原始</th>
            <th style="text-align: right">分组</th>
            <th style="text-align: right">规则</th>
            <th style="text-align: right">缓存</th>
            <th style="width: 90px">状态</th>
          </tr>
        </thead>
        <tbody>
          <template v-for="entry in history" :key="entry.id">
            <tr class="is-clickable" @click="toggleExpand(entry.id)">
              <td>
                <IconChevronDown
                  :size="15"
                  :stroke-width="1.8"
                  class="sf-faint ov-caret"
                  :class="{ 'is-open': expanded === entry.id }"
                  aria-hidden="true"
                />
              </td>
              <td class="sf-table__num">{{ fmtTime(entry.at) }}</td>
              <td>{{ entry.target }}</td>
              <td class="sf-table__num" style="text-align: right">{{ entry.nodes }}</td>
              <td class="sf-table__num sf-faint" style="text-align: right">{{ entry.rawNodes }}</td>
              <td class="sf-table__num" style="text-align: right">{{ entry.groups }}</td>
              <td class="sf-table__num" style="text-align: right">{{ entry.rules }}</td>
              <td class="sf-table__num sf-faint" style="text-align: right">{{ entry.cached }}/{{ entry.urlCount }}</td>
              <td>
                <span v-if="entry.failures !== null && entry.failures.length > 0" class="sf-badge sf-badge--danger">失败 {{ entry.failures.length }}</span>
                <span v-else-if="entry.warnings !== null && entry.warnings.length > 0" class="sf-badge sf-badge--warn">告警 {{ entry.warnings.length }}</span>
                <span v-else class="sf-badge sf-badge--ok">正常</span>
              </td>
            </tr>
            <tr v-if="expanded === entry.id" class="ov-expand-row">
              <td colspan="9">
                <div class="ov-expand">
                  <template v-if="(entry.failures !== null && entry.failures.length > 0) || (entry.warnings !== null && entry.warnings.length > 0)">
                    <div v-if="entry.failures !== null && entry.failures.length > 0">
                      <div class="ov-expand__label">FAILURES</div>
                      <div v-for="(f, i) in entry.failures" :key="'f' + i" class="ov-expand__line">{{ f }}</div>
                    </div>
                    <div v-if="entry.warnings !== null && entry.warnings.length > 0">
                      <div class="ov-expand__label">WARNINGS</div>
                      <div v-for="(w, i) in entry.warnings" :key="'w' + i" class="ov-expand__line">{{ w }}</div>
                    </div>
                  </template>
                  <div v-else class="sf-faint">本次全部来源成功，无跳过项。</div>
                </div>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>
  </section>

  <SfNote class="sf-rise sf-d5" tone="info" style="margin-top: 16px">
    本页能正常显示数据表示服务正在运行；请求失败或数据异常时请查看日志页。
  </SfNote>
</template>

<style scoped>
.ov-hero {
  padding: 6px 0 0;
}

.ov-hero__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
}

.ov-scale {
  display: flex;
  flex-wrap: wrap;
  gap: 0 6px;
  align-items: baseline;
  font-size: 16px;
  line-height: 1.5;
}

.ov-version__meta {
  margin-top: 6px;
  font: 500 10.5px/1.5 var(--sf-font-mono);
  letter-spacing: 0.04em;
  color: var(--sf-text-faint);
  overflow-wrap: anywhere;
}

.ov-scale__seg {
  white-space: nowrap;
}

.ov-scale__dot {
  color: var(--sf-text-faint);
  margin-left: 6px;
}

.ov-hero__grid > * + * {
  border-left: 1px solid var(--sf-border);
}

/* 单列堆叠时改用横向分隔线；小屏遥测 2×2 */
@media (max-width: 700px) {
  .ov-hero__grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .ov-hero__grid > * + * {
    border-left: none;
  }

  .ov-hero__grid > *:nth-child(n + 3) {
    border-top: 1px solid var(--sf-border);
  }

  .ov-hero__grid > *:nth-child(2n + 1) {
    border-left: none;
  }

  .ov-hero__grid > *:nth-child(2n) {
    border-left: 1px solid var(--sf-border);
  }
}

.sf-stat__value--sm {
  font-size: 17px;
  line-height: 1.5;
  overflow-wrap: anywhere;
}

.ov-section-label {
  margin-bottom: 10px;
}

/* 资源遥测 9 格：末格在 5 列 / 2 列布局下跨 2 列，铺满末行不留空槽 */
.ov-resources > *:last-child {
  grid-column: span 2;
}

@media (min-width: 640px) and (max-width: 999px) {
  .ov-resources {
    grid-template-columns: repeat(3, 1fr);
  }

  .ov-resources > *:last-child {
    grid-column: auto;
  }
}

@media (min-width: 1000px) {
  .ov-resources {
    grid-template-columns: repeat(5, 1fr);
  }
}

/* 资源遥测 9 格：末格在 5 列 / 2 列布局下跨 2 列，铺满末行不留空槽 */
.ov-resources > *:last-child {
  grid-column: span 2;
}

@media (min-width: 640px) and (max-width: 999px) {
  .ov-resources {
    grid-template-columns: repeat(3, 1fr);
  }

  .ov-resources > *:last-child {
    grid-column: auto;
  }
}

@media (min-width: 1000px) {
  .ov-resources {
    grid-template-columns: repeat(5, 1fr);
  }
}

.ov-boot {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 10px;
  padding: 14px;
}

@media (min-width: 640px) {
  .ov-boot {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (min-width: 1000px) {
  .ov-boot {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }
}

.ov-boot__step {
  display: flex;
  align-items: center;
  gap: 11px;
  padding: 12px 14px;
  border-radius: var(--sf-radius);
  background: transparent;
  color: var(--sf-text);
  text-decoration: none;
  transition: background var(--sf-t-fast) var(--sf-ease);
}

.ov-boot__step:hover {
  background: var(--sf-accent-dim);
}

.ov-boot__index {
  font: 500 10.5px/1 var(--sf-font-mono);
  color: var(--sf-accent);
  letter-spacing: 0.08em;
}

.ov-boot__icon {
  color: var(--sf-text-faint);
  flex: none;
}

.ov-boot__body {
  display: flex;
  flex-direction: column;
  gap: 1px;
  min-width: 0;
  flex: 1;
}

.ov-boot__title {
  font-weight: 600;
  font-size: 13.5px;
}

.ov-boot__desc {
  font-size: 12px;
  color: var(--sf-text-muted);
}

.ov-boot__arrow {
  color: var(--sf-text-faint);
  flex: none;
  transition: transform var(--sf-t-fast) var(--sf-ease), color var(--sf-t-fast) var(--sf-ease);
}

.ov-boot__step:hover .ov-boot__arrow {
  transform: translateX(2px);
  color: var(--sf-accent);
}

.ov-caret {
  transition: transform var(--sf-t-fast) var(--sf-ease);
}

.ov-caret.is-open {
  transform: rotate(180deg);
}

.ov-expand-row td {
  background: var(--sf-surface-dim);
}

.ov-expand {
  display: flex;
  flex-direction: column;
  gap: 10px;
  font-size: 12.5px;
}

.ov-expand__label {
  font: 500 10px/1 var(--sf-font-mono);
  letter-spacing: 0.14em;
  color: var(--sf-text-faint);
  margin-bottom: 4px;
}

.ov-expand__line {
  color: var(--sf-text-muted);
  line-height: 1.6;
  overflow-wrap: anywhere;
}

.sf-spin {
  animation: sf-rotate 0.9s linear infinite;
}

@keyframes sf-rotate {
  to {
    transform: rotate(360deg);
  }
}
</style>
