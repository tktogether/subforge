<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterView, useRoute } from 'vue-router'
import AppNav from './AppNav.vue'
import AppTopBar from './AppTopBar.vue'
import { closeNotify, notifyState, snackbarColor } from '../composables/useNotify'

const drawer = ref(true)
const route = useRoute()

const pageTitle = computed(() => {
  const title = route.meta.title
  return title === undefined ? 'SubForge' : String(title)
})

const snackbarTone = computed(() => snackbarColor(notifyState.kind))
</script>

<template>
  <v-app>
    <AppNav v-model="drawer" />
    <AppTopBar :title="pageTitle" @toggle-drawer="drawer = !drawer" />
    <v-main>
      <v-container fluid class="pa-6">
        <RouterView />
      </v-container>
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
