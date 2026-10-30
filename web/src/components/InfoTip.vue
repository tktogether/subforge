<script setup lang="ts">
const props = defineProps<{
  text?: string
  example?: string
  label?: string
}>()
</script>

<template>
  <span v-if="props.text || props.example" class="sf-info-tip-wrap">
    <v-tooltip location="top" max-width="360" open-on-click open-on-focus content-class="sf-tooltip">
      <template #activator="{ props: activatorProps }">
        <v-btn
          v-bind="activatorProps"
          icon="mdi-help-circle-outline"
          variant="text"
          size="x-small"
          density="comfortable"
          class="sf-info-tip"
          :aria-label="props.label ?? '查看说明'"
          data-testid="info-tip"
        />
      </template>
      <div v-if="props.text" class="text-body-2">{{ props.text }}</div>
      <div v-if="props.example" class="text-caption mt-1">示例：{{ props.example }}</div>
    </v-tooltip>
    <span class="sf-visually-hidden" data-testid="info-tip-text">
      {{ props.text }}<template v-if="props.example"> 示例：{{ props.example }}</template>
    </span>
  </span>
</template>

<style scoped>
.sf-info-tip-wrap {
  display: inline-flex;
  align-items: center;
}
.sf-info-tip {
  margin-inline: 2px;
  color: var(--sf-text-muted);
}
.sf-visually-hidden {
  position: absolute;
  width: 1px;
  height: 1px;
  margin: -1px;
  padding: 0;
  overflow: hidden;
  clip: rect(0 0 0 0);
  white-space: nowrap;
  border: 0;
}
</style>
