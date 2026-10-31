<script setup lang="ts">
import type { NodePreviewResult } from '../api/endpoints'

defineProps<{
  modelValue: boolean
  title: string
  loading: boolean
  error: string
  result: NodePreviewResult | null
}>()

const emit = defineEmits<{ 'update:modelValue': [boolean] }>()

function close(): void {
  emit('update:modelValue', false)
}
</script>

<template>
  <v-dialog :model-value="modelValue" max-width="820" scrollable data-testid="node-preview-dialog" @update:model-value="emit('update:modelValue', $event)">
    <v-card>
      <v-card-title>{{ title }}</v-card-title>
      <v-card-text>
        <div v-if="loading" class="preview-loading" data-testid="preview-loading">
          <v-progress-circular indeterminate size="22" />
          <span>正在处理节点…</span>
        </div>
        <v-alert v-else-if="error !== ''" type="error" variant="tonal" data-testid="preview-error">{{ error }}</v-alert>
        <template v-else-if="result !== null">
          <div class="preview-summary" data-testid="preview-summary">
            <span class="preview-count">处理前 <strong>{{ result.before_count }}</strong> 个</span>
            <v-icon size="16" icon="mdi-arrow-right" />
            <span class="preview-count">处理后 <strong>{{ result.after_count }}</strong> 个</span>
            <v-chip v-if="result.pipeline" size="small" variant="tonal">{{ result.pipeline }}</v-chip>
          </div>
          <div class="preview-columns">
            <div>
              <div class="preview-column-title">处理前</div>
              <div class="preview-table-wrap">
                <v-table density="compact" class="sf-table-rows preview-table" data-testid="preview-before-table">
                  <thead><tr><th>名称</th><th>类型</th><th>服务器</th></tr></thead>
                  <tbody>
                    <tr v-for="(n, i) in result.before" :key="'b' + i">
                      <td>{{ n.name }}</td><td>{{ n.type }}</td><td :title="n.server + ':' + n.port">{{ n.server }}:{{ n.port }}</td>
                    </tr>
                  </tbody>
                </v-table>
              </div>
              <div v-if="result.before.length === 0" class="preview-empty">没有节点</div>
            </div>
            <div>
              <div class="preview-column-title">处理后</div>
              <div class="preview-table-wrap">
                <v-table density="compact" class="sf-table-rows preview-table" data-testid="preview-after-table">
                  <thead><tr><th>名称</th><th>类型</th><th>服务器</th></tr></thead>
                  <tbody>
                    <tr v-for="(n, i) in result.after" :key="'a' + i">
                      <td>{{ n.name }}</td><td>{{ n.type }}</td><td :title="n.server + ':' + n.port">{{ n.server }}:{{ n.port }}</td>
                    </tr>
                  </tbody>
                </v-table>
              </div>
              <div v-if="result.after.length === 0" class="preview-empty">处理后没有节点</div>
            </div>
          </div>
          <v-alert v-if="result.warnings && result.warnings.length > 0" type="warning" variant="tonal" class="mt-3" data-testid="preview-warnings">
            <div v-for="(w, i) in result.warnings" :key="i">{{ w }}</div>
          </v-alert>
        </template>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" data-testid="preview-close" @click="close">关闭</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<style scoped>
.preview-loading {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 24px 0;
  color: var(--sf-text-muted);
}
.preview-summary {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
}
.preview-count {
  color: var(--sf-text-muted);
  font-size: 13px;
}
.preview-count strong {
  color: var(--sf-text);
}
.preview-columns {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}
@media (max-width: 700px) {
  .preview-columns {
    grid-template-columns: minmax(0, 1fr);
  }
}
.preview-column-title {
  margin-bottom: 6px;
  font-size: 13px;
  font-weight: 600;
}
.preview-table-wrap {
  max-height: 320px;
  overflow-y: auto;
  border: 1px solid var(--sf-border);
  border-radius: 6px;
}
.preview-table {
  border: 0;
  border-radius: 0;
}
.preview-table :deep(td) {
  max-width: 180px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.preview-empty {
  padding: 12px;
  border: 1px dashed var(--sf-border);
  border-radius: 6px;
  color: var(--sf-text-muted);
  font-size: 13px;
  text-align: center;
}
</style>
