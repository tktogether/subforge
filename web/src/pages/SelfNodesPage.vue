<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { IconEye } from '@tabler/icons-vue'
import SfResourcePage from '../components/ui/SfResourcePage.vue'
import SfNodePreview from '../components/ui/SfNodePreview.vue'
import SfField from '../components/ui/SfField.vue'
import { endpoints, type NodePreviewResult } from '../api/endpoints'
import { notify } from '../composables/useNotify'
import { errorMessage } from '../composables/useAsync'

const pipeline = ref('')
const pipelineOptions = ref<{ title: string; value: string }[]>([])
const pipelineSaving = ref(false)
const previewOpen = ref(false)
const previewLoading = ref(false)
const previewError = ref('')
const previewResult = ref<NodePreviewResult | null>(null)
const formatsOpen = ref(false)

async function previewProcess(): Promise<void> {
  previewOpen.value = true
  previewLoading.value = true
  previewError.value = ''
  previewResult.value = null
  try {
    previewResult.value = await endpoints.previewSelfNodesProcess(pipeline.value.trim())
  } catch (err) {
    previewError.value = errorMessage(err)
  } finally {
    previewLoading.value = false
  }
}

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
  { key: 'link', title: '链接', mono: true, width: '40%' },
  { key: 'enabled', title: '启用', width: '110px' },
]

const selfNodeFields = [
  { key: 'name', label: '名称', type: 'text', hint: '输出到客户端显示的名字；留空则用链接里自带的名字', example: '自建-香港' },
  { key: 'link', label: '分享链接', type: 'text', hint: '标准分享链接，以 ss:// / vmess:// / vless:// / trojan:// / hysteria2:// 开头', example: 'vless://uuid@1.2.3.4:443?encryption=none&security=tls#香港' },
  { key: 'enabled', label: '启用', type: 'switch', hint: '关闭后不参与输出' },
]

const resource = {
  list: endpoints.listSelfNodes as unknown as () => Promise<Record<string, unknown>[]>,
  create: (item: Record<string, unknown>) => endpoints.createSelfNode(item as unknown as Parameters<typeof endpoints.createSelfNode>[0]) as unknown as Promise<unknown>,
  update: (item: Record<string, unknown>) => endpoints.updateSelfNode(item['id'] as number, item as unknown as Parameters<typeof endpoints.updateSelfNode>[1]) as unknown as Promise<unknown>,
  remove: (item: Record<string, unknown>) => endpoints.deleteSelfNode(item['id'] as number) as unknown as Promise<unknown>,
}
</script>

<template>
  <div>

    <SfResourcePage
      page-title="自建节点"
      :columns="selfNodeColumns"
      :fields="selfNodeFields"
      :resource="resource"
      empty-title="还没有自建节点"
      empty-desc="添加一条标准分享链接（ss:// / vmess:// / vless:// / trojan:// / hysteria2://）。"
    >
      <template #cell-link="{ value }">
        <span class="sf-mono link-cell" :title="String(value)">{{ value }}</span>
      </template>
    </SfResourcePage>

    <section class="sf-panel sf-rise sf-d1" data-testid="self-source-card" style="margin-bottom: 16px">
      <div class="sf-panel__head">
        <span class="sf-panel__title">自建节点处理</span>
      </div>
      <div class="sf-panel__body">
        <p class="sf-field__hint" style="margin: 0 0 12px">
          自建节点先经过这条管线，再与机场来源合并；留空表示不处理。
        </p>
        <div class="source-row">
          <SfField label="处理管线" style="max-width: 360px; flex: 1 1 240px">
            <span class="sf-select-wrap">
              <select v-model="pipeline" class="sf-select" data-testid="self-source-pipeline">
                <option value="">不处理</option>
                <option v-for="item in pipelineItems" :key="item.value" :value="item.value">{{ item.title }}</option>
              </select>
            </span>
          </SfField>
          <button class="sf-btn sf-btn--primary" type="button" data-testid="self-source-save" :disabled="pipelineSaving" @click="saveSourcePipeline">
            {{ pipelineSaving ? '保存中…' : '保存管线' }}
          </button>
          <button class="sf-btn sf-btn--ghost" type="button" data-testid="self-preview-process" @click="previewProcess">
            <IconEye :size="15" :stroke-width="1.7" />
            预览处理
          </button>
        </div>
      </div>
    </section>

    <section class="sf-panel sf-rise sf-d2" style="margin-top: 16px" data-testid="self-node-formats">
      <button type="button" class="formats-head" @click="formatsOpen = !formatsOpen">
        <span class="sf-panel__title">查看格式示例</span>
        <span class="sf-badge">ss:// vmess:// vless://</span>
      </button>
      <div class="sf-collapse" :class="{ 'is-open': formatsOpen }">
        <div class="sf-collapse__inner">
          <ul class="formats-list">
            <li><code>ss://base64(method:password)@server:port#名称</code></li>
            <li><code>vmess://base64({...})</code></li>
            <li><code>vless://uuid@server:port?encryption=none&security=tls#名称</code></li>
            <li><code>trojan://password@server:port?sni=example.com#名称</code></li>
          </ul>
          <p class="sf-field__hint" style="margin: 0; padding: 0 18px 16px">以上均为示例，请替换成自己的服务器信息。</p>
        </div>
      </div>
    </section>

    <SfNodePreview
      :open="previewOpen"
      title="自建节点处理预览"
      :loading="previewLoading"
      :error="previewError"
      :result="previewResult"
      @close="previewOpen = false"
    />
  </div>
</template>

<style scoped>
.source-row {
  display: flex;
  gap: 10px;
  align-items: flex-end;
  flex-wrap: wrap;
}

.link-cell {
  display: inline-block;
  max-width: 380px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  vertical-align: middle;
  font-size: 12px;
}

.formats-head {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 13px 18px;
  background: none;
  border: none;
  color: var(--sf-text);
  cursor: pointer;
  font: inherit;
  text-align: left;
}

.formats-list {
  margin: 0;
  padding: 4px 18px 10px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-size: 13px;
}

.formats-list code {
  font-family: var(--sf-font-code);
  font-size: 12.5px;
  color: var(--sf-text-muted);
  background: var(--sf-surface-dim);
  border: 1px solid var(--sf-border);
  border-radius: 3px;
  padding: 2px 7px;
  display: inline-block;
  overflow-wrap: anywhere;
}
</style>
