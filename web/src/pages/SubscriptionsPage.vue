<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { endpoints, subscribeUrl } from '../api/endpoints'
import type { CrudResource } from '../composables/useCrud'
import PageHeader from '../components/PageHeader.vue'
import ResourcePage from '../components/ResourcePage.vue'
import { notify } from '../composables/useNotify'

type Row = Record<string, any>

const tokens = ref<any[]>([])
const selectedToken = ref('')
const origin = ref(window.location.origin)
const copied = ref(false)

const resource: CrudResource<Row> = {
  list: async () => {
    const r = await endpoints.listTokens()
    tokens.value = r
    return r as unknown as Row[]
  },
  create: endpoints.createToken as unknown as (item: Row) => Promise<unknown>,
  update: async (item: Row) => item,
  remove: async (item: Row) => endpoints.deleteToken(String(item.token)),
}

const selected = computed(() => tokens.value.find((t) => t.token === selectedToken.value) ?? null)

async function copyUrl(): Promise<void> {
  if (selected.value === null) return
  try {
    await navigator.clipboard.writeText(subscribeUrl(origin.value, selected.value.token))
    copied.value = true
    notify('已复制', 'ok')
  } catch {
    copied.value = false
    notify('复制失败', 'error')
  }
}

onMounted(() => {
  void resource.list()
})
</script>

<template>
  <PageHeader title="订阅" subtitle="令牌与订阅 URL" />
  <ResourcePage
    title="令牌"
    subtitle="订阅令牌"
    :columns="[{ key: 'token', title: '令牌' }, { key: 'name', title: '名称' }]"
    :fields="[{ key: 'token', label: '令牌', type: 'text' }, { key: 'name', label: '名称', type: 'text' }]"
    :resource="resource"
  />
  <v-card class="mt-4" variant="tonal">
    <v-card-title>订阅 URL</v-card-title>
    <v-card-text>
      <v-select v-model="selectedToken" :items="tokens.map((t) => t.token)" label="选择令牌" />
      <div v-if="selected !== null">
        <code>{{ subscribeUrl(origin, selected.token) }}</code>
        <v-btn variant="text" @click="copyUrl">{{ copied ? '已复制' : '复制' }}</v-btn>
      </div>
      <div v-else class="text-medium-emphasis">请选择一个令牌</div>
    </v-card-text>
  </v-card>
</template>
