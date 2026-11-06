<script setup lang="ts">
import { computed } from 'vue'
import {
  IconMenu2,
  IconSun,
  IconMoon,
  IconDeviceDesktop,
  IconKey,
  IconLogout,
  IconHelp,
} from '@tabler/icons-vue'
import type { VersionInfo } from '../api/endpoints'
import { tokenState } from '../api/token'
import type { ThemeMode } from '../composables/useThemeMode'

const props = defineProps<{
  title: string
  group: string
  version: VersionInfo | null
  engineOnline: boolean | null
  themeMode: ThemeMode
}>()

const emit = defineEmits<{
  (e: 'toggle-drawer'): void
  (e: 'cycle-theme'): void
  (e: 'open-token'): void
  (e: 'logout'): void
  (e: 'open-help'): void
}>()

const themeIcon = computed(() => (props.themeMode === 'dark' ? IconMoon : props.themeMode === 'light' ? IconSun : IconDeviceDesktop))
const themeTip = computed(() => '主题：' + (props.themeMode === 'dark' ? '深色' : props.themeMode === 'light' ? '浅色' : '跟随系统') + '（点击切换）')

const ledClass = computed(() => {
  if (props.engineOnline === null) {
    return 'sf-led--idle'
  }
  return props.engineOnline ? 'sf-led--ok sf-led--pulse' : 'sf-led--danger sf-led--pulse'
})

const ledText = computed(() => {
  if (props.engineOnline === null) {
    return 'ENGINE …'
  }
  return props.engineOnline ? 'ENGINE ONLINE' : 'ENGINE OFFLINE'
})

// 顶栏只显示短版本号（如 dev）；完整构建元数据在概览页呈现
const versionShort = computed(() => {
  const v = props.version?.version ?? ''
  const m = v.match(/^subforge\s+(\S+)/)
  return m !== null ? m[1] : v.split(' ')[0] || ''
})

const hasToken = computed(() => tokenState.value !== '')
</script>

<template>
  <header class="sf-topbar" data-testid="topbar">
    <button
      class="sf-btn sf-btn--subtle sf-btn--icon sf-rail-toggle"
      type="button"
      aria-label="打开导航"
      data-testid="drawer-toggle"
      @click="emit('toggle-drawer')"
    >
      <IconMenu2 :size="18" :stroke-width="1.8" />
    </button>

    <div class="sf-topbar__crumb sf-mono" aria-hidden="true">{{ title }}</div>

    <span class="sf-spacer" />

    <div class="sf-topbar__engine sf-status" aria-hidden="true">
      <span class="sf-led" :class="ledClass" />
      <span class="sf-topbar__engine-text">{{ ledText }}</span>
      <span v-if="versionShort !== ''" class="sf-topbar__version">{{ versionShort }}</span>
    </div>

    <button
      class="sf-btn sf-btn--subtle sf-btn--icon"
      type="button"
      :aria-label="themeTip"
      data-testid="theme-button"
      @click="emit('cycle-theme')"
    >
      <span class="sf-tip" :data-tip="themeTip" style="display: inline-flex">
        <component :is="themeIcon" :size="17" :stroke-width="1.7" />
      </span>
    </button>

    <button class="sf-btn sf-btn--subtle sf-btn--icon" type="button" aria-label="访问令牌" data-testid="token-button" @click="emit('open-token')">
      <span class="sf-tip" :data-tip="hasToken ? '已设置访问令牌，点击修改' : '设置访问令牌'" style="display: inline-flex">
        <IconKey :size="17" :stroke-width="1.7" :style="{ color: hasToken ? 'var(--sf-accent)' : undefined }" />
      </span>
    </button>

    <button v-if="hasToken" class="sf-btn sf-btn--subtle sf-btn--icon" type="button" aria-label="退出登录" data-testid="logout" @click="emit('logout')">
      <span class="sf-tip" data-tip="退出登录" style="display: inline-flex">
        <IconLogout :size="17" :stroke-width="1.7" />
      </span>
    </button>

    <button class="sf-btn sf-btn--subtle sf-btn--icon" type="button" aria-label="本页说明" data-testid="help-button" @click="emit('open-help')">
      <span class="sf-tip" data-tip="本页说明" style="display: inline-flex">
        <IconHelp :size="17" :stroke-width="1.7" />
      </span>
    </button>
  </header>
</template>

<style scoped>
.sf-topbar {
  position: sticky;
  top: 0;
  z-index: 50;
  height: var(--sf-topbar-h);
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 0 14px;
  background: oklch(from var(--sf-bg) l c h / 0.93);
  border-bottom: 1px solid var(--sf-border);
}

.sf-topbar__crumb {
  font-size: 11.5px;
  letter-spacing: 0.06em;
  color: var(--sf-text-faint);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  margin-left: 4px;
}

.sf-topbar__crumb-root {
  color: var(--sf-accent);
}

.sf-topbar__crumb-page {
  color: var(--sf-text-muted);
}

.sf-topbar__engine {
  margin-right: 8px;
}

.sf-topbar__engine-text {
  font: 500 9.5px/1 var(--sf-font-mono);
  letter-spacing: 0.16em;
  color: var(--sf-text-faint);
}

.sf-topbar__version {
  font: 500 10px/1 var(--sf-font-mono);
  color: var(--sf-text-faint);
  letter-spacing: 0.04em;
  padding-left: 9px;
  border-left: 1px solid var(--sf-border);
}

.sf-rail-toggle {
  display: none;
}

@media (max-width: 960px) {
  .sf-rail-toggle {
    display: inline-flex;
  }

  .sf-topbar__engine {
    display: none;
  }
}
</style>
