<script setup lang="ts">
import { watch, onBeforeUnmount } from 'vue'
import { IconCircleCheck, IconAlertTriangle, IconInfoCircle } from '@tabler/icons-vue'
import { notifyState, closeNotify } from '../../composables/useNotify'

let timer: number | undefined

watch(
  () => notifyState.seq,
  () => {
    window.clearTimeout(timer)
    timer = window.setTimeout(closeNotify, notifyState.kind === 'error' ? 6000 : 3400)
  },
)

onBeforeUnmount(() => window.clearTimeout(timer))

const icons = { ok: IconCircleCheck, error: IconAlertTriangle, info: IconInfoCircle } as const
</script>

<template>
  <Teleport to="body">
    <div class="sf-toast-region" aria-live="polite">
      <Transition name="sf-toast">
        <div v-if="notifyState.open" class="sf-toast">
          <component
            :is="icons[notifyState.kind]"
            :size="17"
            :stroke-width="1.8"
            class="sf-toast__icon"
            :style="{ color: notifyState.kind === 'error' ? 'var(--sf-danger)' : notifyState.kind === 'info' ? 'var(--sf-info)' : 'var(--sf-ok)' }"
          />
          <span style="min-width: 0">{{ notifyState.text }}</span>
        </div>
      </Transition>
    </div>
  </Teleport>
</template>
