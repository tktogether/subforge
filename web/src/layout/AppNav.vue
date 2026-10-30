<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { navGroups } from '../router'

const props = defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [boolean] }>()

const route = useRoute()
const groups = computed(() => navGroups())

function isActive(path: string): boolean {
  return route.path === path
}

function onUpdate(value: boolean): void {
  emit('update:modelValue', value)
}
</script>

<template>
  <v-navigation-drawer
    :model-value="props.modelValue"
    width="248"
    data-testid="nav"
    @update:model-value="onUpdate"
  >
    <v-list-item title="SubForge" subtitle="订阅转换与规则管理" prepend-icon="mdi-anvil" nav />
    <v-divider />
    <template v-for="group in groups" :key="group.name">
      <v-list-subheader class="nav-group-label">{{ group.name }}</v-list-subheader>
      <v-list-item
        v-for="item in group.items"
        :key="item.path"
        :to="item.path"
        :prepend-icon="item.icon"
        :title="item.title"
        :active="isActive(item.path)"
        nav
        class="nav-item"
      />
    </template>
  </v-navigation-drawer>
</template>

<style scoped>
.nav-group-label {
  font-size: 11px;
  color: var(--sf-text-faint);
  text-transform: none;
  letter-spacing: 0;
  height: 24px;
  line-height: 24px;
}
.nav-item {
  min-height: 32px;
  height: 36px;
}
.nav-item :deep(.v-list-item__overlay) {
  background: transparent;
}
.nav-item.v-list-item--active {
  background: var(--sf-surface-raised);
  color: var(--sf-text);
  border-left: 1px solid var(--sf-accent);
}
.nav-item:focus-visible {
  outline: 2px solid var(--sf-accent);
  outline-offset: -2px;
}
</style>
