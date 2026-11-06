<script setup lang="ts">
import { RouterLink } from 'vue-router'
</script>

<template>
  <div class="nf" data-testid="not-found">
    <div class="nf__code">404</div>
    <div class="nf__title">页面不存在</div>
    <p class="nf__desc">请从左侧导航选择页面，或返回概览。</p>
    <RouterLink to="/" class="sf-btn sf-btn--primary" style="margin-top: 18px">返回概览</RouterLink>
  </div>
</template>

<style scoped>
.nf {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 96px 24px;
  min-height: 50vh;
}

.nf__code {
  font: 500 48px/1 var(--sf-font-mono);
  letter-spacing: 0.1em;
  color: var(--sf-text-faint);
}

.nf__title {
  margin-top: 16px;
  font: 650 18px/1.4 var(--sf-font-sans);
  color: var(--sf-text);
}

.nf__desc {
  margin: 8px 0 0;
  font-size: 13.5px;
  color: var(--sf-text-muted);
}
</style>
