<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { endpoints, type ConvertResponse } from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import PageHeader from '../components/PageHeader.vue'
import AsyncSection from '../components/AsyncSection.vue'

const targets = ref<string[]>([])
const pipelines = ref<string[]>([])
const selectedTarget = ref('')
const urls = ref('')
const text = ref('')
const include = ref('')
const exclude = ref('')
const pipeline = ref('')
const sort = ref('false')
const list = ref(false)
const loading = ref(false)
const error = ref('')
const result = ref<ConvertResponse | null>(null)
const copied = ref(false)

async function loadMeta(): Promise<void> {
  try {
    const [t, p] = await Promise.all([endpoints.targets(), endpoints.pipelines('')])
    targets.value = t
    pipelines.value = p
  } catch (err) {
    error.value = errorMessage(err)
  }
}

async function submit(): Promise<void> {
  if (urls.value.trim() === '' && text.value.trim() === '') {
    error.value = 'URL 与文本不能同时为空'
    return
  }
  loading.value = true
  error.value = ''
  result.value = null
  try {
    result.value = await endpoints.convert({
      target: selectedTarget.value,
      urls: urls.value.split('\n').map((s) => s.trim()).filter((s) => s !== ''),
      text: text.value,
      types: [],
      include: include.value.split(',').map((s) => s.trim()).filter((s) => s !== ''),
      exclude: exclude.value.split(',').map((s) => s.trim()).filter((s) => s !== ''),
      pipeline: pipeline.value,
      config: '',
      sort: sort.value,
      list: list.value,
      override: '',
    })
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

async function copy(): Promise<void> {
  if (result.value === null) return
  try {
    await navigator.clipboard.writeText(result.value.output)
    copied.value = true
  } catch {
    copied.value = false
  }
}

onMounted(() => {
  void loadMeta()
})
</script>

<template>
  <PageHeader title="转换" subtitle="将订阅转换为目标格式" />
  <v-row>
    <v-col cols="12" md="6">
      <v-card variant="tonal" class="pa-4">
        <v-select v-model="selectedTarget" :items="targets" label="目标格式" />
        <v-textarea v-model="urls" label="URL（每行一个）" rows="4" />
        <v-textarea v-model="text" label="粘贴文本" rows="3" />
        <v-text-field v-model="include" label="包含正则" />
        <v-text-field v-model="exclude" label="排除正则" />
        <v-select v-model="pipeline" :items="pipelines" label="管线" />
        <v-switch v-model="list" label="输出节点列表" color="primary" />
        <v-btn color="primary" :loading="loading" @click="submit">转换</v-btn>
      </v-card>
    </v-col>
    <v-col cols="12" md="6">
      <AsyncSection :loading="loading" :error="error" :empty="result === null" empty-text="结果将显示在这里">
        <v-textarea :model-value="result?.output ?? ''" readonly rows="12" />
        <v-btn variant="text" @click="copy">{{ copied ? '已复制' : '复制' }}</v-btn>
      </AsyncSection>
    </v-col>
  </v-row>
</template>
