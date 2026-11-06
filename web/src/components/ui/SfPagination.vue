<script setup lang="ts">
import { computed } from 'vue'
import { IconChevronLeft, IconChevronRight } from '@tabler/icons-vue'

const props = defineProps<{
  total: number
  limit: number
  offset: number
}>()

const emit = defineEmits<{ (e: 'update:offset', value: number): void }>()

const page = computed(() => Math.floor(props.offset / Math.max(props.limit, 1)) + 1)
const pages = computed(() => Math.max(1, Math.ceil(props.total / Math.max(props.limit, 1))))

function go(next: number): void {
  const clamped = Math.min(Math.max(next, 1), pages.value)
  emit('update:offset', (clamped - 1) * props.limit)
}
</script>

<template>
  <div class="sf-pagination">
    <span class="sf-pagination__info">PAGE {{ page }} / {{ pages }} · 共 {{ total }} 条</span>
    <template v-if="pages > 1">
      <span class="sf-spacer" />
      <button class="sf-btn sf-btn--ghost sf-btn--icon sf-btn--sm" type="button" aria-label="上一页" :disabled="page <= 1" @click="go(page - 1)">
        <IconChevronLeft :size="15" :stroke-width="1.8" />
      </button>
      <button class="sf-btn sf-btn--ghost sf-btn--icon sf-btn--sm" type="button" aria-label="下一页" :disabled="page >= pages" @click="go(page + 1)">
        <IconChevronRight :size="15" :stroke-width="1.8" />
      </button>
    </template>
  </div>
</template>
