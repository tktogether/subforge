<script setup lang="ts">
const props = defineProps<{ title: string; group?: string }>()
const emit = defineEmits<{ 'toggle-drawer': [] }>()

function toggle(): void {
  emit('toggle-drawer')
}
</script>

<template>
  <v-app-bar
    flat
    density="comfortable"
    :height="56"
    data-testid="topbar"
    class="topbar"
  >
    <v-app-bar-nav-icon data-testid="drawer-toggle" @click="toggle" />
    <v-app-bar-title class="breadcrumb">
      <template v-if="props.group">
        {{ props.group }} / {{ props.title }}
      </template>
      <template v-else>
        {{ props.title }}
      </template>
    </v-app-bar-title>
    <v-spacer />
    <v-chip size="small" variant="tonal" color="primary">SubForge</v-chip>
  </v-app-bar>
</template>

<style scoped>
.topbar {
  border-bottom: 1px solid var(--sf-border);
  background: var(--sf-surface);
}
.breadcrumb {
  font-size: 14px;
  color: var(--sf-text);
}
</style>