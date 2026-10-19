<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { endpoints, type TaskEntry } from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import { pageHelp } from '../content/help'
import PageHeader from '../components/PageHeader.vue'
import AsyncSection from '../components/AsyncSection.vue'

const help = pageHelp('tasks')

const loading = ref(false)
const error = ref('')
const tasks = ref<TaskEntry[]>([])
const unavailable = ref(false)

function jobNameCn(name: string): string {
  if (name === 'rule-sources') return '规则源更新'
  if (name === 'subscheck') return '订阅测活'
  return name
}

function formatTaskDuration(ns: number): string {
  const ms = ns / 1e6
  if (ms < 1000) {
    return ms.toFixed(2) + ' ms'
  }
  return (ms / 1000).toFixed(2) + ' s'
}

function formatStartedAt(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString('zh-CN', { hour12: false })
}

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  unavailable.value = false
  try {
    tasks.value = await endpoints.listTasks()
  } catch (err) {
    const msg = errorMessage(err)
    if (msg.includes('503') || msg.includes('调度器未启用')) {
      unavailable.value = true
      tasks.value = []
    } else {
      error.value = msg
    }
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void load()
})
</script>

<template>
  <PageHeader :title="help.title" :subtitle="help.oneLiner" help-key="tasks" />

  <v-alert type="info" variant="tonal" class="mb-4">
    <div class="mb-1"><strong>这里是什么：</strong>这里只显示<strong>调度器最近执行过的任务记录</strong>（内存态，最多保留最近若干条，<strong>进程重启即清空</strong>）。它不是任务配置页。</div>
    <div data-testid="task-retry-note" class="mb-1"><strong>失败会怎样：</strong>调度任务失败会自动重试 3 次（指数退避），仍失败才告警。</div>
    <div class="mb-1"><strong>任务有哪些：</strong>正好两个——
      <ul class="mb-1">
        <li><strong>规则源更新</strong>（rule-sources）：按 cron 拉取 manifest.yaml 里所有规则源的最新版本，失败会重试 3 次（指数退避），仍失败则触发告警通道。默认 cron <code>0 4 * * *</code>（每天 04:00），配置键 <code>rules.update_cron</code>。</li>
        <li><strong>订阅测活</strong>（subscheck）：按 cron 对所有启用机场的订阅地址做可用性检查。默认 cron <code>*/30 * * * *</code>（每 30 分钟），配置键 <code>subscheck.cron</code>。<strong>受 <code>subscheck.enabled</code> 设置控制</strong>，关掉后该任务不执行。</li>
      </ul>
    </div>
    <div class="mb-1"><strong>cron 怎么改：</strong>改 cron 表达式要写配置项，本页没有编辑入口（<strong>不许自己加端点</strong>）。</div>
    <div><strong>手动触发的后果：</strong>本页<strong>没有</strong>手动触发按钮。想立刻跑一次规则源更新，去「规则源」页点「立即更新」；想立刻测活，去「机场」页点「测活」。<strong>两条路径都会真的发起网络请求并写库</strong>。</div>
  </v-alert>

  <div v-if="unavailable" data-testid="scheduler-unavailable" class="mb-4">
    <v-card class="pa-4">
      <v-card-title>调度器未启用</v-card-title>
      <v-card-text>当前进程没有装配调度器，定时任务不会执行。通常是启动时没带数据目录。</v-card-text>
      <v-card-actions>
        <v-btn variant="text" data-testid="scheduler-retry" @click="load">重试</v-btn>
      </v-card-actions>
    </v-card>
  </div>

  <AsyncSection
    :loading="loading"
    :error="error"
    :empty="tasks.length === 0 ? { title: '还没有任务执行记录', detail: '任务按 cron 到点才会执行，记录保存在内存里，进程重启后这里会重新变空。也可以去「规则源」页点「立即更新」立刻产生一条。', action: { label: '去规则源', to: '/rule-sources' } } : false"
    empty-text="暂无任务记录"
    @retry="load"
  >
    <v-table data-testid="tasks-table">
      <thead>
        <tr>
          <th>任务</th>
          <th>状态</th>
          <th>开始时间</th>
          <th>耗时</th>
          <th>错误</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="(t, i) in tasks" :key="i" data-testid="task-row">
          <td>{{ jobNameCn(t.JobName) }}</td>
          <td>
            <span class="sf-status">
              <span class="sf-status__dot" :class="t.Success ? 'sf-status__dot--ok' : 'sf-status__dot--danger'"></span>
              <span>{{ t.Success ? '成功' : '失败' }}</span>
            </span>
          </td>
          <td>{{ formatStartedAt(t.StartedAt) }}</td>
          <td>{{ formatTaskDuration(t.Duration) }}</td>
          <td>
            <pre v-if="t.Err !== ''" class="text-body-2" style="white-space: pre-wrap; word-break: break-all;">{{ t.Err }}</pre>
            <span v-else class="text-medium-emphasis">—</span>
          </td>
        </tr>
      </tbody>
    </v-table>
  </AsyncSection>
</template>
