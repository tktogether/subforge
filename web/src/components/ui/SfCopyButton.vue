<script setup lang="ts">
import { ref } from 'vue'
import { IconCopy, IconCheck } from '@tabler/icons-vue'
import { copyText } from '../../utils/clipboard'

const props = withDefaults(
  defineProps<{
    text: string
    label?: string
  }>(),
  { label: '' },
)

const copied = ref(false)
let timer: number | undefined

async function copy(): Promise<void> {
  const ok = await copyText(props.text)
  if (ok) {
    copied.value = true
    window.clearTimeout(timer)
    timer = window.setTimeout(() => (copied.value = false), 1400)
  }
}
</script>

<template>
  <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" @click="copy">
    <IconCheck v-if="copied" :size="14" :stroke-width="2" style="color: var(--sf-ok)" />
    <IconCopy v-else :size="14" :stroke-width="1.8" />
    {{ copied ? '已复制' : label === '' ? '复制' : label }}
  </button>
</template>
