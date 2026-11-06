<script setup lang="ts">
import type { NodePreviewResult } from '../../api/endpoints'
import SfDialog from './SfDialog.vue'
import { IconArrowRight, IconAlertTriangle } from '@tabler/icons-vue'

defineProps<{
  open: boolean
  title: string
  loading: boolean
  error: string
  result: NodePreviewResult | null
}>()

const emit = defineEmits<{ (e: 'close'): void }>()
</script>

<template>
  <SfDialog :open="open" label="NODE PREVIEW" :title="title" wide @close="emit('close')">
    <div v-if="loading" class="np-loading" data-testid="preview-loading">
      <span class="sf-skeleton" style="width: 130px; height: 12px" />
      <span class="sf-skeleton" style="width: 220px; height: 12px" />
      <span class="sf-faint">正在处理节点…</span>
    </div>
    <div v-else-if="error !== ''" class="sf-note sf-note--danger" data-testid="preview-error">
      <IconAlertTriangle :size="16" :stroke-width="1.8" class="sf-note__icon" />
      <span class="sf-note__body">{{ error }}</span>
    </div>
    <template v-else-if="result !== null">
      <div class="np-summary" data-testid="preview-summary">
        <span class="np-count">处理前 <b>{{ result.before_count }}</b> 个</span>
        <IconArrowRight :size="15" :stroke-width="1.8" class="sf-faint" aria-hidden="true" />
        <span class="np-count">处理后 <b>{{ result.after_count }}</b> 个</span>
        <span v-if="result.pipeline" class="sf-badge sf-badge--accent">{{ result.pipeline }}</span>
      </div>
      <div class="np-columns">
        <div data-testid="preview-before-table">
          <div class="np-column-title sf-label-mono">BEFORE / 处理前</div>
          <div class="np-table-wrap">
            <table class="sf-table">
              <thead><tr><th>名称</th><th>类型</th><th>服务器</th></tr></thead>
              <tbody>
                <tr v-for="(n, i) in result.before" :key="'b' + i">
                  <td class="np-name">{{ n.name }}</td>
                  <td class="sf-faint">{{ n.type }}</td>
                  <td class="sf-table__num" :title="n.server + ':' + n.port">{{ n.server }}:{{ n.port }}</td>
                </tr>
                <tr v-if="result.before.length === 0"><td colspan="3" class="np-empty">没有节点</td></tr>
              </tbody>
            </table>
          </div>
        </div>
        <div data-testid="preview-after-table">
          <div class="np-column-title sf-label-mono">AFTER / 处理后</div>
          <div class="np-table-wrap">
            <table class="sf-table">
              <thead><tr><th>名称</th><th>类型</th><th>服务器</th></tr></thead>
              <tbody>
                <tr v-for="(n, i) in result.after" :key="'a' + i">
                  <td class="np-name">{{ n.name }}</td>
                  <td class="sf-faint">{{ n.type }}</td>
                  <td class="sf-table__num" :title="n.server + ':' + n.port">{{ n.server }}:{{ n.port }}</td>
                </tr>
                <tr v-if="result.after.length === 0"><td colspan="3" class="np-empty">处理后没有节点</td></tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
      <div v-if="result.warnings !== undefined && result.warnings.length > 0" class="sf-note sf-note--warn" data-testid="preview-warnings">
        <span class="sf-note__body">
          <div v-for="(w, i) in result.warnings" :key="i">{{ w }}</div>
        </span>
      </div>
    </template>
    <template #foot>
      <button class="sf-btn sf-btn--ghost" type="button" data-testid="preview-close" @click="emit('close')">关闭</button>
    </template>
  </SfDialog>
</template>

<style scoped>
.np-loading {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 20px 0;
  color: var(--sf-text-muted);
}

.np-summary {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}

.np-count {
  color: var(--sf-text-muted);
  font-size: 13px;
}

.np-count b {
  font: 600 16px/1 var(--sf-font-display);
  color: var(--sf-text);
  font-variant-numeric: tabular-nums;
}

.np-columns {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

@media (max-width: 700px) {
  .np-columns {
    grid-template-columns: minmax(0, 1fr);
  }
}

.np-column-title {
  margin-bottom: 8px;
}

.np-table-wrap {
  max-height: 320px;
  overflow-y: auto;
  border: 1px solid var(--sf-border);
  border-radius: var(--sf-radius);
}

.np-name {
  max-width: 180px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.np-empty {
  padding: 14px;
  color: var(--sf-text-faint);
  text-align: center;
  font-size: 13px;
}
</style>
