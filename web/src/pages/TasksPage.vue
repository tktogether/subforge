<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { IconRefresh } from '@tabler/icons-vue'
import { endpoints, type TaskEntry } from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import SfPageHead from '../components/ui/SfPageHead.vue'
import SfEmpty from '../components/ui/SfEmpty.vue'
import SfNote from '../components/ui/SfNote.vue'
import { IconClock, IconAlertTriangle } from '@tabler/icons-vue'

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
  <div>
    <SfPageHead>
      <template #actions>
        <button class="sf-btn sf-btn--ghost" type="button" data-testid="refresh" :disabled="loading" @click="load">
          <IconRefresh :size="15" :stroke-width="1.8" :class="{ 'tk-spin': loading }" />
          刷新
        </button>
      </template>
    </SfPageHead>

    <SfNote tone="info" class="sf-rise sf-d1" style="margin-bottom: 16px">
      任务失败后自动重试 3 次；手动触发请前往规则源页或机场页。记录保存在内存中，服务重启后清空。
    </SfNote>

    <div v-if="unavailable" data-testid="scheduler-unavailable" style="margin-bottom: 16px">
      <SfNote tone="warn">
        <b>调度器未启用</b>——当前进程未启用调度器，定时任务不会执行。通常是因为启动时未指定数据目录。
        <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" data-testid="scheduler-retry" style="margin-left: 10px" @click="load">重试</button>
      </SfNote>
    </div>

    <div v-if="error !== ''" class="sf-note sf-note--danger sf-rise" style="margin-bottom: 16px">
      <IconAlertTriangle :size="16" :stroke-width="1.8" class="sf-note__icon" />
      <span class="sf-note__body">{{ error }}</span>
      <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" @click="load">重试</button>
    </div>

    <div v-if="tasks.length === 0 && !unavailable && error === '' && !loading" class="sf-panel">
      <SfEmpty
        title="暂无任务记录"
        desc="任务按计划执行；记录保存在内存中，服务重启后清空。也可以在规则源页立即更新一次。"
      >
        <template #icon><IconClock :size="30" :stroke-width="1.4" /></template>
        <template #action>
          <RouterLink to="/rule-sources" class="sf-btn sf-btn--primary sf-btn--sm">前往规则源</RouterLink>
        </template>
      </SfEmpty>
    </div>

    <div v-else-if="tasks.length > 0" class="sf-table-wrap sf-rise sf-d1" data-testid="tasks-table">
      <table class="sf-table">
        <thead>
          <tr>
            <th>任务</th>
            <th>状态</th>
            <th>开始时间</th>
            <th style="text-align: right">耗时</th>
            <th>错误</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(t, i) in tasks" :key="i" data-testid="task-row">
            <td>{{ jobNameCn(t.JobName) }}</td>
            <td>
              <span class="sf-status">
                <span class="sf-led" :class="t.Success ? 'sf-led--ok' : 'sf-led--danger'" />
                <span>{{ t.Success ? '成功' : '失败' }}</span>
              </span>
            </td>
            <td class="sf-table__num">{{ formatStartedAt(t.StartedAt) }}</td>
            <td class="sf-table__num" style="text-align: right">{{ formatTaskDuration(t.Duration) }}</td>
            <td style="max-width: 420px">
              <span v-if="t.Err !== ''" class="tk-err">{{ t.Err }}</span>
              <span v-else class="sf-faint">—</span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
.tk-err {
  font-family: var(--sf-font-code);
  font-size: 12px;
  color: var(--sf-danger);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  line-height: 1.6;
}

.tk-spin {
  animation: tk-rotate 0.9s linear infinite;
}

@keyframes tk-rotate {
  to {
    transform: rotate(360deg);
  }
}
</style>
