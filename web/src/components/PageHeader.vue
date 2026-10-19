<script setup lang="ts">
import HelpPanel from './HelpPanel.vue'
import { pageHelp } from '../content/help'

const props = defineProps<{ title: string; subtitle: string; helpKey?: string }>()
</script>

<template>
  <div class="page-header">
    <div class="page-header__main">
      <h1 class="page-header__title">{{ props.title }}</h1>
      <p class="text-body-2 page-header__subtitle">{{ props.subtitle }}</p>
    </div>
    <v-spacer />
    <div class="page-header__actions d-flex ga-2">
      <slot name="actions" />
    </div>
  </div>
  <!-- 副标题已经承载了该页 oneLiner，帮助面板的入口行改用固定标签，避免同一句话出现两遍。 -->
  <HelpPanel v-if="props.helpKey" :help="pageHelp(props.helpKey)" label="使用说明" />
</template>

<style scoped>
.page-header {
  display: flex;
  align-items: center;
  gap: var(--sf-space);
  margin-bottom: 16px;
}
.page-header__main {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.page-header__title {
  font-size: 18px;
  font-weight: 600;
  color: var(--sf-text);
  margin: 0;
}
.page-header__subtitle {
  font-size: 13px;
  color: var(--sf-text-muted);
  margin: 0;
}
</style>
