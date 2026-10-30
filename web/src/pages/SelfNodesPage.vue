<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { pageHelp } from '../content/help'
import ResourcePage from '../components/ResourcePage.vue'
import { endpoints } from '../api/endpoints'
import { notify } from '../composables/useNotify'
import { errorMessage } from '../composables/useAsync'

const help = pageHelp('self-nodes')

const pipeline = ref('')
const pipelineOptions = ref<{ title: string; value: string }[]>([])
const pipelineSaving = ref(false)
const pipelineItems = computed(() => {
  const items = [...pipelineOptions.value]
  const current = pipeline.value.trim()
  if (current !== '' && !items.some((i) => i.value === current)) {
    items.unshift({ title: current, value: current })
  }
  return items
})

async function loadSourcePipeline(): Promise<void> {
  try {
    const [source, list] = await Promise.all([endpoints.getSelfSourcePipeline(), endpoints.pipelines('')])
    pipeline.value = source.pipeline
    pipelineOptions.value = list.filter((p) => p.enabled).map((p) => ({ title: p.name, value: p.name }))
  } catch {
    // 来源处理配置读取失败不阻塞自建节点列表
  }
}

async function saveSourcePipeline(): Promise<void> {
  pipelineSaving.value = true
  try {
    await endpoints.setSelfSourcePipeline(pipeline.value.trim())
    notify('已保存自建节点处理管线')
  } catch (err) {
    notify(errorMessage(err), 'error')
  } finally {
    pipelineSaving.value = false
  }
}

onMounted(() => {
  void loadSourcePipeline()
})

const selfNodeColumns = [
  { key: 'name', title: '名称' },
  { key: 'link', title: '链接' },
  { key: 'enabled', title: '启用' },
]

const selfNodeFields = [
  { key: 'name', label: '名称', type: 'text', hint: '输出到客户端显示的名字；留空则用链接里自带的名字', example: '自建-香港' },
  { key: 'link', label: '分享链接', type: 'text', hint: '标准分享链接，以 ss:// / vmess:// / vless:// / trojan:// / hysteria2:// 开头', example: 'vless://uuid@1.2.3.4:443?encryption=none&security=tls#香港' },
  { key: 'enabled', label: '启用', type: 'switch', hint: '关闭后不参与输出' },
]

const resource: any = {
  list: endpoints.listSelfNodes,
  create: endpoints.createSelfNode,
  update: (item: any) => endpoints.updateSelfNode(item.id, item),
  remove: (item: any) => endpoints.deleteSelfNode(item.id),
}
</script>

<template>
  <div class="self-nodes-page">
    <p class="text-body-2 text-medium-emphasis mb-3">
      自建节点与机场节点合并输出。分享链接需为标准格式，保存时会校验名称、链接与必要字段。
    </p>

    <v-card class="mb-4 pa-4" data-testid="self-source-card">
      <div class="text-subtitle-2 mb-1">自建节点处理</div>
      <div class="text-caption text-medium-emphasis mb-3">
        所有选中的自建节点作为同一个来源，先经过这条管线，再与机场来源合并；留空表示不处理。
      </div>
      <div class="d-flex align-start ga-3">
        <v-select
          v-model="pipeline"
          :items="pipelineItems"
          label="处理管线"
          variant="outlined"
          density="comfortable"
          hide-details
          clearable
          data-testid="self-source-pipeline"
          class="self-source-select"
        />
        <v-btn color="primary" :loading="pipelineSaving" data-testid="self-source-save" @click="saveSourcePipeline">保存</v-btn>
      </div>
    </v-card>

    <ResourcePage
      title="自建节点"
      :subtitle="help.oneLiner"
      help-key="self-nodes"
      :columns="selfNodeColumns"
      :fields="selfNodeFields"
      :resource="resource"
      :empty="help.emptyState"
    />

    <v-expansion-panels class="mt-4" data-testid="self-node-formats">
      <v-expansion-panel>
        <v-expansion-panel-title>查看格式示例</v-expansion-panel-title>
        <v-expansion-panel-text>
          <ul class="text-body-2">
            <li><code>ss://base64(method:password)@server:port#名称</code></li>
            <li><code>vmess://base64({...})</code></li>
            <li><code>vless://uuid@server:port?encryption=none&security=tls#名称</code></li>
            <li><code>trojan://password@server:port?sni=example.com#名称</code></li>
          </ul>
          <p class="text-caption text-medium-emphasis mb-0">以上均为示例，请替换成自己的服务器信息。</p>
        </v-expansion-panel-text>
      </v-expansion-panel>
    </v-expansion-panels>
  </div>
</template>

<style scoped>
.self-source-select {
  max-width: 360px;
  flex: 1 1 240px;
}
</style>
