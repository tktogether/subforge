<script setup lang="ts">
import { computed } from 'vue'
import { tokenState } from '../api/token'

const props = defineProps<{ title: string; group?: string }>()
const emit = defineEmits<{ 'toggle-drawer': []; 'open-token': [] }>()

// 顶栏是用户随时能找到令牌入口的地方；已设置时给出明确提示，避免重复填写。
const tokenHint = computed(() =>
  tokenState.value === '' ? '设置访问令牌' : '已设置访问令牌，点击修改',
)

function toggle(): void {
  emit('toggle-drawer')
}

function openToken(): void {
  emit('open-token')
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
    <v-btn
      icon="mdi-key-variant"
      variant="text"
      size="small"
      :title="tokenHint"
      :aria-label="tokenHint"
      data-testid="token-button"
      @click="openToken"
    />
    <span class="wordmark">SubForge</span>
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
.wordmark {
  font-weight: 600;
  color: var(--sf-text);
}
</style>