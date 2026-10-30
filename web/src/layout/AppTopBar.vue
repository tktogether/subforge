<script setup lang="ts">
import { computed } from 'vue'
import { tokenState } from '../api/token'
import { useThemeMode, type ThemeMode } from '../composables/useThemeMode'

const props = defineProps<{ title: string; group?: string }>()
const emit = defineEmits<{ 'toggle-drawer': []; 'open-token': []; logout: [] }>()

const theme = useThemeMode()
const themeOptions: { value: ThemeMode; label: string; icon: string }[] = [
  { value: 'light', label: '浅色', icon: 'mdi-white-balance-sunny' },
  { value: 'dark', label: '深色', icon: 'mdi-weather-night' },
  { value: 'system', label: '跟随系统', icon: 'mdi-monitor' },
]

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

// requestLogout 只往上抛意图：清令牌是不可逆操作，真正的确认由 AppShell 的对话框完成。
function requestLogout(): void {
  emit('logout')
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
    <v-menu location="bottom end">
      <template #activator="{ props: menuProps }">
        <v-btn
          v-bind="menuProps"
          icon="mdi-theme-light-dark"
          variant="text"
          size="small"
          title="页面模式"
          aria-label="页面模式"
          data-testid="theme-button"
        />
      </template>
      <v-list density="compact" data-testid="theme-menu">
        <v-list-item
          v-for="option in themeOptions"
          :key="option.value"
          :prepend-icon="option.icon"
          :title="option.label"
          :active="theme.mode.value === option.value"
          :data-testid="'theme-' + option.value"
          @click="theme.setMode(option.value)"
        />
      </v-list>
    </v-menu>
    <v-btn
      icon="mdi-key-variant"
      variant="text"
      size="small"
      :title="tokenHint"
      :aria-label="tokenHint"
      data-testid="token-button"
      @click="openToken"
    />
    <v-btn
      icon="mdi-logout"
      variant="text"
      size="small"
      title="退出登录"
      aria-label="退出登录"
      data-testid="logout"
      @click="requestLogout"
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