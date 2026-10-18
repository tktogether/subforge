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
    <v-list-item title="SubForge" subtitle="订阅转换与规则工厂" prepend-icon="mdi-anvil" nav />
    <v-divider />
    <template v-for="group in groups" :key="group.name">
      <v-list-subheader>{{ group.name }}</v-list-subheader>
      <v-list-item
        v-for="item in group.items"
        :key="item.path"
        :to="item.path"
        :prepend-icon="item.icon"
        :title="item.title"
        :active="isActive(item.path)"
        nav
      />
    </template>
  </v-navigation-drawer>
</template>
