<script setup lang="ts">
import { ref, watch, onMounted, onBeforeUnmount, nextTick, type Component } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import {
  IconGauge,
  IconArrowsLeftRight,
  IconRoute,
  IconFileCode,
  IconAntennaBars5,
  IconServer,
  IconFilter,
  IconDatabase,
  IconFolders,
  IconStack2,
  IconKey,
  IconClock,
  IconAdjustments,
  IconTerminal2,
} from '@tabler/icons-vue'
import { navGroups } from '../router'

const icons: Record<string, Component> = {
  gauge: IconGauge,
  swap: IconArrowsLeftRight,
  route: IconRoute,
  code: IconFileCode,
  antenna: IconAntennaBars5,
  server: IconServer,
  filter: IconFilter,
  database: IconDatabase,
  folders: IconFolders,
  layers: IconStack2,
  key: IconKey,
  clock: IconClock,
  adjust: IconAdjustments,
  terminal: IconTerminal2,
}

defineProps<{ drawerOpen: boolean }>()
const emit = defineEmits<{ (e: 'close'): void }>()

const route = useRoute()
const navEl = ref<HTMLElement | null>(null)
const marker = ref({ top: '0px', height: '0px' })

// 选中指示块滑到当前项：transform 过渡，窗口变化时重新对位。
function moveMarker(): void {
  nextTick(() => {
    const active = navEl.value?.querySelector<HTMLAnchorElement>('.sf-rail__item.is-active')
    if (active !== null && active !== undefined && navEl.value !== null) {
      marker.value = { top: active.offsetTop + 'px', height: active.offsetHeight + 'px' }
    }
  })
}

watch(() => route.name, moveMarker, { flush: 'post' })
onMounted(() => {
  moveMarker()
  window.addEventListener('resize', moveMarker)
})
onBeforeUnmount(() => window.removeEventListener('resize', moveMarker))
</script>

<template>
  <!-- 遮罩不 Teleport：#app 自带 z-index:1 堆叠上下文，Teleport 到 body 的
       遮罩(z69) 会盖住整个 #app(根层面只有 z1)，手机端抽屉会被遮罩挡死。 -->
  <Transition name="sf-fade">
    <div v-if="drawerOpen" class="sf-rail__scrim" @click="emit('close')" />
  </Transition>

  <aside class="sf-rail" :class="{ 'is-open': drawerOpen }">
    <RouterLink to="/" class="sf-rail__brand" @click="emit('close')">
      <span class="sf-rail__mark" aria-hidden="true">SF</span>
      <span class="sf-rail__brand-text">
        <span class="sf-rail__name">SUB<em>FORGE</em></span>
        <span class="sf-rail__tag">DEEP-RANGE CONSOLE</span>
      </span>
    </RouterLink>

    <nav ref="navEl" class="sf-rail__nav" aria-label="主导航">
      <template v-for="group in navGroups()" :key="group.name">
        <div class="sf-rail__group-label">{{ group.name }}</div>
        <RouterLink
          v-for="item in group.items"
          :key="item.path"
          :to="item.path"
          class="sf-rail__item"
          :class="{ 'is-active': route.name === item.name }"
          @click="emit('close')"
        >
          <component :is="icons[item.icon]" :size="17" :stroke-width="1.7" class="sf-rail__ic" aria-hidden="true" />
          <span class="sf-rail__text">{{ item.title }}</span>
        </RouterLink>
      </template>
      <span class="sf-rail__marker" :style="{ top: marker.top, height: marker.height }" aria-hidden="true" />
    </nav>

    <div class="sf-rail__foot">
      <span class="sf-rail__foot-label">ORCHESTRATION ENGINE</span>
    </div>
  </aside>
</template>

<style scoped>
.sf-rail {
  width: var(--sf-rail-w);
  flex: none;
  position: sticky;
  top: 0;
  height: 100vh;
  display: flex;
  flex-direction: column;
  background: var(--sf-bg-deep);
  border-right: 1px solid var(--sf-border);
  z-index: 70;
}

.sf-rail__scrim {
  position: fixed;
  inset: 0;
  background: oklch(0.08 0.015 250 / 0.62);
  z-index: 69;
}

[data-theme='light'] .sf-rail__scrim {
  background: oklch(0.3 0.02 255 / 0.35);
}

.sf-rail__brand {
  display: flex;
  align-items: center;
  gap: 11px;
  padding: 15px 18px 13px;
  text-decoration: none;
  border-bottom: 1px solid var(--sf-border);
}

.sf-rail__mark {
  width: 30px;
  height: 30px;
  flex: none;
  display: grid;
  place-content: center;
  border: 1px solid var(--sf-accent-border);
  border-radius: var(--sf-radius-sm);
  font: 700 12px/1 var(--sf-font-display);
  color: var(--sf-accent);
  background: var(--sf-accent-dim);
  letter-spacing: 0.04em;
}

.sf-rail__brand-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.sf-rail__name {
  font: 650 14.5px/1 var(--sf-font-display);
  letter-spacing: 0.14em;
  color: var(--sf-text);
}

.sf-rail__name em {
  font-style: normal;
  color: var(--sf-accent);
}

.sf-rail__tag {
  font: 500 8.5px/1 var(--sf-font-mono);
  letter-spacing: 0.18em;
  color: var(--sf-text-faint);
  white-space: nowrap;
}

.sf-rail__nav {
  position: relative;
  flex: 1;
  overflow-y: auto;
  padding: 10px 10px 16px;
  display: flex;
  flex-direction: column;
}

.sf-rail__group-label {
  font: 500 9.5px/1 var(--sf-font-mono);
  letter-spacing: 0.22em;
  color: var(--sf-text-faint);
  padding: 16px 10px 7px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.sf-rail__group-label::after {
  content: '';
  flex: 1;
  height: 1px;
  background: var(--sf-border);
  opacity: 0.6;
}

.sf-rail__item {
  position: relative;
  display: flex;
  align-items: center;
  gap: 11px;
  height: 34px;
  padding: 0 10px;
  margin: 1px 0;
  border-radius: var(--sf-radius);
  color: var(--sf-text-muted);
  text-decoration: none;
  font: 500 13px/1 var(--sf-font-sans);
  transition: color var(--sf-t-fast) var(--sf-ease), background var(--sf-t-fast) var(--sf-ease);
  white-space: nowrap;
}

.sf-rail__item:hover {
  color: var(--sf-text);
  background: var(--sf-surface-dim);
}

.sf-rail__item.is-active {
  color: var(--sf-accent);
  background: var(--sf-accent-dim);
}

.sf-rail__ic {
  flex: none;
}

.sf-rail__marker {
  position: absolute;
  left: 0;
  width: 2px;
  border-radius: 1px;
  background: var(--sf-accent);
  box-shadow: 0 0 8px oklch(from var(--sf-accent) l c h / 0.7);
  transition: top var(--sf-t-med) var(--sf-ease), height var(--sf-t-med) var(--sf-ease);
  pointer-events: none;
}

.sf-rail__foot {
  padding: 12px 18px;
  border-top: 1px solid var(--sf-border);
}

.sf-rail__foot-label {
  font: 500 8.5px/1.5 var(--sf-font-mono);
  letter-spacing: 0.2em;
  color: var(--sf-text-faint);
}

/* 中屏：只留图标 */
@media (max-width: 1200px) {
  .sf-rail {
    width: var(--sf-rail-w-compact);
  }

  .sf-rail__text,
  .sf-rail__brand-text,
  .sf-rail__group-label,
  .sf-rail__foot-label {
    display: none;
  }

  .sf-rail__brand {
    justify-content: center;
    padding-inline: 0;
  }

  .sf-rail__item {
    justify-content: center;
    padding: 0;
  }

  .sf-rail__group-label {
    padding: 12px 0 6px;
    justify-content: center;
  }

  .sf-rail__group-label::after {
    display: none;
  }

  .sf-rail__marker {
    left: 0;
  }
}

/* 窄屏：抽屉 */
@media (max-width: 960px) {
  .sf-rail {
    position: fixed;
    left: 0;
    top: 0;
    transform: translateX(-100%);
    transition: transform var(--sf-t-med) var(--sf-ease);
    width: var(--sf-rail-w);
    box-shadow: var(--sf-shadow-pop);
  }

  .sf-rail.is-open {
    transform: translateX(0);
  }

  /* 恢复完整标签：brand-text 必须保持 flex 列，
     恢复成 inline 会让名称与副标排成一行向右溢出抽屉。 */
  .sf-rail__text,
  .sf-rail__foot-label {
    display: initial;
  }

  .sf-rail__brand-text {
    display: flex;
  }

  .sf-rail__group-label {
    display: flex;
  }

  .sf-rail__group-label::after {
    display: flex;
  }

  .sf-rail__brand {
    justify-content: flex-start;
    padding-inline: 18px;
  }

  .sf-rail__item {
    justify-content: flex-start;
    padding: 0 10px;
  }
}
</style>
