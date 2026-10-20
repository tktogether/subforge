<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterView, useRoute } from 'vue-router'
import { useTheme } from 'vuetify'
import AppNav from './AppNav.vue'
import AppTopBar from './AppTopBar.vue'
import TokenDialog from '../components/TokenDialog.vue'
import { closeNotify, notifyState, snackbarColor } from '../composables/useNotify'
import { applyTheme, useThemeMode } from '../composables/useThemeMode'
import { logout, openTokenDialog, tokenDialogOpen } from '../api/token'

const drawer = ref(true)
const route = useRoute()
const theme = useTheme()
const { isDark } = useThemeMode()

// 令牌是保存后才生效的，而当前页面早已挂载并取过数据；换 key 强制重建 RouterView，
// 让页面用新令牌重新请求一遍，用户不必手动刷新浏览器。
const reloadKey = ref(0)

const pageTitle = computed(() => {
  const title = route.meta.title
  return title === undefined ? 'SubForge' : String(title)
})

const pageGroup = computed(() => {
  const group = route.meta.group
  return group === undefined ? '' : String(group)
})

const snackbarTone = computed(() => snackbarColor(notifyState.kind))

function reloadPages(): void {
  reloadKey.value += 1
}

// 退出登录是不可逆操作，先弹确认框：确认后才清令牌，然后立刻打开令牌输入框，
// 用户要么重新输入新令牌继续，要么直接关掉输入框（等于不登录）。
const logoutConfirmOpen = ref(false)

function askLogout(): void {
  logoutConfirmOpen.value = true
}

function cancelLogout(): void {
  logoutConfirmOpen.value = false
}

function doLogout(): void {
  logout()
  logoutConfirmOpen.value = false
  openTokenDialog()
  reloadPages()
}

watch(isDark, (dark) => applyTheme(dark, theme), { immediate: true })
</script>

<template>
  <v-app>
    <AppNav v-model="drawer" />
    <AppTopBar
      :title="pageTitle"
      :group="pageGroup"
      @toggle-drawer="drawer = !drawer"
      @open-token="openTokenDialog"
      @logout="askLogout"
    />
    <v-main style="background: var(--sf-bg);">
      <div class="main-content">
        <RouterView :key="reloadKey" />
      </div>
    </v-main>
    <TokenDialog v-model="tokenDialogOpen" @saved="reloadPages" />
    <v-dialog v-model="logoutConfirmOpen" max-width="480">
      <v-card>
        <v-card-title>确认退出登录</v-card-title>
        <v-card-text>
          退出后会清空本机保存的访问令牌，之后所有请求都不再带令牌。
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" data-testid="logout-cancel" @click="cancelLogout">取消</v-btn>
          <v-btn color="error" data-testid="logout-confirm" @click="doLogout">退出登录</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
    <v-snackbar
      :model-value="notifyState.open"
      :color="snackbarTone"
      data-testid="snackbar"
      @update:model-value="closeNotify"
    >
      {{ notifyState.text }}
    </v-snackbar>
  </v-app>
</template>

<style scoped>
.main-content {
  max-width: var(--sf-shell);
  margin: 0 auto;
  padding: 24px;
  background: var(--sf-bg);
}
</style>