<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, RouterLink } from 'vue-router'
import SfDialog from './ui/SfDialog.vue'
import { PAGE_HELP } from '../content/help'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ (e: 'close'): void }>()

const route = useRoute()

const help = computed(() => {
  const name = String(route.name ?? '')
  return PAGE_HELP[name] ?? null
})
</script>

<template>
  <SfDialog :open="props.open && help !== null" label="FIELD MANUAL" :title="help?.title ?? '说明'" wide @close="emit('close')">
    <template v-if="help !== null">
      <p class="help-lead">{{ help.oneLiner }}</p>
      <div class="help-block">
        <span class="sf-label-mono">何时使用</span>
        <p class="help-text">{{ help.whenToUse }}</p>
      </div>
      <div v-if="help.steps !== undefined && help.steps.length > 0" class="help-block">
        <span class="sf-label-mono">操作步骤</span>
        <ol class="help-steps">
          <li v-for="step in help.steps" :key="step.title">
            <b>{{ step.title }}</b>
            <span>{{ step.detail }}</span>
          </li>
        </ol>
      </div>
      <div v-if="help.concepts !== undefined && help.concepts.length > 0" class="help-block">
        <span class="sf-label-mono">概念</span>
        <dl class="help-dl">
          <div v-for="concept in help.concepts" :key="concept.term">
            <dt>{{ concept.term }}</dt>
            <dd>{{ concept.explain }}</dd>
          </div>
        </dl>
      </div>
      <div v-if="help.faq !== undefined && help.faq.length > 0" class="help-block">
        <span class="sf-label-mono">常见问题</span>
        <dl class="help-dl">
          <div v-for="item in help.faq" :key="item.q">
            <dt>{{ item.q }}</dt>
            <dd>{{ item.a }}</dd>
          </div>
        </dl>
      </div>
      <div v-if="help.links !== undefined && help.links.length > 0" class="help-block">
        <span class="sf-label-mono">相关页面</span>
        <div class="help-links">
          <RouterLink
            v-for="link in help.links"
            :key="link.to"
            :to="link.to"
            class="sf-link"
            @click="emit('close')"
          >{{ link.label }}</RouterLink>
        </div>
      </div>
    </template>
    <template #foot>
      <button class="sf-btn sf-btn--primary" type="button" @click="emit('close')">知道了</button>
    </template>
  </SfDialog>
</template>

<style scoped>
.help-lead {
  margin: 0;
  font-size: 13.5px;
  line-height: 1.7;
  color: var(--sf-text);
}

.help-block {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.help-text {
  margin: 0;
  font-size: 13px;
  line-height: 1.7;
  color: var(--sf-text-muted);
}

.help-steps {
  margin: 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.help-steps li {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding-left: 18px;
  position: relative;
  font-size: 13px;
  line-height: 1.65;
}

.help-steps li::before {
  content: counter(list-item, decimal-leading-zero);
  position: absolute;
  left: 0;
  top: 2px;
  font: 500 10.5px/1.4 var(--sf-font-mono);
  color: var(--sf-accent);
}

.help-steps b {
  font-weight: 600;
  color: var(--sf-text);
}

.help-steps span {
  color: var(--sf-text-muted);
}

.help-dl {
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.help-dl dt {
  font-weight: 600;
  font-size: 13px;
  color: var(--sf-text);
}

.help-dl dd {
  margin: 2px 0 0;
  font-size: 13px;
  line-height: 1.65;
  color: var(--sf-text-muted);
}

.help-links {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
}
</style>
