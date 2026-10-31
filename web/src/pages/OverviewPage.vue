<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { endpoints, type Stats, type HistoryEntry, type Overview } from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import { pageHelp } from '../content/help'
import PageHeader from '../components/PageHeader.vue'
import AsyncSection from '../components/AsyncSection.vue'

const help = pageHelp('overview')

const loading = ref(false)
const error = ref('')
const overview = ref<Overview | null>(null)
const stats = ref<Stats | null>(null)
const history = ref<HistoryEntry[]>([])
const expanded = ref<string[]>([])

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

const empty = (): boolean => overview.value === null

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

function versionText(v: string): string {
  const s = v.trim()
  if (s === '' || /\bdev\b/i.test(s)) return '开发版'
  return s.replace(/^subforge\s+/i, '')
}

function scaleText(): string {
  if (counts.value === null) return '—'
  const c = counts.value
  if (c.airports === 0 && c.groups === 0 && c.rule_sources === 0) return '暂无资源'
  return c.airports + ' 个机场 · ' + c.groups + ' 个分组 · ' + c.rule_sources + ' 个规则源'
}

onMounted(() => {
  void load()
})
</script>

<template>
  <PageHeader :title="help.title" :subtitle="help.oneLiner" help-key="overview">
    <template #actions>
      <v-btn variant="text" prepend-icon="mdi-refresh" :loading="loading" data-testid="overview-refresh" @click="load">刷新</v-btn>
    </template>
  </PageHeader>

  <AsyncSection :loading="loading" :error="error" :empty="empty()" empty-text="暂无概览数据" @retry="load">
    <!-- Block 1: what this page does -->
    <v-card class="mb-4 pa-4" data-testid="block-intro">
      <div class="text-subtitle-2 mb-3">首次使用路径</div>
      <div class="quick-links">
        <router-link to="/airports" class="quick-link">
          <span class="quick-link__index">1</span>
          <span class="quick-link__body">
            <span class="quick-link__title">机场</span>
            <span class="quick-link__detail">添加上游订阅地址</span>
          </span>
          <v-icon size="18" icon="mdi-arrow-right" />
        </router-link>
        <router-link to="/groups" class="quick-link">
          <span class="quick-link__index">2</span>
          <span class="quick-link__body">
            <span class="quick-link__title">分组与规则</span>
            <span class="quick-link__detail">设置节点分组与流量分流</span>
          </span>
          <v-icon size="18" icon="mdi-arrow-right" />
        </router-link>
        <router-link to="/subscriptions" class="quick-link">
          <span class="quick-link__index">3</span>
          <span class="quick-link__body">
            <span class="quick-link__title">订阅</span>
            <span class="quick-link__detail">创建令牌并获取订阅地址</span>
          </span>
          <v-icon size="18" icon="mdi-arrow-right" />
        </router-link>
        <router-link to="/pipelines" class="quick-link">
          <span class="quick-link__index">4</span>
          <span class="quick-link__body">
            <span class="quick-link__title">管线</span>
            <span class="quick-link__detail">需要过滤、改名或排序时添加步骤</span>
          </span>
          <v-icon size="18" icon="mdi-arrow-right" />
        </router-link>
      </div>
      <div class="text-caption text-medium-emphasis mt-3">概览页只读，所有修改都在对应页面做。</div>
    </v-card>

    <!-- Block 2: persistent resources -->
    <v-card class="mb-4 pa-4" data-testid="block-resources">
      <div class="text-subtitle-2 mb-1">资源概览</div>
      <div class="text-caption text-medium-emphasis mb-3">来自当前数据库的实时数量，服务重启后仍保留。</div>
      <div class="metrics-grid" data-testid="resource-metrics">
        <v-card class="metric-card pa-4">
          <div class="text-caption text-medium-emphasis">机场</div>
          <div class="text-h5" data-testid="resource-airports">{{ countText(counts?.airports) }}</div>
          <div class="text-caption">启用 {{ countText(counts?.airports_enabled) }} 个</div>
        </v-card>
        <v-card class="metric-card pa-4">
          <div class="text-caption text-medium-emphasis">机场节点</div>
          <div class="text-h5" data-testid="resource-airport-nodes">{{ countText(counts?.airport_nodes) }}</div>
          <div class="text-caption">上游最近一次采集的节点总数</div>
        </v-card>
        <v-card class="metric-card pa-4">
          <div class="text-caption text-medium-emphasis">自建节点</div>
          <div class="text-h5" data-testid="resource-self-nodes">{{ countText(counts?.self_nodes) }}</div>
          <div class="text-caption">启用 {{ countText(counts?.self_nodes_enabled) }} 个</div>
        </v-card>
        <v-card class="metric-card pa-4">
          <div class="text-caption text-medium-emphasis">方案</div>
          <div class="text-h5" data-testid="resource-profiles">{{ countText(counts?.profiles) }}</div>
          <div class="text-caption">启用 {{ countText(counts?.profiles_enabled) }} 个</div>
        </v-card>
        <v-card class="metric-card pa-4">
          <div class="text-caption text-medium-emphasis">订阅令牌</div>
          <div class="text-h5" data-testid="resource-tokens">{{ countText(counts?.tokens) }}</div>
          <div class="text-caption">用于生成订阅地址</div>
        </v-card>
        <v-card class="metric-card pa-4">
          <div class="text-caption text-medium-emphasis">代理分组</div>
          <div class="text-h5" data-testid="resource-groups">{{ countText(counts?.groups) }}</div>
          <div class="text-caption">含机场导入与自定义分组</div>
        </v-card>
        <v-card class="metric-card pa-4">
          <div class="text-caption text-medium-emphasis">自定义规则</div>
          <div class="text-h5" data-testid="resource-rules">{{ countText(counts?.rules) }}</div>
          <div class="text-caption">启用 {{ countText(counts?.rules_enabled) }} 条</div>
        </v-card>
        <v-card class="metric-card pa-4">
          <div class="text-caption text-medium-emphasis">规则源</div>
          <div class="text-h5" data-testid="resource-rule-sources">{{ countText(counts?.rule_sources) }}</div>
          <div class="text-caption">有产物 {{ countText(counts?.rule_sources_ready) }} 个</div>
        </v-card>
        <v-card class="metric-card pa-4">
          <div class="text-caption text-medium-emphasis">规则源规则</div>
          <div class="text-h5" data-testid="resource-rule-source-rules">{{ countText(counts?.rule_source_rules) }}</div>
          <div class="text-caption">各源当前版本合计</div>
        </v-card>
      </div>
    </v-card>

    <!-- Block 3: conversion runtime data -->
    <v-card class="mb-4" data-testid="block-conversion">
      <v-card-title class="d-flex align-center flex-wrap ga-2">
        转换运行数据
        <v-chip size="small" variant="tonal" data-testid="conversion-reset-hint">本次启动</v-chip>
        <v-spacer />
        <span class="text-caption text-medium-emphasis" data-testid="conversion-meta">
          最近记录 {{ stats?.history.total ?? 0 }} 条 · 日志缓冲 {{ stats?.logs ?? 0 }} 行
        </span>
      </v-card-title>
      <v-card-text>
        <div v-if="stats?.last == null" class="text-body-2 text-medium-emphasis mb-3" data-testid="no-records-hint">
          本次启动还没有转换记录。在转换页执行一次转换，或请求一次订阅地址后，这里会显示最近一次转换的节点、分组、规则与缓存数据。
        </div>
        <div v-else class="metrics-grid" data-testid="conversion-metrics">
          <v-card class="metric-card pa-4">
            <div class="text-caption text-medium-emphasis">节点数</div>
            <div class="text-h5" data-testid="metric-nodes">{{ stats.last.nodes }}</div>
            <div class="text-caption">最近一次转换输出的节点数</div>
          </v-card>
          <v-card class="metric-card pa-4">
            <div class="text-caption text-medium-emphasis">原始节点数</div>
            <div class="text-h5" data-testid="metric-raw-nodes">{{ stats.last.rawNodes }}</div>
            <div class="text-caption">去重与过滤前解析到的节点数</div>
          </v-card>
          <v-card class="metric-card pa-4">
            <div class="text-caption text-medium-emphasis">分组数</div>
            <div class="text-h5" data-testid="metric-groups">{{ stats.last.groups }}</div>
            <div class="text-caption">最近一次转换实际生效的分组数</div>
          </v-card>
          <v-card class="metric-card pa-4">
            <div class="text-caption text-medium-emphasis">规则数</div>
            <div class="text-h5" data-testid="metric-rules">{{ stats.last.rules }}</div>
            <div class="text-caption">最近一次转换实际生效的规则行数</div>
          </v-card>
          <v-card class="metric-card pa-4">
            <div class="text-caption text-medium-emphasis">最近一次缓存命中</div>
            <div class="text-h5" data-testid="metric-last-cached">{{ stats.last.cached }}/{{ stats.last.urlCount }}</div>
            <div class="text-caption">命中缓存的来源数与来源总数之比</div>
          </v-card>
        </div>
        <v-divider class="my-4" />
        <div class="text-subtitle-2 mb-2">最近转换历史</div>
        <div v-if="history.length === 0" data-testid="history-empty" class="history-empty pa-4">
          <div class="text-subtitle-1">还没有转换记录</div>
          <div class="text-caption text-medium-emphasis mb-3">在转换页执行一次转换，或请求一次订阅地址后，这里会显示记录。</div>
          <v-btn color="primary" variant="tonal" size="small" prepend-icon="mdi-arrow-right" data-testid="history-empty-action" to="/convert">去转换</v-btn>
        </div>
        <v-data-table v-else :items="history" density="comfortable" hover show-expand :expanded="expanded" @update:expanded="(v: readonly string[]) => (expanded = [...v])" data-testid="history-table" class="history-table">
          <template #item.at="{ value }">{{ fmtTime(String(value)) }}</template>
          <template #expanded-row="{ columns, item }">
            <tr>
              <td :colspan="columns.length" class="pa-4">
                <div v-if="item.failures && item.failures.length" class="mb-2">
                  <div class="text-caption font-weight-bold">failures</div>
                  <v-list density="compact">
                    <v-list-item v-for="(f, i) in item.failures" :key="'f'+i">{{ f }}</v-list-item>
                  </v-list>
                </div>
                <div v-if="item.warnings && item.warnings.length" class="mb-2">
                  <div class="text-caption font-weight-bold">warnings</div>
                  <v-list density="compact">
                    <v-list-item v-for="(w, i) in item.warnings" :key="'w'+i">{{ w }}</v-list-item>
                  </v-list>
                </div>
                <div v-if="!(item.failures && item.failures.length) && !(item.warnings && item.warnings.length)" class="text-caption text-medium-emphasis">
                  本次全部来源成功，无跳过项
                </div>
              </td>
            </tr>
          </template>
        </v-data-table>
      </v-card-text>
    </v-card>

    <!-- Block 4: service status -->
    <v-card class="pa-4" data-testid="block-health">
      <div class="d-flex align-center flex-wrap ga-2 mb-3">
        <div class="text-subtitle-2">服务状态</div>
        <v-chip color="success" variant="tonal" size="small" data-testid="service-status">
          <span class="status-dot"></span>正常
        </v-chip>
        <v-spacer />
        <v-btn variant="text" size="small" prepend-icon="mdi-text-box-search-outline" to="/logs">查看日志</v-btn>
      </div>
      <div class="status-grid">
        <div class="status-item" data-testid="service-uptime">
          <div class="text-caption text-medium-emphasis">运行时长</div>
          <div class="status-value">{{ uptimeText() }}</div>
        </div>
        <div class="status-item" data-testid="service-started">
          <div class="text-caption text-medium-emphasis">启动时间</div>
          <div class="status-value">{{ overview === null ? '—' : fmtTime(overview.startedAt) }}</div>
        </div>
        <div class="status-item" data-testid="service-version">
          <div class="text-caption text-medium-emphasis">版本</div>
          <div class="status-value">{{ versionText(overview?.version ?? '') }}</div>
        </div>
        <div class="status-item" data-testid="service-scale">
          <div class="text-caption text-medium-emphasis">数据规模</div>
          <div class="status-value">{{ scaleText() }}</div>
        </div>
      </div>
      <div class="text-caption text-medium-emphasis mt-3">本页能正常显示数据表示服务正在运行；请求失败或数据异常时请查看日志页。</div>
    </v-card>
  </AsyncSection>
</template>

<style scoped>
.metrics-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  margin-bottom: 16px;
}
@media (min-width: 960px) {
  .metrics-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}
.metric-card {
  border: 1px solid var(--sf-border);
  border-radius: var(--sf-radius);
  box-shadow: none;
  min-height: 126px;
}
.metric-card .text-h5 {
  font-variant-numeric: tabular-nums;
}
.quick-links {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}
@media (min-width: 960px) {
  .quick-links {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }
}
.quick-link {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border: 1px solid var(--sf-border);
  border-radius: 8px;
  background: var(--sf-surface-raised);
  color: var(--sf-text);
  text-decoration: none;
  transition: border-color var(--sf-dur-micro), background var(--sf-dur-micro);
}
.quick-link:hover {
  border-color: var(--sf-accent);
  background: var(--sf-surface);
}
.quick-link__index {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 auto;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: var(--sf-accent);
  color: var(--sf-accent-fg);
  font-size: 12px;
  font-weight: 600;
}
.quick-link__body {
  display: flex;
  flex-direction: column;
  min-width: 0;
  flex: 1;
}
.quick-link__title {
  font-size: 13px;
  font-weight: 600;
}
.quick-link__detail {
  color: var(--sf-text-muted);
  font-size: 12px;
  overflow-wrap: anywhere;
}
.history-empty {
  border: 1px dashed var(--sf-border);
  border-radius: 8px;
  background: var(--sf-surface-raised);
  text-align: center;
}
.history-table {
  border: 1px solid var(--sf-border);
  border-radius: var(--sf-radius);
}
.history-table :deep(thead th) {
  background: var(--sf-surface-raised);
  color: var(--sf-text-muted);
  font-size: 12px;
  font-weight: 600;
  position: sticky;
  top: 0;
}
.status-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px 24px;
}
@media (min-width: 960px) {
  .status-grid {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }
}
.status-item {
  min-width: 0;
}
.status-value {
  margin-top: 2px;
  font-size: 15px;
  font-weight: 600;
  color: var(--sf-text);
  overflow-wrap: anywhere;
}
.status-dot {
  width: 7px;
  height: 7px;
  margin-right: 6px;
  border-radius: 50%;
  background: currentColor;
}
</style>
