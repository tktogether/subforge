<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { endpoints, type TaskEntry } from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import PageHeader from '../components/PageHeader.vue'
import AsyncSection from '../components/AsyncSection.vue'

const loading = ref(false)
const error = ref('')
const tasks = ref<TaskEntry[]>([])
const unavailable = ref(false)

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
  <PageHeader title="任务" subtitle="定时任务执行历史" />
  <div class="d-flex ga-2 mb-4">
    <v-btn variant="tonal" @click="load">手动刷新</v-btn>
  </div>
  <div v-if="unavailable" class="text-medium-emphasis mb-4">调度器未启用</div>
  <AsyncSection :loading="loading" :error="error" :empty="tasks.length === 0" empty-text="暂无任务记录" @retry="load">
    <v-table>
      <thead>
        <tr>
          <th>任务</th>
          <th>状态</th>
          <th>开始时间</th>
          <th>耗时(ms)</th>
          <th>错误</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="(t, i) in tasks" :key="i">
          <td>{{ t.JobName }}</td>
          <td>{{ t.Success ? '成功' : '失败' }}</td>
          <td>{{ t.StartedAt }}</td>
          <td>{{ t.Duration }}</td>
          <td>{{ t.Err }}</td>
        </tr>
      </tbody>
    </v-table>
  </AsyncSection>
</template>
