<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { endpoints, type RulesetsResponse } from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import PageHeader from '../components/PageHeader.vue'
import AsyncSection from '../components/AsyncSection.vue'

const loading = ref(false)
const error = ref('')
const data = ref<RulesetsResponse | null>(null)

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    data.value = await endpoints.rulesets('')
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void load()
})
</script>

<template>
  <PageHeader title="规则源" subtitle="规则集映射与规则列表" />
  <AsyncSection :loading="loading" :error="error" :empty="data === null || (Object.keys(data.rulesets).length === 0 && data.rules.length === 0)" empty-text="暂无规则源" @retry="load">
    <v-card class="mb-4" variant="tonal">
      <v-card-title>规则集映射</v-card-title>
      <v-data-table :items="Object.entries(data?.rulesets ?? {}).map(([k, v]) => ({ name: k, path: v }))" density="comfortable" hover>
        <template #item.name="{ value }">{{ value }}</template>
        <template #item.path="{ value }">{{ value }}</template>
      </v-data-table>
    </v-card>
    <v-card variant="tonal">
      <v-card-title>规则列表</v-card-title>
      <v-data-table :items="(data?.rules ?? []).map((r) => ({ rule: r }))" density="comfortable" hover>
        <template #item.rule="{ value }">{{ value }}</template>
      </v-data-table>
    </v-card>
  </AsyncSection>
</template>
