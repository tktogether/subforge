<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { PAGE_HELP } from '../../content/help'

const props = defineProps<{
  title?: string
  desc?: string
}>()

const route = useRoute()

const path = computed(() => 'SF://' + String(route.name ?? '').toUpperCase())
const title = computed(() => props.title ?? PAGE_HELP[String(route.name ?? '')]?.title ?? '')
const desc = computed(() => props.desc ?? PAGE_HELP[String(route.name ?? '')]?.oneLiner ?? '')
</script>

<template>
  <header class="sf-page-head sf-rise">
    <div style="min-width: 0">
      <div class="sf-page-head__path">
        <span class="sf-led sf-led--accent sf-led--pulse" aria-hidden="true" />
        {{ path }}
      </div>
      <h1 class="sf-page-head__title">{{ title }}</h1>
      <p v-if="desc !== ''" class="sf-page-head__desc">{{ desc }}</p>
    </div>
    <div class="sf-page-head__actions">
      <!-- 页面通过 <template #actions> 传入动作按钮；这里必须渲染具名插槽。 -->
      <slot name="actions" />
    </div>
  </header>
</template>
