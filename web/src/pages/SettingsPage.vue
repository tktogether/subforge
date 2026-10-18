<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { endpoints } from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import { useThemeMode } from '../composables/useThemeMode'
import PageHeader from '../components/PageHeader.vue'

const targets = ref<string[]>([])
const statsHistory = ref(0)
const error = ref('')
const theme = useThemeMode()

async function load(): Promise<void> {
  try {
    const [t, s] = await Promise.all([endpoints.targets(), endpoints.stats()])
    targets.value = t
    statsHistory.value = s.history.total
  } catch (err) {
    error.value = errorMessage(err)
  }
}

onMounted(() => {
  void load()
})
</script>

<template>
  <PageHeader title="设置" subtitle="能力清单与主题" />
  <v-card variant="tonal" class="pa-4 mb-4">
    <div class="mb-2">支持的目标格式：{{ targets.length === 0 ? '（无）' : targets.join(', ') }}</div>
    <div class="mb-2">历史条数：{{ statsHistory }}</div>
    <div class="d-flex ga-2">
      <v-btn variant="tonal" @click="theme.setMode('light')">浅色</v-btn>
      <v-btn variant="tonal" @click="theme.setMode('dark')">深色</v-btn>
      <v-btn variant="tonal" @click="theme.setMode('system')">跟随系统</v-btn>
    </div>
    <div v-if="error !== ''" class="text-error mt-2">{{ error }}</div>
  </v-card>
</template>
