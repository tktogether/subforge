<script setup lang="ts">
import SfCopyButton from './SfCopyButton.vue'

withDefaults(
  defineProps<{
    code: string
    label?: string
    plain?: boolean
    maxHeight?: string
  }>(),
  { label: '', plain: false, maxHeight: '' },
)
</script>

<template>
  <div class="sf-code" :class="{ 'sf-code--plain': plain }" :style="maxHeight === '' ? undefined : { maxHeight }">
    <div v-if="label !== '' || $slots.bar" class="sf-code__bar">
      <span v-if="label" class="sf-label-mono">{{ label }}</span>
      <slot name="bar" />
      <span class="sf-spacer" />
      <slot name="extra" />
      <SfCopyButton :text="code" />
    </div>
    <pre class="sf-code__body">{{ code }}</pre>
  </div>
</template>
