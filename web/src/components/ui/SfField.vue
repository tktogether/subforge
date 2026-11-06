<script setup lang="ts">
withDefaults(
  defineProps<{
    label?: string
    hint?: string
    error?: string
    required?: boolean
  }>(),
  { label: '', hint: '', error: '' },
)
</script>

<template>
  <div class="sf-field" :class="{ 'sf-field--invalid': error !== '' }">
    <span v-if="label !== '' || $slots.label !== undefined" class="sf-field__label">
      <slot name="label">
        {{ label }}
        <span v-if="required" aria-hidden="true" style="color: var(--sf-danger)">*</span>
      </slot>
    </span>
    <slot />
    <p v-if="error" class="sf-field__error">{{ error }}</p>
    <p v-else-if="hint" class="sf-field__hint">{{ hint }}</p>
  </div>
</template>
