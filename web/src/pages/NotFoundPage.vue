<script setup lang="ts">
import { RouterLink } from 'vue-router'
</script>

<template>
  <v-card class="pa-6" data-testid="not-found">
    <v-card-title>页面不存在</v-card-title>
    <v-card-text>
      <p class="mb-3">请从左侧导航选择模块。</p>
      <RouterLink to="/">返回概览</RouterLink>
    </v-card-text>
  </v-card>
</template>
