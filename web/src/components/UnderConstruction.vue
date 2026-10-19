<script setup lang="ts">
const props = defineProps<{ page: string; note?: string; plan?: string }>()
</script>

<template>
  <v-card class="pa-6" data-testid="under-construction">
    <v-card-title>{{ props.page }} · 迁移中</v-card-title>
    <v-card-text>
      <p class="mb-2">{{ props.note ?? '该页面已纳入 Web UI 重写计划，正按页面规格逐页实现。' }}</p>
      <p v-if="props.plan !== undefined" class="text-caption text-medium-emphasis mb-0">
        规格来源：{{ props.plan }}
      </p>
    </v-card-text>
  </v-card>
</template>
