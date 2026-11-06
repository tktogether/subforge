<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterView, useRoute } from 'vue-router'
import AppRail from './AppRail.vue'
import AppTopBar from './AppTopBar.vue'
import TokenDialog from '../components/TokenDialog.vue'
import HelpDialog from '../components/HelpDialog.vue'
import SfToast from '../components/ui/SfToast.vue'
import SfConfirm from '../components/ui/SfConfirm.vue'
import { endpoints, type VersionInfo } from '../api/endpoints'
import { useAsync } from '../composables/useAsync'
import { applyTheme, useThemeMode, watchSystemTheme } from '../composables/useThemeMode'
import { logout, openTokenDialog, tokenDialogOpen } from '../api/token'
import { confirmAction } from '../composables/useConfirm'

const route = useRoute()
const drawerOpen = ref(false)
const helpOpen = ref(false)

// 令牌是保存后才生效的，而当前页面早已挂载并取过数据；换 key 强制重建 RouterView，
// 让页面用新令牌重新请求一遍，用户不必手动刷新浏览器。
const reloadKey = ref(0)

function reloadPages(): void {
  reloadKey.value += 1
}

// 令牌保存后除重建页面外，还要重探引擎心跳：首次探测发生在无令牌时，
// 401 后 versionInfo 停留在失败态，顶栏会一直显示 OFFLINE。
function onTokenSaved(): void {
  reloadPages()
  loadVersion()
}

const pageTitle = computed(() => {
  const title = route.meta.title
  return title === undefined ? 'SubForge' : String(title)
})

const pageGroup = computed(() => {
  const group = route.meta.group
  return group === undefined ? '' : String(group)
})

// 引擎心跳：顶栏状态灯的数据源。null = 探测中。
const {
  data: versionInfo,
  loading: versionLoading,
  load: loadVersion,
} = useAsync<VersionInfo>(() => endpoints.versionInfo())
loadVersion()

const engineOnline = computed(() => {
  if (versionLoading.value) {
    return null
  }
  return versionInfo.value !== null
})

const { mode: themeMode, isDark, cycle } = useThemeMode()
watchSystemTheme()
watch(isDark, (dark) => applyTheme(dark), { immediate: true })

async function askLogout(): Promise<void> {
  const ok = await confirmAction({
    title: '确认退出登录？',
    body: '退出后会清空本机保存的访问令牌，之后所有请求都不再携带令牌。',
    confirmText: '退出登录',
    danger: true,
  })
  if (ok) {
    logout()
    openTokenDialog()
    reloadPages()
  }
}

watch(
  () => route.path,
  () => {
    drawerOpen.value = false
  },
)
</script>

<template>
  <div class="sf-shell">
    <AppRail :drawer-open="drawerOpen" @close="drawerOpen = false" />
    <div class="sf-shell__main">
      <AppTopBar
        :title="pageTitle"
        :group="pageGroup"
        :version="versionInfo"
        :engine-online="engineOnline"
        :theme-mode="themeMode"
        @toggle-drawer="drawerOpen = !drawerOpen"
        @cycle-theme="cycle()"
        @open-token="openTokenDialog"
        @logout="askLogout"
        @open-help="helpOpen = true"
      />
      <main class="sf-container">
        <RouterView v-slot="{ Component }">
          <Transition name="sf-page" mode="out-in">
            <component :is="Component" :key="route.path + '-' + reloadKey" />
          </Transition>
        </RouterView>
      </main>
    </div>

    <TokenDialog v-model="tokenDialogOpen" @saved="onTokenSaved" />
    <HelpDialog :open="helpOpen" @close="helpOpen = false" />
    <SfConfirm />
    <SfToast />
  </div>
</template>

<style scoped>
.sf-shell {
  display: flex;
  min-height: 100vh;
}

.sf-shell__main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}
</style>
