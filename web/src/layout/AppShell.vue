<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterView, useRoute } from 'vue-router'
import { useTheme } from 'vuetify'
import AppNav from './AppNav.vue'
import AppTopBar from './AppTopBar.vue'
import { closeNotify, notifyState, snackbarColor } from '../composables/useNotify'
import { applyTheme, useThemeMode } from '../composables/useThemeMode'

const drawer = ref(true)
const route = useRoute()
const theme = useTheme()
const { isDark } = useThemeMode()

const pageTitle = computed(() => {
  const title = route.meta.title
  return title === undefined ? 'SubForge' : String(title)
})

const pageGroup = computed(() => {
  const group = route.meta.group
  return group === undefined ? '' : String(group)
})

const snackbarTone = computed(() => snackbarColor(notifyState.kind))

watch(isDark, (dark) => applyTheme(dark, theme), { immediate: true })
</script>

<template>
  <v-app>
    <AppNav v-model="drawer" />
    <AppTopBar :title="pageTitle" :group="pageGroup" @toggle-drawer="drawer = !drawer" />
    <v-main style="background: var(--sf-bg);">
      <div class="main-content">
        <RouterView />
      </div>
    </v-main>
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