<script setup lang="ts">
import { computed } from 'vue'
import { IconAlertTriangle } from '@tabler/icons-vue'
import SfDialog from './SfDialog.vue'
import { confirmState, settleConfirm } from '../../composables/useConfirm'

const options = computed(() => confirmState.options)
</script>

<template>
  <SfDialog
    :open="confirmState.open && options !== null"
    :label="'CONFIRM'"
    :title="options?.title ?? ''"
    @close="settleConfirm(false)"
  >
    <p v-if="options?.body" class="sf-muted" style="margin: 0; line-height: 1.65">{{ options.body }}</p>
    <template #foot>
      <button class="sf-btn sf-btn--ghost" type="button" @click="settleConfirm(false)">
        {{ options?.cancelText ?? '取消' }}
      </button>
      <button
        class="sf-btn"
        :class="options?.danger ? 'sf-btn--danger' : 'sf-btn--primary'"
        type="button"
        @click="settleConfirm(true)"
      >
        <IconAlertTriangle v-if="options?.danger" :size="15" :stroke-width="1.8" />
        {{ options?.confirmText ?? '确认' }}
      </button>
    </template>
  </SfDialog>
</template>
