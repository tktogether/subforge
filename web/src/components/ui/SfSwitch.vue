<script setup lang="ts">
withDefaults(
  defineProps<{
    modelValue: boolean
    label?: string
    disabled?: boolean
  }>(),
  { label: '', disabled: false },
)

const emit = defineEmits<{ (e: 'update:modelValue', value: boolean): void }>()
</script>

<template>
  <label class="sf-switch">
    <input
      type="checkbox"
      role="switch"
      :checked="modelValue"
      :disabled="disabled"
      @change="emit('update:modelValue', ($event.target as HTMLInputElement).checked)"
    />
    <span class="sf-switch__track" aria-hidden="true" />
    <span v-if="label" class="sf-switch__label">{{ label }}</span>
  </label>
</template>

<style scoped>
.sf-switch__label {
  font-size: 13px;
  color: var(--sf-text-muted);
  margin-left: 9px;
  user-select: none;
}
</style>
