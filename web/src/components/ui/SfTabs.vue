<script setup lang="ts">
import { ref, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'

export interface SfTabItem {
  value: string
  label: string
  count?: number
}

const props = defineProps<{
  tabs: SfTabItem[]
  modelValue: string
}>()

const emit = defineEmits<{ (e: 'update:modelValue', value: string): void }>()

const root = ref<HTMLElement | null>(null)
const bar = ref({ left: '0px', width: '0px' })

function move(): void {
  nextTick(() => {
    const active = root.value?.querySelector<HTMLButtonElement>('.sf-tab.is-active')
    if (active !== null && active !== undefined && root.value !== null) {
      bar.value = { left: active.offsetLeft + 'px', width: active.offsetWidth + 'px' }
    }
  })
}

watch(() => [props.modelValue, props.tabs], move, { flush: 'post', deep: true })
onMounted(() => {
  move()
  window.addEventListener('resize', move)
})
onBeforeUnmount(() => window.removeEventListener('resize', move))
</script>

<template>
  <div ref="root" class="sf-tabs" role="tablist">
    <button
      v-for="tab in tabs"
      :key="tab.value"
      type="button"
      class="sf-tab"
      :class="{ 'is-active': tab.value === modelValue }"
      role="tab"
      :aria-selected="tab.value === modelValue"
      @click="emit('update:modelValue', tab.value)"
    >
      {{ tab.label }}
      <span v-if="tab.count !== undefined" class="sf-tab__count">{{ tab.count }}</span>
    </button>
    <span class="sf-tabs__indicator" :style="{ left: bar.left, width: bar.width }" aria-hidden="true" />
  </div>
</template>
