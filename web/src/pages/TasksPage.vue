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
  if (name === 'subscheck') return '订阅可用性检测'
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

  <p class="text-body-2 text-medium-emphasis mb-3">
    任务失败后自动重试 3 次；手动触发请前往规则源页或机场页。记录保存在内存中，服务重启后清空。
  </p>

  <div v-if="unavailable" data-testid="scheduler-unavailable" class="mb-4">
    <v-card class="pa-4">
      <v-card-title>调度器未启用</v-card-title>
      <v-card-text>当前进程未启用调度器，定时任务不会执行。通常是因为启动时未指定数据目录。</v-card-text>
      <v-card-actions>
        <v-btn variant="text" data-testid="scheduler-retry" @click="load">重试</v-btn>
      </v-card-actions>
    </v-card>
  </div>

  <AsyncSection
    :loading="loading"
    :error="error"
    :empty="tasks.length === 0 ? { title: '暂无任务记录', detail: '任务按计划执行；记录保存在内存中，服务重启后清空。也可以在规则源页立即更新一次。', action: { label: '前往规则源', to: '/rule-sources' } } : false"
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
