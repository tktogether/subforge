<script setup lang="ts">
import { IconInbox } from '@tabler/icons-vue'

withDefaults(
  defineProps<{
    title?: string
    desc?: string
  }>(),
  { title: '暂无数据', desc: '' },
)
</script>

<template>
  <div class="sf-empty">
    <div class="sf-empty__icon">
      <slot name="icon"><IconInbox :size="30" :stroke-width="1.4" /></slot>
    </div>
    <div class="sf-empty__title">{{ title }}</div>
    <p v-if="desc" class="sf-empty__desc">{{ desc }}</p>
    <div v-if="$slots.action" class="sf-empty__action">
      <slot name="action" />
    </div>
  </div>
</template>
