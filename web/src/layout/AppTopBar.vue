<script setup lang="ts">
const props = defineProps<{ title: string }>()
const emit = defineEmits<{ 'toggle-drawer': [] }>()

function toggle(): void {
  emit('toggle-drawer')
}
</script>

<template>
  <v-app-bar flat density="comfortable" color="surface" data-testid="topbar">
    <v-app-bar-nav-icon data-testid="drawer-toggle" @click="toggle" />
    <v-app-bar-title>{{ props.title }}</v-app-bar-title>
    <v-spacer />
    <v-chip size="small" variant="tonal" color="primary">SubForge</v-chip>
  </v-app-bar>
</template>
