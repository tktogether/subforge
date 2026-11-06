<script setup lang="ts">
import { computed, type Component } from 'vue'
import { IconInfoCircle, IconAlertTriangle, IconCircleCheck } from '@tabler/icons-vue'

const props = withDefaults(
  defineProps<{
    tone?: 'info' | 'warn' | 'danger' | 'ok'
  }>(),
  { tone: 'info' },
)

const icons: Record<string, Component> = {
  info: IconInfoCircle,
  warn: IconAlertTriangle,
  danger: IconAlertTriangle,
  ok: IconCircleCheck,
}

const icon = computed(() => icons[props.tone])
</script>

<template>
  <div class="sf-note" :class="'sf-note--' + tone" role="note">
    <component :is="icon" :size="16" :stroke-width="1.8" class="sf-note__icon" aria-hidden="true" />
    <div class="sf-note__body"><slot /></div>
  </div>
</template>
