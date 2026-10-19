<script setup lang="ts">
import { computed } from 'vue'
import type { EmptyStateHelp } from '../content/help'

type EmptyProp = boolean | EmptyStateHelp

const props = defineProps<{
  loading: boolean
  error: string
  empty: EmptyProp
  emptyText: string
  hint?: string
}>()

const emit = defineEmits<{ retry: [] }>()

const isEmptyHelp = computed(() => typeof props.empty === 'object' && props.empty !== null)
const isEmptyBool = computed(() => props.empty === true || isEmptyHelp.value)

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
    <div v-if="props.error !== '' && props.hint" data-testid="async-hint" class="text-body-2 text-medium-emphasis mb-4">
      {{ props.hint }}
    </div>
    <v-skeleton-loader v-else-if="props.loading" type="table" data-testid="async-loading" />
    <div v-else-if="isEmptyHelp" data-testid="empty-state">
      <div data-testid="empty-state-title">{{ (props.empty as EmptyStateHelp).title }}</div>
      <div data-testid="empty-state-detail">{{ (props.empty as EmptyStateHelp).detail }}</div>
      <router-link
        v-if="(props.empty as EmptyStateHelp).action && (props.empty as EmptyStateHelp).action!.to"
        :to="(props.empty as EmptyStateHelp).action!.to as string"
        data-testid="empty-state-action"
      >
        {{ (props.empty as EmptyStateHelp).action!.label }}
      </router-link>
      <v-btn
        v-else-if="(props.empty as EmptyStateHelp).action"
        data-testid="empty-state-action"
        variant="text"
      >
        {{ (props.empty as EmptyStateHelp).action!.label }}
      </v-btn>
    </div>
    <v-alert v-else-if="isEmptyBool" type="info" variant="tonal" data-testid="async-empty">
      {{ props.emptyText }}
    </v-alert>
    <slot v-else />
  </div>
</template>
