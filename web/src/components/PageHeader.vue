<script setup lang="ts">
defineProps<{ title: string; subtitle: string }>()
</script>

<template>
  <div class="d-flex align-center mb-4 ga-4">
    <div>
      <h1 class="text-h5">{{ title }}</h1>
      <p class="text-body-2 text-medium-emphasis mb-0">{{ subtitle }}</p>
    </div>
    <v-spacer />
    <div class="d-flex ga-2">
      <slot name="actions" />
    </div>
  </div>
</template>
