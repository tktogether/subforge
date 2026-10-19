<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { endpoints, type Stats, type HistoryEntry } from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import { pageHelp } from '../content/help'
import PageHeader from '../components/PageHeader.vue'
import AsyncSection from '../components/AsyncSection.vue'

const help = pageHelp('overview')

const loading = ref(false)
const error = ref('')
const stats = ref<Stats | null>(null)
const history = ref<HistoryEntry[]>([])
const expanded = ref<string[]>([])

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

const empty = (): boolean => stats.value === null

function fmtTime(at: string): string {
  try {
    return new Date(at).toLocaleString()
  } catch {
    return at
  }
}

function lastOrDash(n: number | null | undefined): string {
  return stats.value?.last == null ? '—' : String(n ?? 0)
}

onMounted(() => {
  void load()
})
</script>

<template>
  <PageHeader :title="help.title" :subtitle="help.oneLiner" help-key="overview" />

  <AsyncSection :loading="loading" :error="error" :empty="empty()" empty-text="暂无转换记录" @retry="load">
    <!-- Block 1: what this page does -->
    <v-card class="mb-4 pa-4" data-testid="block-intro">
      <div class="text-subtitle-2 mb-1">首次使用路径</div>
      <ol class="pl-4 mb-2">
        <li><router-link to="/airports">机场</router-link> —— 添加上游订阅地址（这是所有节点的来源）</li>
        <li><router-link to="/groups">分组 / 规则</router-link> —— 决定节点怎么归组、流量怎么分流</li>
        <li><router-link to="/subscriptions">订阅</router-link> —— 生成令牌，拿到客户端要填的订阅地址</li>
        <li><router-link to="/pipelines">管线</router-link> —— 需要改名/过滤/排序时，在节点进来之后加处理步骤</li>
      </ol>
      <div class="text-caption text-medium-emphasis">概览页只读，所有修改都在对应页面做。</div>
    </v-card>

    <!-- Block 2: metrics with captions -->
    <div class="metrics-grid mb-4" data-testid="block-metrics">
      <v-card variant="tonal" class="metric-card pa-4">
        <div class="text-caption text-medium-emphasis">最近记录条数</div>
        <div class="text-h5" data-testid="metric-history-total">{{ stats?.history.total ?? 0 }}</div>
        <div class="text-caption">服务进程内存里保留的最近转换记录条数（上限 20，重启清零），不是历史累计。</div>
      </v-card>
      <v-card variant="tonal" class="metric-card pa-4">
        <div class="text-caption text-medium-emphasis">缓存命中率</div>
        <div class="text-h5" data-testid="metric-cache-hit-rate">{{ ((stats?.history.cacheHitRate ?? 0) * 100).toFixed(1) }}%</div>
        <div class="text-caption">命中磁盘缓存的来源 URL 数 ÷ 总来源 URL 数。缓存命中意味着没有真的去上游拉取。</div>
      </v-card>
      <v-card variant="tonal" class="metric-card pa-4">
        <div class="text-caption text-medium-emphasis">来源 URL 总数</div>
        <div class="text-h5" data-testid="metric-url-total">{{ stats?.history.urlTotal ?? 0 }}</div>
        <div class="text-caption">这些转换请求里一共出现过多少个来源 URL（含重复）。</div>
      </v-card>
      <v-card variant="tonal" class="metric-card pa-4">
        <div class="text-caption text-medium-emphasis">日志行数</div>
        <div class="text-h5" data-testid="metric-logs">{{ stats?.logs ?? 0 }}</div>
        <div class="text-caption">日志缓冲区里的行数（有上限，超出后最早的会被丢弃）。</div>
      </v-card>
      <v-card variant="tonal" class="metric-card pa-4">
        <div class="text-caption text-medium-emphasis">节点数</div>
        <div class="text-h5" data-testid="metric-nodes">{{ lastOrDash(stats?.last?.nodes) }}</div>
        <div class="text-caption">最近一次转换输出的节点数。</div>
      </v-card>
      <v-card variant="tonal" class="metric-card pa-4">
        <div class="text-caption text-medium-emphasis">原始节点数</div>
        <div class="text-h5" data-testid="metric-raw-nodes">{{ lastOrDash(stats?.last?.rawNodes) }}</div>
        <div class="text-caption">去重/过滤之前解析到的节点数；两者差得越多，说明去重或管线过滤越有效。</div>
      </v-card>
      <v-card variant="tonal" class="metric-card pa-4">
        <div class="text-caption text-medium-emphasis">分组数</div>
        <div class="text-h5" data-testid="metric-groups">{{ lastOrDash(stats?.last?.groups) }}</div>
        <div class="text-caption">最近一次转换实际生效的分组数（模板与请求取并集后的结果）。</div>
      </v-card>
      <v-card variant="tonal" class="metric-card pa-4">
        <div class="text-caption text-medium-emphasis">规则数</div>
        <div class="text-h5" data-testid="metric-rules">{{ lastOrDash(stats?.last?.rules) }}</div>
        <div class="text-caption">最近一次转换实际生效的规则行数。</div>
      </v-card>
      <v-card variant="tonal" class="metric-card pa-4">
        <div class="text-caption text-medium-emphasis">最近一次缓存命中</div>
        <div class="text-h5" data-testid="metric-last-cached">{{ stats?.last == null ? '—' : (stats.last.cached + '/' + stats.last.urlCount) }}</div>
        <div class="text-caption">最近一次转换里，命中缓存的来源数 ÷ 来源总数。</div>
      </v-card>
    </div>
    <div v-if="stats?.last == null" class="text-caption text-medium-emphasis mb-4" data-testid="no-records-hint">还没有转换记录</div>

    <!-- Block 3: history -->
    <v-card class="mb-4" data-testid="block-history">
      <v-card-title>最近转换历史</v-card-title>
      <v-card-text>
        <div v-if="history.length === 0" data-testid="history-empty" class="text-center pa-4">
          <div class="text-subtitle-1">还没有转换记录</div>
          <div class="text-caption text-medium-emphasis">转换页点一次「转换」，或让客户端请求一次订阅地址，这里就会出现记录。</div>
          <router-link to="/convert" data-testid="history-empty-action">转换</router-link>
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

    <!-- Block 4: health/version info -->
    <v-card class="pa-4" data-testid="block-health">
      <div class="text-subtitle-2 mb-1">服务状态</div>
      <div class="text-caption text-medium-emphasis">如果这里能看到数据，说明服务在正常工作；请求不通请看「日志」页。</div>
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
.metric-card .metric-value {
  font-size: 21px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  line-height: 1.2;
  color: var(--sf-text);
}
.metric-card .metric-label {
  font-size: 12px;
  color: var(--sf-text-muted);
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
</style>
