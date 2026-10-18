<script setup lang="ts">
const props = defineProps<{
  loading: boolean
  error: string
  empty: boolean
  emptyText: string
}>()

const emit = defineEmits<{ retry: [] }>()

function retry(): void {
  emit('retry')
}
</script>

<template>
  <div>
    <v-alert v-if="props.error !== ''" type="error" variant="tonal" class="mb-4" data-testid="async-error">
      {{ props.error }}
      <template #append>
        <v-btn variant="text" size="small" data-testid="async-retry" @click="retry">重试</v-btn>
      </template>
    </v-alert>
    <v-skeleton-loader v-else-if="props.loading" type="table" data-testid="async-loading" />
    <v-alert v-else-if="props.empty" type="info" variant="tonal" data-testid="async-empty">
      {{ props.emptyText }}
    </v-alert>
    <slot v-else />
  </div>
</template>
