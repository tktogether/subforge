<script setup lang="ts">
import { watch, onBeforeUnmount } from 'vue'
import { IconX } from '@tabler/icons-vue'

const props = withDefaults(
  defineProps<{
    open: boolean
    title?: string
    label?: string
    wide?: boolean
  }>(),
  { title: '', label: '', wide: false },
)

const emit = defineEmits<{ (e: 'close'): void }>()

function onKeydown(ev: KeyboardEvent): void {
  if (ev.key === 'Escape' && props.open) {
    ev.stopPropagation()
    emit('close')
  }
}

watch(
  () => props.open,
  (open) => {
    if (open) {
      window.addEventListener('keydown', onKeydown)
      document.body.style.overflow = 'hidden'
    } else {
      window.removeEventListener('keydown', onKeydown)
      document.body.style.overflow = ''
    }
  },
)

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
  document.body.style.overflow = ''
})
</script>

<template>
  <Teleport to="body">
    <Transition name="sf-fade">
      <div v-if="open" class="sf-dialog-scrim" @click="emit('close')" />
    </Transition>
    <Transition name="sf-pop">
      <div
        v-if="open"
        class="sf-dialog"
        :class="{ 'sf-dialog--wide': wide }"
        role="dialog"
        aria-modal="true"
        :aria-label="title || label || '对话框'"
      >
        <div class="sf-dialog__head">
          <div style="flex: 1; min-width: 0">
            <span v-if="label" class="sf-dialog__label">{{ label }}</span>
            <div class="sf-dialog__title">{{ title }}</div>
          </div>
          <button class="sf-btn sf-btn--subtle sf-btn--icon sf-btn--sm" type="button" aria-label="关闭" @click="emit('close')">
            <IconX :size="16" :stroke-width="1.8" />
          </button>
        </div>
        <div class="sf-dialog__body">
          <slot />
        </div>
        <div v-if="$slots.foot" class="sf-dialog__foot">
          <slot name="foot" />
        </div>
      </div>
    </Transition>
  </Teleport>
</template>
