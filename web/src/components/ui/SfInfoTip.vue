<script setup lang="ts">
import { IconInfoCircle } from '@tabler/icons-vue'

defineProps<{ text: string }>()
</script>

<template>
  <span class="sf-tip" :data-tip="text" tabindex="0" role="note" :aria-label="text" style="display: inline-flex; cursor: help">
    <IconInfoCircle :size="13.5" :stroke-width="1.8" style="color: var(--sf-text-faint)" aria-hidden="true" />
  </span>
</template>
