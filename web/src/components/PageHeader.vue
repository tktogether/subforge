<script setup lang="ts">
import HelpPanel from './HelpPanel.vue'
import { pageHelp } from '../content/help'

const props = defineProps<{ title: string; subtitle: string; helpKey?: string }>()
</script>

<template>
  <div class="d-flex align-center mb-4 ga-4">
    <div>
      <h1 class="text-h5">{{ props.title }}</h1>
      <p class="text-body-2 text-medium-emphasis mb-0">{{ props.subtitle }}</p>
    </div>
    <v-spacer />
    <div class="d-flex ga-2">
      <slot name="actions" />
    </div>
  </div>
  <HelpPanel v-if="props.helpKey" :help="pageHelp(props.helpKey)" />
</template>
